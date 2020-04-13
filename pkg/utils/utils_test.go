package utils

import (
	"reflect"
	"testing"

	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestContainsPullSecret(t *testing.T) {
	superset := []corev1.LocalObjectReference{{Name: "foo"}, {Name: "bar"}}
	type args struct {
		pullSecrets []corev1.LocalObjectReference
		ps          corev1.LocalObjectReference
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"Contains pull secret",
			args{
				pullSecrets: superset,
				ps:          corev1.LocalObjectReference{Name: "foo"},
			},
			true,
		},
		{
			"Does not contain pull secret",
			args{
				pullSecrets: superset,
				ps:          corev1.LocalObjectReference{Name: "baz"},
			},
			false,
		},
		{
			"Empty list",
			args{
				pullSecrets: []corev1.LocalObjectReference{},
				ps:          corev1.LocalObjectReference{Name: "baz"},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContainsPullSecret(tt.args.pullSecrets, tt.args.ps); got != tt.want {
				t.Errorf("ContainsPullSecret() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainsMap(t *testing.T) {
	superset := map[string]string{
		"hello":     "world",
		"goodnight": "moon",
		"yip":       "yip",
	}
	type args struct {
		all      map[string]string
		expected map[string]string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"Superset",
			args{
				all:      superset,
				expected: map[string]string{"hello": "world", "yip": "yip"},
			},
			true,
		},
		{
			"Partial overlap",
			args{
				all:      superset,
				expected: map[string]string{"hello": "world", "greetings": "traveler"},
			},
			false,
		},
		{
			"Empty superset",
			args{
				all:      map[string]string{},
				expected: map[string]string{"yip": "yip"},
			},
			false,
		},
		{
			"Empty subset",
			args{
				all:      superset,
				expected: map[string]string{},
			},
			true,
		},
		{
			"Same keys, different values",
			args{
				all:      superset,
				expected: map[string]string{"hello": "moon", "yip": "yip"},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContainsMap(tt.args.all, tt.args.expected); got != tt.want {
				t.Errorf("ContainsMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMchIsValid(t *testing.T) {
	validMCH := &operatorsv1alpha1.MultiClusterHub{
		TypeMeta:   metav1.TypeMeta{Kind: "MultiClusterHub"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: operatorsv1alpha1.MultiClusterHubSpec{
			Version:         "latest",
			ImageRepository: "quay.io/open-cluster-management",
			ImagePullPolicy: "Always",
			ImagePullSecret: "test",
			ReplicaCount:    1,
			NodeSelector: &operatorsv1alpha1.NodeSelector{
				OS:                  "test",
				CustomLabelSelector: "test",
				CustomLabelValue:    "test",
			},
			Mongo: operatorsv1alpha1.Mongo{
				Storage:      "mongoStorage",
				StorageClass: "mongoStorageClass",
				ReplicaCount: 1,
			},
			Etcd: operatorsv1alpha1.Etcd{
				Storage:      "etcdStorage",
				StorageClass: "etcdStorageClass",
			},
		},
	}
	noRepo := validMCH.DeepCopy()
	noRepo.Spec.ImageRepository = ""
	noMongoReplicas := validMCH.DeepCopy()
	noMongoReplicas.Spec.Mongo.ReplicaCount = 0

	type args struct {
		m *operatorsv1alpha1.MultiClusterHub
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"Valid MCH",
			args{validMCH},
			true,
		},
		{
			"Missing Image Repository",
			args{noRepo},
			false,
		},
		{
			"Zero Mongo Replicas",
			args{noMongoReplicas},
			false,
		},
		{
			"Empty object",
			args{&operatorsv1alpha1.MultiClusterHub{}},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MchIsValid(tt.args.m); got != tt.want {
				t.Errorf("MchIsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateDeployment(t *testing.T) {
	mch := &operatorsv1alpha1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: operatorsv1alpha1.MultiClusterHubSpec{
			Version:         "latest",
			ImageRepository: "quay.io/open-cluster-management",
			ImagePullPolicy: "Always",
			ImagePullSecret: "test",
			ReplicaCount:    1,
			NodeSelector: &operatorsv1alpha1.NodeSelector{
				OS:                  "test",
				CustomLabelSelector: "test",
				CustomLabelValue:    "test",
			},
			Mongo: operatorsv1alpha1.Mongo{},
		},
	}
	replicas := int32(1)
	image := "quay.io/open-cluster-management/image:1.0.0-xyz"
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
			Labels:    map[string]string{"app": "test"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image:           image,
						ImagePullPolicy: mch.Spec.ImagePullPolicy,
						Name:            "test",
						Ports: []corev1.ContainerPort{{
							ContainerPort: 8443,
							Name:          "helmrepo",
						}},
					}},
					ImagePullSecrets: []corev1.LocalObjectReference{{Name: mch.Spec.ImagePullSecret}},
					NodeSelector:     NodeSelectors(mch),
				},
			},
		},
	}

	// 1. Valid mch

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

	// 6. Modified ReplicaCount
	dep5 := dep.DeepCopy()
	dep5.Spec.Replicas = new(int32)

	type args struct {
		m     *operatorsv1alpha1.MultiClusterHub
		dep   *appsv1.Deployment
		image string
	}
	tests := []struct {
		name  string
		args  args
		want  *appsv1.Deployment
		want1 bool
	}{
		{
			name:  "Valid Deployment",
			args:  args{mch, dep, image},
			want:  dep,
			want1: false,
		},
		{
			name:  "Modified ImagePullSecret",
			args:  args{mch, dep1, image},
			want:  dep,
			want1: true,
		},
		{
			name:  "Modified Image",
			args:  args{mch, dep2, image},
			want:  dep,
			want1: true,
		},
		{
			name:  "Modified PullPolicy",
			args:  args{mch, dep3, image},
			want:  dep,
			want1: true,
		},
		{
			name:  "Modified NodeSelector",
			args:  args{mch, dep4, image},
			want:  dep,
			want1: true,
		},
		{
			name:  "Modified ReplicaCount",
			args:  args{mch, dep5, image},
			want:  dep,
			want1: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := ValidateDeployment(tt.args.m, tt.args.dep, tt.args.image)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ValidateDeployment() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("ValidateDeployment() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
