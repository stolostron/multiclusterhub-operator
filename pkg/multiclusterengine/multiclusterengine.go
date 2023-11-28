// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package multiclusterengine

import (
	"context"
	"fmt"

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
	/*
		channel represents the channel used for the MultiClusterEngine (MCE) subscription.
		In the production environment, it is set to "stable-2.4" by default.
		The channel specifies the update channel from which updates and new versions of the MCE are obtained.
	*/
	channel = "stable-2.4"

	/*
		installPlanApproval sets the approval strategy for the installation plan of the MCE subscription.
		In the production environment, it is set to `subv1alpha1.ApprovalAutomatic`, indicating that
		installation approvals are automatic. InstallPlanApproval defines how installation approval is handled
		within Operator Lifecycle Manager (OLM).
	*/
	installPlanApproval = subv1alpha1.ApprovalAutomatic

	/*
		packageName specifies the name of the package corresponding to the MultiClusterEngine.
		In the production environment, it is set to "multicluster-engine." The package name is used to identify
		the MCE package within the OLM.
	*/
	packageName = "multicluster-engine"

	/*
		catalogSourceName stores the name of the catalog source for the MCE. In the production environment,
		it is set to "redhat-operators," which is the source where the MCE operator is cataloged.
		Catalog sources provide a repository of operators for OLM.
	*/
	catalogSourceName = "redhat-operators"

	/*
		catalogSourceNamespace the namespace in which the catalog source is located. In the production environment,
		it is set to "openshift-marketplace." This namespace contains the catalog source that provides
		access to operators, including the MultiClusterEngine.

		https://olm.operatorframework.io/docs/troubleshooting/subscription
	*/
	catalogSourceNamespace = "openshift-marketplace"

	/*
		operandNameSpace the namespace where the MultiClusterEngine (MCE) operates. In the production environment,
		it is set to "multicluster-engine." The operand namespace is where the MCE resources and components are
		deployed.
	*/
	operandNameSpace = "multicluster-engine"

	/*
		communityChannel represents the channel used for the Community version of the MultiClusterEngine (MCE).
		In the community environment, it is set to "community-0.1." The community channel is used for the community
		edition of the MCE.
	*/
	communityChannel = "community-0.1"

	/*
		communityPackageName specifies the name of the package for the Community MultiClusterEngine.
		In the community environment, it is set to "stolostron-engine." This package name is used to identify
		the Community MCE package within the OLM.
	*/
	communityPackageName = "stolostron-engine"

	/*
		communityCatalogSourceName stores the name of the catalog source for the Community MultiClusterEngine.
		In the community environment, it is set to "community-operators." The catalog source provides access to
		operators in the community edition.
	*/
	communityCatalogSourceName = "community-operators"

	/*
		communityOperandNamepace specifies the namespace in which the Community MultiClusterEngine operates.
		In the community environment, it is set to "stolostron-engine." The community operand namespace is
		where the resources for the Community MCE are deployed.
	*/
	communityOperandNamepace = "stolostron-engine"

	/*
		MulticlusterengineName defines the default name for the MultiClusterEngine instance. In this case, it is
		set to "multiclusterengine," which is the default name for an instance of the MultiClusterEngine.
		This name is used to identify and manage the MCE instance.
	*/
	MulticlusterengineName = "multiclusterengine"

	/*
		operatorGroupName specifies the name of the operator group associated with the MultiClusterEngine.
		In the default configuration, it is set to "default." Operator groups are used to control the deployment
		and management of operators in a cluster, including the MultiClusterEngine operator.
	*/
	operatorGroupName = "default"
)

/*
mockPackageManifests returns a mock PackageManifestList, which is used for testing or simulating the presence of package
manifests. The mock PackageManifestList contains a single PackageManifest item with specific metadata and status,
including catalog source information and channels.
*/
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
							Name: desiredChannel(),
						},
					},
				},
			},
		},
	}
}

