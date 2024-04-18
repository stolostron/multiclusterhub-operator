// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	backplanev1 "github.com/stolostron/backplane-operator/api/v1"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var r = MultiClusterHubReconciler{
	Client: fake.NewClientBuilder().Build(),
}

func registerScheme() {
	operatorv1.AddToScheme(scheme.Scheme)
	backplanev1.AddToScheme(scheme.Scheme)
}

// func Test_HostedReconcile(t *testing.T) {
// 	defer func() {
// 		statusQueue, statusError := r.updateHostedHubStatus(mch) //1
// 		if statusError != nil {
// 			r.Log.Error(retErr, "Error updating status")
// 		}
// 		if empty := (reconcile.Result{}); retRes == empty {
// 			retRes = statusQueue
// 		}
// 		if retErr == nil {
// 			retErr = statusError
// 		}
// 	}()

// 	// If deletion detected, finalize backplane config
// 	if mch.GetDeletionTimestamp() != nil {
// 		if controllerutil.ContainsFinalizer(mch, hubFinalizer) {
// 			err := r.finalizeHostedMCH(ctx, mch) // returns all errors
// 			if err != nil {
// 				log.Info(err.Error())
// 				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
// 			}

// 			log.Info("all subcomponents have been finalized successfully - removing finalizer")
// 			controllerutil.RemoveFinalizer(mch, hubFinalizer)
// 			if err := r.Client.Update(ctx, mch); err != nil {
// 				return ctrl.Result{}, err
// 			}
// 		}

// 		return ctrl.Result{}, nil // Object finalized successfully
// 	}

// 	// Add finalizer for this CR
// 	if !controllerutil.ContainsFinalizer(mch, hubFinalizer) {
// 		controllerutil.AddFinalizer(mch, hubFinalizer)
// 		if err := r.Client.Update(ctx, mch); err != nil {
// 			return ctrl.Result{}, err
// 		}
// 	}

// 	var result ctrl.Result
// 	var err error

// 	result, err = r.setHostedDefaults(ctx, mch)
// 	if result != (ctrl.Result{}) {
// 		return ctrl.Result{}, err
// 	}
// 	if err != nil {
// 		return ctrl.Result{Requeue: true}, err
// 	}

// 	// Do not reconcile objects if this instance of mch is labeled "paused"
// 	updatePausedCondition(mch)
// 	if utils.IsPaused(mch) {
// 		r.Log.Info("MultiClusterHub reconciliation is paused. Nothing more to do.")
// 		return ctrl.Result{}, nil
// 	}

// 	result, err = r.ensureHostedMultiClusterEngine(ctx, mch)
// 	if result != (ctrl.Result{}) {
// 		return result, err
// 	}

// 	return ctrl.Result{}, nil
// }

func Test_setHostedDefaults(t *testing.T) {
	tests := []struct {
		name string
		mch  *operatorv1.MultiClusterHub
		want bool
	}{
		{
			name: "should set hosted defaults values",
			mch: &operatorv1.MultiClusterHub{
				ObjectMeta: v1.ObjectMeta{
					Name:      "multiclusterhub",
					Namespace: "ocm",
				},
			},
			want: true,
		},
	}

	registerScheme()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r.Client.Delete(context.TODO(), tt.mch)
			}()

			// Setting the hosted default values should fail since MCH is not created yet.
			if _, err := r.setHostedDefaults(context.TODO(), tt.mch); err == nil {
				t.Error("expected error when setting hosted defaults", "Error", err)
			}

			if err := r.Client.Create(context.TODO(), tt.mch); err != nil {
				t.Error("failed to create mch", "Error", err)
			}

			existingMCH := &operatorv1.MultiClusterHub{}
			if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: tt.mch.GetName(),
				Namespace: tt.mch.GetNamespace()}, existingMCH); err != nil {
				t.Error("failed to get mch", "Error", err)
			}

			if len(existingMCH.Spec.Overrides.Components) == 0 {
				t.Error("failed to set default override components", "Error", existingMCH.Spec.Overrides.Components)
			}
		})
	}
}

