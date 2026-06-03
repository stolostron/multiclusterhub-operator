// Copyright Contributors to the Open Cluster Management project

/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"context"
	"fmt"
	"os"

	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	admissionregistration "k8s.io/api/admissionregistration/v1"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	// Current annotation keys
	annotationIgnoreOCPVersion = "installer.open-cluster-management.io/ignore-ocp-version"
	annotationImageOverridesCM = "installer.open-cluster-management.io/image-overrides-configmap"
	annotationImageRepo        = "installer.open-cluster-management.io/image-repository"
	annotationKubeconfig       = "installer.open-cluster-management.io/kubeconfig"
	annotationMCHPause         = "installer.open-cluster-management.io/pause"

	// OLM version-specific annotations
	annotationMCESubscriptionSpec     = "installer.open-cluster-management.io/mce-subscription-spec"
	annotationMCEClusterExtensionSpec = "installer.open-cluster-management.io/mce-clusterextension-spec"

	// Deprecated annotation keys
	deprecatedAnnotationIgnoreOCPVersion = "ignoreOCPVersion"
	deprecatedAnnotationImageOverridesCM = "mch-imageOverridesCM"
	deprecatedAnnotationImageRepo        = "mch-imageRepository"
	deprecatedAnnotationKubeconfig       = "mch-kubeconfig"
	deprecatedAnnotationMCHPause         = "mch-pause"
)

type BlockDeletionResource struct {
	Name            string
	GVK             schema.GroupVersionKind
	ExceptionTotal  int
	NameExceptions  []string
	LabelExceptions map[string]string
}

var (
	blockDeletionResources = []BlockDeletionResource{
		{
			Name: "MultiClusterObservability",
			GVK: schema.GroupVersionKind{
				Group:   "observability.open-cluster-management.io",
				Version: "v1beta2",
				Kind:    "MultiClusterObservabilityList",
			},
			ExceptionTotal: 0,
			NameExceptions: []string{},
		},
		{
			Name: "DiscoveryConfig",
			GVK: schema.GroupVersionKind{
				Group:   "discovery.open-cluster-management.io",
				Version: "v1",
				Kind:    "DiscoveryConfigList",
			},
			ExceptionTotal: 0,
			NameExceptions: []string{},
		},
		{
			Name: "AgentServiceConfig",
			GVK: schema.GroupVersionKind{
				Group:   "agent-install.openshift.io",
				Version: "v1beta1",
				Kind:    "AgentServiceConfigList",
			},
			ExceptionTotal: 0,
			NameExceptions: []string{},
		},
	}
)

var (
	mchlog = log.Log.WithName("multiclusterhub-resource")
	Client client.Client
)

func (r *MultiClusterHub) SetupWebhookWithManager(mgr ctrl.Manager) error {
	Client = mgr.GetClient()
	return builder.WebhookManagedBy(mgr, r).
		WithDefaulter(r).
		WithValidator(r).
		Complete()
}

var _ admission.Defaulter[*MultiClusterHub] = &MultiClusterHub{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *MultiClusterHub) Default(ctx context.Context, obj *MultiClusterHub) error {
	mchlog.Info("default", "name", obj.Name)
	return nil
}

//+kubebuilder:webhook:name=multiclusterhub-operator-validating-webhook,path=/validate-operator-open-cluster-management-io-v1-multiclusterhub,mutating=false,failurePolicy=fail,sideEffects=None,groups=operator.open-cluster-management.io,resources=multiclusterhubs,verbs=create;update;delete,versions=v1,name=multiclusterhub.validating-webhook.open-cluster-management.io,admissionReviewVersions={v1,v1beta1}

