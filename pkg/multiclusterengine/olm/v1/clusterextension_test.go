// Copyright Contributors to the Open Cluster Management project

package v1

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	ocv1 "github.com/operator-framework/operator-controller/api/v1"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengine"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengineutils"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_NewClusterExtension(t *testing.T) {
	mch := &operatorv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-mch",
			Namespace: "open-cluster-management",
		},
	}

	ce := NewClusterExtension(mch)

	// Verify basic structure
	if ce.Name != multiclusterengine.MCEDefaultName {
		t.Errorf("NewClusterExtension() Name = %v, want %v", ce.Name, multiclusterengine.MCEDefaultName)
	}

	// Verify labels
	expectedLabels := map[string]string{
		"installer.name":                          "test-mch",
		"installer.namespace":                     "open-cluster-management",
		multiclusterengineutils.MCEManagedByLabel: "true",
	}
	if !reflect.DeepEqual(ce.Labels, expectedLabels) {
		t.Errorf("NewClusterExtension() Labels = %v, want %v", ce.Labels, expectedLabels)
	}

	// Verify spec
	if ce.Spec.Namespace != multiclusterengine.OperandNamespace() {
		t.Errorf("NewClusterExtension() Namespace = %v, want %v", ce.Spec.Namespace, multiclusterengine.OperandNamespace())
	}

	if ce.Spec.ServiceAccount.Name != MCEInstallerServiceAccountName {
		t.Errorf("NewClusterExtension() ServiceAccount = %v, want %v", ce.Spec.ServiceAccount.Name, MCEInstallerServiceAccountName)
	}

	if ce.Spec.Source.SourceType != "Catalog" {
		t.Errorf("NewClusterExtension() SourceType = %v, want Catalog", ce.Spec.Source.SourceType)
	}

	if ce.Spec.Source.Catalog == nil {
		t.Fatal("NewClusterExtension() Catalog is nil")
	}

	if ce.Spec.Source.Catalog.PackageName != multiclusterengine.DesiredPackage() {
		t.Errorf("NewClusterExtension() PackageName = %v, want %v", ce.Spec.Source.Catalog.PackageName, multiclusterengine.DesiredPackage())
	}

	expectedChannels := []string{multiclusterengine.DesiredChannel()}
	if !reflect.DeepEqual(ce.Spec.Source.Catalog.Channels, expectedChannels) {
		t.Errorf("NewClusterExtension() Channels = %v, want %v", ce.Spec.Source.Catalog.Channels, expectedChannels)
	}
}

func Test_RenderClusterExtension(t *testing.T) {
	existing := &ocv1.ClusterExtension{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ce",
		},
		Spec: ocv1.ClusterExtensionSpec{
			Namespace: "multicluster-engine",
			ServiceAccount: ocv1.ServiceAccountReference{
				Name: MCEInstallerServiceAccountName,
			},
			Source: ocv1.SourceConfig{
				SourceType: "Catalog",
				Catalog: &ocv1.CatalogFilter{
					PackageName: "multicluster-engine",
					Channels:    []string{"old-channel"},
				},
			},
		},
	}

	mch := &operatorv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-mch",
			Namespace: "open-cluster-management",
		},
	}

	rendered := RenderClusterExtension(existing, mch)

	// Verify channels were updated
	expectedChannels := []string{multiclusterengine.DesiredChannel()}
	if !reflect.DeepEqual(rendered.Spec.Source.Catalog.Channels, expectedChannels) {
		t.Errorf("RenderClusterExtension() Channels = %v, want %v", rendered.Spec.Source.Catalog.Channels, expectedChannels)
	}

	// Verify immutable fields were not changed
	if rendered.Spec.Namespace != existing.Spec.Namespace {
		t.Errorf("RenderClusterExtension() changed immutable Namespace field")
	}

	if rendered.Spec.ServiceAccount.Name != existing.Spec.ServiceAccount.Name {
		t.Errorf("RenderClusterExtension() changed immutable ServiceAccount field")
	}
}

