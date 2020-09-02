// Copyright (c) 2020 Red Hat, Inc.

package webhook

import (
	"testing"

	operatorsv1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
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
		},
	}
)

func Test_validateDelete(t *testing.T) {
	v, err := getTestValidator(full_mch)
	if err != nil {
		t.Fatalf("Failed to getTestValidator: %s", err.Error())
	}

	// Ensure Deletion Works
	req := admission.Request{}
	err = v.validateDelete(req)
	if err != nil {
		t.Fatalf("Failed to validate deletion of mch: %s", err.Error())
	}
}

func getTestValidator(m *operatorsv1.MultiClusterHub) (*multiClusterHubValidator, error) {
	objs := []runtime.Object{m}

	// Register operator types with the runtime scheme.
	s := scheme.Scheme

	s.AddKnownTypes(operatorsv1.SchemeGroupVersion, m)

	// Create a fake client to mock API calls.
	cl := fake.NewFakeClient(objs...)

	// Create a ReconcileMultiClusterHub object with the scheme and fake client.
	return &multiClusterHubValidator{client: cl}, nil
}