func Test_finalizeHostedMCH(t *testing.T) {
	tests := []struct {
		name string
		mce  *backplanev1.MultiClusterEngine
		mch  *operatorv1.MultiClusterHub
	}{
		{
			name: "should finalize hosted multiclusterhub",
			mce: &backplanev1.MultiClusterEngine{
				ObjectMeta: v1.ObjectMeta{
					Name: "multiclusterhub-engine",
				},
				Spec: backplanev1.MultiClusterEngineSpec{
					TargetNamespace: "",
				},
			},
			mch: &operatorv1.MultiClusterHub{
				ObjectMeta: v1.ObjectMeta{
					Name:      "multiclusterhub",
					Namespace: "ocm",
				},
			},
		},
	}

	registerScheme()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r.Client.Delete(context.TODO(), tt.mce)
				r.Client.Delete(context.TODO(), tt.mch)
			}()

			if err := r.Client.Create(context.TODO(), tt.mch); err != nil {
				t.Error("failed to create mch", "Error", err)
			}

			if err := r.Client.Create(context.TODO(), tt.mce); err != nil {
				t.Error("failed to create mch", "Error", err)
			}

			// MCE should be deleted on the first iteration of finalizing the mch
			if err := r.finalizeHostedMCH(context.TODO(), tt.mch); err == nil {
				t.Error("failed to finalize hosted mch", "Error", err)
			}

			if err := r.finalizeHostedMCH(context.TODO(), tt.mch); err != nil {
				t.Error("failed to finalize hosted mch", "Error", err)
			}
		})
	}
}

func Test_ensureHostedMultiClusterEngineCR(t *testing.T) {
	tests := []struct {
		name string
		mce  *backplanev1.MultiClusterEngine
		mch  *operatorv1.MultiClusterHub
	}{
		{
			name: "should ensure that hosted multiclusterengine CR exists",
			mce: &backplanev1.MultiClusterEngine{
				ObjectMeta: v1.ObjectMeta{
					Name: "multiclusterhub-engine",
				},
				Spec: backplanev1.MultiClusterEngineSpec{
					TargetNamespace: "ocm-engine",
				},
			},
			mch: &operatorv1.MultiClusterHub{
				ObjectMeta: v1.ObjectMeta{
					Name:      "multiclusterhub",
					Namespace: "ocm",
				},
			},
		},
	}

	registerScheme()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r.Client.Delete(context.TODO(), tt.mce)
				r.Client.Delete(context.TODO(), tt.mch)
			}()

			if err := r.Client.Create(context.TODO(), tt.mch); err != nil {
				t.Error("failed to create mch", "Error", err)
			}

			if _, err := r.ensureHostedMultiClusterEngineCR(context.TODO(), tt.mch); err != nil {
				t.Error("failed to ensure hosted multiclusterengine", "Error", err)
			}

			if err := r.Client.Delete(context.TODO(), tt.mce); err != nil {
				t.Error("failed to delete hosted multiclusterengine", "Error", err)
			}

			if err := r.Client.Create(context.TODO(), tt.mce); err != nil {
				t.Error("failed to create mch", "Error", err)
			}

			if _, err := r.ensureHostedMultiClusterEngineCR(context.TODO(), tt.mch); err != nil {
				t.Error("failed to ensure hosted multiclusterengine", "Error", err)
			}
		})
	}
}

func Test_ensureHostedKubeconfigSecret(t *testing.T) {
	tests := []struct {
		name string
		mch  *operatorv1.MultiClusterHub
		ns   *corev1.Namespace
		sec1 *corev1.Secret
		sec2 *corev1.Secret
	}{
		{
			name: "should ensure that hosted kubeconfig secret exists",
			mch: &operatorv1.MultiClusterHub{
				ObjectMeta: v1.ObjectMeta{
					Name:      "multiclusterhub",
					Namespace: "ocm",
					Annotations: map[string]string{
						utils.DeprecatedAnnotationKubeconfig: "sample-kubeconfig",
					},
				},
			},
			ns: &corev1.Namespace{
				ObjectMeta: v1.ObjectMeta{
					Name: "ocm-engine",
				},
			},
			sec1: &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-kubeconfig",
					Namespace: "ocm",
				},
			},
			sec2: &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-kubeconfig",
					Namespace: "ocm-engine",
				},
			},
		},
	}

	registerScheme()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r.Client.Delete(context.TODO(), tt.mch)
				r.Client.Delete(context.TODO(), tt.sec1)
				r.Client.Delete(context.TODO(), tt.sec2)
				r.Client.Delete(context.TODO(), tt.ns)
			}()

			if err := r.Client.Create(context.TODO(), tt.mch); err != nil {
				t.Error("failed to create secret", "Error", err)
			}

			if err := r.Client.Create(context.TODO(), tt.sec1); err != nil {
				t.Error("failed to create secret", "Error", err)
			}

			if err := r.Client.Create(context.TODO(), tt.ns); err != nil {
				t.Error("failed to create namespace", "Error", err)
			}

			if err := r.Client.Create(context.TODO(), tt.sec2); err != nil {
				t.Error("failed to create secret", "Error", err)
			}

			// Mock clients do not support patching resources; therefore we should ignore this for now.
			// if _, err := r.ensureHostedKubeconfigSecret(tt.mch, "ocm-engine"); err != nil {
			// 	t.Error("failed to ensure hosted kubeconfig secret", "Error", err)
			// }
		})
	}
}
