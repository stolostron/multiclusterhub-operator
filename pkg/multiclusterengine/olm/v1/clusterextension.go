// Copyright Contributors to the Open Cluster Management project

package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	ocv1 "github.com/operator-framework/operator-controller/api/v1"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengine"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengineutils"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// ServiceAccount for MCE ClusterExtension controller operations (installing bundle)
	MCEInstallerServiceAccountName = "mce-installer"
	// ClusterRoleBinding for installer ServiceAccount
	MCEInstallerClusterRoleBindingName = "mce-installer-admin"
)

// NewClusterExtension returns an MCE ClusterExtension configured from a MultiClusterHub
func NewClusterExtension(m *operatorv1.MultiClusterHub) *ocv1.ClusterExtension {
	labels := map[string]string{
		"installer.name":                          m.GetName(),
		"installer.namespace":                     m.GetNamespace(),
		multiclusterengineutils.MCEManagedByLabel: "true",
	}

	channels := []string{multiclusterengine.DesiredChannel()}
	packageName := multiclusterengine.DesiredPackage()
	namespace := multiclusterengine.OperandNamespace()

	return &ocv1.ClusterExtension{
		ObjectMeta: metav1.ObjectMeta{
			Name:   multiclusterengine.MCEDefaultName,
			Labels: labels,
		},
		Spec: ocv1.ClusterExtensionSpec{
			Namespace: namespace,
			ServiceAccount: ocv1.ServiceAccountReference{
				Name: MCEInstallerServiceAccountName,
			},
			Source: ocv1.SourceConfig{
				SourceType: "Catalog",
				Catalog: &ocv1.CatalogFilter{
					PackageName: packageName,
					Channels:    channels,
				},
			},
			Config: &ocv1.ClusterExtensionConfig{
				ConfigType: "Inline",
				Inline: &apiextensionsv1.JSON{
					Raw: []byte(fmt.Sprintf(`{"watchNamespace": "%s"}`, namespace)),
				},
			},
		},
	}
}

// RenderClusterExtension updates an existing ClusterExtension based on MCH spec
func RenderClusterExtension(existing *ocv1.ClusterExtension, m *operatorv1.MultiClusterHub) *ocv1.ClusterExtension {
	copy := existing.DeepCopy()

	// Update channels based on current desired channel
	newChannels := []string{multiclusterengine.DesiredChannel()}
	if copy.Spec.Source.Catalog != nil {
		oldChannels := copy.Spec.Source.Catalog.Channels

		// Update channels
		copy.Spec.Source.Catalog.Channels = newChannels

		// Clear version constraint when channel changes to allow bundle resolution
		// to select the latest version in the new channel
		if !channelsEqual(oldChannels, newChannels) {
			copy.Spec.Source.Catalog.Version = ""
		}
	}

	// Namespace and ServiceAccount are immutable, so we don't update them

	return copy
}

// channelsEqual compares two channel lists for equality
func channelsEqual(a, b []string) bool {
	// Treat nil and empty slices as different to ensure version clearing
	// when transitioning from uninitialized to initialized state
	if (a == nil) != (b == nil) {
		return false
	}
	return slices.Equal(a, b)
}

// GetManagedMCEClusterExtension finds MCE ClusterExtension by managed label. Returns nil if none found.
func GetManagedMCEClusterExtension(ctx context.Context, k8sClient client.Client) (*ocv1.ClusterExtension, error) {
	ceList := &ocv1.ClusterExtensionList{}
	opts := []client.ListOption{
		client.MatchingLabels{multiclusterengineutils.MCEManagedByLabel: "true"},
	}

	if err := k8sClient.List(ctx, ceList, opts...); err != nil {
		return nil, err
	}

	if len(ceList.Items) == 0 {
		return nil, nil
	}

	if len(ceList.Items) > 1 {
		return nil, fmt.Errorf("multiple MCE ClusterExtensions found with managed-by label. Only one is supported")
	}

	return &ceList.Items[0], nil
}

// FindAndManageMCEClusterExtension finds MCE ClusterExtension, labels it for future. Returns nil if not found.
func FindAndManageMCEClusterExtension(ctx context.Context, k8sClient client.Client,
	desiredPackage string) (*ocv1.ClusterExtension, error) {
	log := log.Log.WithName("reconcile")

	// First try via managed label
	ce, err := GetManagedMCEClusterExtension(ctx, k8sClient)
	if err != nil {
		return nil, err
	}
	if ce != nil {
		return ce, nil
	}

	// If label doesn't work, find by package name
	log.Info("Failed to find ClusterExtension via label, searching by package name")
	ceList := &ocv1.ClusterExtensionList{}
	if err := k8sClient.List(ctx, ceList); err != nil {
		return nil, err
	}

	if len(ceList.Items) == 0 {
		return nil, nil
	}

	// Find ClusterExtension with matching package name
	var matchingCE *ocv1.ClusterExtension
	for i := range ceList.Items {
		if ceList.Items[i].Spec.Source.Catalog != nil &&
			ceList.Items[i].Spec.Source.Catalog.PackageName == desiredPackage {
			if matchingCE != nil {
				return nil, fmt.Errorf("multiple MCE ClusterExtensions found with package %s. Only one is supported",
					desiredPackage)
			}
			matchingCE = &ceList.Items[i]
		}
	}

	if matchingCE == nil {
		return nil, nil
	}

	// Add managed-by label
	labels := matchingCE.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[multiclusterengineutils.MCEManagedByLabel] = "true"
	matchingCE.SetLabels(labels)
	log.Info("Adding managed-by label to ClusterExtension")

	if err := k8sClient.Update(ctx, matchingCE); err != nil {
		log.Error(err, "Failed to add managedBy label to preexisting ClusterExtension")
		return matchingCE, err
	}

	return matchingCE, nil
}

