// Copyright (c) 2026 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

// Package multiclusterengine manages MCE CR lifecycle.
//
// This package handles MultiClusterEngine resource creation, updates,
// and integration with the ACM operator.

import (
	"context"
	"fmt"

	olmapi "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"
	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengineutils"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// Production MCE variables
	MCEProdChannel          = "stable-5.0"
	MCEProdPackageName      = "multicluster-engine"
	MCEProdOperandNamespace = "multicluster-engine"

	// Community MCE variables
	MCECommunityChannel          = "community-0.10"
	MCECommunityPackageName      = "stolostron-engine"
	MCECommunityOperandNamespace = "stolostron-engine"

	// Default names
	MCEDefaultName = "multiclusterengine"
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
func NewMultiClusterEngine(m *operatorv1.MultiClusterHub, targetNamespace string) *mcev1.MultiClusterEngine {
	labels := map[string]string{
		"installer.name":                          m.GetName(),
		"installer.namespace":                     m.GetNamespace(),
		multiclusterengineutils.MCEManagedByLabel: "true",
	}
	annotations := GetSupportedAnnotations(m)
	availConfig := mcev1.HAHigh
	if m.Spec.AvailabilityConfig == operatorv1.HABasic {
		availConfig = mcev1.HABasic
	}

	mce := &mcev1.MultiClusterEngine{
		ObjectMeta: metav1.ObjectMeta{
			Name:        MCEDefaultName,
			Labels:      labels,
			Annotations: annotations,
		},

		Spec: mcev1.MultiClusterEngineSpec{
			LocalClusterName:   m.Spec.LocalClusterName,
			ImagePullSecret:    m.Spec.ImagePullSecret,
			Tolerations:        utils.GetTolerations(m),
			NodeSelector:       m.Spec.NodeSelector,
			AvailabilityConfig: availConfig,
			TargetNamespace:    targetNamespace,
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
	copy.Spec.LocalClusterName = m.Spec.LocalClusterName

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

		} else if val, ok := m.GetAnnotations()[utils.DeprecatedAnnotationImageRepo]; ok && val != "" {
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

// DesiredChannel is determined by whether operator is running in community mode or production mode
func DesiredChannel() string {
	if utils.IsCommunityMode() {
		return MCECommunityChannel
	}
	return MCEProdChannel
}

// DesiredPackage is determined by whether operator is running in community mode or production mode
func DesiredPackage() string {
	if utils.IsCommunityMode() {
		return MCECommunityPackageName
	}
	return MCEProdPackageName
}

// OperandNamespace is determined by whether operator is running in community mode or production mode
func OperandNamespace() string {
	if utils.IsCommunityMode() {
		return MCECommunityOperandNamespace
	}
	return MCEProdOperandNamespace
}

// find MCE. label it for future. return nil if no mce found.
func FindAndManageMCE(ctx context.Context, k8sClient client.Client) (*mcev1.MultiClusterEngine, error) {
	// first find subscription via managed-by label
	mce, err := multiclusterengineutils.GetManagedMCE(ctx, k8sClient)
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

	if len(wholeList.Items) > 1 {
		return nil, fmt.Errorf("multiple MCEs found managed by MCH. Only one MCE is supported")
	}
	labels := wholeList.Items[0].GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[multiclusterengineutils.MCEManagedByLabel] = "true"
	wholeList.Items[0].SetLabels(labels)
	log.Log.WithName("reconcile").Info("Adding label to MCE")

	if err := k8sClient.Update(ctx, &wholeList.Items[0]); err != nil {
		log.Log.WithName("reconcile").Error(err, "Failed to add managedBy label to preexisting MCE")
		return &wholeList.Items[0], err
	}
	return &wholeList.Items[0], nil
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