var _ admission.Validator[*MultiClusterHub] = &MultiClusterHub{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *MultiClusterHub) ValidateCreate(ctx context.Context, obj *MultiClusterHub) (admission.Warnings, error) {
	mchlog.Info("validate create", "Name", obj.Name, "Namespace", obj.Namespace)

	// Check for deprecated annotations and collect warnings
	warnings := checkDeprecatedAnnotations(obj)

	// Validate OLM version-specific annotations
	if err := validateOLMAnnotations(ctx, obj); err != nil {
		return warnings, err
	}

	// Validate that cnv-mtv-integrations is not enabled when disableHubSelfManagement is true
	if err := validateMTVAndSelfManagement(obj); err != nil {
		return warnings, err
	}

	multiClusterHubList := &MultiClusterHubList{}
	if err := Client.List(context.Background(), multiClusterHubList); err != nil {
		return warnings, fmt.Errorf("unable to list MultiClusterHubs: %s", err)
	}

	// Prevent two standalone MCH's
	if len(multiClusterHubList.Items) > 0 {
		existingMCH := multiClusterHubList.Items[0]
		return warnings, fmt.Errorf("MultiClusterHub in Standalone mode already exists: `%s`", existingMCH.GetName())
	}

	if (obj.Spec.AvailabilityConfig != HABasic) && (obj.Spec.AvailabilityConfig != HAHigh) && (obj.Spec.AvailabilityConfig != "") {
		return warnings, fmt.Errorf("invalid AvailabilityConfig given")
	}

	// Validate components
	if obj.Spec.Overrides != nil {
		for _, c := range obj.Spec.Overrides.Components {
			if !ValidComponent(c, MCHComponents) {
				return warnings, fmt.Errorf("invalid component config: %s is not a known component", c.Name)
			}
		}
	}

	// validate local-cluster name length
	if err := validateLocalClusterNameLength(obj.Spec.LocalClusterName); err != nil {
		return warnings, err
	}

	// If MCE CR exists, then spec.localClusterName must match
	mceList := &mcev1.MultiClusterEngineList{}
	// If installing ACM standalone, then MCE will fail to list. This is expected
	if err := Client.List(ctx, mceList); err != nil {
		if !apimeta.IsNoMatchError(err) && !apierrors.IsNotFound(err) {
			return warnings, fmt.Errorf("unable to list MultiClusterEngine: %w", err)
		}
		// MCE API not installed; expected in standalone mode
	}
	if len(mceList.Items) == 1 {
		mce := mceList.Items[0]
		if mce.Spec.LocalClusterName != obj.Spec.LocalClusterName {
			return warnings, fmt.Errorf("Spec.LocalClusterName does not match MCE Spec.LocalClusterName: %s", mce.Spec.LocalClusterName)
		}
	}

	return warnings, nil
}

