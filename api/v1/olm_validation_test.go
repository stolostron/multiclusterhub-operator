// Copyright Contributors to the Open Cluster Management project

package v1

import (
	"context"
	"os"
	"testing"

	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestValidateOLMAnnotations(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		olmVersion  string
		setupEnv    func()
		cleanupEnv  func()
		wantErr     bool
		errContains string
	}{
		{
			name:        "No annotations - valid",
			annotations: nil,
			olmVersion:  "v0",
			wantErr:     false,
		},
		{
			name:        "Empty annotations - valid",
			annotations: map[string]string{},
			olmVersion:  "v1",
			wantErr:     false,
		},
		{
			name: "V0 annotation on v0 cluster - valid",
			annotations: map[string]string{
				annotationMCESubscriptionSpec: `{"channel": "stable-2.6"}`,
			},
			olmVersion: "v0",
			setupEnv: func() {
				os.Setenv("OPERATOR_CONDITION_NAME", "multiclusterhub-operator")
			},
			cleanupEnv: func() {
				os.Unsetenv("OPERATOR_CONDITION_NAME")
			},
			wantErr: false,
		},
		{
			name: "V0 annotation on v1 cluster - invalid",
			annotations: map[string]string{
				annotationMCESubscriptionSpec: `{"channel": "stable-2.6"}`,
			},
			olmVersion:  "v1",
			wantErr:     true,
			errContains: "only valid for OLM v0 clusters",
		},
		{
			name: "V1 annotation on v0 cluster - invalid",
			annotations: map[string]string{
				annotationMCEClusterExtensionSpec: `{"channels": ["stable-2.6"]}`,
			},
			olmVersion: "v0",
			setupEnv: func() {
				os.Setenv("OPERATOR_CONDITION_NAME", "multiclusterhub-operator")
			},
			cleanupEnv: func() {
				os.Unsetenv("OPERATOR_CONDITION_NAME")
			},
			wantErr:     true,
			errContains: "only valid for OLM v1 clusters",
		},
		{
			name: "V1 annotation when no OLM - invalid",
			annotations: map[string]string{
				annotationMCEClusterExtensionSpec: `{"channels": ["stable-2.6"]}`,
			},
			olmVersion:  "",
			wantErr:     true,
			errContains: "requires OLM v1, but no OLM detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment if needed
			if tt.setupEnv != nil {
				tt.setupEnv()
			}
			if tt.cleanupEnv != nil {
				defer tt.cleanupEnv()
			}

			// Create MCH with test annotations
			mch := &MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-mch",
					Namespace:   "default",
					Annotations: tt.annotations,
				},
			}

			// Setup fake client for OLM v1 detection
			scheme := runtime.NewScheme()
			_ = apixv1.AddToScheme(scheme)

			objects := []runtime.Object{}
			if tt.olmVersion == "v1" {
				// Add ClusterExtension CRD for OLM v1 detection
				objects = append(objects, &apixv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "clusterextensions.olm.operatorframework.io",
					},
				})
			}

			Client = fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(objects...).
				Build()

			// Test validation
			ctx := context.Background()
			err := validateOLMAnnotations(ctx, mch)

			if tt.wantErr {
				if err == nil {
					t.Errorf("validateOLMAnnotations() expected error but got none")
					return
				}
				if tt.errContains != "" {
					errMsg := err.Error()
					found := false
					for i := 0; i <= len(errMsg)-len(tt.errContains); i++ {
						if errMsg[i:i+len(tt.errContains)] == tt.errContains {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("validateOLMAnnotations() error = %v, want error containing %q", err, tt.errContains)
					}
				}
			} else {
				if err != nil {
					t.Errorf("validateOLMAnnotations() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateMCEChannelAnnotation(t *testing.T) {
	tests := []struct {
		name           string
		annotations    map[string]string
		desiredChannel func() string
		wantErr        bool
		errContains    string
	}{
		{
			name:           "No annotations - valid",
			annotations:    nil,
			desiredChannel: func() string { return "stable-5.1" },
			wantErr:        false,
		},
		{
			name:           "Annotation present but no channels field - valid",
			annotations:    map[string]string{annotationMCEClusterExtensionSpec: `{"version":"5.1.0"}`},
			desiredChannel: func() string { return "stable-5.1" },
			wantErr:        false,
		},
		{
			name:           "Annotation channel matches desired channel - valid",
			annotations:    map[string]string{annotationMCEClusterExtensionSpec: `{"channels":["stable-5.1"]}`},
			desiredChannel: func() string { return "stable-5.1" },
			wantErr:        false,
		},
		{
			name:           "Annotation channel conflicts with desired channel - invalid",
			annotations:    map[string]string{annotationMCEClusterExtensionSpec: `{"channels":["stable-5.0"]}`},
			desiredChannel: func() string { return "stable-5.1" },
			wantErr:        true,
			errContains:    `channel(s) [stable-5.0], which conflicts with channel "stable-5.1"`,
		},
		{
			name: "Multiple annotation channels, one matches - valid",
			annotations: map[string]string{
				annotationMCEClusterExtensionSpec: `{"channels":["stable-5.0","stable-5.1"]}`,
			},
			desiredChannel: func() string { return "stable-5.1" },
			wantErr:        false,
		},
		{
			name: "Multiple annotation channels, none match - invalid",
			annotations: map[string]string{
				annotationMCEClusterExtensionSpec: `{"channels":["stable-5.0","stable-5.2"]}`,
			},
			desiredChannel: func() string { return "stable-5.1" },
			wantErr:        true,
			errContains:    "conflicts with channel",
		},
		{
			name:           "Malformed JSON annotation - skipped, not blocked here",
			annotations:    map[string]string{annotationMCEClusterExtensionSpec: `{not valid json`},
			desiredChannel: func() string { return "stable-5.1" },
			wantErr:        false,
		},
		{
			name:           "DesiredMCEChannelFunc unset - skip validation",
			annotations:    map[string]string{annotationMCEClusterExtensionSpec: `{"channels":["stable-5.0"]}`},
			desiredChannel: nil,
			wantErr:        false,
		},
	}

	origFunc := DesiredMCEChannelFunc
	defer func() { DesiredMCEChannelFunc = origFunc }()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			DesiredMCEChannelFunc = tt.desiredChannel

			mch := &MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-mch",
					Namespace:   "default",
					Annotations: tt.annotations,
				},
			}

			err := validateMCEChannelAnnotation(mch)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("validateMCEChannelAnnotation() expected error but got none")
				}
				if tt.errContains != "" {
					errMsg := err.Error()
					found := false
					for i := 0; i <= len(errMsg)-len(tt.errContains); i++ {
						if errMsg[i:i+len(tt.errContains)] == tt.errContains {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("validateMCEChannelAnnotation() error = %v, want error containing %q", err, tt.errContains)
					}
				}
			} else if err != nil {
				t.Errorf("validateMCEChannelAnnotation() unexpected error: %v", err)
			}
		})
	}
}

