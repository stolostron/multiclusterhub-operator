// Copyright Contributors to the Open Cluster Management project

package v1

import (
	"context"
	"testing"

	ocv1 "github.com/operator-framework/operator-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_GetClusterCatalog(t *testing.T) {
	tests := []struct {
		name           string
		catalogs       []ocv1.ClusterCatalog
		desiredPackage string
		wantName       string
		wantErr        bool
		errContains    string
	}{
		{
			name:           "No ClusterCatalogs",
			catalogs:       []ocv1.ClusterCatalog{},
			desiredPackage: "multicluster-engine",
			wantErr:        true,
			errContains:    "no ClusterCatalogs found",
		},
		{
			name: "All catalogs unavailable",
			catalogs: []ocv1.ClusterCatalog{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "catalog-1"},
					Spec: ocv1.ClusterCatalogSpec{
						Priority:         0,
						AvailabilityMode: "Unavailable",
					},
				},
			},
			desiredPackage: "multicluster-engine",
			wantErr:        true,
			errContains:    "no serving ClusterCatalogs found",
		},
		{
			name: "Catalog available but not serving",
			catalogs: []ocv1.ClusterCatalog{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "not-serving"},
					Spec: ocv1.ClusterCatalogSpec{
						Priority:         0,
						AvailabilityMode: "Available",
					},
					Status: ocv1.ClusterCatalogStatus{
						Conditions: []metav1.Condition{
							{
								Type:   "Serving",
								Status: "False",
								Reason: "Unpacking",
							},
						},
					},
				},
			},
			desiredPackage: "multicluster-engine",
			wantErr:        true,
			errContains:    "no serving ClusterCatalogs found",
		},
		{
			name: "Single available catalog",
			catalogs: []ocv1.ClusterCatalog{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "redhat-operators"},
					Spec: ocv1.ClusterCatalogSpec{
						Priority:         0,
						AvailabilityMode: "Available",
					},
					Status: ocv1.ClusterCatalogStatus{
						Conditions: []metav1.Condition{
							{
								Type:   "Serving",
								Status: "True",
							},
						},
					},
				},
			},
			desiredPackage: "multicluster-engine",
			wantName:       "redhat-operators",
			wantErr:        false,
		},
		{
			name: "Multiple catalogs - select highest priority",
			catalogs: []ocv1.ClusterCatalog{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "low-priority"},
					Spec: ocv1.ClusterCatalogSpec{
						Priority:         0,
						AvailabilityMode: "Available",
					},
					Status: ocv1.ClusterCatalogStatus{
						Conditions: []metav1.Condition{{Type: "Serving", Status: "True"}},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "high-priority"},
					Spec: ocv1.ClusterCatalogSpec{
						Priority:         100,
						AvailabilityMode: "Available",
					},
					Status: ocv1.ClusterCatalogStatus{
						Conditions: []metav1.Condition{{Type: "Serving", Status: "True"}},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "medium-priority"},
					Spec: ocv1.ClusterCatalogSpec{
						Priority:         50,
						AvailabilityMode: "Available",
					},
					Status: ocv1.ClusterCatalogStatus{
						Conditions: []metav1.Condition{{Type: "Serving", Status: "True"}},
					},
				},
			},
			desiredPackage: "multicluster-engine",
			wantName:       "high-priority",
			wantErr:        false,
		},
		{
			name: "Multiple catalogs with same highest priority - error",
			catalogs: []ocv1.ClusterCatalog{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "catalog-1"},
					Spec: ocv1.ClusterCatalogSpec{
						Priority:         100,
						AvailabilityMode: "Available",
					},
					Status: ocv1.ClusterCatalogStatus{
						Conditions: []metav1.Condition{{Type: "Serving", Status: "True"}},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "catalog-2"},
					Spec: ocv1.ClusterCatalogSpec{
						Priority:         100,
						AvailabilityMode: "Available",
					},
					Status: ocv1.ClusterCatalogStatus{
						Conditions: []metav1.Condition{{Type: "Serving", Status: "True"}},
					},
				},
			},
			desiredPackage: "multicluster-engine",
			wantErr:        true,
			errContains:    "found more than one ClusterCatalog with highest priority",
		},
		{
			name: "Skip unavailable catalogs",
			catalogs: []ocv1.ClusterCatalog{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "unavailable-high"},
					Spec: ocv1.ClusterCatalogSpec{
						Priority:         200,
						AvailabilityMode: "Unavailable",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "available-medium"},
					Spec: ocv1.ClusterCatalogSpec{
						Priority:         100,
						AvailabilityMode: "Available",
					},
					Status: ocv1.ClusterCatalogStatus{
						Conditions: []metav1.Condition{{Type: "Serving", Status: "True"}},
					},
				},
			},
			desiredPackage: "multicluster-engine",
			wantName:       "available-medium",
			wantErr:        false,
		},
		{
			name: "Skip non-serving catalogs - prefer lower priority serving catalog",
			catalogs: []ocv1.ClusterCatalog{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "high-but-not-serving"},
					Spec: ocv1.ClusterCatalogSpec{
						Priority:         200,
						AvailabilityMode: "Available",
					},
					Status: ocv1.ClusterCatalogStatus{
						Conditions: []metav1.Condition{{Type: "Serving", Status: "False"}},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "medium-and-serving"},
					Spec: ocv1.ClusterCatalogSpec{
						Priority:         100,
						AvailabilityMode: "Available",
					},
					Status: ocv1.ClusterCatalogStatus{
						Conditions: []metav1.Condition{{Type: "Serving", Status: "True"}},
					},
				},
			},
			desiredPackage: "multicluster-engine",
			wantName:       "medium-and-serving",
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build runtime scheme
			scheme := runtime.NewScheme()
			_ = ocv1.AddToScheme(scheme)

			// Create fake client with test catalogs
			objs := make([]runtime.Object, len(tt.catalogs))
			for i := range tt.catalogs {
				objs[i] = &tt.catalogs[i]
			}
			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(objs...).
				Build()

			// Run test
			gotName, err := GetClusterCatalog(context.TODO(), client, tt.desiredPackage)

			// Verify error expectations
			if (err != nil) != tt.wantErr {
				t.Errorf("GetClusterCatalog() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetClusterCatalog() expected error containing %q, got nil", tt.errContains)
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("GetClusterCatalog() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			// Verify catalog name
			if gotName != tt.wantName {
				t.Errorf("GetClusterCatalog() = %v, want %v", gotName, tt.wantName)
			}
		})
	}
}

// contains checks if s contains substr
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