func Test_RenderClusterExtension_ClearsVersionOnChannelChange(t *testing.T) {
	tests := []struct {
		name            string
		existingChannel string
		existingVersion string
		wantVersion     string
	}{
		{
			name:            "Channel change clears version",
			existingChannel: "stable-2.6",
			existingVersion: "2.6.0",
			wantVersion:     "",
		},
		{
			name:            "Same channel preserves version",
			existingChannel: multiclusterengine.DesiredChannel(),
			existingVersion: "2.7.0",
			wantVersion:     "2.7.0",
		},
		{
			name:            "Channel change clears empty version",
			existingChannel: "stable-2.6",
			existingVersion: "",
			wantVersion:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existing := &ocv1.ClusterExtension{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-ce",
				},
				Spec: ocv1.ClusterExtensionSpec{
					Namespace: "multicluster-engine",
					ServiceAccount: ocv1.ServiceAccountReference{
						Name: MCEInstallerServiceAccountName,
					},
					Source: ocv1.SourceConfig{
						SourceType: "Catalog",
						Catalog: &ocv1.CatalogFilter{
							PackageName: "multicluster-engine",
							Channels:    []string{tt.existingChannel},
							Version:     tt.existingVersion,
						},
					},
				},
			}

			mch := &operatorv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: "open-cluster-management",
				},
			}

			rendered := RenderClusterExtension(existing, mch)

			if rendered.Spec.Source.Catalog.Version != tt.wantVersion {
				t.Errorf("RenderClusterExtension() Version = %v, want %v", rendered.Spec.Source.Catalog.Version, tt.wantVersion)
			}
		})
	}
}

func Test_channelsEqual(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want bool
	}{
		{
			name: "Equal single channel",
			a:    []string{"stable-2.6"},
			b:    []string{"stable-2.6"},
			want: true,
		},
		{
			name: "Equal multiple channels",
			a:    []string{"stable-2.6", "fast"},
			b:    []string{"stable-2.6", "fast"},
			want: true,
		},
		{
			name: "Different channels",
			a:    []string{"stable-2.6"},
			b:    []string{"stable-2.7"},
			want: false,
		},
		{
			name: "Different lengths",
			a:    []string{"stable-2.6"},
			b:    []string{"stable-2.6", "fast"},
			want: false,
		},
		{
			name: "Empty slices",
			a:    []string{},
			b:    []string{},
			want: true,
		},
		{
			name: "Nil vs empty",
			a:    nil,
			b:    []string{},
			want: false,
		},
		{
			name: "Both nil",
			a:    nil,
			b:    nil,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := channelsEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("channelsEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_GetManagedMCEClusterExtension(t *testing.T) {
	tests := []struct {
		name        string
		extensions  []ocv1.ClusterExtension
		wantName    string
		wantNil     bool
		wantErr     bool
		errContains string
	}{
		{
			name:       "No ClusterExtensions",
			extensions: []ocv1.ClusterExtension{},
			wantNil:    true,
			wantErr:    false,
		},
		{
			name: "One managed ClusterExtension",
			extensions: []ocv1.ClusterExtension{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "mce-managed",
						Labels: map[string]string{
							multiclusterengineutils.MCEManagedByLabel: "true",
						},
					},
				},
			},
			wantName: "mce-managed",
			wantErr:  false,
		},
		{
			name: "ClusterExtension without label",
			extensions: []ocv1.ClusterExtension{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "not-managed",
					},
				},
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name: "Multiple managed ClusterExtensions - error",
			extensions: []ocv1.ClusterExtension{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "mce-1",
						Labels: map[string]string{
							multiclusterengineutils.MCEManagedByLabel: "true",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "mce-2",
						Labels: map[string]string{
							multiclusterengineutils.MCEManagedByLabel: "true",
						},
					},
				},
			},
			wantErr:     true,
			errContains: "multiple MCE ClusterExtensions found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = ocv1.AddToScheme(scheme)

			objs := make([]runtime.Object, len(tt.extensions))
			for i := range tt.extensions {
				objs[i] = &tt.extensions[i]
			}

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(objs...).
				Build()

			got, err := GetManagedMCEClusterExtension(context.TODO(), client)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetManagedMCEClusterExtension() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("GetManagedMCEClusterExtension() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			if tt.wantNil {
				if got != nil {
					t.Errorf("GetManagedMCEClusterExtension() = %v, want nil", got.Name)
				}
				return
			}

			if got == nil {
				t.Fatal("GetManagedMCEClusterExtension() returned nil, want non-nil")
			}

			if got.Name != tt.wantName {
				t.Errorf("GetManagedMCEClusterExtension() Name = %v, want %v", got.Name, tt.wantName)
			}
		})
	}
}

