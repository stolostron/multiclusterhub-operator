// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"fmt"

	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func newUnstructured(nn types.NamespacedName, gvk schema.GroupVersionKind) *unstructured.Unstructured {
	u := unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetName(nn.Name)
	u.SetNamespace((nn.Namespace))
	return &u
}

// uninstall return true if resource does not exist and returns an error if a GET or DELETE errors unexpectedly. A false response without error
// means the resource is in the process of deleting.
func (r *MultiClusterHubReconciler) uninstall(m *operatorsv1.MultiClusterHub, u *unstructured.Unstructured) (bool, error) {
	obLog := r.Log.WithValues("Namespace", u.GetNamespace(), "Kind", u.GetKind(), "Name", u.GetName())

	err := r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      u.GetName(),
		Namespace: u.GetNamespace(),
	}, u)

	if errors.IsNotFound(err) {
		return true, nil
	}

	// Get resource. Successful if it doesn't exist.
	if err != nil {
		// Error that isn't due to the resource not existing
		obLog.Error(err, "Error getting resource")
		return false, err
	}

	// If resource has deletionTimestamp then re-reconcile and don't try deleting
	if u.GetDeletionTimestamp() != nil {
		condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionFalse, OldComponentNotRemovedReason, fmt.Sprintf("Resource %s/%s finalizing", u.GetKind(), u.GetName()))
		SetHubCondition(&m.Status, *condition)
		obLog.Info("Waiting for resource to finalize")
		return false, nil
	}

	// Attempt deleting resource. No error does not necessarily mean the resource is gone.
	err = r.Client.Delete(context.TODO(), u)
	if err != nil {
		condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionFalse, OldComponentNotRemovedReason, fmt.Sprintf("Failed to remove resource %s/%s", u.GetKind(), u.GetName()))
		SetHubCondition(&m.Status, *condition)
		obLog.Error(err, "Failed to delete resource")
		return false, err
	}

	condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionTrue, OldComponentRemovedReason, "Removed old resource")
	SetHubCondition(&m.Status, *condition)
	obLog.Info("Deleted instance")
	return false, nil
}
