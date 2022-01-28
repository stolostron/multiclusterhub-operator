// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_ensureKlusterletAddonConfigPausedStatus(t *testing.T) {
	klusterletaddonconfig := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "agent.open-cluster-management.io/v1",
			"kind":       "KlusterletAddonConfig",
			"metadata": map[string]interface{}{
				"name":      "test-name",
				"namespace": "test-namespace",
			},
			"spec": map[string]interface{}{},
		},
	}
	klusterletaddonconfigPaused := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "agent.open-cluster-management.io/v1",
			"kind":       "KlusterletAddonConfig",
			"metadata": map[string]interface{}{
				"name":      "test-name",
				"namespace": "test-namespace",
				"annotations": map[string]interface{}{
					"klusterletaddonconfig-pause": "true",
				},
			},
			"spec": map[string]interface{}{},
		},
	}
	type args struct {
		client     client.Client
		name       string
		namespace  string
		wantPaused bool
	}
	tests := []struct {
		name       string
		args       args
		wantErr    bool
		wantPaused bool // will check the pause annotation when error is nil
	}{
		{
			name: "should return error if not found",
			args: args{
				client:     fake.NewFakeClient(),
				name:       "test-name",
				namespace:  "test-namespace",
				wantPaused: true,
			},
			wantErr:    true,
			wantPaused: false,
		},
		{
			name: "should set pause if want pause and not in pause staus",
			args: args{
				client:     fake.NewFakeClient(klusterletaddonconfig),
				name:       "test-name",
				namespace:  "test-namespace",
				wantPaused: true,
			},
			wantErr:    false,
			wantPaused: true,
		},
		{
			name: "should set pause if want pause and not in pause staus",
			args: args{
				client:     fake.NewFakeClient(klusterletaddonconfig),
				name:       "test-name",
				namespace:  "test-namespace",
				wantPaused: true,
			},
			wantErr:    false,
			wantPaused: true,
		},
		{
			name: "should do nothing if want pause and already in pause",
			args: args{
				client:     fake.NewFakeClient(klusterletaddonconfigPaused),
				name:       "test-name",
				namespace:  "test-namespace",
				wantPaused: true,
			},
			wantErr:    false,
			wantPaused: true,
		},
		{
			name: "should resume if don't want pause and already in pause",
			args: args{
				client:     fake.NewFakeClient(klusterletaddonconfigPaused),
				name:       "test-name",
				namespace:  "test-namespace",
				wantPaused: false,
			},
			wantErr:    false,
			wantPaused: false,
		},
		{
			name: "should do nothing if don't want pause and not in pause",
			args: args{
				client:     fake.NewFakeClient(klusterletaddonconfig),
				name:       "test-name",
				namespace:  "test-namespace",
				wantPaused: false,
			},
			wantErr:    false,
			wantPaused: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ensureKlusterletAddonConfigPausedStatus(tt.args.client, tt.args.name, tt.args.namespace, tt.args.wantPaused)
			if (err != nil) != tt.wantErr {
				t.Fatalf("want error: %t, error: %v", tt.wantErr, err)
			}
			// check paused
			tempKAC := klusterletaddonconfig.DeepCopy()

			_ = tt.args.client.Get(context.TODO(),
				types.NamespacedName{Name: tt.args.name, Namespace: tt.args.namespace},
				tempKAC,
			)
			if err == nil {
				a := tempKAC.GetAnnotations()
				if (tt.wantPaused && (a == nil || a["klusterletaddonconfig-pause"] != "true")) ||
					(!tt.wantPaused && (a != nil && a["klusterletaddonconfig-pause"] == "true")) {
					t.Fatalf("want pause: %t, annotations: %v", tt.wantPaused, a)
				}
			}
		})
	}
}
