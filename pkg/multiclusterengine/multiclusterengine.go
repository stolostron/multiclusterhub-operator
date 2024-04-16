// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package multiclusterengine

import (
	"context"
	"fmt"
	"math"

	"github.com/Masterminds/semver/v3"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	olmapi "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"
	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	// prod MCE variables
	channel                = "stable-2.6"
	installPlanApproval    = subv1alpha1.ApprovalAutomatic
	packageName            = "multicluster-engine"
	catalogSourceName      = "redhat-operators"
	catalogSourceNamespace = "openshift-marketplace" // https://olm.operatorframework.io/docs/tasks/troubleshooting/subscription/#a-subscription-in-namespace-x-cant-install-operators-from-a-catalogsource-in-namespace-y
	operandNamespace       = "multicluster-engine"

	// community MCE variables
	communityChannel           = "community-0.5"
	communityPackageName       = "stolostron-engine"
	communityCatalogSourceName = "community-operators"
	communityOperandNamepace   = "stolostron-engine"

	// default names
	MulticlusterengineName = "multiclusterengine"
	operatorGroupName      = "default"
)

// mocks returning a single manifest
var mockPackageManifests = func() *olmapi.PackageManifestList {
	return &olmapi.PackageManifestList{
		Items: []olmapi.PackageManifest{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: DesiredPackage(),
				},
				Status: olmapi.PackageManifestStatus{
					CatalogSource:          "multiclusterengine-catalog",
					CatalogSourceNamespace: "openshift-marketplace",
					Channels: []olmapi.PackageChannel{
						{
							Name: DesiredChannel(),
						},
					},
				},
			},
		},
	}
}

// NewMultiClusterEngine returns an MCE configured from a Multiclusterhub
func NewMultiClusterEngine(m *operatorv1.MultiClusterHub) *mcev1.MultiClusterEngine {
	labels := map[string]string{
		"installer.name":        m.GetName(),
		"installer.namespace":   m.GetNamespace(),
		utils.MCEManagedByLabel: "true",
	}
	annotations := GetSupportedAnnotations(m)
	availConfig := mcev1.HAHigh
	if m.Spec.AvailabilityConfig == operatorv1.HABasic {
		availConfig = mcev1.HABasic
	}

	mce := &mcev1.MultiClusterEngine{
		ObjectMeta: metav1.ObjectMeta{
			Name:        MulticlusterengineName,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: mcev1.MultiClusterEngineSpec{
			ImagePullSecret:    m.Spec.ImagePullSecret,
			Tolerations:        utils.GetTolerations(m),
			NodeSelector:       m.Spec.NodeSelector,
			AvailabilityConfig: availConfig,
			TargetNamespace:    OperandNamespace(),
			HubSize:            mcev1.HubSize(m.Spec.HubSize),
			Overrides: &mcev1.Overrides{
				Components: utils.GetMCEComponents(m),
			},
		},
	}

	if m.Spec.Overrides != nil && m.Spec.Overrides.ImagePullPolicy != "" {
		mce.Spec.Overrides.ImagePullPolicy = m.Spec.Overrides.ImagePullPolicy
	}

	return mce
}

func RenderMultiClusterEngine(existingMCE *mcev1.MultiClusterEngine, m *operatorv1.MultiClusterHub) *mcev1.MultiClusterEngine {
	copy := existingMCE.DeepCopy()

	// add annotations
	annotations := GetSupportedAnnotations(m)
	if len(annotations) > 0 {
		newAnnotations := copy.GetAnnotations()
		if newAnnotations == nil {
			newAnnotations = make(map[string]string)
		}
		for key, val := range annotations {
			newAnnotations[key] = val
		}
		copy.SetAnnotations(newAnnotations)
	} else {
		RemoveSupportedAnnotations(copy)
	}

	if m.Spec.AvailabilityConfig == operatorv1.HABasic {
		copy.Spec.AvailabilityConfig = mcev1.HABasic
	} else {
		copy.Spec.AvailabilityConfig = mcev1.HAHigh
	}

	copy.Spec.ImagePullSecret = m.Spec.ImagePullSecret
	copy.Spec.Tolerations = utils.GetTolerations(m)
	copy.Spec.NodeSelector = m.Spec.NodeSelector

	for _, component := range utils.GetMCEComponents(m) {
		if component.Enabled {
			copy.Enable(component.Name)
		} else {
			copy.Disable(component.Name)
		}
	}

	if m.Spec.Overrides != nil && m.Spec.Overrides.ImagePullPolicy != "" {
		copy.Spec.Overrides.ImagePullPolicy = m.Spec.Overrides.ImagePullPolicy
	}

	return copy
}

// GetSupportedAnnotations copies annotations relevant to MCE from MCH. Currently this only
// applies to the imageRepository override
func GetSupportedAnnotations(m *operatorv1.MultiClusterHub) map[string]string {
	mceAnnotations := make(map[string]string)
	if m.GetAnnotations() != nil {
		if val, ok := m.GetAnnotations()[utils.AnnotationImageRepo]; ok && val != "" {
			mceAnnotations["imageRepository"] = val
		}
	}
	return mceAnnotations
}

// RemoveSupportedAnnotations removes annotations relevant to MCE from MCE. If the annotation is
// already present then sets value to empty rather than removing the key
func RemoveSupportedAnnotations(mce *mcev1.MultiClusterEngine) map[string]string {
	mceAnnotations := mce.GetAnnotations()
	if mceAnnotations != nil {
		if _, ok := mceAnnotations["imageRepository"]; ok {
			mceAnnotations["imageRepository"] = ""
		}
	}
	return mceAnnotations
}

func Namespace() *corev1.Namespace {
	namespace := OperandNamespace()
	return &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Labels: map[string]string{
				utils.OpenShiftClusterMonitoringLabel: "true",
			},
		},
	}
}

