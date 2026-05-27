// Copyright Contributors to the Open Cluster Management project

package v0

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/blang/semver/v4"
	olmversion "github.com/operator-framework/api/pkg/lib/version"
	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	olmapi "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengine"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

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
										Version: olmversion.OperatorVersion{Version: semver.MustParse("2.0.6")},
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
										Version: olmversion.OperatorVersion{Version: semver.MustParse("2.0.6-2")},
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
										Version: olmversion.OperatorVersion{Version: semver.MustParse("2.0.6-5")},
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
										Version: olmversion.OperatorVersion{Version: semver.MustParse("2.0.6-4")},
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
								Version: olmversion.OperatorVersion{Version: semver.MustParse("2.0.6-5")},
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
										Version: olmversion.OperatorVersion{Version: semver.MustParse("2.0.6")},
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
										Version: olmversion.OperatorVersion{Version: semver.MustParse("2.0.6")},
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
									Version: olmversion.OperatorVersion{Version: semver.MustParse("2.0.6")},
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
									Version: olmversion.OperatorVersion{Version: semver.MustParse("2.0.6")},
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
										Version: olmversion.OperatorVersion{Version: semver.MustParse("2.0.6")},
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
										Version: olmversion.OperatorVersion{Version: semver.MustParse("2.0.7")},
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
										Version: olmversion.OperatorVersion{Version: semver.MustParse("2.0.7")},
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
										Version: olmversion.OperatorVersion{Version: semver.MustParse("2.0.7")},
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
									Version: olmversion.OperatorVersion{Version: semver.MustParse("2.0.7")},
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
									Version: olmversion.OperatorVersion{Version: semver.MustParse("2.0.7")},
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
									Version: olmversion.OperatorVersion{Version: semver.MustParse("2.0.7")},
								},
							},
						},
					},
				},
			},
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
		channel     string
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
					Name:      multiclusterengine.MCEProdPackageName,
					Namespace: "default",
				},
				Status: olmapi.PackageManifestStatus{
					CatalogSource:            "mce-custom-registry",
					CatalogSourceDisplayName: "sample multicluster engine",
					CatalogSourceNamespace:   "openshift-marketplace",
					Channels: []olmapi.PackageChannel{
						{
							Name: "stable-5.0",
						},
					},
				},
			},
			channel:     multiclusterengine.MCEProdChannel,
			packageName: multiclusterengine.MCEProdPackageName,
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
					Name:      multiclusterengine.MCECommunityPackageName,
					Namespace: "default",
				},
				Status: olmapi.PackageManifestStatus{
					CatalogSource:          "mce-custom-registry",
					CatalogSourceNamespace: "openshift-marketplace",
					Channels: []olmapi.PackageChannel{
						{
							Name: "community-0.10",
						},
					},
				},
			},
			channel:     multiclusterengine.MCECommunityChannel,
			packageName: multiclusterengine.MCECommunityPackageName,
			want: types.NamespacedName{
				Name:      "mce-custom-registry",
				Namespace: "openshift-marketplace",
			},
		},
	}

	scheme := runtime.NewScheme()
	_ = subv1alpha1.AddToScheme(scheme)
	_ = olmapi.AddToScheme(scheme)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := fake.NewClientBuilder().WithScheme(scheme).Build()

			os.Setenv("OPERATOR_PACKAGE", tt.packageName)

			if err := mockClient.Create(context.TODO(), tt.catalog); err != nil {
				t.Errorf("failed to create catalog: %v", err)
			}

			if err := mockClient.Create(context.TODO(), tt.manifest); err != nil {
				t.Errorf("failed to create manifest: %v", err)
			}

			got, err := GetCatalogSource(mockClient, tt.channel, tt.packageName)
			if err != nil {
				t.Errorf("GetCatalogSource(mockClient) error = %v", err)
			}
			if got != tt.want {
				t.Errorf("GetCatalogSource(mockClient) = got %v, want %v", got, tt.want)
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

	scheme := runtime.NewScheme()
	_ = subv1alpha1.AddToScheme(scheme)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := fake.NewClientBuilder().WithScheme(scheme).Build()

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

func TestOperatorGroup(t *testing.T) {
	if got := OperatorGroup(multiclusterengine.MCEProdOperandNamespace).Namespace; got != multiclusterengine.MCEProdOperandNamespace {
		t.Errorf("OperatorGroup().Namespace = %v, want %v", got, multiclusterengine.MCEProdOperandNamespace)
	}
	if got := OperatorGroup(multiclusterengine.MCEProdOperandNamespace).Spec.TargetNamespaces[0]; got != multiclusterengine.MCEProdOperandNamespace {
		t.Errorf("OperatorGroup().Spec.TargetNamespaces[0] = %v, want %v", got, multiclusterengine.MCEProdOperandNamespace)
	}

	if got := OperatorGroup(multiclusterengine.MCECommunityOperandNamespace).Namespace; got != multiclusterengine.MCECommunityOperandNamespace {
		t.Errorf("OperatorGroup().Namespace = %v, want %v", got, multiclusterengine.MCECommunityOperandNamespace)
	}
	if got := OperatorGroup(multiclusterengine.MCECommunityOperandNamespace).Spec.TargetNamespaces[0]; got != multiclusterengine.MCECommunityOperandNamespace {
		t.Errorf("OperatorGroup().Spec.TargetNamespaces[0] = %v, want %v", got, multiclusterengine.MCECommunityOperandNamespace)
	}
}
