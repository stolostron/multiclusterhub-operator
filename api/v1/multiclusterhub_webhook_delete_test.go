// Copyright Contributors to the Open Cluster Management project

package v1

import (
	"context"
	"testing"

	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengineutils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestShouldBlockAgentServiceConfig(t *testing.T) {
	mch := &MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multiclusterhub",
			Namespace: "open-cluster-management",
		},
	}

	adoptedMCE := &mcev1.MultiClusterEngine{
		ObjectMeta: metav1.ObjectMeta{
			Name: "adopted-mce",
			Labels: map[string]string{
				multiclusterengineutils.MCEManagedByLabel: "true",
			},
		},
	}
	createdMCE := &mcev1.MultiClusterEngine{
		ObjectMeta: metav1.ObjectMeta{
			Name: "created-mce",
			Labels: map[string]string{
				"installer.name":      mch.GetName(),
				"installer.namespace": mch.GetNamespace(),
			},
		},
	}
	otherMCE := &mcev1.MultiClusterEngine{
		ObjectMeta: metav1.ObjectMeta{
			Name: "other-mce",
			Labels: map[string]string{
				"installer.name":      "another-mch",
				"installer.namespace": mch.GetNamespace(),
			},
		},
	}

	tests := []struct {
		name    string
		objects []client.Object
		want    bool
	}{
		{
			name: "no MultiClusterEngine",
			want: true,
		},
		{
			name:    "adopted MultiClusterEngine",
			objects: []client.Object{adoptedMCE},
			want:    false,
		},
		{
			name:    "MultiClusterEngine created by this MultiClusterHub",
			objects: []client.Object{createdMCE},
			want:    true,
		},
		{
			name:    "MultiClusterEngine created by another MultiClusterHub",
			objects: []client.Object{otherMCE},
			want:    false,
		},
		{
			name:    "multiple MultiClusterEngines",
			objects: []client.Object{adoptedMCE, createdMCE},
			want:    true,
		},
	}

	scheme := runtime.NewScheme()
	if err := mcev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add MultiClusterEngine to scheme: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.objects...).Build()
			got, err := shouldBlockAgentServiceConfig(context.Background(), c, mch)
			if err != nil {
				t.Fatalf("shouldBlockAgentServiceConfig() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("shouldBlockAgentServiceConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}
