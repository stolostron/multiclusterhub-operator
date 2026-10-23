// Copyright Contributors to the Open Cluster Management project

package v1

import (
	"context"
	"fmt"
	"testing"

	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengineutils"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
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
				"installer.name":                          "another-mch",
				"installer.namespace":                     mch.GetNamespace(),
				multiclusterengineutils.MCEManagedByLabel: "true",
			},
		},
	}
	unmanagedMCE := &mcev1.MultiClusterEngine{
		ObjectMeta: metav1.ObjectMeta{
			Name: "unmanaged-mce",
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
			name:    "unmanaged MultiClusterEngine",
			objects: []client.Object{unmanagedMCE},
			want:    true,
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

func TestShouldBlockAgentServiceConfig_ListError(t *testing.T) {
	mch := &MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multiclusterhub",
			Namespace: "open-cluster-management",
		},
	}

	scheme := runtime.NewScheme()
	if err := mcev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}

	t.Run("generic list error returns error", func(t *testing.T) {
		listErr := fmt.Errorf("connection refused")
		c := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
			List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
				return listErr
			},
		}).Build()

		got, err := shouldBlockAgentServiceConfig(context.Background(), c, mch)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if got != false {
			t.Errorf("expected false on error, got %v", got)
		}
	})

	t.Run("MCE API unavailable returns block with no error", func(t *testing.T) {
		// Simulate apimeta.IsNoMatchError by returning a *meta.NoKindMatchError.
		noMatchErr := &apimeta.NoKindMatchError{GroupKind: schema.GroupKind{Group: "agent-install.openshift.io", Kind: "MultiClusterEngine"}}
		c := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
			List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
				return noMatchErr
			},
		}).Build()

		got, err := shouldBlockAgentServiceConfig(context.Background(), c, mch)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != true {
			t.Errorf("expected true (block) when MCE API unavailable, got %v", got)
		}
	})
}