// CreatedByMCH returns true if the provided ClusterExtension was created by multiclusterhub-operator
func CreatedByMCH(ce *ocv1.ClusterExtension, m *operatorv1.MultiClusterHub) bool {
	if ce == nil {
		return false
	}
	l := ce.GetLabels()
	if l == nil {
		return false
	}
	return l["installer.name"] == m.GetName() && l["installer.namespace"] == m.GetNamespace()
}

// ServiceAccount returns a ServiceAccount for ClusterExtension operations
func ServiceAccount(namespace string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      MCEInstallerServiceAccountName,
			Namespace: namespace,
		},
	}
}

// ClusterExtensionOverrides contains mutable fields that can be overridden via annotation.
// Note: Namespace, ServiceAccount, and PackageName are immutable and cannot be overridden.
type ClusterExtensionOverrides struct {
	// Channels is an optional list of channels to constrain upgrades
	Channels []string `json:"channels,omitempty"`
	// Version is an optional semver constraint for version selection
	Version string `json:"version,omitempty"`
	// CRDUpgradeSafetyEnforcement controls CRD upgrade safety checks
	// Allowed values: "None" or "Strict"
	// Similar to v0 InstallPlanApproval
	CRDUpgradeSafetyEnforcement string `json:"crdUpgradeSafetyEnforcement,omitempty"`
	// Config contains arbitrary configuration (JSON/YAML)
	// Similar to v0 SubscriptionConfig
	Config *ClusterExtensionConfigOverride `json:"config,omitempty"`
}

// ClusterExtensionConfigOverride allows specifying inline config
type ClusterExtensionConfigOverride struct {
	// Inline contains JSON or YAML configuration values
	Inline *apiextensionsv1.JSON `json:"inline,omitempty"`
}

// GetAnnotationOverrides returns ClusterExtension overrides based on annotation in MultiClusterHub
func GetAnnotationOverrides(m *operatorv1.MultiClusterHub) (*ClusterExtensionOverrides, error) {
	mceAnnotationOverrides := utils.GetMCEClusterExtensionAnnotationOverrides(m)
	if mceAnnotationOverrides == "" {
		return nil, nil
	}
	overrides := &ClusterExtensionOverrides{}
	err := json.Unmarshal([]byte(mceAnnotationOverrides), overrides)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal annotation %q: %w", utils.AnnotationMCEClusterExtensionSpec, err)
	}
	return overrides, nil
}

// ApplyAnnotationOverrides updates a ClusterExtension with override values
func ApplyAnnotationOverrides(ce *ocv1.ClusterExtension, overrides *ClusterExtensionOverrides) {
	if overrides == nil {
		return
	}
	if ce.Spec.Source.Catalog == nil {
		return
	}

	// Apply catalog overrides
	if len(overrides.Channels) > 0 {
		ce.Spec.Source.Catalog.Channels = overrides.Channels
	}
	if overrides.Version != "" {
		ce.Spec.Source.Catalog.Version = overrides.Version
	}

	// Apply CRD upgrade safety enforcement
	if overrides.CRDUpgradeSafetyEnforcement != "" {
		if ce.Spec.Install == nil {
			ce.Spec.Install = &ocv1.ClusterExtensionInstallConfig{}
		}
		if ce.Spec.Install.Preflight == nil {
			ce.Spec.Install.Preflight = &ocv1.PreflightConfig{}
		}
		if ce.Spec.Install.Preflight.CRDUpgradeSafety == nil {
			ce.Spec.Install.Preflight.CRDUpgradeSafety = &ocv1.CRDUpgradeSafetyPreflightConfig{}
		}
		ce.Spec.Install.Preflight.CRDUpgradeSafety.Enforcement = ocv1.CRDUpgradeSafetyEnforcement(overrides.CRDUpgradeSafetyEnforcement)
	}

	// Apply config overrides
	if overrides.Config != nil && overrides.Config.Inline != nil {
		if ce.Spec.Config == nil {
			ce.Spec.Config = &ocv1.ClusterExtensionConfig{}
		}
		ce.Spec.Config.ConfigType = "Inline"
		// Copy the JSON value
		ce.Spec.Config.Inline = &apiextensionsv1.JSON{
			Raw: overrides.Config.Inline.Raw,
		}
	}
}

// ClusterRoleBinding returns ClusterRoleBinding for MCE installer ServiceAccount
func ClusterRoleBinding(namespace string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: MCEInstallerClusterRoleBindingName,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      MCEInstallerServiceAccountName,
				Namespace: namespace,
			},
		},
	}
}
