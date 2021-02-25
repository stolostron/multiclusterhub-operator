// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project


package multiclusterhub

import (
	"context"
	"fmt"
	"testing"

	operatorsv1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operator/v1"
)

func Test_ensureHubIsImported(t *testing.T) {

	tests := []struct {
		Name   string
		MCH    *operatorsv1.MultiClusterHub
		Result error
	}{
		{
			Name:   "Status phase 'running'",
			MCH:    full_mch,
			Result: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			r, err := getTestReconciler(tt.MCH)
			if err != nil {
				t.Fatalf("Failed to create test reconciler")
			}

			_, err = r.ensureHubIsImported(tt.MCH)
			if !errorEquals(err, tt.Result) {
				t.Fatalf("Err: %s", err)
			}
		})
	}
}

func Test_ensureHubIsExported(t *testing.T) {
	tests := []struct {
		Name   string
		MCH    *operatorsv1.MultiClusterHub
		Result error
	}{
		{
			Name:   "Status phase 'running'",
			MCH:    full_mch,
			Result: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			r, err := getTestReconciler(tt.MCH)
			if err != nil {
				t.Fatalf("Failed to create test reconciler")
			}

			_, err = r.ensureHubIsImported(tt.MCH)
			if !errorEquals(err, tt.Result) {
				t.Fatalf("Err: %s", err)
			}

			_, err = r.ensureHubIsExported(tt.MCH)
			if !errorEquals(err, tt.Result) {
				t.Fatalf("Err: %s", err)
			}
		})
	}
}

func Test_ensureHubNamespaceIsRemoved(t *testing.T) {

	tests := []struct {
		Name     string
		MCH      *operatorsv1.MultiClusterHub
		CreateNS bool
		Result   error
	}{
		{
			Name:     "Create Namespace",
			MCH:      full_mch,
			CreateNS: true,
			Result:   fmt.Errorf("Waiting on namespace: local-cluster to be removed"),
		},
		{
			Name:     "Namespace nonexistant",
			MCH:      full_mch,
			CreateNS: false,
			Result:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			r, err := getTestReconciler(tt.MCH)
			if err != nil {
				t.Fatalf("Failed to create test reconciler")
			}

			if tt.CreateNS {
				r.client.Create(context.TODO(), getHubNamespace())
			}

			_, err = r.ensureHubNamespaceIsRemoved(tt.MCH)
			if !errorEquals(err, tt.Result) {
				t.Fatalf("Err: %s", err)
			}

		})
	}
}

func Test_ensureManagedCluster(t *testing.T) {
	r, err := getTestReconciler(full_mch)
	if err != nil {
		t.Fatalf("Failed to create test reconciler")
	}

	// Call first time to create
	_, err = r.ensureManagedCluster(full_mch)
	if !errorEquals(err, nil) {
		t.Fatalf("Err: %s", err)
	}

	// Call second time to get
	_, err = r.ensureManagedCluster(full_mch)
	if !errorEquals(err, nil) {
		t.Fatalf("Err: %s", err)
	}
}

func Test_removeManagedCluster(t *testing.T) {
	r, err := getTestReconciler(full_mch)
	if err != nil {
		t.Fatalf("Failed to create test reconciler")
	}

	_, err = r.ensureManagedCluster(full_mch)
	if !errorEquals(err, nil) {
		t.Fatalf("Err: %s", err)
	}

	// Call first time to delete
	_, err = r.removeManagedCluster(full_mch)
	if !errorEquals(err, nil) {
		t.Fatalf("Err: %s", err)
	}

	// Call second time to ensure nil is returned if nonexistant
	_, err = r.removeManagedCluster(full_mch)
	if !errorEquals(err, nil) {
		t.Fatalf("Err: %s", err)
	}
}

func Test_ensureKlusterletAddonConfig(t *testing.T) {
	r, err := getTestReconciler(full_mch)
	if err != nil {
		t.Fatalf("Failed to create test reconciler")
	}

	// Call first time to create
	_, err = r.ensureKlusterletAddonConfig(full_mch)
	if !errorEquals(err, nil) {
		t.Fatalf("Err: %s", err)
	}

	// Call second time to get
	_, err = r.ensureKlusterletAddonConfig(full_mch)
	if !errorEquals(err, nil) {
		t.Fatalf("Err: %s", err)
	}
}