func validateLocalClusterNameLength(name string) (err error) {
	if len(name) >= 35 {
		return fmt.Errorf("local-cluster name must be shorter than 35 characters")
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *MultiClusterHub) ValidateUpdate(ctx context.Context, oldObj, newObj *MultiClusterHub) (admission.Warnings, error) {
	mchlog.Info("validate update", "Name", newObj.Name, "Namespace", newObj.Namespace)

	// Check for deprecated annotations and collect warnings
	warnings := checkDeprecatedAnnotations(newObj)

	// Validate OLM version-specific annotations
	if err := validateOLMAnnotations(ctx, newObj); err != nil {
		return warnings, err
	}

	oldMCH := oldObj

	// Note: SeparateCertificateManagement and Hive are deprecated fields.
	// Validation blocks were removed to allow users to clear these fields during migration.
	// Deprecation warnings are still shown via checkDeprecatedAnnotations().

	if (newObj.Spec.AvailabilityConfig != HABasic) && (newObj.Spec.AvailabilityConfig != HAHigh) && (newObj.Spec.AvailabilityConfig != "") {
		return warnings, fmt.Errorf("invalid AvailabilityConfig given")
	}

	// Validate components
	if newObj.Spec.Overrides != nil {
		for _, c := range newObj.Spec.Overrides.Components {
			if !ValidComponent(c, MCHComponents) {
				return warnings, fmt.Errorf("invalid componentconfig: %s is not a known component", c.Name)
			}
		}
	}

	// Validate that cnv-mtv-integrations is not enabled when disableHubSelfManagement is true
	if err := validateMTVAndSelfManagement(newObj); err != nil {
		return warnings, err
	}

	// Block changing localClusterName if ManagdCluster with label `local-cluster = true` exists
	// if the Spec.LocalClusterName field has changed
	if oldMCH.Spec.LocalClusterName != newObj.Spec.LocalClusterName {
		if err := validateLocalClusterNameLength(newObj.Spec.LocalClusterName); err != nil {
			return warnings, err
		}
		managedClusterGVK := schema.GroupVersionKind{
			Group:   "cluster.open-cluster-management.io",
			Version: "v1",
			Kind:    "ManagedClusterList",
		}
		mcName := oldMCH.Spec.LocalClusterName

		// list ManagedClusters
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(managedClusterGVK)
		if err := Client.List(ctx, list); err != nil {
			return warnings, fmt.Errorf("unable to list ManagedCluster: %v", err)
		}

		// Error if any of the ManagedClusters is the `local-cluster`
		for _, managedCluster := range list.Items {
			if managedCluster.GetName() == mcName || managedCluster.GetLabels()["local-cluster"] == "true" {
				return warnings, fmt.Errorf("cannot update Spec.LocalClusterName while local-cluster is enabled")
			}
		}
	}

	return warnings, nil
}

var cfg *rest.Config

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *MultiClusterHub) ValidateDelete(ctx context.Context, obj *MultiClusterHub) (admission.Warnings, error) {
	mchlog.Info("validate delete", "Name", obj.Name, "Namespace", obj.Namespace)

	if val, ok := os.LookupEnv("ENV_TEST"); !ok || val == "false" {
		var err error
		cfg, err = config.GetConfig()
		if err != nil {
			return nil, err
		}
	}

	c, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, err
	}

	tmpBlockDeletionResources := append(blockDeletionResources, BlockDeletionResource{
		Name: "ManagedCluster",
		GVK: schema.GroupVersionKind{
			Group:   "cluster.open-cluster-management.io",
			Version: "v1",
			Kind:    "ManagedClusterList",
		},
		ExceptionTotal:  1,
		NameExceptions:  []string{obj.Spec.LocalClusterName},
		LabelExceptions: map[string]string{"local-cluster": "true"},
	})
	for _, resource := range tmpBlockDeletionResources {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(resource.GVK)
		err := discovery.ServerSupportsVersion(c, list.GroupVersionKind().GroupVersion())
		if err != nil {
			continue
		}
		// List all resources
		if err := Client.List(context.Background(), list); err != nil {
			return nil, fmt.Errorf("unable to list %s: %s", resource.Name, err)
		}
		// If there are any unexpected resources, deny deletion
		if len(list.Items) > resource.ExceptionTotal {
			return nil, fmt.Errorf("cannot delete MultiClusterHub resource because %s resource(s) exist", resource.Name)
		}
		// if exception resources are present, check if they are the same as the exception resources
		if resource.ExceptionTotal > 0 {
			for _, item := range list.Items {
				if !contains(resource.NameExceptions, item.GetName()) {
					return nil, fmt.Errorf("cannot delete MultiClusterHub resource because %s resource(s) exist", resource.Name)
				}
				if !hasIntersection(resource.LabelExceptions, item.GetLabels()) {
					return nil, fmt.Errorf("cannot delete MultiClusterHub resource because %s resource(s) are missing %v labels", resource.Name, resource.LabelExceptions)
				}
			}
		}
	}
	return nil, nil
}

func hasIntersection(smallerMap map[string]string, largerMap map[string]string) bool {
	// iterate through the keys of the smaller map to save time
	for k, sVal := range smallerMap {
		if lVal := largerMap[k]; lVal == sVal {
			return true // return true if A and B share any complete key-value pair
		}
	}
	return false
}

// ValidatingWebhook returns the ValidatingWebhookConfiguration used for the multiclusterhub
// linked to a service in the provided namespace
func ValidatingWebhook(namespace string) *admissionregistration.ValidatingWebhookConfiguration {
	fail := admissionregistration.Fail
	none := admissionregistration.SideEffectClassNone
	path := "/validate-operator-open-cluster-management-io-v1-multiclusterhub"
	return &admissionregistration.ValidatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admissionregistration.k8s.io/v1",
			Kind:       "ValidatingWebhookConfiguration",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "multiclusterhub-operator-validating-webhook",
			Annotations: map[string]string{"service.beta.openshift.io/inject-cabundle": "true"},
		},
		Webhooks: []admissionregistration.ValidatingWebhook{
			{
				AdmissionReviewVersions: []string{
					"v1",
					"v1beta1",
				},
				Name: "multiclusterhub.validating-webhook.open-cluster-management.io",
				ClientConfig: admissionregistration.WebhookClientConfig{
					Service: &admissionregistration.ServiceReference{
						Name:      "multiclusterhub-operator-webhook",
						Namespace: namespace,
						Path:      &path,
					},
				},
				FailurePolicy: &fail,
				Rules: []admissionregistration.RuleWithOperations{
					{
						Rule: admissionregistration.Rule{
							APIGroups:   []string{GroupVersion.Group},
							APIVersions: []string{GroupVersion.Version},
							Resources:   []string{"multiclusterhubs"},
						},
						Operations: []admissionregistration.OperationType{
							admissionregistration.Create,
							admissionregistration.Update,
							admissionregistration.Delete,
						},
					},
				},
				SideEffects: &none,
			},
		},
	}
}