/*
NewMultiClusterEngine creates and configures a MultiClusterEngine (MCE) based on the provided MultiClusterHub (MCH)
and an infrastructure custom namespace. It sets various properties and attributes for the MCE, such as labels,
annotations, image pull secrets, tolerations, node selectors, availability configuration, target namespace,
and component overrides.
*/
func NewMultiClusterEngine(m *operatorv1.MultiClusterHub, infrastructureCustomNamespace string,
) *mcev1.MultiClusterEngine {
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
			TargetNamespace:    OperandNameSpace(),
			Overrides: &mcev1.Overrides{
				Components: utils.GetMCEComponents(m),
			},
		},
	}

	if m.Spec.Overrides != nil && m.Spec.Overrides.ImagePullPolicy != "" {
		mce.Spec.Overrides.ImagePullPolicy = m.Spec.Overrides.ImagePullPolicy
	}

	if infrastructureCustomNamespace != "" {
		mce.Spec.Overrides.InfrastructureCustomNamespace = infrastructureCustomNamespace
	}

	return mce
}

/*
RenderMultiClusterEngine takes an existing MultiClusterEngine (MCE) and a MultiClusterHub (MCH) as input and produces a
modified MCE by applying changes from the MCH. The modifications include updating annotations, image pull secret,
tolerations, node selector, availability configuration, and component states.
*/
func RenderMultiClusterEngine(existingMCE *mcev1.MultiClusterEngine, m *operatorv1.MultiClusterHub,
) *mcev1.MultiClusterEngine {
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

/*
GetSupportedAnnotations retrieves annotations from the provided MultiClusterHub (MCH) that are relevant to the
MultiClusterEngine (MCE). It specifically focuses on the "imageRepository" annotation.
*/
func GetSupportedAnnotations(m *operatorv1.MultiClusterHub) map[string]string {
	mceAnnotations := make(map[string]string)
	if m.GetAnnotations() != nil {
		if val, ok := m.GetAnnotations()[utils.AnnotationImageRepo]; ok && val != "" {
			mceAnnotations["imageRepository"] = val
		}
	}
	return mceAnnotations
}

/*
RemoveSupportedAnnotations removes or empties annotations from a MultiClusterEngine (MCE) object that are relevant
to the MCE. If the annotation is already present, it sets its value to an empty string, effectively
removing its significance.
*/
func RemoveSupportedAnnotations(mce *mcev1.MultiClusterEngine) map[string]string {
	mceAnnotations := mce.GetAnnotations()
	if mceAnnotations != nil {
		if _, ok := mceAnnotations["imageRepository"]; ok {
			mceAnnotations["imageRepository"] = ""
		}
	}
	return mceAnnotations
}

/*
Namespace generates and returns a Kubernetes namespace object with specific metadata and labels.
The namespace's name is determined based on the OperandNameSpace function.
*/
func Namespace() *corev1.Namespace {
	namespace := OperandNameSpace()
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

/*
OperatorGroup generates and returns an OperatorGroup object for use in the Kubernetes cluster.
The OperatorGroup specifies the target namespaces where operators should be deployed.
*/
func OperatorGroup() *olmv1.OperatorGroup {
	namespace := OperandNameSpace()
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

/*
GetCatalogSource retrieves the name and namespace of an MCE catalogSource with a specific required channel.
It returns an error if multiple catalog sources meet the criteria.
*/
func GetCatalogSource(k8sClient client.Client) (types.NamespacedName, error) {
	nn := types.NamespacedName{}

	pkgs, err := GetMCEPackageManifests(k8sClient)
	if err != nil {
		return nn, err
	}
	if len(pkgs) == 0 {
		return nn, fmt.Errorf("no %s packageManifests found", DesiredPackage())
	}

	filtered := filterPackageManifests(pkgs, desiredChannel())

	// fail if more than one package satisfies requirements
	if len(filtered) == 1 {
		nn.Name = filtered[0].Status.CatalogSource
		nn.Namespace = filtered[0].Status.CatalogSourceNamespace
		return nn, nil
	}
	if len(filtered) > 1 {
		return nn, fmt.Errorf("found more than one %s catalogSource with expected channel %s", DesiredPackage(),
			desiredChannel())
	}

	return nn, fmt.Errorf("no %s packageManifests found with desired channel %s", DesiredPackage(), desiredChannel())
}

/*
filterPackageManifests filters a list of PackageManifests, returning those that include the desired channel at the
latest available version. It may return multiple package manifests if they share the same latest version.
*/
func filterPackageManifests(pkgManifests []olmapi.PackageManifest, desiredChannel string) []olmapi.PackageManifest {
	filtered := []olmapi.PackageManifest{}
	latestVersion := &semver.Version{}
	for _, p := range pkgManifests {
		for _, c := range p.Status.Channels {
			if c.Name == desiredChannel {
				versionString := c.CurrentCSVDesc.Version.String()
				v, err := semver.NewVersion(versionString)
				if err != nil {
					log.FromContext(context.Background()).Info("failed to parse version from packagemanifest",
						"catalogsource", p.Status.CatalogSource)
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
				}
			}
		}
	}
	return filtered
}

/*
desiredChannel determines the desired channel based on whether the operator is running in community mode or
production mode. It returns the appropriate channel name.
*/
func desiredChannel() string {
	if utils.IsCommunityMode() {
		return communityChannel
	} else {
		return channel
	}
}

/*
DesiredPackage determines the desired package name based on whether the operator is running in community mode or
production mode. It returns the appropriate package name.
*/
func DesiredPackage() string {
	if utils.IsCommunityMode() {
		return communityPackageName
	} else {
		return packageName
	}
}

/*
OperandNameSpace determines the operand namespace based on whether the operator is running in community mode or
production mode. It returns the appropriate namespace name.
*/
func OperandNameSpace() string {
	if utils.IsCommunityMode() {
		return communityOperandNamepace
	} else {
		return operandNameSpace
	}
}

/*
GetMCEPackageManifests retrieves PackageManifests with the name "multicluster-engine" from the Kubernetes cluster.
It may return an error if the retrieval process fails.
*/
func GetMCEPackageManifests(k8sClient client.Client) ([]olmapi.PackageManifest, error) {
	ctx := context.Background()
	log := log.FromContext(ctx)
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

/*
GetManagedMCE finds and returns the MultiClusterEngine (MCE) managed by the MultiClusterHub (MCH). It queries the
Kubernetes cluster to identify MCE instances with specific labels and returns the appropriate MCE.
*/
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

/*
FindAndManageMCE responsible for finding and managing the MultiClusterEngine (MCE). It first attempts to locate the MCE
through labels and, if unsuccessful, uses a list-based approach. If found, it adds a label to the MCE indicating
management by the MultiClusterHub.
*/
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
	log.FromContext(ctx).Info("Failed to find subscription via label")
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
	log.FromContext(ctx).Info("Adding label to MCE")
	if err := k8sClient.Update(ctx, &filteredMCEs[0]); err != nil {
		log.FromContext(ctx).Error(err, "Failed to add managedBy label to preexisting MCE")
		return &filteredMCEs[0], err
	}
	return &filteredMCEs[0], nil
}

/*
MCECreatedByMCH determines whether the provided MultiClusterEngine (MCE) was created by the multiclusterhub-operator
based on the presence of installer-related labels. It returns a boolean indicating whether the MCE was created
by the MultiClusterHub.
*/
func MCECreatedByMCH(mce *mcev1.MultiClusterEngine, m *operatorv1.MultiClusterHub) bool {
	l := mce.GetLabels()
	if l == nil {
		return false
	}
	return l["installer.name"] == m.GetName() && l["installer.namespace"] == m.GetNamespace()
}
