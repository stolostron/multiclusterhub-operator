package multiclusterengine

import (
	"os"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
