package multiclusterengine

import (
	"context"
	"os"
	"testing"

	"github.com/onsi/gomega"
	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	olmapi "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"
	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengineutils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var mockClient = fake.NewClientBuilder().Build()

func registerScheme() {
	olmapi.AddToScheme(scheme.Scheme)
	subv1alpha1.AddToScheme(scheme.Scheme)
}

func TestDesiredPackage(t *testing.T) {
	os.Setenv("OPERATOR_PACKAGE", "advanced-cluster-management")
	if got := DesiredPackage(); got != MCEProdPackageName {
		t.Errorf("DesiredPackage() = %v, want %v", got, MCEProdPackageName)
	}
	os.Unsetenv("OPERATOR_PACKAGE")
	if got := DesiredPackage(); got != MCECommunityPackageName {
		t.Errorf("DesiredPackage() = %v, want %v", got, MCECommunityPackageName)
	}
}

func TestOperandNamespace(t *testing.T) {
	os.Setenv("OPERATOR_PACKAGE", "advanced-cluster-management")
	if got := OperandNamespace(); got != MCEProdOperandNamespace {
		t.Errorf("OperandNamespace() = %v, want %v", got, MCEProdOperandNamespace)
	}
	os.Unsetenv("OPERATOR_PACKAGE")
	if got := OperandNamespace(); got != MCECommunityOperandNamespace {
		t.Errorf("OperandNamespace() = %v, want %v", got, MCECommunityOperandNamespace)
	}
}

func TestNameSpace(t *testing.T) {
	os.Setenv("OPERATOR_PACKAGE", "advanced-cluster-management")
	if got := Namespace().Name; got != MCEProdOperandNamespace {
		t.Errorf("OperandNamespace() = %v, want %v", got, MCEProdOperandNamespace)
	}
	os.Unsetenv("OPERATOR_PACKAGE")
	if got := Namespace().Name; got != MCECommunityOperandNamespace {
		t.Errorf("OperandNamespace() = %v, want %v", got, MCECommunityOperandNamespace)
	}
}

// TestOperatorGroup - TODO: move to v0 package tests
func TestOperatorGroup(t *testing.T) {
	t.Skip("v0-specific test - needs to be in olm/v0 package")
}
func TestFindAndManageMCE(t *testing.T) {

	managedmce1 := &mcev1.MultiClusterEngine{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mce-sub",
			Labels: map[string]string{
				multiclusterengineutils.MCEManagedByLabel: "true",
			},
		},
	}
	managedmce2 := &mcev1.MultiClusterEngine{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mce-sub2",
			Labels: map[string]string{
				multiclusterengineutils.MCEManagedByLabel: "true",
			},
		},
	}
	unmanagedmce1 := &mcev1.MultiClusterEngine{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mce-unsub",
		},
	}

	scheme := runtime.NewScheme()
	err := mcev1.AddToScheme(scheme)
	if err != nil {
		t.Fatalf("Couldn't set up scheme")
	}

	// One good mce
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithLists(&mcev1.MultiClusterEngineList{Items: []mcev1.MultiClusterEngine{*managedmce1}}).
		Build()

	got, err := FindAndManageMCE(context.Background(), cl)
	if err != nil {
		t.Errorf("FindAndManageMCE() should have found mce by label. Got %v", err)
	}
	if got.Name != managedmce1.Name {
		t.Errorf("FindAndManageMCE() return mce %s, want %s", got.Name, managedmce1.Name)
	}

	// Conflicting mces
	cl = fake.NewClientBuilder().
		WithScheme(scheme).
		WithLists(&mcev1.MultiClusterEngineList{Items: []mcev1.MultiClusterEngine{*managedmce1, *managedmce2}}).
		Build()

	_, err = FindAndManageMCE(context.Background(), cl)
	if err == nil {
		t.Errorf("FindAndManageMCE() should have errored due to multiple mces")
	}

	// Eligible mce without label
	cl = fake.NewClientBuilder().
		WithScheme(scheme).
		WithLists(&mcev1.MultiClusterEngineList{Items: []mcev1.MultiClusterEngine{*unmanagedmce1}}).
		Build()

	got, err = FindAndManageMCE(context.Background(), cl)
	if err != nil {
		t.Errorf("FindAndManageMCE() should have found mce and labeled it. Got error %v", err)
	}
	if got.Name != unmanagedmce1.Name {
		t.Errorf("FindAndManageMCE() return mce %s, want %s", got.Name, managedmce1.Name)
	}
	if got.Labels[multiclusterengineutils.MCEManagedByLabel] != "true" {
		t.Errorf("FindAndManageMCE() should have set the managed label on the mce")
	}
	gotMCE := &mcev1.MultiClusterEngine{}
	key := types.NamespacedName{Name: unmanagedmce1.Name}
	err = cl.Get(context.Background(), key, gotMCE)
	if err != nil {
		t.Errorf("Got error from mock client %v", err)
	}
	if gotMCE.Labels[multiclusterengineutils.MCEManagedByLabel] != "true" {
		t.Errorf("FindAndManageMCE() should have updated the managed label on the mce")
	}

}

