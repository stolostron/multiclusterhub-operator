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

package controllers

import (
	"context"
	"fmt"

	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *MultiClusterHubReconciler) ensureInternalHubComponent(ctx context.Context, m *operatorv1.MultiClusterHub,
	component string) (ctrl.Result, error) {

	ihc := &operatorv1.InternalHubComponent{
		TypeMeta: metav1.TypeMeta{
			APIVersion: operatorv1.GroupVersion.String(),
			Kind:       "InternalHubComponent",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      component,
			Namespace: m.GetNamespace(),
		},
	}

	if err := r.Client.Get(
		ctx, types.NamespacedName{Name: ihc.GetName(), Namespace: ihc.GetNamespace()}, ihc); err != nil {

		if errors.IsNotFound(err) {
			if err := r.Client.Create(ctx, ihc); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to create InternalHubComponent CR: %s/%s: %v",
					ihc.GetNamespace(), ihc.GetName(), err)
			}
		} else {
			return ctrl.Result{}, fmt.Errorf("failed to get InternalHubComponent CR: %s/%s: %v",
				ihc.GetNamespace(), ihc.GetName(), err)
		}
	}

	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) ensureNoInternalHubComponent(ctx context.Context, m *operatorv1.MultiClusterHub,
	component string) (ctrl.Result, error) {

	ihc := &operatorv1.InternalHubComponent{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: component, Namespace: m.GetNamespace()}, ihc); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get InternalHubComponent: %s/%s: %v",
			m.GetNamespace(), component, err)
	}

	// Check if it has a deletion timestamp (indicating it's in the process of being deleted)
	if ihc.GetDeletionTimestamp() != nil {
		log.Info("InternalHubComponent deletion in progress", "Name", ihc.GetName(), "Namespace", ihc.GetNamespace(),
			"DeletionTimestamp", ihc.GetDeletionTimestamp())

		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}

	log.Info("Deleting InternalHubComponent", "Name", ihc.GetName(), "Namespace", ihc.GetNamespace())
	if err := r.Client.Delete(ctx, ihc); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to delete InternalHubComponent CR: %s/%s: %v",
				ihc.GetNamespace(), ihc.GetName(), err)
		}
	}

	// Ensure that the resource is fully deleted by attempting to refetch it
	if err := r.Client.Get(ctx,
		types.NamespacedName{Name: ihc.GetName(), Namespace: ihc.GetNamespace()}, ihc); err != nil {
		if errors.IsNotFound(err) {
			logf.Log.Info("InternalHubComponent successfully deleted", "Name", ihc.GetName(), "Namespace", ihc.GetNamespace())
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get InternalHubComponent %s/%s: %v",
			ihc.GetNamespace(), ihc.GetName(), err)
	}

	// Requeue to check again after a short delay
	return ctrl.Result{RequeueAfter: resyncPeriod}, nil
}