func OperatorGroup() *olmv1.OperatorGroup {
	namespace := OperandNamespace()
	return &olmv1.OperatorGroup{
		TypeMeta: metav1.TypeMeta{
			APIVersion: olmv1.GroupVersion.String(),
			Kind:       olmv1.OperatorGroupKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      operatorGroupName,
			Namespace: namespace,
		},
		Spec: olmv1.OperatorGroupSpec{
			TargetNamespaces: []string{namespace},
		},
	}
}

// GetCatalogSource returns the name and namespace of an MCE catalogSource with the required channel.
// Returns error if two or more catalogsources satsify criteria.
func GetCatalogSource(k8sClient client.Client) (types.NamespacedName, error) {
	nn := types.NamespacedName{}

	pkgs, err := GetMCEPackageManifests(k8sClient)
	if err != nil {
		return nn, err
	}

	// Return an error if there are no package manifests found with the desired MCE package name.
	if len(pkgs) == 0 {
		return nn, fmt.Errorf("no %s packageManifests found", DesiredPackage())
	}

	filtered := filterPackageManifests(pkgs, DesiredChannel())
	// Return an error if there are no package manifests found with the desired MCE channel name.
	if len(filtered) == 0 {
		return nn, fmt.Errorf("no %s packageManifests found with desired channel %s", DesiredPackage(), DesiredChannel())
	}

	/*
		Catalog sources managed by the Operator Lifecycle Manager (OLM) include a 'priority' option in their
		specifications. By default, this value is set to 0 for most catalog sources. However, for the 'redhat-operator'
		catalog source, it is assigned a default priority of -100. Leveraging the priority value, we can determine the
		preferred catalog source to utilize when assembling the Multicluster Engine operator.
	*/
	catalogSource, err := findHighestPriorityCatalogSource(k8sClient, filtered)
	if err != nil {
		return nn, err
	}

	nn.Name = catalogSource.Name
	nn.Namespace = catalogSource.Namespace
	return nn, nil
}

// extractCatalogSource extracts namespaced name from the given PackageManifest.
func extractCatalogSource(pm olmapi.PackageManifest) types.NamespacedName {
	return types.NamespacedName{
		Name:      pm.Status.CatalogSource,
		Namespace: pm.Status.CatalogSourceNamespace,
	}
}