func TestMCECreatedByMCH(t *testing.T) {
	mch := &operatorv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mch",
			Namespace: "mch-ns",
		},
	}
	tests := []struct {
		name string
		mce  *mcev1.MultiClusterEngine
		m    *operatorv1.MultiClusterHub
		want bool
	}{
		{
			name: "Created by MCH",
			mce: &mcev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"installer.name":      "mch",
						"installer.namespace": "mch-ns",
					},
				},
			},
			m:    mch,
			want: true,
		},
		{
			name: "Adopted by MCH",
			mce: &mcev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						multiclusterengineutils.MCEManagedByLabel: "true",
					},
				},
			},
			m:    mch,
			want: false,
		},
		{
			name: "Unlabeled",
			mce: &mcev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{},
			},
			m:    mch,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MCECreatedByMCH(tt.mce, tt.m); got != tt.want {
				t.Errorf("CreatedByMCH() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewMultiClusterEngine(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	type args struct {
		m                             *operatorv1.MultiClusterHub
		infrastructureCustomNamespace string
	}
	tests := []struct {
		name string
		args args
		want *mcev1.MultiClusterEngine
	}{
		{
			name: "Basic",
			args: args{
				m: &operatorv1.MultiClusterHub{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mch",
						Namespace: "mch-ns",
					},
				},
				infrastructureCustomNamespace: "",
			},
			want: &mcev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Name: MCEDefaultName,
					Labels: map[string]string{
						"installer.name":                          "mch",
						"installer.namespace":                     "mch-ns",
						multiclusterengineutils.MCEManagedByLabel: "true",
					},
				},
				Spec: mcev1.MultiClusterEngineSpec{
					ImagePullSecret: "",
					Tolerations: []corev1.Toleration{
						{
							Effect:   "NoSchedule",
							Key:      "node-role.kubernetes.io/infra",
							Operator: "Exists",
						},
					},
					NodeSelector:       nil,
					AvailabilityConfig: mcev1.HAHigh,
					TargetNamespace:    OperandNamespace(),
					Overrides: &mcev1.Overrides{
						Components: []mcev1.ComponentConfig{
							{Name: operatorv1.MCELocalCluster, Enabled: true},
						},
					},
				},
			},
		},
		{
			name: "Several configurations",
			args: args{
				m: &operatorv1.MultiClusterHub{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mch",
						Namespace: "mch-ns",
					},
					Spec: operatorv1.MultiClusterHubSpec{
						AvailabilityConfig: operatorv1.HABasic,
						NodeSelector: map[string]string{
							"select": "this",
						},
						Tolerations: []corev1.Toleration{
							{
								Key:    "tolerate",
								Value:  "this",
								Effect: "now",
							},
						},
						DisableHubSelfManagement: true,
						Overrides: &operatorv1.Overrides{
							ImagePullPolicy: corev1.PullNever,
							Components: []operatorv1.ComponentConfig{
								{Name: operatorv1.MCEDiscovery, Enabled: false},
							},
						},
					},
				},
			},
			want: &mcev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Name: MCEDefaultName,
					Labels: map[string]string{
						"installer.name":                          "mch",
						"installer.namespace":                     "mch-ns",
						multiclusterengineutils.MCEManagedByLabel: "true",
					},
				},
				Spec: mcev1.MultiClusterEngineSpec{
					ImagePullSecret: "",
					Tolerations: []corev1.Toleration{
						{
							Key:    "tolerate",
							Value:  "this",
							Effect: "now",
						},
					},
					NodeSelector: map[string]string{
						"select": "this",
					},
					AvailabilityConfig: mcev1.HABasic,
					TargetNamespace:    OperandNamespace(),
					Overrides: &mcev1.Overrides{
						ImagePullPolicy: corev1.PullNever,
						Components: []mcev1.ComponentConfig{
							{Name: operatorv1.MCEDiscovery, Enabled: false},
							{Name: operatorv1.MCELocalCluster, Enabled: false},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewMultiClusterEngine(tt.args.m, OperandNamespace())
			g.Expect(got.Name).To(gomega.Equal(tt.want.Name))
			g.Expect(got.Labels).To(gomega.Equal(tt.want.Labels))
			g.Expect(got.Spec.ImagePullSecret).To(gomega.Equal(tt.want.Spec.ImagePullSecret))
			g.Expect(got.Spec.Tolerations).To(gomega.Equal(tt.want.Spec.Tolerations))
			g.Expect(got.Spec.NodeSelector).To(gomega.Equal(tt.want.Spec.NodeSelector))
			g.Expect(got.Spec.AvailabilityConfig).To(gomega.Equal(tt.want.Spec.AvailabilityConfig))
			g.Expect(got.Spec.TargetNamespace).To(gomega.Equal(tt.want.Spec.TargetNamespace))
			g.Expect(got.Spec.Overrides.Components).To(gomega.Equal(tt.want.Spec.Overrides.Components))
			g.Expect(got.Spec.Overrides.ImagePullPolicy).To(gomega.Equal(tt.want.Spec.Overrides.ImagePullPolicy))
		})
	}
}

