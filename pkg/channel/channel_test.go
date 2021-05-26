// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package channel

import (
	"reflect"
	"testing"

	operatorsv1 "github.com/open-cluster-management/multiclusterhub-operator/pkg/apis/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestValidate(t *testing.T) {
	m := &operatorsv1.MultiClusterHub{ObjectMeta: metav1.ObjectMeta{Namespace: "test"}}
	annotatedCurrent := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps.open-cluster-management.io/v1",
			"kind":       "Channel",
			"metadata": map[string]interface{}{
				"name":      ChannelName,
				"namespace": "test",
			},
			"spec": map[string]interface{}{
				"type":     "HelmRepo",
				"pathname": "http://multiclusterhub-repo.test.svc.cluster.local:3000/charts",
			},
		},
	}
	annotatedCurrent.SetAnnotations(map[string]string{"foo": "bar"})

	annotatedDesired := Channel(m)
	annotatedDesired.SetAnnotations(map[string]string{
		"foo": "bar",
		"apps.open-cluster-management.io/reconcile-rate": "low",
	})

	tests := []struct {
		name  string
		found *unstructured.Unstructured
		want  *unstructured.Unstructured
		want1 bool
	}{
		{
			name:  "Latest channel",
			found: Channel(m),
			want:  Channel(m),
			want1: false,
		},
		{
			name: "Existing channel without annotations",
			found: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps.open-cluster-management.io/v1",
					"kind":       "Channel",
					"metadata": map[string]interface{}{
						"name":      ChannelName,
						"namespace": "test",
					},
					"spec": map[string]interface{}{
						"type":     "HelmRepo",
						"pathname": "http://multiclusterhub-repo.test.svc.cluster.local:3000/charts",
					},
				},
			},
			want:  Channel(m),
			want1: true,
		},
		{
			name:  "Existing channel with annotations",
			found: annotatedCurrent,
			want:  annotatedDesired,
			want1: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := Validate(tt.found)
			if !reflect.DeepEqual(got.GetAnnotations(), tt.want.GetAnnotations()) {
				t.Errorf("Validate() annotations got = %v, want %v", got.GetAnnotations(), tt.want.GetAnnotations())
			}
			if !reflect.DeepEqual(got.Object["spec"], tt.want.Object["spec"]) {
				t.Errorf("Validate() spec got = %v, want %v", got.Object["spec"], tt.want.Object["spec"])
			}
			if got1 != tt.want1 {
				t.Errorf("Validate() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
