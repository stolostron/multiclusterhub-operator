// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package multiclusterengine

import (
	"context"
	"encoding/json"
	"fmt"

	mcev1alpha1 "github.com/open-cluster-management/backplane-operator/api/v1alpha1"
	operatorsv1 "github.com/open-cluster-management/multiclusterhub-operator/api/v1"
	"github.com/open-cluster-management/multiclusterhub-operator/pkg/utils"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	channel                = "stable-2.0"
	installPlanApproval    = subv1alpha1.ApprovalAutomatic
	packageName            = "multicluster-engine"
	catalogSourceName      = "redhat-operators"
	catalogSourceNamespace = "openshift-marketplace" // https://olm.operatorframework.io/docs/tasks/troubleshooting/subscription/#a-subscription-in-namespace-x-cant-install-operators-from-a-catalogsource-in-namespace-y

	MulticlusterengineName = "multiclusterengine"

	operatorGroupName = "default"
)

func labels(m *operatorsv1.MultiClusterHub) map[string]string {
	return map[string]string{
		"installer.name":      m.GetName(),
		"installer.namespace": m.GetNamespace(),
	}
}

func MultiClusterEngine(m *operatorsv1.MultiClusterHub) *mcev1alpha1.MultiClusterEngine {
	mce := &mcev1alpha1.MultiClusterEngine{
		TypeMeta: metav1.TypeMeta{
			APIVersion: mcev1alpha1.GroupVersion.String(),
			Kind:       "MultiClusterEngine",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: MulticlusterengineName,
		},
		Spec: mcev1alpha1.MultiClusterEngineSpec{},
	}
	return mce
}

// Subscription for the helm repo serving charts
func Subscription(m *operatorsv1.MultiClusterHub) *subv1alpha1.Subscription {
	sub := &subv1alpha1.Subscription{
		TypeMeta: metav1.TypeMeta{
			APIVersion: subv1alpha1.SubscriptionCRDAPIVersion,
			Kind:       subv1alpha1.SubscriptionKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.MCESubscriptionName,
			Namespace: utils.MCESubscriptionNamespace,
			Labels:    labels(m),
		},
		Spec: &subv1alpha1.SubscriptionSpec{
			Channel:                channel,
			InstallPlanApproval:    installPlanApproval,
			Package:                packageName,
			CatalogSource:          catalogSourceName,
			CatalogSourceNamespace: catalogSourceNamespace,
		},
	}

	if mceAnnotationOverrides := utils.GetMCEAnnotationOverrides(m); mceAnnotationOverrides != "" {
		sub = overrideSub(sub, mceAnnotationOverrides)
	}

	return sub
}

func overrideSub(sub *subv1alpha1.Subscription, mceAnnotationOverrides string) *subv1alpha1.Subscription {
	log := log.FromContext(context.Background())
	log.Info(fmt.Sprintf("Overridding MultiClusterEngine Subscription: %s", mceAnnotationOverrides))
	mceSub := &subv1alpha1.SubscriptionSpec{}
	err := json.Unmarshal([]byte(mceAnnotationOverrides), mceSub)
	if err != nil {
		log.Info(fmt.Sprintf("Failed to unmarshal MultiClusterEngine annotation: %s.", mceAnnotationOverrides))
		return sub
	}

	if mceSub.Channel != "" {
		sub.Spec.Channel = mceSub.Channel
	}
	if mceSub.Package != "" {
		sub.Spec.Package = mceSub.Package
	}
	if mceSub.CatalogSource != "" {
		sub.Spec.CatalogSource = mceSub.CatalogSource
	}
	if mceSub.CatalogSourceNamespace != "" {
		sub.Spec.CatalogSourceNamespace = mceSub.CatalogSourceNamespace
	}
	if mceSub.StartingCSV != "" {
		sub.Spec.StartingCSV = mceSub.StartingCSV
	}
	if mceSub.InstallPlanApproval != "" {
		sub.Spec.InstallPlanApproval = mceSub.InstallPlanApproval
	}
	return sub
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
