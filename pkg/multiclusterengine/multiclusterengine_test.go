package multiclusterengine

import (
	"context"
	"os"
	"reflect"
	"testing"

	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetCatalogSource(t *testing.T) {
	os.Setenv("UNIT_TEST", "true")
	os.Setenv("OPERATOR_PACKAGE", "advanced-cluster-management")
	defer os.Unsetenv("UNIT_TEST")
	defer os.Unsetenv("OPERATOR_PACKAGE")

	type args struct {
		k8sClient client.Client
	}
	tests := []struct {
		name      string
		k8sClient client.Client
		want      types.NamespacedName
		wantErr   bool
	}{
		{
			name:      "Get catalogsource",
			k8sClient: nil,
			want: types.NamespacedName{
				Name:      "multiclusterengine-catalog",
				Namespace: "openshift-marketplace",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetCatalogSource(tt.k8sClient)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCatalogSource() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetCatalogSource() = %v, want %v", got, tt.want)
			}
		})
	}
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
	mch := &operatorsv1.MultiClusterHub{
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
