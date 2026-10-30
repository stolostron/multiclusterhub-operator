// Copyright (c) 2026 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestDeploymentReconcileRequests(t *testing.T) {
	hubOwner := metav1.OwnerReference{
		APIVersion: "operator.open-cluster-management.io/v1",
		Kind:       "MultiClusterHub",
		Name:       "multiclusterhub",
		Controller: boolPtr(true),
	}
	otherOwner := metav1.OwnerReference{
		APIVersion: "multicluster.openshift.io/v1",
		Kind:       "MultiClusterEngine",
		Name:       "multiclusterengine",
		Controller: boolPtr(true),
	}
	installerLabels := map[string]string{
		"installer.name":      "multiclusterhub",
		"installer.namespace": "open-cluster-management",
	}

	tests := []struct {
		name     string
		deploy   *appsv1.Deployment
		expected []reconcile.Request
	}{
		{
			name: "hub-owned deployment without installer labels",
			deploy: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
				Name:            "console",
				Namespace:       "open-cluster-management",
				OwnerReferences: []metav1.OwnerReference{hubOwner},
			}},
			expected: []reconcile.Request{{NamespacedName: types.NamespacedName{
				Name: "multiclusterhub", Namespace: "open-cluster-management",
			}}},
		},
		{
			name: "mce-owned deployment with installer labels",
			deploy: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
				Name:            "mce-component",
				Namespace:       "multicluster-engine",
				Labels:          installerLabels,
				OwnerReferences: []metav1.OwnerReference{otherOwner},
			}},
			expected: []reconcile.Request{{NamespacedName: types.NamespacedName{
				Name: "multiclusterhub", Namespace: "open-cluster-management",
			}}},
		},
		{
			name: "deployment that is both owned and labelled is enqueued once",
			deploy: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
				Name:            "console",
				Namespace:       "open-cluster-management",
				Labels:          installerLabels,
				OwnerReferences: []metav1.OwnerReference{hubOwner},
			}},
			expected: []reconcile.Request{{NamespacedName: types.NamespacedName{
				Name: "multiclusterhub", Namespace: "open-cluster-management",
			}}},
		},
		{
			name: "unrelated deployment is ignored",
			deploy: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
				Name:      "unrelated",
				Namespace: "default",
			}},
			expected: []reconcile.Request{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deploymentReconcileRequests(context.Background(), tt.deploy)
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d requests, got %d: %v", len(tt.expected), len(got), got)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("request %d: expected %v, got %v", i, tt.expected[i], got[i])
				}
			}
		})
	}
}
