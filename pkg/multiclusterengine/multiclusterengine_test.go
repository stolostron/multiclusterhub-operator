package multiclusterengine

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/blang/semver/v4"
	"github.com/onsi/gomega"
	olmversion "github.com/operator-framework/api/pkg/lib/version"
	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	olmapi "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"
	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	mceutils "github.com/stolostron/backplane-operator/pkg/utils"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
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
	if got := DesiredPackage(); got != packageName {
		t.Errorf("DesiredPackage() = %v, want %v", got, packageName)
	}
	os.Unsetenv("OPERATOR_PACKAGE")
	if got := DesiredPackage(); got != communityPackageName {
		t.Errorf("DesiredPackage() = %v, want %v", got, communityPackageName)
	}
}

func TestOperandNamespace(t *testing.T) {
	os.Setenv("OPERATOR_PACKAGE", "advanced-cluster-management")
	if got := OperandNamespace(); got != operandNamespace {
		t.Errorf("OperandNamespace() = %v, want %v", got, operandNamespace)
	}
	os.Unsetenv("OPERATOR_PACKAGE")
	if got := OperandNamespace(); got != communityOperandNamepace {
		t.Errorf("OperandNamespace() = %v, want %v", got, communityOperandNamepace)
	}
}

func TestNameSpace(t *testing.T) {
	os.Setenv("OPERATOR_PACKAGE", "advanced-cluster-management")
	if got := Namespace().Name; got != operandNamespace {
		t.Errorf("OperandNamespace() = %v, want %v", got, operandNamespace)
	}
	os.Unsetenv("OPERATOR_PACKAGE")
	if got := Namespace().Name; got != communityOperandNamepace {
		t.Errorf("OperandNamespace() = %v, want %v", got, communityOperandNamepace)
	}
}

