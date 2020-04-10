package mcm

import (
	"reflect"
	"testing"

	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateDeployment(t *testing.T) {
	// 2. Modified ImagePullSecret
	// 3. Modified image
	// 4. Modified pullPolicy
	// 5. Modified NodeSelector

	mch := &operatorsv1alpha1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: operatorsv1alpha1.MultiClusterHubSpec{
			Version:         "latest",
			ImageRepository: "quay.io/open-cluster-management",
			ImagePullPolicy: "Always",
			ImagePullSecret: "test",
			NodeSelector: &operatorsv1alpha1.NodeSelector{
				OS:                  "test",
				CustomLabelSelector: "test",
				CustomLabelValue:    "test",
			},
			Mongo: operatorsv1alpha1.Mongo{},
		},
	}
	// 1. Valid mch
	dep := ControllerDeployment(mch)

	// 2. Modified ImagePullSecret
	dep1 := dep.DeepCopy()
	dep1.Spec.Template.Spec.ImagePullSecrets = nil

	// 3. Modified image
	dep2 := dep.DeepCopy()
	dep2.Spec.Template.Spec.Containers[0].Image = "differentImage"

	// 4. Modified pullPolicy
	dep3 := dep.DeepCopy()
	dep3.Spec.Template.Spec.Containers[0].ImagePullPolicy = corev1.PullNever

	// 5. Modified NodeSelector
	dep4 := dep.DeepCopy()
	dep4.Spec.Template.Spec.NodeSelector = nil

	type args struct {
		m   *operatorsv1alpha1.MultiClusterHub
		dep *appsv1.Deployment
	}
	tests := []struct {
		name  string
		args  args
		want  *appsv1.Deployment
		want1 bool
	}{
		{
			name:  "Valid Deployment",
			args:  args{mch, dep},
			want:  dep,
			want1: false,
		},
		{
			name:  "Modified ImagePullSecret",
			args:  args{mch, dep1},
			want:  dep,
			want1: true,
		},
		{
			name:  "Modified Image",
			args:  args{mch, dep2},
			want:  dep,
			want1: true,
		},
		{
			name:  "Modified PullPolicy",
			args:  args{mch, dep3},
			want:  dep,
			want1: true,
		},
		{
			name:  "Modified NodeSelector",
			args:  args{mch, dep4},
			want:  dep,
			want1: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := ValidateDeployment(tt.args.m, tt.args.dep)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ValidateDeployment() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("ValidateDeployment() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_nodeSelectors(t *testing.T) {
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
			if got := nodeSelectors(tt.args.mch); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("nodeSelectors() = %v, want %v", got, tt.want)
			}
		})
	}
}
