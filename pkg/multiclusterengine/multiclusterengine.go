// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package multiclusterengine

import (
	"context"
	"errors"

	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	olmapi "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"
	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	// prod MCE variables
	channel                = "stable-2.2"
	installPlanApproval    = subv1alpha1.ApprovalAutomatic
	packageName            = "multicluster-engine"
	catalogSourceName      = "redhat-operators"
	catalogSourceNamespace = "openshift-marketplace" // https://olm.operatorframework.io/docs/tasks/troubleshooting/subscription/#a-subscription-in-namespace-x-cant-install-operators-from-a-catalogsource-in-namespace-y

	// community MCE variables
	communityChannel           = "community-2.2"
	communityPackageName       = "stolostron-engine"
	communityCatalogSourceName = "community-operators"

	// default names
	MulticlusterengineName      = "multiclusterengine"
	MulticlusterengineNamespace = "multicluster-engine"
	operatorGroupName           = "default"
)

// mocks returning a single manifest
var mockPackageManifests = func() *olmapi.PackageManifestList {
	return &olmapi.PackageManifestList{
		Items: []olmapi.PackageManifest{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "multicluster-engine",
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

func labels(m *operatorsv1.MultiClusterHub) map[string]string {
	return map[string]string{
		// "installer.name":        m.GetName(),
		// "installer.namespace":   m.GetNamespace(),
		utils.MCEManagedByLabel: "true",
	}
}

func MultiClusterEngine(m *operatorsv1.MultiClusterHub) *mcev1.MultiClusterEngine {
	mce := &mcev1.MultiClusterEngine{
		TypeMeta: metav1.TypeMeta{
			APIVersion: mcev1.GroupVersion.String(),
			Kind:       "MultiClusterEngine",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        MulticlusterengineName,
			Labels:      labels(m),
			Annotations: GetSupportedAnnotations(m),
		},
		Spec: mcev1.MultiClusterEngineSpec{
			ImagePullSecret: m.Spec.ImagePullSecret,
			Tolerations:     utils.GetTolerations(m),
			NodeSelector:    m.Spec.NodeSelector,
			Overrides: &mcev1.Overrides{
				Components: utils.GetMCEComponents(m),
			},
		},
	}
	return mce
}

// NewMultiClusterEngine returns an MCE configured from a Multiclusterhub
func NewMultiClusterEngine(m *operatorsv1.MultiClusterHub) *mcev1.MultiClusterEngine {
	labels := map[string]string{
		"installer.name":        m.GetName(),
		"installer.namespace":   m.GetNamespace(),
		utils.MCEManagedByLabel: "true",
	}
	annotations := GetSupportedAnnotations(m)
	availConfig := mcev1.HAHigh
	if m.Spec.AvailabilityConfig == operatorsv1.HABasic {
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
			TargetNamespace:    MulticlusterengineNamespace,
			Overrides: &mcev1.Overrides{
				Components: utils.GetMCEComponents(m),
			},
		},
	}
	return mce
}

// GetSupportedAnnotations copies annotations relevant to MCE from MCH. Currently this only
// applies to the imageRepository override
func GetSupportedAnnotations(m *operatorsv1.MultiClusterHub) map[string]string {
	mceAnnotations := make(map[string]string)
	if m.GetAnnotations() != nil {
		if val, ok := m.GetAnnotations()[utils.AnnotationImageRepo]; ok && val != "" {
			mceAnnotations["imageRepository"] = val
		}
	}
	return mceAnnotations
}

func Namespace() *corev1.Namespace {
	return &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: utils.MCESubscriptionNamespace,
		},
	}
}

func OperatorGroup() *olmv1.OperatorGroup {
	return &olmv1.OperatorGroup{
		TypeMeta: metav1.TypeMeta{
			APIVersion: olmv1.GroupVersion.String(),
			Kind:       olmv1.OperatorGroupKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      operatorGroupName,
			Namespace: utils.MCESubscriptionNamespace,
		},
		Spec: olmv1.OperatorGroupSpec{
			TargetNamespaces: []string{utils.MCESubscriptionNamespace},
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
	if len(pkgs) == 0 {
		return nn, errors.New("No MCE packageManifests found")
	}

	filtered := []olmapi.PackageManifest{}
	for _, p := range pkgs {
		if hasDesiredChannel(p) {
			filtered = append(filtered, p)
		}
	}

	// fail if more than one package satisfies requirements
	if len(filtered) == 1 {
		nn.Name = filtered[0].Status.CatalogSource
		nn.Namespace = filtered[0].Status.CatalogSourceNamespace
		return nn, nil
	}
	if len(filtered) > 1 {
		return nn, errors.New("Found more than one catalogSource with expected channel")
	}

	return nn, errors.New("No MCE packageManifests found with desired channel")
}

// hasDesiredChannel returns true if the packagemanifest contains the desired channel
func hasDesiredChannel(pm olmapi.PackageManifest) bool {
	for _, c := range pm.Status.Channels {
		if c.Name == desiredChannel() {
			return true
		}
	}
	return false
}

// desiredChannel is determined by whether operator is running in community mode or production mode
func desiredChannel() string {
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

// returns packagemanifests with the name multicluster-engine
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
	packageName := "multicluster-engine"
	for _, p := range packageManifests.Items {
		if p.Name == packageName {
			pkgList = append(pkgList, p)
		}
	}
	return pkgList, nil
}