func TestOperatorGroup(t *testing.T) {
	os.Setenv("OPERATOR_PACKAGE", "advanced-cluster-management")
	if got := OperatorGroup().Namespace; got != operandNamespace {
		t.Errorf("OperandNamespace() = %v, want %v", got, operandNamespace)
	}
	if got := OperatorGroup().Spec.TargetNamespaces[0]; got != operandNamespace {
		t.Errorf("OperandNamespace() = %v, want %v", got, operandNamespace)
	}
	os.Unsetenv("OPERATOR_PACKAGE")
	if got := OperatorGroup().Namespace; got != communityOperandNamepace {
		t.Errorf("OperandNamespace() = %v, want %v", got, communityOperandNamepace)
	}
	if got := OperatorGroup().Spec.TargetNamespaces[0]; got != communityOperandNamepace {
		t.Errorf("OperandNamespace() = %v, want %v", got, communityOperandNamepace)
	}
}
func TestFindAndManageMCE(t *testing.T) {

	managedmce1 := &mcev1.MultiClusterEngine{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mce-sub",
			Labels: map[string]string{
				utils.MCEManagedByLabel: "true",
			},
		},
	}
	managedmce2 := &mcev1.MultiClusterEngine{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mce-sub2",
			Labels: map[string]string{
				utils.MCEManagedByLabel: "true",
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
	if got.Labels[utils.MCEManagedByLabel] != "true" {
		t.Errorf("FindAndManageMCE() should have set the managed label on the mce")
	}
	gotMCE := &mcev1.MultiClusterEngine{}
	key := types.NamespacedName{Name: unmanagedmce1.Name}
	err = cl.Get(context.Background(), key, gotMCE)
	if err != nil {
		t.Errorf("Got error from mock client %v", err)
	}
	if gotMCE.Labels[utils.MCEManagedByLabel] != "true" {
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
						utils.MCEManagedByLabel: "true",
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
					Name: MulticlusterengineName,
					Labels: map[string]string{
						"installer.name":        "mch",
						"installer.namespace":   "mch-ns",
						utils.MCEManagedByLabel: "true",
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
					Name: MulticlusterengineName,
					Labels: map[string]string{
						"installer.name":        "mch",
						"installer.namespace":   "mch-ns",
						utils.MCEManagedByLabel: "true",
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
		// TODO: change this back to spec when needed
		{
			name: "Adopt hubSize",
			args: args{
				m: &operatorv1.MultiClusterHub{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "mch",
						Namespace:   "mch-ns",
						Annotations: map[string]string{utils.AnnotationHubSize: string(operatorv1.Large)},
					},
				},
			},
			want: &mcev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Name: MulticlusterengineName,
					Labels: map[string]string{
						"installer.name":        "mch",
						"installer.namespace":   "mch-ns",
						utils.MCEManagedByLabel: "true",
					},
					Annotations: map[string]string{mceutils.AnnotationHubSize: string(mcev1.Large)},
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewMultiClusterEngine(tt.args.m)
			g.Expect(got.Labels).To(gomega.Equal(tt.want.Labels))
			g.Expect(got.Spec.ImagePullSecret).To(gomega.Equal(tt.want.Spec.ImagePullSecret))
			g.Expect(got.Spec.Tolerations).To(gomega.Equal(tt.want.Spec.Tolerations))
			g.Expect(got.Spec.NodeSelector).To(gomega.Equal(tt.want.Spec.NodeSelector))
			g.Expect(got.Spec.AvailabilityConfig).To(gomega.Equal(tt.want.Spec.AvailabilityConfig))
			g.Expect(got.Spec.TargetNamespace).To(gomega.Equal(tt.want.Spec.TargetNamespace))
			g.Expect(got.Spec.Overrides.Components).To(gomega.Equal(tt.want.Spec.Overrides.Components))
			g.Expect(got.Spec.Overrides.ImagePullPolicy).To(gomega.Equal(tt.want.Spec.Overrides.ImagePullPolicy))

			// TODO: put this back later
			// g.Expect(got.Spec.HubSize).To(gomega.Equal(tt.want.Spec.HubSize))
		})
	}
}

func TestRenderMultiClusterEngine(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	existingMCE := &mcev1.MultiClusterEngine{
		ObjectMeta: metav1.ObjectMeta{
			Name: "randomName",
			Labels: map[string]string{
				"random":                "label",
				utils.MCEManagedByLabel: "true",
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

func Test_filterPackageManifests(t *testing.T) {
	type args struct {
		pkgManifests []olmapi.PackageManifest
		channel      string
	}
	tests := []struct {
		name string
		args args
		want []olmapi.PackageManifest
	}{
		{
			name: "No packagemanifests with desired channel",
			args: args{
				pkgManifests: []olmapi.PackageManifest{
					{
						Status: olmapi.PackageManifestStatus{
							CatalogSource: "redhat-operators",
							Channels: []olmapi.PackageChannel{
								{
									Name:       "fast",
									CurrentCSV: "multicluster-engine.v2.0.6",
									CurrentCSVDesc: olmapi.CSVDescription{
										Version: olmversion.OperatorVersion{
											Version: semver.MustParse("2.0.6"),
										},
									},
								},
							},
						},
					},
				},
				channel: "stable",
			},
			want: []olmapi.PackageManifest{},
		},
		{
			name: "Return packagemanifest with more recent version",
			args: args{
				pkgManifests: []olmapi.PackageManifest{
					{
						Status: olmapi.PackageManifestStatus{
							CatalogSource: "redhat-operators",
							Channels: []olmapi.PackageChannel{
								{
									Name:       "stable",
									CurrentCSV: "multicluster-engine.v2.0.6-2",
									CurrentCSVDesc: olmapi.CSVDescription{
										Version: olmversion.OperatorVersion{
											Version: semver.MustParse("2.0.6-2")},
									},
								},
							},
						},
					},
					{
						Status: olmapi.PackageManifestStatus{
							CatalogSource: "custom-operators-1",
							Channels: []olmapi.PackageChannel{
								{
									Name:       "stable",
									CurrentCSV: "multicluster-engine.v2.0.6-5",
									CurrentCSVDesc: olmapi.CSVDescription{
										Version: olmversion.OperatorVersion{
											Version: semver.MustParse("2.0.6-5"),
										},
									},
								},
							},
						},
					},
					{
						Status: olmapi.PackageManifestStatus{
							CatalogSource: "custom-operators-2",
							Channels: []olmapi.PackageChannel{
								{
									Name:       "stable",
									CurrentCSV: "multicluster-engine.v2.0.6-4",
									CurrentCSVDesc: olmapi.CSVDescription{
										Version: olmversion.OperatorVersion{
											Version: semver.MustParse("2.0.6-4"),
										},
									},
								},
							},
						},
					},
				},
				channel: "stable",
			},
			want: []olmapi.PackageManifest{{
				Status: olmapi.PackageManifestStatus{
					CatalogSource: "custom-operators-1",
					Channels: []olmapi.PackageChannel{
						{
							Name:       "stable",
							CurrentCSV: "multicluster-engine.v2.0.6-5",
							CurrentCSVDesc: olmapi.CSVDescription{
								Version: olmversion.OperatorVersion{
									Version: semver.MustParse("2.0.6-5"),
								},
							},
						},
					},
				},
			}},
		},
		{
			name: "Return both packagemanifests if two have the same versions",
			args: args{
				pkgManifests: []olmapi.PackageManifest{
					{
						Status: olmapi.PackageManifestStatus{
							CatalogSource: "redhat-operators",
							Channels: []olmapi.PackageChannel{
								{
									Name:       "stable",
									CurrentCSV: "multicluster-engine.v2.0.6",
									CurrentCSVDesc: olmapi.CSVDescription{
										Version: olmversion.OperatorVersion{
											Version: semver.MustParse("2.0.6"),
										},
									},
								},
							},
						},
					},
					{
						Status: olmapi.PackageManifestStatus{
							CatalogSource: "custom-operators",
							Channels: []olmapi.PackageChannel{
								{
									Name:       "stable",
									CurrentCSV: "multicluster-engine.v2.0.6",
									CurrentCSVDesc: olmapi.CSVDescription{
										Version: olmversion.OperatorVersion{
											Version: semver.MustParse("2.0.6"),
										},
									},
								},
							},
						},
					},
				},
				channel: "stable",
			},
			want: []olmapi.PackageManifest{
				{
					Status: olmapi.PackageManifestStatus{
						CatalogSource: "redhat-operators",
						Channels: []olmapi.PackageChannel{
							{
								Name:       "stable",
								CurrentCSV: "multicluster-engine.v2.0.6",
								CurrentCSVDesc: olmapi.CSVDescription{
									Version: olmversion.OperatorVersion{
										Version: semver.MustParse("2.0.6"),
									},
								},
							},
						},
					},
				},
				{
					Status: olmapi.PackageManifestStatus{
						CatalogSource: "custom-operators",
						Channels: []olmapi.PackageChannel{
							{
								Name:       "stable",
								CurrentCSV: "multicluster-engine.v2.0.6",
								CurrentCSVDesc: olmapi.CSVDescription{
									Version: olmversion.OperatorVersion{
										Version: semver.MustParse("2.0.6"),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Return multiple packagemanifests if they have the same versions",
			args: args{
				pkgManifests: []olmapi.PackageManifest{
					{
						Status: olmapi.PackageManifestStatus{
							CatalogSource: "redhat-operators",
							Channels: []olmapi.PackageChannel{
								{
									Name:       "stable",
									CurrentCSV: "multicluster-engine.v2.0.6",
									CurrentCSVDesc: olmapi.CSVDescription{
										Version: olmversion.OperatorVersion{
											Version: semver.MustParse("2.0.6"),
										},
									},
								},
							},
						},
					},
					{
						Status: olmapi.PackageManifestStatus{
							CatalogSource: "custom-operators-1",
							Channels: []olmapi.PackageChannel{
								{
									Name:       "stable",
									CurrentCSV: "multicluster-engine.v2.0.7",
									CurrentCSVDesc: olmapi.CSVDescription{
										Version: olmversion.OperatorVersion{
											Version: semver.MustParse("2.0.7"),
										},
									},
								},
							},
						},
					},
					{
						Status: olmapi.PackageManifestStatus{
							CatalogSource: "custom-operators-2",
							Channels: []olmapi.PackageChannel{
								{
									Name:       "stable",
									CurrentCSV: "multicluster-engine.v2.0.7",
									CurrentCSVDesc: olmapi.CSVDescription{
										Version: olmversion.OperatorVersion{
											Version: semver.MustParse("2.0.7"),
										},
									},
								},
							},
						},
					},
					{
						Status: olmapi.PackageManifestStatus{
							CatalogSource: "custom-operators-3",
							Channels: []olmapi.PackageChannel{
								{
									Name:       "stable",
									CurrentCSV: "multicluster-engine.v2.0.7",
									CurrentCSVDesc: olmapi.CSVDescription{
										Version: olmversion.OperatorVersion{
											Version: semver.MustParse("2.0.7"),
										},
									},
								},
							},
						},
					},
				},
				channel: "stable",
			},
			want: []olmapi.PackageManifest{
				{
					Status: olmapi.PackageManifestStatus{
						CatalogSource: "custom-operators-1",
						Channels: []olmapi.PackageChannel{
							{
								Name:       "stable",
								CurrentCSV: "multicluster-engine.v2.0.7",
								CurrentCSVDesc: olmapi.CSVDescription{
									Version: olmversion.OperatorVersion{
										Version: semver.MustParse("2.0.7"),
									},
								},
							},
						},
					},
				},
				{
					Status: olmapi.PackageManifestStatus{
						CatalogSource: "custom-operators-2",
						Channels: []olmapi.PackageChannel{
							{
								Name:       "stable",
								CurrentCSV: "multicluster-engine.v2.0.7",
								CurrentCSVDesc: olmapi.CSVDescription{
									Version: olmversion.OperatorVersion{
										Version: semver.MustParse("2.0.7"),
									},
								},
							},
						},
					},
				},
				{
					Status: olmapi.PackageManifestStatus{
						CatalogSource: "custom-operators-3",
						Channels: []olmapi.PackageChannel{
							{
								Name:       "stable",
								CurrentCSV: "multicluster-engine.v2.0.7",
								CurrentCSVDesc: olmapi.CSVDescription{
									Version: olmversion.OperatorVersion{
										Version: semver.MustParse("2.0.7"),
									},
								},
							},
						},
					},
				},
			},
		},

		{
			name: "Return the non-prerelease version",
			args: args{
				pkgManifests: []olmapi.PackageManifest{
					{
						Status: olmapi.PackageManifestStatus{
							CatalogSource: "redhat-operators",
							Channels: []olmapi.PackageChannel{
								{
									Name:       "stable",
									CurrentCSV: "multicluster-engine.v2.0.6",
									CurrentCSVDesc: olmapi.CSVDescription{
										Version: olmversion.OperatorVersion{
											Version: semver.MustParse("2.0.6"),
										},
									},
								},
							},
						},
					},
					{
						Status: olmapi.PackageManifestStatus{
							CatalogSource: "custom-operators",
							Channels: []olmapi.PackageChannel{
								{
									Name:       "stable",
									CurrentCSV: "multicluster-engine.v2.0.6-5",
									CurrentCSVDesc: olmapi.CSVDescription{
										Version: olmversion.OperatorVersion{
											Version: semver.MustParse("2.0.6-5"),
										},
									},
								},
							},
						},
					},
				},
				channel: "stable",
			},
			want: []olmapi.PackageManifest{{
				Status: olmapi.PackageManifestStatus{
					CatalogSource: "redhat-operators",
					Channels: []olmapi.PackageChannel{
						{
							Name:       "stable",
							CurrentCSV: "multicluster-engine.v2.0.6",
							CurrentCSVDesc: olmapi.CSVDescription{
								Version: olmversion.OperatorVersion{
									Version: semver.MustParse("2.0.6"),
								},
							},
						},
					},
				},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filterPackageManifests(tt.args.pkgManifests, tt.args.channel); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("filterPackageManifests() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_GetCatalogSource(t *testing.T) {
	tests := []struct {
		name        string
		catalog     *subv1alpha1.CatalogSource
		manifest    *olmapi.PackageManifest
		packageName string
		want        types.NamespacedName
	}{
		{
			name: "should get catalog source",
			catalog: &subv1alpha1.CatalogSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mce-custom-registry",
					Namespace: "openshift-marketplace",
				},
				Spec: subv1alpha1.CatalogSourceSpec{
					Priority: 0,
				},
			},
			manifest: &olmapi.PackageManifest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      packageName,
					Namespace: "default",
				},
				Status: olmapi.PackageManifestStatus{
					CatalogSource:            "mce-custom-registry",
					CatalogSourceDisplayName: "sample multicluster engine",
					CatalogSourceNamespace:   "openshift-marketplace",
					Channels: []olmapi.PackageChannel{
						{
							Name: "stable-2.6",
						},
					},
				},
			},
			packageName: "advanced-cluster-management",
			want: types.NamespacedName{
				Name:      "mce-custom-registry",
				Namespace: "openshift-marketplace",
			},
		},
		{
			name: "should get community catalog source",
			catalog: &subv1alpha1.CatalogSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mce-custom-registry",
					Namespace: "openshift-marketplace",
				},
				Spec: subv1alpha1.CatalogSourceSpec{
					Priority: 0,
				},
			},
			manifest: &olmapi.PackageManifest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      communityPackageName,
					Namespace: "default",
				},
				Status: olmapi.PackageManifestStatus{
					CatalogSource:          "mce-custom-registry",
					CatalogSourceNamespace: "openshift-marketplace",
					Channels: []olmapi.PackageChannel{
						{
							Name: "community-0.5",
						},
					},
				},
			},
			want: types.NamespacedName{
				Name:      "mce-custom-registry",
				Namespace: "openshift-marketplace",
			},
		},
	}

	registerScheme()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("OPERATOR_PACKAGE", tt.packageName)

			if err := mockClient.Create(context.TODO(), tt.catalog); err != nil {
				t.Errorf("failed to create catalog: %v", err)
			}

			if err := mockClient.Create(context.TODO(), tt.manifest); err != nil {
				t.Errorf("failed to create manifest: %v", err)
			}

			if got, err := GetCatalogSource(mockClient); err != nil {
				t.Errorf("GetCatalogSource(mockClient) = got %v, want %v, err %v", got, tt.want, err)
			}

			os.Unsetenv("OPERATOR_PACKAGE")
			mockClient.Delete(context.TODO(), tt.catalog)
			mockClient.Delete(context.TODO(), tt.manifest)
		})
	}
}

func Test_extractCatalogSource(t *testing.T) {
	tests := []struct {
		name string
		pm   *olmapi.PackageManifest
		want types.NamespacedName
	}{
		{
			name: "should extract catalog source from package manifest",
			pm: &olmapi.PackageManifest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sample-package-manifest",
					Namespace: "sample-namespace",
				},
				Status: olmapi.PackageManifestStatus{
					CatalogSource:          "sample-catalog-source",
					CatalogSourceNamespace: "sample-namespace",
				},
			},
			want: types.NamespacedName{
				Name:      "sample-catalog-source",
				Namespace: "sample-namespace",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractCatalogSource(*tt.pm); got != tt.want {
				t.Errorf("extractCatalogSource(*tt.pm) = want %v, got %v", tt.want, got)
			}
		})
	}
}

func Test_findHighestPriorityCatalogSource(t *testing.T) {
	tests := []struct {
		name     string
		catalogs []subv1alpha1.CatalogSource
		pkgs     []olmapi.PackageManifest
		want     bool
	}{
		{
			name: "should find highest priority catalog source",
			catalogs: []subv1alpha1.CatalogSource{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "redhat-operators",
						Namespace: "openshift-marketplace",
					},
					Spec: subv1alpha1.CatalogSourceSpec{Priority: -100},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "multiclusterengine-catalog",
						Namespace: "openshift-marketplace",
					},
				},
			},
			pkgs: []olmapi.PackageManifest{
				{
					ObjectMeta: metav1.ObjectMeta{},
					Status: olmapi.PackageManifestStatus{
						CatalogSource:          "redhat-operators",
						CatalogSourceNamespace: "openshift-marketplace",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{},
					Status: olmapi.PackageManifestStatus{
						CatalogSource:          "multiclusterengine-catalog",
						CatalogSourceNamespace: "openshift-marketplace",
					},
				},
			},
			want: false,
		},
		{
			name: "should find more than one catalogsource with highest priority",
			catalogs: []subv1alpha1.CatalogSource{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "redhat-operators",
						Namespace: "openshift-marketplace",
					},
					Spec: subv1alpha1.CatalogSourceSpec{Priority: -100},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "multiclusterengine-catalog",
						Namespace: "openshift-marketplace",
					},
					Spec: subv1alpha1.CatalogSourceSpec{Priority: -100},
				},
			},
			pkgs: []olmapi.PackageManifest{
				{
					ObjectMeta: metav1.ObjectMeta{},
					Status: olmapi.PackageManifestStatus{
						CatalogSource:          "redhat-operators",
						CatalogSourceNamespace: "openshift-marketplace",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{},
					Status: olmapi.PackageManifestStatus{
						CatalogSource:          "multiclusterengine-catalog",
						CatalogSourceNamespace: "openshift-marketplace",
					},
				},
			},
			want: true,
		},
	}

	registerScheme()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				for _, cs := range tt.catalogs {
					if err := mockClient.Delete(context.TODO(), &cs); err != nil {
						t.Errorf("failed to delete catalogsource: %v", err)
					}
				}
			}()

			for _, cs := range tt.catalogs {
				if err := mockClient.Create(context.TODO(), &cs); err != nil {
					t.Errorf("failed to create catalogsource: %v", err)
				}
			}

			_, err := findHighestPriorityCatalogSource(mockClient, tt.pkgs)
			if got := err != nil; got != tt.want {
				t.Errorf("findHighestPriorityCatalogSource(mockClient, tt.pkgs) = got: %v, want: %v", got, tt.want)
			}
		})
	}
}