func TestDetectOLMVersion(t *testing.T) {
	tests := []struct {
		name       string
		setupEnv   func()
		cleanupEnv func()
		hasCRD     bool
		want       string
	}{
		{
			name: "OLM v0 via env var",
			setupEnv: func() {
				os.Setenv("OPERATOR_CONDITION_NAME", "multiclusterhub-operator")
			},
			cleanupEnv: func() {
				os.Unsetenv("OPERATOR_CONDITION_NAME")
			},
			want: "v0",
		},
		{
			name:   "OLM v1 via CRD",
			hasCRD: true,
			want:   "v1",
		},
		{
			name: "No OLM detected",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupEnv != nil {
				tt.setupEnv()
			}
			if tt.cleanupEnv != nil {
				defer tt.cleanupEnv()
			}

			// Setup fake client
			scheme := runtime.NewScheme()
			_ = apixv1.AddToScheme(scheme)

			objects := []runtime.Object{}
			if tt.hasCRD {
				objects = append(objects, &apixv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "clusterextensions.olm.operatorframework.io",
					},
				})
			}

			Client = fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(objects...).
				Build()

			got, err := detectOLMVersion(context.Background())
			if err != nil {
				t.Errorf("detectOLMVersion() unexpected error: %v", err)
				return
			}

			if got != tt.want {
				t.Errorf("detectOLMVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}
