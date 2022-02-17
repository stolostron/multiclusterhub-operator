// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package multiclusterengine

import (
	"context"
	"encoding/json"
	"fmt"

	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
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

func MultiClusterEngine(m *operatorsv1.MultiClusterHub) *mcev1.MultiClusterEngine {
	mce := &mcev1.MultiClusterEngine{
		TypeMeta: metav1.TypeMeta{
			APIVersion: mcev1.GroupVersion.String(),
			Kind:       "MultiClusterEngine",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   MulticlusterengineName,
			Labels: labels(m),
		},
		Spec: mcev1.MultiClusterEngineSpec{
			ImagePullSecret: m.Spec.ImagePullSecret,
		},
	}
	if (m.ComponentEnabled(operatorsv1.ManagedServiceAccount)){
		componentConfig := &mcev1.ComponentConfig{
			ManagedServiceAccount: &mcev1.ManagedServiceAccountConfig{
				Enable: m.ComponentEnabled(operatorsv1.ManagedServiceAccount),
			},
		}
		mce.Spec.ComponentConfig = componentConfig
	} 
	return mce
}

// Subscription for the helm repo serving charts
func Subscription(m *operatorsv1.MultiClusterHub, c *subv1alpha1.SubscriptionConfig) *subv1alpha1.Subscription {
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
			Config:                 c,
		},
	}

	if mceAnnotationOverrides := utils.GetMCEAnnotationOverrides(m); mceAnnotationOverrides != "" {
		sub = overrideSub(sub, mceAnnotationOverrides, c)
	}

	return sub
}

func overrideSub(sub *subv1alpha1.Subscription, mceAnnotationOverrides string, c *subv1alpha1.SubscriptionConfig) *subv1alpha1.Subscription {
	log := log.FromContext(context.Background())
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
	sub.Spec.Config = c
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
