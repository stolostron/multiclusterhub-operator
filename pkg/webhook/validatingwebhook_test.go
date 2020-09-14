// Copyright (c) 2020 Red Hat, Inc.

package webhook

import (
	"context"
	"fmt"
	"os"
	"testing"

	operatorsv1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operator/v1"
	observability "github.com/open-cluster-management/multicluster-monitoring-operator/pkg/apis"
	observabilityv1beta1 "github.com/open-cluster-management/multicluster-monitoring-operator/pkg/apis/observability/v1beta1"
	netv1 "github.com/openshift/api/config/v1"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
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

	observabilityCR = &observabilityv1beta1.MultiClusterObservability{
		ObjectMeta: metav1.ObjectMeta{
			Name: "observability",
		},
		Spec: observabilityv1beta1.MultiClusterObservabilitySpec{
			StorageConfig: &observabilityv1beta1.StorageConfigObject{
				MetricObjectStorage: &observabilityv1beta1.PreConfiguredStorage{
					Name: "thanos-object-storage",
					Key:  "thanos.yaml",
				},
			},
		},
	}
)

func Test_validateDelete(t *testing.T) {
	v, err := getTestValidator(full_mch)
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Ensure deletion works without any resources available
	req := admission.Request{}
	err = v.validateDelete(req)
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Create observability CR and ensure deletion is blocked
	v.client.Create(context.TODO(), observabilityCR)
	err = v.validateDelete(req)
	if !errorEquals(err, fmt.Errorf("Cannot delete MultiClusterHub resource because MultiClusterObservability resource(s) exist")) {
		t.Fatalf(err.Error())
	}
	// Delete observability CR and ensure deletion succeeds
	v.client.Delete(context.TODO(), observabilityCR)
	err = v.validateDelete(req)
	if !errorEquals(err, nil) {
		t.Fatalf(err.Error())
	}

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

func getTestValidator(m *operatorsv1.MultiClusterHub) (*multiClusterHubValidator, error) {
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

	if err := observability.AddToScheme(s); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}
	s.AddKnownTypes(operatorsv1.SchemeGroupVersion, m)

	// Create a fake client to mock API calls.
	cl := fake.NewFakeClient(objs...)

	// Create a ReconcileMultiClusterHub object with the scheme and fake client.
	return &multiClusterHubValidator{client: cl}, nil
}