// validateMTVAndSelfManagement returns an error if cnv-mtv-integrations is enabled
// while disableHubSelfManagement is true, as this combination is unsupported
// and will cause the MCH to be stuck in Pending.
func validateMTVAndSelfManagement(r *MultiClusterHub) error {
	if r.Spec.DisableHubSelfManagement && r.Enabled(MTVIntegrations) {
		return fmt.Errorf(
			"cannot enable %s while disableHubSelfManagement is true; "+
				"the local-cluster must be enabled for MTV integrations to function",
			MTVIntegrations,
		)
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// validateOLMAnnotations validates that MCE annotations match the detected OLM version
func validateOLMAnnotations(ctx context.Context, mch *MultiClusterHub) error {
	annotations := mch.GetAnnotations()
	if annotations == nil {
		return nil
	}

	// Detect OLM version using same logic as main.go
	olmVersion, err := detectOLMVersion(ctx)
	if err != nil {
		mchlog.Error(err, "Failed to detect OLM version for annotation validation")
		// Don't block on detection failure - let operator handle it
		return nil
	}

	hasV0Annotation := annotations[annotationMCESubscriptionSpec] != ""
	hasV1Annotation := annotations[annotationMCEClusterExtensionSpec] != ""

	// No annotations set - valid
	if !hasV0Annotation && !hasV1Annotation {
		return nil
	}

	// Reject v0 annotation on v1 cluster
	if olmVersion == "v1" && hasV0Annotation {
		return fmt.Errorf("annotation %q is only valid for OLM v0 clusters. This cluster uses OLM v1. Use %q instead",
			annotationMCESubscriptionSpec, annotationMCEClusterExtensionSpec)
	}

	// Reject v1 annotation on v0 cluster
	if olmVersion == "v0" && hasV1Annotation {
		return fmt.Errorf("annotation %q is only valid for OLM v1 clusters. This cluster uses OLM v0. Use %q instead",
			annotationMCEClusterExtensionSpec, annotationMCESubscriptionSpec)
	}

	// Reject v1 annotation when no OLM detected
	if olmVersion == "" && hasV1Annotation {
		return fmt.Errorf("annotation %q requires OLM v1, but no OLM detected on this cluster",
			annotationMCEClusterExtensionSpec)
	}

	return nil
}

// detectOLMVersion detects which OLM version is present on the cluster
// Returns "v0", "v1", or "" (no OLM)
func detectOLMVersion(ctx context.Context) (string, error) {
	// Check for OLM v0 via environment variable
	if os.Getenv("OPERATOR_CONDITION_NAME") != "" {
		return "v0", nil
	}

	// Check for OLM v1 by looking for ClusterExtension CRD
	crd := &apixv1.CustomResourceDefinition{}
	err := Client.Get(ctx, types.NamespacedName{
		Name: "clusterextensions.olm.operatorframework.io",
	}, crd)

	if err == nil {
		return "v1", nil
	} else if apierrors.IsNotFound(err) {
		return "", nil
	}

	return "", fmt.Errorf("failed to check for OLM v1 CRD: %w", err)
}

// checkDeprecatedAnnotations examines the MultiClusterHub resource for deprecated annotations
// and spec fields, returning warnings for any that are found.
func checkDeprecatedAnnotations(r *MultiClusterHub) admission.Warnings {
	var warnings admission.Warnings

	// Check for deprecated annotations
	annotations := r.GetAnnotations()
	if annotations != nil {
		// Map of deprecated annotation keys to their current replacements
		deprecatedAnnotations := map[string]string{
			deprecatedAnnotationIgnoreOCPVersion: annotationIgnoreOCPVersion,
			deprecatedAnnotationImageOverridesCM: annotationImageOverridesCM,
			deprecatedAnnotationImageRepo:        annotationImageRepo,
			deprecatedAnnotationKubeconfig:       annotationKubeconfig,
			deprecatedAnnotationMCHPause:         annotationMCHPause,
		}

		for deprecatedKey, currentKey := range deprecatedAnnotations {
			if _, exists := annotations[deprecatedKey]; exists {
				warning := fmt.Sprintf("annotation '%s' is deprecated and will be removed in a future release. Please use '%s' instead",
					deprecatedKey, currentKey)
				warnings = append(warnings, warning)
			}
		}
	}

	return warnings
}