func TestRenderMultiClusterEngine(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	existingMCE := &mcev1.MultiClusterEngine{
		ObjectMeta: metav1.ObjectMeta{
			Name: "randomName",
			Labels: map[string]string{
				"random": "label",
				multiclusterengineutils.MCEManagedByLabel: "true",
			},
			Annotations: map[string]string{
				"random": "annotation",
			},
		},
		Spec: mcev1.MultiClusterEngineSpec{
			ImagePullSecret: "",
			Tolerations: []corev1.Toleration{
				{
					Key:    "tolerate",
					Value:  "this",
					Effect: "now",
				},
			},
			NodeSelector: map[string]string{
				"select": "this",
			},
			AvailabilityConfig: mcev1.HABasic,
			TargetNamespace:    "random",
			Overrides: &mcev1.Overrides{
				ImagePullPolicy: corev1.PullNever,
				Components: []mcev1.ComponentConfig{
					{Name: operatorv1.MCEDiscovery, Enabled: false},
					{Name: operatorv1.MCELocalCluster, Enabled: false},
				},
				InfrastructureCustomNamespace: "open-cluster-management",
			},
		},
	}

	mch := &operatorv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mch",
			Namespace: "mch-ns",
			Annotations: map[string]string{
				"mch-imageRepository": "quay.io",
			},
		},
	}

	got := RenderMultiClusterEngine(existingMCE, mch)

	t.Run("Preserve some fields", func(t *testing.T) {
		g.Expect(got.Name).To(gomega.Equal(existingMCE.Name), "Name should be kept")
		g.Expect(got.Labels["random"]).To(gomega.Equal(existingMCE.Labels["random"]), "Labels should not be erased")
		g.Expect(got.Annotations["random"]).To(gomega.Equal(existingMCE.Annotations["random"]), "Annotations should not be erased")
		g.Expect(got.Spec.Overrides.InfrastructureCustomNamespace).To(gomega.Equal(existingMCE.Spec.Overrides.InfrastructureCustomNamespace), "Infra namespace should not change")
		g.Expect(got.Spec.TargetNamespace).To(gomega.Equal(existingMCE.Spec.TargetNamespace), "Target namespace should not change")
	})

	t.Run("Overwrite some fields", func(t *testing.T) {
		g.Expect(got.Annotations["imageRepository"]).To(gomega.Equal(mch.Annotations["mch-imageRepository"]), "Override annotations should be updated")
	})

	// Annotation on MCE but not MCH
	existingMCE.Annotations["imageRepository"] = "quay.io"
	mch.SetAnnotations(map[string]string{})
	got = RenderMultiClusterEngine(existingMCE, mch)
	t.Run("Remove override annotation", func(t *testing.T) {
		g.Expect(got.Annotations["random"]).To(gomega.Equal(existingMCE.Annotations["random"]), "Unrelated annotations should not be erased")
		g.Expect(got.Annotations["imageRepository"]).To(gomega.Equal(""), "Override annotation should be be emptied")
	})

}

// Test_filterPackageManifests - TODO: move to v0 package tests
func Test_filterPackageManifests(t *testing.T) {
	t.Skip("v0-specific test - needs to be in olm/v0 package")
}

// Test_GetCatalogSource - TODO: move to v0 package tests
func Test_GetCatalogSource(t *testing.T) {
	t.Skip("v0-specific test - needs to be in olm/v0 package")
}

// Test_extractCatalogSource - TODO: move to v0 package tests
func Test_extractCatalogSource(t *testing.T) {
	t.Skip("v0-specific test - needs to be in olm/v0 package")
}

// Test_findHighestPriorityCatalogSource - TODO: move to v0 package tests
func Test_findHighestPriorityCatalogSource(t *testing.T) {
	t.Skip("v0-specific test - needs to be in olm/v0 package")
}
