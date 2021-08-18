// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package foundation

import (
	"reflect"
	"testing"

	operatorsv1 "github.com/open-cluster-management/multiclusterhub-operator/pkg/apis/operator/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateDeployment(t *testing.T) {
	mch := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: operatorsv1.MultiClusterHubSpec{
			ImagePullSecret: "test",
			NodeSelector: map[string]string{
				"test": "test",
			},
		},
	}
	ovr := map[string]string{}

	// 1. Valid mch
	dep := OCMControllerDeployment(mch, ovr)

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

	// 6. Modified replica count
	dep5 := dep.DeepCopy()
	dep5.Spec.Replicas = new(int32)

	// 7. Modified Tolerations
	dep6 := dep.DeepCopy()
	dep6.Spec.Template.Spec.Tolerations = nil

	// 8. Modified volumes
	dep7 := dep.DeepCopy()
	dep7.Spec.Template.Spec.Volumes = []corev1.Volume{
		{Name: "webhook-cert",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: "abc"},
			},
		},
	}

	// 9. Missing deployment labels
	dep8 := dep.DeepCopy()
	dep8.Labels = nil

	// 10. Missing pod labels
	dep9 := dep.DeepCopy()
	dep9.Spec.Template.Labels = nil

	type args struct {
		m   *operatorsv1.MultiClusterHub
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
		{
			name:  "Modified number of replicas",
			args:  args{mch, dep5},
			want:  dep,
			want1: true,
		},
		{
			name:  "Modified Tolerations",
			args:  args{mch, dep6},
			want:  dep,
			want1: true,
		},
		{
			name:  "Modified volumes",
			args:  args{mch, dep7},
			want:  dep,
			want1: true,
		},
		{
			name:  "Missing deployment labels",
			args:  args{mch, dep8},
			want:  dep,
			want1: true,
		},
		{
			name:  "Missing pod labels",
			args:  args{mch, dep9},
			want:  dep,
			want1: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := ValidateDeployment(tt.args.m, ovr, dep, tt.args.dep)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ValidateDeployment() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("ValidateDeployment() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
