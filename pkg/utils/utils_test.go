package utils

import (
	"reflect"
	"testing"

	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNodeSelectors(t *testing.T) {
	mch := &operatorsv1alpha1.MultiClusterHub{
		Spec: operatorsv1alpha1.MultiClusterHubSpec{
			NodeSelector: &operatorsv1alpha1.NodeSelector{
				OS:                  "linux",
				CustomLabelSelector: "kubernetes.io/arch",
				CustomLabelValue:    "amd64",
			},
		},
	}
	mchNoSelector := &operatorsv1alpha1.MultiClusterHub{}
	mchEmptySelector := &operatorsv1alpha1.MultiClusterHub{
		Spec: operatorsv1alpha1.MultiClusterHubSpec{
			NodeSelector: &operatorsv1alpha1.NodeSelector{
				CustomLabelSelector: "kubernetes.io/arch",
				CustomLabelValue:    "",
			},
		},
	}

	type args struct {
		mch *operatorsv1alpha1.MultiClusterHub
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "With node selectors",
			args: args{mch},
			want: map[string]string{
				"kubernetes.io/os":   "linux",
				"kubernetes.io/arch": "amd64",
			},
		},
		{
			name: "No node selector",
			args: args{mchNoSelector},
			want: nil,
		},
		{
			name: "Empty selector value",
			args: args{mchEmptySelector},
			want: map[string]string{
				"kubernetes.io/arch": "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NodeSelectors(tt.args.mch); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("nodeSelectors() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAddInstallerLabel(t *testing.T) {
	name := "example-installer"
	ns := "default"

	t.Run("Should add labels when none exist", func(t *testing.T) {
		u := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps.open-cluster-management.io/v1",
				"kind":       "Channel",
			},
		}
		want := 2

		AddInstallerLabel(u, name, ns)
		if got := len(u.GetLabels()); got != want {
			t.Errorf("got %v labels, want %v", got, want)
		}
	})

	t.Run("Should not replace existing labels", func(t *testing.T) {
		u := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps.open-cluster-management.io/v1",
				"kind":       "Channel",
				"metadata": map[string]interface{}{
					"name": "channelName",
					"labels": map[string]interface{}{
						"hello": "world",
					},
				},
			},
		}
		want := 3

		AddInstallerLabel(u, name, ns)
		if got := len(u.GetLabels()); got != want {
			t.Errorf("got %v labels, want %v", got, want)
		}
	})
}