// findHighestPriorityCatalogSource finds the catalog source with the highest priority among the given list.
func findHighestPriorityCatalogSource(k8sClient client.Client, pkgs []olmapi.PackageManifest) (
	*subv1alpha1.CatalogSource, error) {
	var (
		highestPriorityCatalogSources []*subv1alpha1.CatalogSource
		maxPriority                   = math.MinInt64
		log                           = log.Log.WithName("reconcile")
	)

	for _, pm := range pkgs {
		cs := &subv1alpha1.CatalogSource{}
		nn := extractCatalogSource(pm)

		if err := k8sClient.Get(context.TODO(), nn, cs); err != nil {
			// Log the error and continue to the next iteration
			log.Error(err, fmt.Sprintf("failed to retrieve catalog source %s/%s", nn.Namespace, nn.Name))
			continue
		}

		if cs.Spec.Priority > maxPriority {
			// Found a new highest priority, reset the slice and update the maxPriority
			maxPriority = cs.Spec.Priority
			highestPriorityCatalogSources = []*subv1alpha1.CatalogSource{cs}

		} else if cs.Spec.Priority == maxPriority {
			// Found another catalog source with the same highest priority, append it to the slice
			highestPriorityCatalogSources = append(highestPriorityCatalogSources, cs)
		}
	}

	switch len(highestPriorityCatalogSources) {
	case 0:
		return nil, fmt.Errorf("no catalog sources found with a positive priority")

	case 1:
		catalogSource := highestPriorityCatalogSources[0]
		log.V(2).Info(fmt.Sprintf("Using catalog source %v/%v with the highest priority: %v",
			catalogSource.Namespace, catalogSource.Name, catalogSource.Spec.Priority))
		return catalogSource, nil

	default:
		// Multiple catalog sources found with the same highest priority
		var catalogNames []string
		for _, cs := range highestPriorityCatalogSources {
			catalogNames = append(catalogNames, fmt.Sprintf("%s/%s", cs.Namespace, cs.Name))
		}

		return nil, fmt.Errorf(
			"found more than one %s catalogSource with expected channel %s with the highest priority:%v",
			DesiredPackage(), DesiredChannel(), catalogNames)
	}
}

// filterPackageManifests returns a list of packagemanifests containing the desired channel
// at the latest available version. Returns an empty list if no packagemanifests include the
// channel. If more than one packagemanifest have the same latest version available it will
// return them all.
func filterPackageManifests(pkgManifests []olmapi.PackageManifest, desiredChannel string) []olmapi.PackageManifest {
	filtered := []olmapi.PackageManifest{}
	latestVersion := &semver.Version{}
	for _, p := range pkgManifests {
		for _, c := range p.Status.Channels {
			if c.Name == desiredChannel {
				versionString := c.CurrentCSVDesc.Version.String()
				v, err := semver.NewVersion(versionString)
				if err != nil {
					log.Log.WithName("reconcile").Info("failed to parse version from packagemanifest", "catalogsource", p.Status.CatalogSource)
					continue
				}
				if len(filtered) == 0 {
					filtered = append(filtered, p)
					latestVersion = v
					continue
				}
				if v.Equal(latestVersion) {
					filtered = append(filtered, p)
				} else if v.GreaterThan(latestVersion) {
					filtered = []olmapi.PackageManifest{p}
					latestVersion = v
				}
			}
		}
	}
	return filtered
}

// DesiredChannel is determined by whether operator is running in community mode or production mode
func DesiredChannel() string {
	if utils.IsCommunityMode() {
		return communityChannel
	} else {
		return channel
	}
}

// DesiredPackage is determined by whether operator is running in community mode or production mode
func DesiredPackage() string {
	if utils.IsCommunityMode() {
		return communityPackageName
	} else {
		return packageName
	}
}

// OperandNamespace is determined by whether operator is running in community mode or production mode
func OperandNamespace() string {
	if utils.IsCommunityMode() {
		return communityOperandNamepace
	} else {
		return operandNamespace
	}
}

