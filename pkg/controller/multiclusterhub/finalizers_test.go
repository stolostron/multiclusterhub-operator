package multiclusterhub

import (
	"testing"

	operatorsv1beta1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1beta1"
)

func Test_cleanupHiveConfigs(t *testing.T) {

	tests := []struct {
		Name   string
		MCH    *operatorsv1beta1.MultiClusterHub
		Result error
	}{
		{
			Name:   "Full MCH",
			MCH:    full_mch,
			Result: nil,
		},
		{
			Name:   "Empty MCH",
			MCH:    empty_mch,
			Result: nil,
		},
	}

	reqLogger := log.WithValues("Request.Namespace", mch_namespace, "Request.Name", mch_name)

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// Objects to track in the fake client.
			r, err := getTestReconciler(tt.MCH)
			if err != nil {
				t.Fatalf("Failed to create test reconciler")
			}

			err = r.cleanupHiveConfigs(reqLogger, full_mch)
			if err != tt.Result {
				t.Fatal("Failed to cleanup Hive Config")
			}
		})
	}
}

func Test_cleanupAPIServices(t *testing.T) {
	tests := []struct {
		Name   string
		MCH    *operatorsv1beta1.MultiClusterHub
		Result error
	}{
		{
			Name:   "Full MCH",
			MCH:    full_mch,
			Result: nil,
		},
		{
			Name:   "Empty MCH",
			MCH:    empty_mch,
			Result: nil,
		},
	}

	reqLogger := log.WithValues("Request.Namespace", mch_namespace, "Request.Name", mch_name)

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// Objects to track in the fake client.
			r, err := getTestReconciler(tt.MCH)
			if err != nil {
				t.Fatalf("Failed to create test reconciler")
			}

			err = r.cleanupAPIServices(reqLogger, full_mch)
			if err != tt.Result {
				t.Fatal("Failed to cleanup API services")
			}
		})
	}
}

func Test_cleanupClusterRoles(t *testing.T) {
	tests := []struct {
		Name   string
		MCH    *operatorsv1beta1.MultiClusterHub
		Result error
	}{
		{
			Name:   "Full MCH",
			MCH:    full_mch,
			Result: nil,
		},
		{
			Name:   "Empty MCH",
			MCH:    empty_mch,
			Result: nil,
		},
	}

	reqLogger := log.WithValues("Request.Namespace", mch_namespace, "Request.Name", mch_name)

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// Objects to track in the fake client.
			r, err := getTestReconciler(tt.MCH)
			if err != nil {
				t.Fatalf("Failed to create test reconciler")
			}

			err = r.cleanupClusterRoles(reqLogger, full_mch)
			if err != tt.Result {
				t.Fatal("Failed to cleanup clusterroles")
			}
		})
	}
}

func Test_cleanupClusterRoleBindings(t *testing.T) {
	tests := []struct {
		Name   string
		MCH    *operatorsv1beta1.MultiClusterHub
		Result error
	}{
		{
			Name:   "Full MCH",
			MCH:    full_mch,
			Result: nil,
		},
		{
			Name:   "Empty MCH",
			MCH:    empty_mch,
			Result: nil,
		},
	}

	reqLogger := log.WithValues("Request.Namespace", mch_namespace, "Request.Name", mch_name)

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// Objects to track in the fake client.
			r, err := getTestReconciler(tt.MCH)
			if err != nil {
				t.Fatalf("Failed to create test reconciler")
			}

			err = r.cleanupClusterRoleBindings(reqLogger, full_mch)
			if err != tt.Result {
				t.Fatal("Failed to cleanup clusterrolebindings")
			}
		})
	}
}

func Test_cleanupMutatingWebhooks(t *testing.T) {
	tests := []struct {
		Name   string
		MCH    *operatorsv1beta1.MultiClusterHub
		Result error
	}{
		{
			Name:   "Full MCH",
			MCH:    full_mch,
			Result: nil,
		},
		{
			Name:   "Empty MCH",
			MCH:    empty_mch,
			Result: nil,
		},
	}

	reqLogger := log.WithValues("Request.Namespace", mch_namespace, "Request.Name", mch_name)

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// Objects to track in the fake client.
			r, err := getTestReconciler(tt.MCH)
			if err != nil {
				t.Fatalf("Failed to create test reconciler")
			}

			err = r.cleanupMutatingWebhooks(reqLogger, full_mch)
			if err != tt.Result {
				t.Fatal("Failed to cleanup mutatingwebhookconfigurations")
			}
		})
	}
}

func Test_cleanupPullSecret(t *testing.T) {
	tests := []struct {
		Name   string
		MCH    *operatorsv1beta1.MultiClusterHub
		Result error
	}{
		{
			Name:   "Full MCH",
			MCH:    full_mch,
			Result: nil,
		},
		{
			Name:   "Empty MCH",
			MCH:    empty_mch,
			Result: nil,
		},
	}

	reqLogger := log.WithValues("Request.Namespace", mch_namespace, "Request.Name", mch_name)

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// Objects to track in the fake client.
			r, err := getTestReconciler(tt.MCH)
			if err != nil {
				t.Fatalf("Failed to create test reconciler")
			}

			err = r.cleanupPullSecret(reqLogger, full_mch)
			if err != tt.Result {
				t.Fatal("Failed to cleanup pull secret")
			}
		})
	}
}

func Test_cleanupCRDS(t *testing.T) {
	tests := []struct {
		Name   string
		MCH    *operatorsv1beta1.MultiClusterHub
		Result error
	}{
		{
			Name:   "Full MCH",
			MCH:    full_mch,
			Result: nil,
		},
		{
			Name:   "Empty MCH",
			MCH:    empty_mch,
			Result: nil,
		},
	}

	reqLogger := log.WithValues("Request.Namespace", mch_namespace, "Request.Name", mch_name)

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// Objects to track in the fake client.
			r, err := getTestReconciler(tt.MCH)
			if err != nil {
				t.Fatalf("Failed to create test reconciler")
			}

			err = r.cleanupCRDs(reqLogger, full_mch)
			if err != tt.Result {
				t.Fatal("Failed to cleanup CRDs")
			}
		})
	}
}
