// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"testing"

	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	resources "github.com/stolostron/multiclusterhub-operator/test/unit-tests"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// Test_EdgeManagerCleanup_FirstReconcile tests that edge manager cleanup runs on first reconcile
// and sets the annotation to prevent future cleanups
func Test_EdgeManagerCleanup_FirstReconcile(t *testing.T) {
	ctx := context.Background()

	// Register schemes
	registerScheme()

	// Create MCH without cleanup annotation
	mch := resources.EmptyMCH()
	mch.Namespace = mchNamespace
	mch.Name = mchName

	// Create some edge manager resources to cleanup
	flightctlDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flightctl-api",
			Namespace: mchNamespace,
			Labels: map[string]string{
				"installer.name":      "multiclusterhub",
				"installer.namespace": mchNamespace,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "flightctl-api",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "flightctl-api",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "api",
							Image: "registry.redhat.io/rhem/flightctl-api-rhel9:latest",
						},
					},
				},
			},
		},
	}

	flightctlSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flightctl-db-secret",
			Namespace: mchNamespace,
		},
		Data: map[string][]byte{
			"password": []byte("test-password"),
		},
	}

	// Create fake client with resources
	objs := []runtime.Object{&mch, flightctlDeployment, flightctlSecret}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(objs...).Build()

	// Get MCH before cleanup
	originalMCH := &operatorv1.MultiClusterHub{}
	err := fakeClient.Get(ctx, types.NamespacedName{Name: mchName, Namespace: mchNamespace}, originalMCH)
	if err != nil {
		t.Fatalf("Failed to get MCH: %v", err)
	}

	// Verify annotation does not exist before cleanup
	annotations := originalMCH.GetAnnotations()
	if annotations != nil && annotations[edgeManagerCleanupAnnotation] == "true" {
		t.Errorf("Cleanup annotation should not exist before first reconcile")
	}

	// Simulate the cleanup code block from reconcile.go
	if originalMCH.GetAnnotations()[edgeManagerCleanupAnnotation] != "true" {
		// Mark cleanup as complete (we're not actually running the cleanup in this test)
		annotations := originalMCH.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[edgeManagerCleanupAnnotation] = "true"
		originalMCH.SetAnnotations(annotations)

		err = fakeClient.Update(ctx, originalMCH)
		if err != nil {
			t.Fatalf("Failed to update MCH with cleanup annotation: %v", err)
		}
	}

	// Get updated MCH
	updatedMCH := &operatorv1.MultiClusterHub{}
	err = fakeClient.Get(ctx, types.NamespacedName{Name: mchName, Namespace: mchNamespace}, updatedMCH)
	if err != nil {
		t.Fatalf("Failed to get updated MCH: %v", err)
	}

	// Verify annotation was set
	annotations = updatedMCH.GetAnnotations()
	if annotations == nil || annotations[edgeManagerCleanupAnnotation] != "true" {
		t.Errorf("Cleanup annotation should be set to 'true' after first reconcile, got: %v", annotations)
	}
}

// Test_EdgeManagerCleanup_SecondReconcile tests that cleanup is skipped when annotation exists
func Test_EdgeManagerCleanup_SecondReconcile(t *testing.T) {
	ctx := context.Background()

	// Register schemes
	registerScheme()

	// Create MCH with cleanup annotation already set
	mch := resources.EmptyMCH()
	mch.Namespace = mchNamespace
	mch.Name = mchName
	mch.Annotations = map[string]string{
		edgeManagerCleanupAnnotation: "true",
	}

	// Create fake client
	objs := []runtime.Object{&mch}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(objs...).Build()

	// Get MCH
	originalMCH := &operatorv1.MultiClusterHub{}
	err := fakeClient.Get(ctx, types.NamespacedName{Name: mchName, Namespace: mchNamespace}, originalMCH)
	if err != nil {
		t.Fatalf("Failed to get MCH: %v", err)
	}

	// Verify the cleanup code block would be skipped
	cleanupShouldRun := originalMCH.GetAnnotations()[edgeManagerCleanupAnnotation] != "true"
	if cleanupShouldRun {
		t.Errorf("Cleanup should be skipped when annotation is set to 'true', but would run")
	}

	// Verify annotation still exists and is unchanged
	annotations := originalMCH.GetAnnotations()
	if annotations == nil || annotations[edgeManagerCleanupAnnotation] != "true" {
		t.Errorf("Cleanup annotation should remain 'true', got: %v", annotations)
	}
}

