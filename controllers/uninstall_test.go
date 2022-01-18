// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func Test_newUnstructured(t *testing.T) {
	type args struct {
		nn  types.NamespacedName
		gvk schema.GroupVersionKind
	}
	tests := []struct {
		name string
		args args
		want *unstructured.Unstructured
	}{
		{
			name: "Subscription",
			args: args{
				nn:  types.NamespacedName{Name: "topology-sub", Namespace: "test"},
				gvk: schema.GroupVersionKind{Group: "apps.open-cluster-management.io", Kind: "Subscription", Version: "v1"},
			},
			want: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps.open-cluster-management.io/v1",
					"kind":       "Subscription",
					"metadata": map[string]interface{}{
						"name":      "topology-sub",
						"namespace": "test",
					},
				},
			},
		},
		{
			name: "Kuisubscription",
			args: args{
				nn:  types.NamespacedName{Name: "kui-web-terminal-sub", Namespace: "test"},
				gvk: schema.GroupVersionKind{Group: "apps.open-cluster-management.io", Kind: "Subscription", Version: "v1"},
			},
			want: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps.open-cluster-management.io/v1",
					"kind":       "Subscription",
					"metadata": map[string]interface{}{
						"name":      "kui-web-terminal-sub",
						"namespace": "test",
					},
				},
			},
		},
		{
			name: "RcmSubscription",
			args: args{
				nn:  types.NamespacedName{Name: "rcm-sub", Namespace: "test"},
				gvk: schema.GroupVersionKind{Group: "apps.open-cluster-management.io", Kind: "Subscription", Version: "v1"},
			},
			want: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps.open-cluster-management.io/v1",
					"kind":       "Subscription",
					"metadata": map[string]interface{}{
						"name":      "rcm-sub",
						"namespace": "test",
					},
				},
			},
		},
		{
			name: "CRD",
			args: args{
				nn:  types.NamespacedName{Name: "searchcollectors.agent.open-cluster-management.io", Namespace: "test"},
				gvk: schema.GroupVersionKind{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition", Version: "v1beta1"},
			},
			want: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apiextensions.k8s.io/v1beta1",
					"kind":       "CustomResourceDefinition",
					"metadata": map[string]interface{}{
						"name":      "searchcollectors.agent.open-cluster-management.io",
						"namespace": "test",
					},
				},
			},
		},
		{
			name: "MirroredManaged",
			args: args{
				nn:  types.NamespacedName{Name: "mirroredmanagedclusters.cluster.open-cluster-management.io", Namespace: "test"},
				gvk: schema.GroupVersionKind{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition", Version: "v1"},
			},
			want: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apiextensions.k8s.io/v1",
					"kind":       "CustomResourceDefinition",
					"metadata": map[string]interface{}{
						"name":      "mirroredmanagedclusters.cluster.open-cluster-management.io",
						"namespace": "test",
					},
				},
			},
		},
		{
			name: "Deployment",
			args: args{
				nn:  types.NamespacedName{Name: "ocm-webhook", Namespace: "test"},
				gvk: schema.GroupVersionKind{Group: "apps", Kind: "Deployment", Version: "v1"},
			},
			want: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "ocm-webhook",
						"namespace": "test",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newUnstructured(tt.args.nn, tt.args.gvk); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newUnstructured() = %v, want %v", got, tt.want)
			}
		})
	}
}

// func TestReconcileMultiClusterHub_uninstall(t *testing.T) {
// 	r, err := getTestReconciler(full_mch)
// 	if err != nil {
// 		t.Fatalf("Failed to create test reconciler")
// 	}

// 	createdCRD := &unstructured.Unstructured{
// 		Object: map[string]interface{}{
// 			"apiVersion": "apiextensions.k8s.io/v1beta1",
// 			"kind":       "CustomResourceDefinition",
// 			"metadata": map[string]interface{}{
// 				"name": "searchcollectors.agent.open-cluster-management.io",
// 			},
// 		},
// 	}
// 	err = r.Client.Create(context.TODO(), createdCRD)

// 	nonexistingCRD := &unstructured.Unstructured{
// 		Object: map[string]interface{}{
// 			"apiVersion": "apiextensions.k8s.io/v1",
// 			"kind":       "CustomResourceDefinition",
// 			"metadata": map[string]interface{}{
// 				"name": "discoveryconfigs.discovery.open-cluster-management.io",
// 			},
// 		},
// 	}

// 	tests := []struct {
// 		name    string
// 		mch     *operatorsv1.MultiClusterHub
// 		obj     *unstructured.Unstructured
// 		want    bool
// 		wantErr bool
// 	}{
// 		{
// 			name:    "Uninstall existing CRD",
// 			mch:     full_mch,
// 			obj:     createdCRD,
// 			want:    false,
// 			wantErr: false,
// 		},
// 		{
// 			name:    "Uninstall nonexisting CRD",
// 			mch:     full_mch,
// 			obj:     nonexistingCRD,
// 			want:    true,
// 			wantErr: false,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			got, err := r.uninstall(full_mch, tt.obj)
// 			if (err != nil) != tt.wantErr {
// 				t.Errorf("ReconcileMultiClusterHub.uninstall() error = %v, wantErr %v", err, tt.wantErr)
// 				return
// 			}
// 			if !reflect.DeepEqual(got, tt.want) {
// 				t.Errorf("ReconcileMultiClusterHub.uninstall() = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }
