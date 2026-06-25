// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package multiclusterengine

import (
	"context"
	"encoding/json"
	"fmt"

	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// NewSubscription returns an MCE subscription with desired default values
func NewSubscription(m *operatorv1.MultiClusterHub, c *subv1alpha1.SubscriptionConfig, subOverrides *subv1alpha1.SubscriptionSpec, community bool) *subv1alpha1.Subscription {
	chName, pkgName, catSourceName := channel, packageName, catalogSourceName
	if community {
		chName = communityChannel
		pkgName = communityPackageName
		catSourceName = communityCatalogSourceName
	}
	labels := map[string]string{
		"installer.name":        m.GetName(),
		"installer.namespace":   m.GetNamespace(),
		utils.MCEManagedByLabel: "true",
	}
	namespace := OperandNamespace()
	sub := &subv1alpha1.Subscription{
		TypeMeta: metav1.TypeMeta{
			APIVersion: subv1alpha1.SubscriptionCRDAPIVersion,
			Kind:       subv1alpha1.SubscriptionKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.MCESubscriptionName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: &subv1alpha1.SubscriptionSpec{
			Channel:                chName,
			InstallPlanApproval:    installPlanApproval,
			Package:                pkgName,
			CatalogSource:          catSourceName,
			CatalogSourceNamespace: catalogSourceNamespace,
			Config:                 c,
		},
	}

	// Apply annotations last because they always take priority
	ApplyAnnotationOverrides(sub, subOverrides)
	return sub
}

// RenderSubscription returns a subscription by modifying the spec of an existing subscription based on overrides
func RenderSubscription(existingSubscription *subv1alpha1.Subscription, config *subv1alpha1.SubscriptionConfig, overrides *subv1alpha1.SubscriptionSpec, ctlSrc types.NamespacedName, community bool) *subv1alpha1.Subscription {
	copy := existingSubscription.DeepCopy()
	copy.ManagedFields = nil
	copy.TypeMeta = metav1.TypeMeta{
		APIVersion: subv1alpha1.SubscriptionCRDAPIVersion,
		Kind:       subv1alpha1.SubscriptionKind,
	}

	chName, pkgName, catSourceName := channel, packageName, catalogSourceName
	if community {
		chName = communityChannel
		pkgName = communityPackageName
		catSourceName = communityCatalogSourceName
	}

	copy.Spec = &subv1alpha1.SubscriptionSpec{
		Channel:                chName,
		InstallPlanApproval:    installPlanApproval,
		Package:                pkgName,
		CatalogSource:          catSourceName,
		CatalogSourceNamespace: catalogSourceNamespace,
		Config:                 config,
	}

	// if updating channel must remove startingCSV
	if copy.Spec.Channel != existingSubscription.Spec.Channel {
		copy.Spec.StartingCSV = ""
	}

	if ctlSrc.Name != "" {
		copy.Spec.CatalogSource = ctlSrc.Name
	}
	if ctlSrc.Namespace != "" {
		copy.Spec.CatalogSourceNamespace = ctlSrc.Namespace
	}

	// Apply annotations last because they always take priority
	ApplyAnnotationOverrides(copy, overrides)
	return copy
}

// GetAnnotationOverrides returns an OLM SubscriptionSpec based on an annotation set in the Multiclusterhub
func GetAnnotationOverrides(m *operatorv1.MultiClusterHub) (*subv1alpha1.SubscriptionSpec, error) {
	mceAnnotationOverrides := utils.GetMCEAnnotationOverrides(m)
	if mceAnnotationOverrides == "" {
		return nil, nil
	}
	mceSub := &subv1alpha1.SubscriptionSpec{}
	err := json.Unmarshal([]byte(mceAnnotationOverrides), mceSub)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal MultiClusterEngine annotation '%s': %w", mceAnnotationOverrides, err)
	}
	return mceSub, nil
}

// ApplyAnnotationOverrides updates an OLM subscription with override values
func ApplyAnnotationOverrides(sub *subv1alpha1.Subscription, subspec *subv1alpha1.SubscriptionSpec) {
	if subspec == nil {
		return
	}
	if subspec.Channel != "" {
		sub.Spec.Channel = subspec.Channel
	}
	if subspec.Package != "" {
		sub.Spec.Package = subspec.Package
	}
	if subspec.CatalogSource != "" {
		sub.Spec.CatalogSource = subspec.CatalogSource
	}
	if subspec.CatalogSourceNamespace != "" {
		sub.Spec.CatalogSourceNamespace = subspec.CatalogSourceNamespace
	}
	if subspec.StartingCSV != "" {
		sub.Spec.StartingCSV = subspec.StartingCSV
	}
	if subspec.InstallPlanApproval != "" {
		sub.Spec.InstallPlanApproval = subspec.InstallPlanApproval
	}
}

// Finds MCE subscription by managed label. Returns nil if none found.
func GetManagedMCESubscription(ctx context.Context, k8sClient client.Client) (*subv1alpha1.Subscription, error) {
	subList := &subv1alpha1.SubscriptionList{}
	if err := k8sClient.List(ctx, subList, &client.MatchingLabels{utils.MCEManagedByLabel: "true"}); err != nil {
		return nil, err

	} else if len(subList.Items) == 1 {
		return &subList.Items[0], nil

	} else if len(subList.Items) > 1 {
		// will require manual resolution
		return nil, fmt.Errorf("multiple engine subscriptions found managed by MCH. Only one MCE subscription is supported")
	}

	return nil, nil
}

// find MCE subscription. label it for future. return nil if no sub found.
func FindAndManageMCESubscription(ctx context.Context, k8sClient client.Client) (*subv1alpha1.Subscription, error) {
	// first find subscription via managed-by label
	sub, err := GetManagedMCESubscription(ctx, k8sClient)
	if err != nil {
		return nil, err
	}
	if sub != nil {
		return sub, nil
	}

	// if label doesn't work find it via .spec.name (it's package)
	// we can't assume it's name or namespace
	log.Log.WithName("reconcile").Info("Failed to find subscription via label")
	wholeList := &subv1alpha1.SubscriptionList{}
	err = k8sClient.List(ctx, wholeList)
	if err != nil {
		return nil, err
	}
	for i := range wholeList.Items {
		if wholeList.Items[i].Spec == nil {
			continue
		}
		if wholeList.Items[i].Spec.Package == DesiredPackage() {
			// adding label so it can be found in the future
			labels := wholeList.Items[i].GetLabels()
			if labels == nil {
				labels = map[string]string{}
			}
			labels[utils.MCEManagedByLabel] = "true"
			wholeList.Items[i].SetLabels(labels)
			log.Log.WithName("reconcile").Info("Adding label to subscription")
			if err := k8sClient.Update(ctx, &wholeList.Items[i]); err != nil {
				log.Log.WithName("reconcile").Error(err, "Failed to add managedBy label to preexisting MCE with MCH spec")
				return &wholeList.Items[i], err
			}
			return &wholeList.Items[i], nil
		}
	}
	return nil, nil

}

// CreatedByMCH returns true if the provided sub was created by the multiclusterhub-operator (as indicated by installer labels)
func CreatedByMCH(sub *subv1alpha1.Subscription, m *operatorv1.MultiClusterHub) bool {
	l := sub.GetLabels()
	if l == nil {
		return false
	}
	return l["installer.name"] == m.GetName() && l["installer.namespace"] == m.GetNamespace()
}
