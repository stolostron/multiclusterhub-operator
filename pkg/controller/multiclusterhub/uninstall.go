// Copyright (c) 2020 Red Hat, Inc.

package multiclusterhub

import (
	"context"
	"fmt"

	operatorsv1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operator/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	// The uninstallList is the list of all resources from previous installs to remove. Items can be removed
	// from this list in future releases if they are sure to not exist prior to the current installer version
	uninstallList = func(m *operatorsv1.MultiClusterHub) []*unstructured.Unstructured {
		return []*unstructured.Unstructured{
			// topology-sub removed in 2.3.0
			newUnstructured(
				types.NamespacedName{Name: "topology-sub", Namespace: m.Namespace},
				schema.GroupVersionKind{Group: "apps.open-cluster-management.io", Kind: "Subscription", Version: "v1"},
			),
			// searchcollectors CRD replaced in ?.?.?
			newUnstructured(
				types.NamespacedName{Name: "searchcollectors.agent.open-cluster-management.io"},
				schema.GroupVersionKind{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition", Version: "v1beta1"},
			),
		}
	}
)

func newUnstructured(nn types.NamespacedName, gvk schema.GroupVersionKind) *unstructured.Unstructured {
	u := unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetName(nn.Name)
	u.SetNamespace((nn.Namespace))
	return &u
}

// ensureRemovalsGone validates successful removal of everything in the uninstallList. Return on first error encounter.
func (r *ReconcileMultiClusterHub) ensureRemovalsGone(m *operatorsv1.MultiClusterHub) (*reconcile.Result, error) {
	removals := uninstallList(m)
	for i := range removals {
		rr, err := r.uninstall(m, removals[i])
		if rr != nil {
			return rr, err
		}
	}
	return nil, nil
}

func (r *ReconcileMultiClusterHub) uninstall(m *operatorsv1.MultiClusterHub, u *unstructured.Unstructured) (*reconcile.Result, error) {
	obLog := log.WithValues("Namespace", u.GetNamespace(), "Name", u.GetName(), "Kind", u.GetKind())

	found := u.NewEmptyInstance()
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      u.GetName(),
		Namespace: u.GetNamespace(),
	}, found)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		// Error that isn't due to the resource not existing
		obLog.Error(err, "Error getting resource")
		return &reconcile.Result{}, err
	}

	err = r.client.Delete(context.TODO(), found)
	if err != nil {
		condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionFalse, OldComponentNotRemovedReason, fmt.Sprintf("Failed to remove resource %s/%s", u.GetKind(), u.GetName()))
		SetHubCondition(&m.Status, *condition)
		obLog.Error(err, "Failed to delete resource")
		return &reconcile.Result{}, err
	}
	condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionTrue, OldComponentRemovedReason, "Removed old resource")
	SetHubCondition(&m.Status, *condition)
	obLog.Info("Deleted instance")
	return nil, nil
}
