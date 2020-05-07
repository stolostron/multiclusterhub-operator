// Copyright (c) 2020 Red Hat, Inc.

package multiclusterhub

import (
	"context"
	"os"
	"testing"

	chnv1alpha1 "github.com/open-cluster-management/multicloud-operators-channel/pkg/apis"
	subalpha1 "github.com/open-cluster-management/multicloud-operators-subscription/pkg/apis"

	"github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1beta1"
	operatorsv1beta1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestReconcileMultiClusterHub(t *testing.T) {
	var (
		name      = "multiclusterhub-operator"
		namespace = "open-cluster-management"
	)
	os.Setenv("TEMPLATES_PATH", "../../../templates")
	// A MultiClusterHub object with metadata and spec.
	multiClusterHub := &operatorsv1beta1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: operatorsv1beta1.MultiClusterHubSpec{
			ImagePullSecret:       "Always",
			Failover:              false,
			IPv6:                  false,
			CloudPakCompatibility: false,
			Mongo: operatorsv1beta1.Mongo{
				Storage:      "5gi",
				StorageClass: "gp2",
			},
			Etcd: operatorsv1beta1.Etcd{
				Storage:      "1gi",
				StorageClass: "gp2",
			},
		},
		Status: operatorsv1beta1.MultiClusterHubStatus{
			CurrentVersion: "1.0.0",
		},
	}

	// Objects to track in the fake client.
	objs := []runtime.Object{multiClusterHub}

	// Register operator types with the runtime scheme.
	s := scheme.Scheme

	if err := chnv1alpha1.AddToScheme(s); err != nil {
		t.Errorf("Could not add Channel to Scheme")
		os.Exit(1)
	}

	if err := subalpha1.AddToScheme(s); err != nil {
		t.Errorf("Could not add Channel to Scheme")
		os.Exit(1)
	}

	s.AddKnownTypes(v1beta1.SchemeGroupVersion, multiClusterHub)

	// Create a fake client to mock API calls.
	cl := fake.NewFakeClient(objs...)

	// Create a ReconcileMultiClusterHub object with the scheme and fake client.
	r := &ReconcileMultiClusterHub{client: cl, scheme: s}

	// Mock request to simulate Reconcile() being called on an event for a
	// watched resource .
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}
	res, err := r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	// Check the result of reconciliation to make sure it has the desired state.
	if res.Requeue {
		t.Error("reconcile did not requeue request as expected")
	}

	// Check if deployment has been created and has the correct size.
	mch := &operatorsv1beta1.MultiClusterHub{}
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, mch)
	if err != nil {
		t.Errorf("Could not find MultiClusterHub resource")
	}
	os.Unsetenv("TEMPLATES_PATH")
}

func TestUpdateStatus(t *testing.T) {
	var (
		name      = "multiclusterhub-operator"
		namespace = "open-cluster-management"
	)
	multiClusterHub := &operatorsv1beta1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: operatorsv1beta1.MultiClusterHubSpec{
			ImagePullSecret:       "Always",
			Failover:              false,
			IPv6:                  false,
			CloudPakCompatibility: false,
			Mongo: operatorsv1beta1.Mongo{
				Storage:      "5gi",
				StorageClass: "gp2",
			},
			Etcd: operatorsv1beta1.Etcd{
				Storage:      "1gi",
				StorageClass: "gp2",
			},
		},
	}

	objs := []runtime.Object{multiClusterHub}

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	s.AddKnownTypes(v1beta1.SchemeGroupVersion, multiClusterHub)

	// Create a fake client to mock API calls.
	cl := fake.NewFakeClient(objs...)

	// Create a ReconcileMultiClusterHub object with the scheme and fake client.
	r := &ReconcileMultiClusterHub{client: cl, scheme: s}

	_, err := r.validateVersion(multiClusterHub)
	if err != nil {
		t.Errorf("Unable to validate version")
	}

	// Check if deployment has been created and has the correct size.
	mch := &operatorsv1beta1.MultiClusterHub{}
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, mch)
	if err != nil {
		t.Errorf("Could not find MCH")
	}

	if mch.Status.CurrentVersion != multiClusterHub.Status.CurrentVersion || mch.Status.DesiredVersion != multiClusterHub.Status.DesiredVersion {
		t.Errorf("Update failed")
	}
}

func Test_generatePass(t *testing.T) {
	t.Run("Test length", func(t *testing.T) {
		length := 16
		if got := generatePass(length); len(got) != length {
			t.Errorf("length of generatePass(%d) = %d, want %d", length, len(got), length)
		}
	})

	t.Run("Test randomness", func(t *testing.T) {
		t1 := generatePass(32)
		t2 := generatePass(32)
		if t1 == t2 {
			t.Errorf("generatePass() did not generate a unique password")
		}
	})
}