func Test_CreatedByMCH(t *testing.T) {
	mch := &operatorv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-mch",
			Namespace: "test-ns",
		},
	}

	tests := []struct {
		name string
		ce   *ocv1.ClusterExtension
		want bool
	}{
		{
			name: "Nil ClusterExtension",
			ce:   nil,
			want: false,
		},
		{
			name: "ClusterExtension with matching labels",
			ce: &ocv1.ClusterExtension{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"installer.name":      "test-mch",
						"installer.namespace": "test-ns",
					},
				},
			},
			want: true,
		},
		{
			name: "ClusterExtension with no labels",
			ce: &ocv1.ClusterExtension{
				ObjectMeta: metav1.ObjectMeta{},
			},
			want: false,
		},
		{
			name: "ClusterExtension with mismatched name",
			ce: &ocv1.ClusterExtension{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"installer.name":      "different-mch",
						"installer.namespace": "test-ns",
					},
				},
			},
			want: false,
		},
		{
			name: "ClusterExtension with mismatched namespace",
			ce: &ocv1.ClusterExtension{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"installer.name":      "test-mch",
						"installer.namespace": "different-ns",
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CreatedByMCH(tt.ce, mch); got != tt.want {
				t.Errorf("CreatedByMCH() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ServiceAccount(t *testing.T) {
	namespace := "test-namespace"
	sa := ServiceAccount(namespace)

	if sa.Name != MCEInstallerServiceAccountName {
		t.Errorf("ServiceAccount() Name = %v, want %v", sa.Name, MCEInstallerServiceAccountName)
	}

	if sa.Namespace != namespace {
		t.Errorf("ServiceAccount() Namespace = %v, want %v", sa.Namespace, namespace)
	}

	if sa.Kind != "ServiceAccount" {
		t.Errorf("ServiceAccount() Kind = %v, want ServiceAccount", sa.Kind)
	}

	if sa.APIVersion != "v1" {
		t.Errorf("ServiceAccount() APIVersion = %v, want v1", sa.APIVersion)
	}
}

func Test_GetAnnotationOverrides(t *testing.T) {
	tests := []struct {
		name        string
		mch         *operatorv1.MultiClusterHub
		want        *ClusterExtensionOverrides
		wantErr     bool
		errContains string
	}{
		{
			name: "No annotation",
			mch: &operatorv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "Valid annotation with channels",
			mch: &operatorv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"installer.open-cluster-management.io/mce-clusterextension-spec": `{"channels":["stable-2.6"],"version":"2.6.0"}`,
					},
				},
			},
			want: &ClusterExtensionOverrides{
				Channels: []string{"stable-2.6"},
				Version:  "2.6.0",
			},
			wantErr: false,
		},
		{
			name: "Valid annotation with CRD safety enforcement",
			mch: &operatorv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"installer.open-cluster-management.io/mce-clusterextension-spec": `{"crdUpgradeSafetyEnforcement":"Strict"}`,
					},
				},
			},
			want: &ClusterExtensionOverrides{
				CRDUpgradeSafetyEnforcement: "Strict",
			},
			wantErr: false,
		},
		{
			name: "Invalid JSON annotation",
			mch: &operatorv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"installer.open-cluster-management.io/mce-clusterextension-spec": `{invalid json}`,
					},
				},
			},
			wantErr:     true,
			errContains: "failed to unmarshal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetAnnotationOverrides(tt.mch)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetAnnotationOverrides() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("GetAnnotationOverrides() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetAnnotationOverrides() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ApplyAnnotationOverrides(t *testing.T) {
	tests := []struct {
		name      string
		ce        *ocv1.ClusterExtension
		overrides *ClusterExtensionOverrides
		verify    func(*testing.T, *ocv1.ClusterExtension)
	}{
		{
			name: "Nil overrides - no changes",
			ce: &ocv1.ClusterExtension{
				Spec: ocv1.ClusterExtensionSpec{
					Source: ocv1.SourceConfig{
						Catalog: &ocv1.CatalogFilter{
							Channels: []string{"original"},
						},
					},
				},
			},
			overrides: nil,
			verify: func(t *testing.T, ce *ocv1.ClusterExtension) {
				if len(ce.Spec.Source.Catalog.Channels) != 1 || ce.Spec.Source.Catalog.Channels[0] != "original" {
					t.Error("Channels should not be modified when overrides is nil")
				}
			},
		},
		{
			name: "Apply channel override",
			ce: &ocv1.ClusterExtension{
				Spec: ocv1.ClusterExtensionSpec{
					Source: ocv1.SourceConfig{
						Catalog: &ocv1.CatalogFilter{
							Channels: []string{"original"},
						},
					},
				},
			},
			overrides: &ClusterExtensionOverrides{
				Channels: []string{"stable-2.6", "fast"},
			},
			verify: func(t *testing.T, ce *ocv1.ClusterExtension) {
				expected := []string{"stable-2.6", "fast"}
				if !reflect.DeepEqual(ce.Spec.Source.Catalog.Channels, expected) {
					t.Errorf("Channels = %v, want %v", ce.Spec.Source.Catalog.Channels, expected)
				}
			},
		},
		{
			name: "Apply version override",
			ce: &ocv1.ClusterExtension{
				Spec: ocv1.ClusterExtensionSpec{
					Source: ocv1.SourceConfig{
						Catalog: &ocv1.CatalogFilter{},
					},
				},
			},
			overrides: &ClusterExtensionOverrides{
				Version: "2.6.0",
			},
			verify: func(t *testing.T, ce *ocv1.ClusterExtension) {
				if ce.Spec.Source.Catalog.Version != "2.6.0" {
					t.Errorf("Version = %v, want 2.6.0", ce.Spec.Source.Catalog.Version)
				}
			},
		},
		{
			name: "Apply CRD safety enforcement",
			ce: &ocv1.ClusterExtension{
				Spec: ocv1.ClusterExtensionSpec{
					Source: ocv1.SourceConfig{
						Catalog: &ocv1.CatalogFilter{},
					},
				},
			},
			overrides: &ClusterExtensionOverrides{
				CRDUpgradeSafetyEnforcement: "Strict",
			},
			verify: func(t *testing.T, ce *ocv1.ClusterExtension) {
				if ce.Spec.Install == nil || ce.Spec.Install.Preflight == nil || ce.Spec.Install.Preflight.CRDUpgradeSafety == nil {
					t.Fatal("CRDUpgradeSafety config not created")
				}
				if string(ce.Spec.Install.Preflight.CRDUpgradeSafety.Enforcement) != "Strict" {
					t.Errorf("CRDUpgradeSafetyEnforcement = %v, want Strict", ce.Spec.Install.Preflight.CRDUpgradeSafety.Enforcement)
				}
			},
		},
		{
			name: "Apply config override",
			ce: &ocv1.ClusterExtension{
				Spec: ocv1.ClusterExtensionSpec{
					Source: ocv1.SourceConfig{
						Catalog: &ocv1.CatalogFilter{},
					},
				},
			},
			overrides: &ClusterExtensionOverrides{
				Config: &ClusterExtensionConfigOverride{
					Inline: &apiextensionsv1.JSON{
						Raw: []byte(`{"key":"value"}`),
					},
				},
			},
			verify: func(t *testing.T, ce *ocv1.ClusterExtension) {
				if ce.Spec.Config == nil || ce.Spec.Config.Inline == nil {
					t.Fatal("Config not created")
				}
				var result map[string]string
				if err := json.Unmarshal(ce.Spec.Config.Inline.Raw, &result); err != nil {
					t.Fatalf("Failed to unmarshal config: %v", err)
				}
				if result["key"] != "value" {
					t.Errorf("Config inline = %v, want {key:value}", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ApplyAnnotationOverrides(tt.ce, tt.overrides)
			tt.verify(t, tt.ce)
		})
	}
}