// returns packagemanifests with the name multicluster-engine
func GetMCEPackageManifests(k8sClient client.Client) ([]olmapi.PackageManifest, error) {
	ctx := context.Background()
	log := log.Log.WithName("reconcile")
	packageManifests := &olmapi.PackageManifestList{}
	var err error
	if utils.IsUnitTest() {
		packageManifests = mockPackageManifests()
	} else {
		err = k8sClient.List(ctx, packageManifests)
	}
	if err != nil {
		log.Error(err, "failed to list package manifests")
		return nil, err
	}

	pkgList := []olmapi.PackageManifest{}
	packageName := DesiredPackage()
	for _, p := range packageManifests.Items {
		if p.Name == packageName {
			pkgList = append(pkgList, p)
		}
	}
	return pkgList, nil
}

// Finds MCE by managed label. Returns nil if none found.
func GetManagedMCE(ctx context.Context, k8sClient client.Client) (*mcev1.MultiClusterEngine, error) {
	mceList := &mcev1.MultiClusterEngineList{}
	err := k8sClient.List(ctx, mceList, &client.MatchingLabels{
		utils.MCEManagedByLabel: "true",
	})
	if err != nil {
		return nil, err
	}
	// filter out hosted MCEs
	filteredMCEs := []mcev1.MultiClusterEngine{}
	for _, mce := range mceList.Items {
		if mce.Annotations == nil || mce.Annotations["deploymentmode"] != "Hosted" {
			filteredMCEs = append(filteredMCEs, mce)
		}
	}

	if err == nil && len(filteredMCEs) == 1 {
		return &filteredMCEs[0], nil
	} else if len(filteredMCEs) > 1 {
		// will require manual resolution
		return nil, fmt.Errorf("multiple MCEs found managed by MCH. Only one MCE is supported")
	}

	return nil, nil
}

// find MCE. label it for future. return nil if no mce found.
func FindAndManageMCE(ctx context.Context, k8sClient client.Client) (*mcev1.MultiClusterEngine, error) {
	// first find subscription via managed-by label
	mce, err := GetManagedMCE(ctx, k8sClient)
	if err != nil {
		return nil, err
	}
	if mce != nil {
		return mce, nil
	}

	// if label doesn't work find it via list
	log.Log.WithName("reconcile").Info("Failed to find subscription via label")
	wholeList := &mcev1.MultiClusterEngineList{}
	err = k8sClient.List(ctx, wholeList)
	if err != nil {
		return nil, err
	}
	if len(wholeList.Items) == 0 {
		return nil, nil
	}

	// filter hosted MCEs
	filteredMCEs := []mcev1.MultiClusterEngine{}
	for _, mce := range wholeList.Items {
		if mce.Annotations == nil || mce.Annotations["deploymentmode"] != "Hosted" {
			filteredMCEs = append(filteredMCEs, mce)
		}
	}

	if len(filteredMCEs) > 1 {
		return nil, fmt.Errorf("multiple MCEs found managed by MCH. Only one MCE is supported")
	}
	labels := filteredMCEs[0].GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[utils.MCEManagedByLabel] = "true"
	filteredMCEs[0].SetLabels(labels)
	log.Log.WithName("reconcile").Info("Adding label to MCE")
	if err := k8sClient.Update(ctx, &filteredMCEs[0]); err != nil {
		log.Log.WithName("reconcile").Error(err, "Failed to add managedBy label to preexisting MCE")
		return &filteredMCEs[0], err
	}
	return &filteredMCEs[0], nil
}

// MCECreatedByMCH returns true if the provided MCE was created by the multiclusterhub-operator (as indicated by installer labels).
// A nil MCE will always return false
func MCECreatedByMCH(mce *mcev1.MultiClusterEngine, m *operatorv1.MultiClusterHub) bool {
	l := mce.GetLabels()
	if l == nil {
		return false
	}
	return l["installer.name"] == m.GetName() && l["installer.namespace"] == m.GetNamespace()
}
