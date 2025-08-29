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

	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/predicate"
	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"
	ctrlpredicate "sigs.k8s.io/controller-runtime/pkg/predicate"

	configv1 "github.com/openshift/api/config/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *MultiClusterHubReconciler) setOperatorUpgradeableStatus(ctx context.Context, m *operatorv1.MultiClusterHub) (bool, error) {
	// Temporary variable
	var upgradeable bool

	// Checking to see if the current version of the MCH matches the desired to determine if we are in an upgrade scenario
	// If the current version doesn't exist, we are currently in a install which will also not allow it to upgrade

	if m.Status.CurrentVersion != m.Status.DesiredVersion {
		upgradeable = false
	} else {
		upgradeable = true
	}
	// These messages are drawn from operator condition
	// Right now, they just indicate between upgrading and not
	msg := utils.UpgradeableAllowMessage
	status := metav1.ConditionTrue
	reason := utils.UpgradeableAllowReason

	// The condition is the only field that affects whether or not we can upgrade
	// The rest are just status info
	if !upgradeable {
		status = metav1.ConditionFalse
		reason = utils.UpgradeableUpgradingReason
		msg = utils.UpgradeableUpgradingMessage

	} else {

		msg = utils.UpgradeableAllowMessage
		status = metav1.ConditionTrue
		reason = utils.UpgradeableAllowReason

	}
	// This error should only occur if the operator condition does not exist for some reason
	if err := r.UpgradeableCond.Set(ctx, status, reason, msg); err != nil {
		return true, err
	}

	if !upgradeable {
		return true, nil
	} else {
		return false, nil
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *MultiClusterHubReconciler) SetupWithManager(mgr ctrl.Manager) (controller.Controller, error) {
	return ctrl.NewControllerManagedBy(mgr).
		For(
			&operatorv1.MultiClusterHub{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Watches(
			&appsv1.Deployment{},
			handler.EnqueueRequestForOwner(
				mgr.GetScheme(), mgr.GetRESTMapper(), &operatorv1.MultiClusterHub{}, handler.OnlyControllerOwner(),
			),
			builder.WithPredicates(
				ctrlpredicate.Or(
					ctrlpredicate.GenerationChangedPredicate{},
					ctrlpredicate.LabelChangedPredicate{},
					ctrlpredicate.AnnotationChangedPredicate{},
				),
			),
		).
		Watches(
			&apiregistrationv1.APIService{},
			handler.Funcs{
				DeleteFunc: func(ctx context.Context, e event.DeleteEvent, q workqueue.RateLimitingInterface) {
					labels := e.Object.GetLabels()
					q.Add(
						reconcile.Request{
							NamespacedName: types.NamespacedName{
								Name:      labels["installer.name"],
								Namespace: labels["installer.namespace"],
							},
						},
					)
				},
			},
			builder.WithPredicates(predicate.DeletePredicate{}),
		).
		Watches(&appsv1.Deployment{},
			handler.EnqueueRequestsFromMapFunc(
				func(ctx context.Context, a client.Object) []reconcile.Request {
					return []reconcile.Request{
						{
							NamespacedName: types.NamespacedName{
								Name:      a.GetLabels()["installer.name"],
								Namespace: a.GetLabels()["installer.namespace"],
							},
						},
					}
				},
			),
			builder.WithPredicates(
				ctrlpredicate.And(
					predicate.InstallerLabelPredicate{},
					ctrlpredicate.Or(
						ctrlpredicate.GenerationChangedPredicate{},
						ctrlpredicate.LabelChangedPredicate{},
						ctrlpredicate.AnnotationChangedPredicate{},
					),
				),
			),
		).
		Watches(
			&configv1.ClusterVersion{},
			handler.EnqueueRequestsFromMapFunc(
				func(ctx context.Context, a client.Object) []reconcile.Request {
					multiClusterHubList := &operatorv1.MultiClusterHubList{}
					if err := r.Client.List(context.TODO(), multiClusterHubList); err == nil && len(multiClusterHubList.Items) > 0 {
						mch := multiClusterHubList.Items[0]
						return []reconcile.Request{
							{
								NamespacedName: types.NamespacedName{
									Name:      mch.GetName(),
									Namespace: mch.GetNamespace(),
								},
							},
						}
					}
					return []reconcile.Request{}
				},
			),
		).
		Build(r)
}
