// Copyright Contributors to the Open Cluster Management project

package multiclusterengineutils

import (
	"context"
	"testing"

	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestMCECreatedByMCH(t *testing.T) {
	tests := []struct {
		name    string
		mce     *mcev1.MultiClusterEngine
		mchName string
		mchNs   string
		want    bool
	}{
		{
			name: "nil MCE",
			mce:  nil,
			want: false,
		},
		{
			name: "no labels",
			mce: &mcev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{Name: "mce"},
			},
			want: false,
		},
		{
			name: "empty labels",
			mce: &mcev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "mce",
					Labels: map[string]string{},
				},
			},
			mchName: "mch",
			mchNs:   "ocm",
			want:    false,
		},
		{
			name: "only managed-by label",
			mce: &mcev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mce",
					Labels: map[string]string{
						MCEManagedByLabel: "true",
					},
				},
			},
			mchName: "mch",
			mchNs:   "ocm",
			want:    false,
		},
		{
			name: "matching installer labels",
			mce: &mcev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mce",
					Labels: map[string]string{
						"installer.name":      "mch",
						"installer.namespace": "ocm",
					},
				},
			},
			mchName: "mch",
			mchNs:   "ocm",
			want:    true,
		},
		{
			name: "mismatched installer name",
			mce: &mcev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mce",
					Labels: map[string]string{
						"installer.name":      "other",
						"installer.namespace": "ocm",
					},
				},
			},
			mchName: "mch",
			mchNs:   "ocm",
			want:    false,
		},
		{
			name: "mismatched installer namespace",
			mce: &mcev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mce",
					Labels: map[string]string{
						"installer.name":      "mch",
						"installer.namespace": "other-ns",
					},
				},
			},
			mchName: "mch",
			mchNs:   "ocm",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MCECreatedByMCH(tt.mce, tt.mchName, tt.mchNs)
			if got != tt.want {
				t.Errorf("MCECreatedByMCH() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetManagedMCE(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := mcev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}

	tests := []struct {
		name     string
		objects  []client.Object
		wantName string
		wantNil  bool
		wantErr  bool
	}{
		{
			name:    "no MCE",
			wantNil: true,
		},
		{
			name: "single managed MCE",
			objects: []client.Object{
				&mcev1.MultiClusterEngine{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "mce-1",
						Labels: map[string]string{MCEManagedByLabel: "true"},
					},
				},
			},
			wantName: "mce-1",
		},
		{
			name: "MCE without managed label",
			objects: []client.Object{
				&mcev1.MultiClusterEngine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "mce-unlabeled",
					},
				},
			},
			wantNil: true,
		},
		{
			name: "multiple managed MCEs",
			objects: []client.Object{
				&mcev1.MultiClusterEngine{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "mce-a",
						Labels: map[string]string{MCEManagedByLabel: "true"},
					},
				},
				&mcev1.MultiClusterEngine{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "mce-b",
						Labels: map[string]string{MCEManagedByLabel: "true"},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.objects...).Build()
			got, err := GetManagedMCE(context.Background(), c)

			if tt.wantErr {
				if err == nil {
					t.Fatal("GetManagedMCE() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("GetManagedMCE() unexpected error: %v", err)
			}
			if tt.wantNil {
				if got != nil {
					t.Errorf("GetManagedMCE() = %v, want nil", got.Name)
				}
				return
			}
			if got == nil {
				t.Fatal("GetManagedMCE() = nil, want non-nil")
			}
			if got.Name != tt.wantName {
				t.Errorf("GetManagedMCE() name = %v, want %v", got.Name, tt.wantName)
			}
		})
	}
}
