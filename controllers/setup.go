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
	"time"

	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/predicate"
	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"
	ctrlpredicate "sigs.k8s.io/controller-runtime/pkg/predicate"

	configv1 "github.com/openshift/api/config/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
	controllerName := "multiclusterhub"
	if utils.IsUnitTest() {
		// Use a unique name per test invocation to avoid controller name conflicts
		controllerName = fmt.Sprintf("multiclusterhub-%d", time.Now().UnixNano())
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		For(
			&operatorv1.MultiClusterHub{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Watches(
			&appsv1.Deployment{},
			handler.EnqueueRequestsFromMapFunc(deploymentReconcileRequests),
			builder.WithPredicates(
				ctrlpredicate.And(
					ctrlpredicate.Or(
						predicate.InstallerLabelPredicate{},
						ctrlpredicate.NewPredicateFuncs(func(obj client.Object) bool {
							owner := metav1.GetControllerOf(obj)
							return owner != nil && owner.Kind == "MultiClusterHub"
						}),
					),
					ctrlpredicate.Or(
						ctrlpredicate.GenerationChangedPredicate{},
						ctrlpredicate.LabelChangedPredicate{},
						ctrlpredicate.AnnotationChangedPredicate{},
					),
				),
			),
		).
		Watches(
			&apiregistrationv1.APIService{},
			handler.Funcs{
				DeleteFunc: func(ctx context.Context, e event.TypedDeleteEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
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
		Watches(
			&corev1.Namespace{},
			handler.EnqueueRequestsFromMapFunc(
				func(ctx context.Context, a client.Object) []reconcile.Request {
					// Only namespaces used as NetworkPolicy overrides (e.g. the observability
					// namespace) matter here; everything else should not trigger a reconcile.
					if !isNetworkPolicyOverrideNamespace(a.GetName()) {
						return []reconcile.Request{}
					}

					multiClusterHubList := &operatorv1.MultiClusterHubList{}
					if err := r.Client.List(ctx, multiClusterHubList); err == nil && len(multiClusterHubList.Items) > 0 {
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
			// Only react to namespace creation; updates/deletes of any namespace are irrelevant
			// to this watch and would otherwise trigger unnecessary reconciles cluster-wide.
			builder.WithPredicates(ctrlpredicate.Funcs{
				CreateFunc:  func(e event.CreateEvent) bool { return true },
				UpdateFunc:  func(e event.UpdateEvent) bool { return false },
				DeleteFunc:  func(e event.DeleteEvent) bool { return false },
				GenericFunc: func(e event.GenericEvent) bool { return false },
			}),
		).
		Build(r)
}

// deploymentReconcileRequests maps a Deployment event to the MultiClusterHub reconciles it
// affects. Deployments controlled by a MultiClusterHub are enqueued through their controller
// owner reference, while Deployments carrying installer labels (for example MCE-owned
// components) are enqueued through those labels. This replaces the two separate Deployment
// watches that previously double-enqueued the same events.
func deploymentReconcileRequests(_ context.Context, obj client.Object) []reconcile.Request {
	requests := make([]reconcile.Request, 0, 2)

	labels := obj.GetLabels()
	if name, namespace := labels["installer.name"], labels["installer.namespace"]; name != "" && namespace != "" {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: name, Namespace: namespace},
		})
	}

	if owner := metav1.GetControllerOf(obj); owner != nil && owner.Kind == "MultiClusterHub" {
		request := reconcile.Request{
			NamespacedName: types.NamespacedName{Name: owner.Name, Namespace: obj.GetNamespace()},
		}
		if len(requests) == 0 || requests[0] != request {
			requests = append(requests, request)
		}
	}

	return requests
}