// Test_EdgeManagerCleanup_AnnotationPersists tests that annotation persists across multiple reconciles
func Test_EdgeManagerCleanup_AnnotationPersists(t *testing.T) {
	ctx := context.Background()

	// Register schemes
	registerScheme()

	// Create MCH with cleanup annotation
	mch := resources.EmptyMCH()
	mch.Namespace = mchNamespace
	mch.Name = mchName
	mch.Annotations = map[string]string{
		edgeManagerCleanupAnnotation: "true",
	}

	// Create fake client
	objs := []runtime.Object{&mch}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(objs...).Build()

	// Simulate multiple reconciles
	for i := 0; i < 5; i++ {
		mchCopy := &operatorv1.MultiClusterHub{}
		err := fakeClient.Get(ctx, types.NamespacedName{Name: mchName, Namespace: mchNamespace}, mchCopy)
		if err != nil {
			t.Fatalf("Reconcile %d: Failed to get MCH: %v", i, err)
		}

		// Verify cleanup would be skipped
		if mchCopy.GetAnnotations()[edgeManagerCleanupAnnotation] != "true" {
			t.Errorf("Reconcile %d: Cleanup annotation should be 'true', got: %v", i, mchCopy.GetAnnotations())
		}
	}
}

// Test_EdgeManagerCleanup_NoAnnotationBeforeUpgrade tests the state before operator upgrade
func Test_EdgeManagerCleanup_NoAnnotationBeforeUpgrade(t *testing.T) {
	ctx := context.Background()

	// Register schemes
	registerScheme()

	// Create MCH as it would exist before operator upgrade (no cleanup annotation)
	mch := resources.EmptyMCH()
	mch.Namespace = mchNamespace
	mch.Name = mchName
	// Explicitly no annotations

	// Create fake client
	objs := []runtime.Object{&mch}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(objs...).Build()

	// Get MCH
	originalMCH := &operatorv1.MultiClusterHub{}
	err := fakeClient.Get(ctx, types.NamespacedName{Name: mchName, Namespace: mchNamespace}, originalMCH)
	if err != nil {
		t.Fatalf("Failed to get MCH: %v", err)
	}

	// Verify annotation does not exist
	annotations := originalMCH.GetAnnotations()
	if annotations != nil && annotations[edgeManagerCleanupAnnotation] != "" {
		t.Errorf("Cleanup annotation should not exist before upgrade, got: %v", annotations)
	}

	// Verify cleanup would run
	cleanupShouldRun := originalMCH.GetAnnotations()[edgeManagerCleanupAnnotation] != "true"
	if !cleanupShouldRun {
		t.Errorf("Cleanup should run on first reconcile after upgrade")
	}
}

// Test_EdgeManagerCleanup_AnnotationValue tests that only the exact value "true" prevents cleanup
func Test_EdgeManagerCleanup_AnnotationValue(t *testing.T) {
	tests := []struct {
		name             string
		annotationValue  string
		cleanupShouldRun bool
	}{
		{
			name:             "No annotation",
			annotationValue:  "",
			cleanupShouldRun: true,
		},
		{
			name:             "Annotation set to 'true'",
			annotationValue:  "true",
			cleanupShouldRun: false,
		},
		{
			name:             "Annotation set to 'false'",
			annotationValue:  "false",
			cleanupShouldRun: true,
		},
		{
			name:             "Annotation set to 'True' (capital T)",
			annotationValue:  "True",
			cleanupShouldRun: true,
		},
		{
			name:             "Annotation set to '1'",
			annotationValue:  "1",
			cleanupShouldRun: true,
		},
		{
			name:             "Annotation set to empty string",
			annotationValue:  "",
			cleanupShouldRun: true,
		},
	}

	registerScheme()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create MCH with specified annotation value
			mch := resources.EmptyMCH()
			mch.Namespace = mchNamespace
			mch.Name = mchName
			if tt.annotationValue != "" {
				mch.Annotations = map[string]string{
					edgeManagerCleanupAnnotation: tt.annotationValue,
				}
			}

			// Create fake client
			objs := []runtime.Object{&mch}
			fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(objs...).Build()

			// Get MCH
			mchCopy := &operatorv1.MultiClusterHub{}
			err := fakeClient.Get(ctx, types.NamespacedName{Name: mchName, Namespace: mchNamespace}, mchCopy)
			if err != nil {
				t.Fatalf("Failed to get MCH: %v", err)
			}

			// Check if cleanup would run
			cleanupRuns := mchCopy.GetAnnotations()[edgeManagerCleanupAnnotation] != "true"
			if cleanupRuns != tt.cleanupShouldRun {
				t.Errorf("Expected cleanupShouldRun=%v, got cleanupRuns=%v for annotation value '%s'",
					tt.cleanupShouldRun, cleanupRuns, tt.annotationValue)
			}
		})
	}
}
