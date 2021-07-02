// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package multiclusterhub

import (
	"context"
	"fmt"
	"os"
	"testing"

	appsubv1 "github.com/open-cluster-management/multicloud-operators-subscription/pkg/apis"
	operatorsv1 "github.com/open-cluster-management/multiclusterhub-operator/pkg/apis/operator/v1"
	netv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	mch_name      = "multiclusterhub-operator"
	mch_namespace = "open-cluster-management"
	// A MultiClusterHub object with metadata and spec.
	full_mch = &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mch_name,
			Namespace: mch_namespace,
		},
		Spec: operatorsv1.MultiClusterHubSpec{
			ImagePullSecret: "pull-secret",
			Ingress: operatorsv1.IngressSpec{
				SSLCiphers: []string{"foo", "bar", "baz"},
			},
			AvailabilityConfig: operatorsv1.HAHigh,
		},
		Status: operatorsv1.MultiClusterHubStatus{
			CurrentVersion: "2.0.0",
			Phase:          "Running",
		},
	}
	// A MultiClusterHub object with metadata and spec.
	empty_mch = &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mch_name,
			Namespace: mch_namespace,
		},
		Spec: operatorsv1.MultiClusterHubSpec{
			ImagePullSecret: "pull-secret",
		},
	}
	mch_namespaced = types.NamespacedName{
		Name:      mch_name,
		Namespace: mch_namespace,
	}
)

func Test_ReconcileMultiClusterHub(t *testing.T) {

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pull-secret",
			Namespace: mch_namespace,
		},
		StringData: map[string]string{
			"test": "data",
		},
	}

	os.Setenv("UNIT_TEST", "true")
	os.Setenv("TEMPLATES_PATH", "../../../templates")
	os.Setenv("MANIFESTS_PATH", "../../../image-manifests")
	os.Setenv("CRDS_PATH", "../../../crds")
	defer os.Unsetenv("TEMPLATES_PATH")
	defer os.Unsetenv("MANIFESTS_PATH")
	defer os.Unsetenv("UNIT_TEST")
	defer os.Unsetenv("CRDS_PATH")

	// Without Status Prefilled
	mch2 := full_mch.DeepCopy()
	mch2.Status = operatorsv1.MultiClusterHubStatus{}

	// AvailabilityConfig
	mch3 := full_mch.DeepCopy()
	mch3.Spec.AvailabilityConfig = operatorsv1.HABasic

	// SeparateCertificateManagement
	mch5 := full_mch.DeepCopy()
	mch5.Spec.SeparateCertificateManagement = true

	tests := []struct {
		Name     string
		MCH      *operatorsv1.MultiClusterHub
		Expected error
	}{
		{
			Name:     "Full Valid MCH",
			MCH:      full_mch,
			Expected: nil,
		},
		{
			Name:     "Without Status",
			MCH:      mch2,
			Expected: nil,
		},
		{
			Name:     "AvailabilityConfig",
			MCH:      mch3,
			Expected: nil,
		},
		{
			Name:     "CloudPakCompatibility",
			MCH:      mch5,
			Expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {

			r, err := getTestReconciler(tt.MCH)
			if err != nil {
				t.Fatalf("Failed to create test reconciler")
			}
			// Mock request to simulate Reconcile() being called on an event for a
			// watched resource .
			req := reconcile.Request{
				NamespacedName: mch_namespaced,
			}

			if tt.MCH.Spec.SeparateCertificateManagement {
				err = r.client.Create(context.TODO(), secret)
				if err != nil {
					t.Fatal(err.Error())
				}
			}

			res, err := r.Reconcile(req)
			if !errorEquals(err, tt.Expected) {
				t.Fatalf("reconcile: (%v)", err)
			}

			// Check the result of reconciliation to make sure it has the desired state.
			if res.Requeue {
				t.Error("reconcile did not requeue request as expected")
			}

			// Check if MCH has been created
			mch := &operatorsv1.MultiClusterHub{}
			err = r.client.Get(context.TODO(), mch_namespaced, mch)
			if err != nil {
				t.Errorf("Could not find MultiClusterHub resource")
			}

		})
	}

}

func Test_setDefaults(t *testing.T) {
	os.Setenv("TEMPLATES_PATH", "../../../templates")

	// Without Status Prefilled
	mch1 := full_mch.DeepCopy()
	mch1.Status = operatorsv1.MultiClusterHubStatus{}

	tests := []struct {
		Name     string
		MCH      *operatorsv1.MultiClusterHub
		Expected error
	}{
		{
			Name:     "Full Valid MCH",
			MCH:      full_mch,
			Expected: nil,
		},
		{
			Name:     "Without Status",
			MCH:      mch1,
			Expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// Objects to track in the fake client.
			r, err := getTestReconciler(tt.MCH)
			if err != nil {
				t.Fatalf("Failed to create test reconciler")
			}

			_, err = r.setDefaults(tt.MCH)
			if !errorEquals(err, tt.Expected) {
				t.Fatalf("reconcile: (%v)", err)
			}
		})
	}
	os.Unsetenv("TEMPLATES_PATH")
}

func errorEquals(err, expected error) bool {
	if err == nil && expected == nil {
		return true
	} else if (err == nil && expected != nil) || (err != nil && expected == nil) {
		return false
	}

	if err.Error() == expected.Error() {
		return true
	}
	return false
}

func getTestReconciler(m *operatorsv1.MultiClusterHub) (*ReconcileMultiClusterHub, error) {
	objs := []runtime.Object{m}

	// Register operator types with the runtime scheme.
	s := scheme.Scheme

	if err := netv1.AddToScheme(s); err != nil {
		return nil, fmt.Errorf("Could not add ingress to test scheme")
	}

	if err := apiregistrationv1.AddToScheme(s); err != nil {
		return nil, fmt.Errorf("Could not add rbac to test scheme")
	}

	if err := apixv1.AddToScheme(s); err != nil {
		return nil, fmt.Errorf("Could not add CRDs to test scheme")
	}

	if err := appsubv1.AddToScheme(s); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}
	s.AddKnownTypes(operatorsv1.SchemeGroupVersion, m)

	// Create a fake client to mock API calls.
	cl := fake.NewFakeClient(objs...)

	// Create a ReconcileMultiClusterHub object with the scheme and fake client.
	return &ReconcileMultiClusterHub{client: cl, scheme: s}, nil
}

func getTestReconcilerWithObjs(objs []runtime.Object) (*ReconcileMultiClusterHub, error) {
	// Register operator types with the runtime scheme.
	s := scheme.Scheme

	if err := netv1.AddToScheme(s); err != nil {
		return nil, fmt.Errorf("Could not add ingress to test scheme")
	}

	if err := apiregistrationv1.AddToScheme(s); err != nil {
		return nil, fmt.Errorf("Could not add rbac to test scheme")
	}

	if err := apixv1.AddToScheme(s); err != nil {
		return nil, fmt.Errorf("Could not add CRDs to test scheme")
	}

	if err := appsubv1.AddToScheme(s); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}
	s.AddKnownTypes(operatorsv1.SchemeGroupVersion, &operatorsv1.MultiClusterHub{})

	// Create a fake client to mock API calls.
	cl := fake.NewFakeClient(objs...)

	// Create a ReconcileMultiClusterHub object with the scheme and fake client.
	return &ReconcileMultiClusterHub{client: cl, scheme: s}, nil
}
