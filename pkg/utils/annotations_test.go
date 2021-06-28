// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package utils

import (
	"reflect"
	"testing"

	operatorsv1 "github.com/open-cluster-management/multiclusterhub-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsPaused(t *testing.T) {
	t.Run("Unpaused MCH", func(t *testing.T) {
		mch := &operatorsv1.MultiClusterHub{}
		want := false
		if got := IsPaused(mch); got != want {
			t.Errorf("IsPaused() = %v, want %v", got, want)
		}
	})
	t.Run("Paused MCH", func(t *testing.T) {
		mch := &operatorsv1.MultiClusterHub{
			ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{AnnotationMCHPause: "true"}},
		}
		want := true
		if got := IsPaused(mch); got != want {
			t.Errorf("IsPaused() = %v, want %v", got, want)
		}
	})
	t.Run("Pause label false MCH", func(t *testing.T) {
		mch := &operatorsv1.MultiClusterHub{
			ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{AnnotationMCHPause: "false"}},
		}
		want := false
		if got := IsPaused(mch); got != want {
			t.Errorf("IsPaused() = %v, want %v", got, want)
		}
	})

}

func Test_getAnnotation(t *testing.T) {
	type args struct {
		instance *operatorsv1.MultiClusterHub
		key      string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Annotation does not exist",
			args: args{
				instance: &operatorsv1.MultiClusterHub{},
				key:      "",
			},
			want: "",
		},
		{
			name: "Annotation exists",
			args: args{
				instance: &operatorsv1.MultiClusterHub{
					ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"foo": "bar"}},
				},
				key: "foo",
			},
			want: "bar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getAnnotation(tt.args.instance, tt.args.key); got != tt.want {
				t.Errorf("getAnnotation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOverrideImageRepository(t *testing.T) {
	tests := []struct {
		ImageOverrides map[string]string
		ImageRepo      string
		Expected       map[string]string
	}{
		{
			ImageOverrides: map[string]string{
				"application_ui": "quay.io/open-cluster-management/application-ui@sha256:c740fc7bac067f003145ab909504287360564016b7a4a51b7ad4987aca123ac1",
				"console_api":    "quay.io/open-cluster-management/console-api@sha256:3ef1043b4e61a09b07ff37f9ad8fc6e707af9813936cf2c0d52f2fa0e489c75f",
				"rcm_controller": "quay.io/open-cluster-management/rcm-controller@sha256:8fab4d788241bf364dbc1b8c1ea5ccf18d3145a640dbd456b0dc7ba204e36819",
			},
			ImageRepo: "quay.io:443/acm-d",
			Expected: map[string]string{
				"application_ui": "quay.io:443/acm-d/application-ui@sha256:c740fc7bac067f003145ab909504287360564016b7a4a51b7ad4987aca123ac1",
				"console_api":    "quay.io:443/acm-d/console-api@sha256:3ef1043b4e61a09b07ff37f9ad8fc6e707af9813936cf2c0d52f2fa0e489c75f",
				"rcm_controller": "quay.io:443/acm-d/rcm-controller@sha256:8fab4d788241bf364dbc1b8c1ea5ccf18d3145a640dbd456b0dc7ba204e36819",
			},
		},
		{
			ImageOverrides: map[string]string{},
			ImageRepo:      "",
			Expected:       map[string]string{},
		},
		{
			ImageOverrides: map[string]string{
				"application_ui": "registry.redhat.io/rhacm2/application-ui@sha256:d7b7b96d679dbbdace044a18cca56544faa65f66e593fc600c08c9f814e0ea6a",
				"console_api":    "registry.redhat.io/rhacm2/console-api@sha256:d7b7b96d679dbbdace044a18cca56544faa65f66e593fc600c08c9f814e0ea6a",
				"rcm_controller": "registry.redhat.io/rhacm2/rcm-controller@sha256:d7b7b96d679dbbdace044a18cca56544faa65f66e593fc600c08c9f814e0ea6a",
			},
			ImageRepo: "quay.io:443/acm-d",
			Expected: map[string]string{
				"application_ui": "quay.io:443/acm-d/application-ui@sha256:d7b7b96d679dbbdace044a18cca56544faa65f66e593fc600c08c9f814e0ea6a",
				"console_api":    "quay.io:443/acm-d/console-api@sha256:d7b7b96d679dbbdace044a18cca56544faa65f66e593fc600c08c9f814e0ea6a",
				"rcm_controller": "quay.io:443/acm-d/rcm-controller@sha256:d7b7b96d679dbbdace044a18cca56544faa65f66e593fc600c08c9f814e0ea6a",
			},
		},
	}

	for _, tt := range tests {
		if !reflect.DeepEqual(OverrideImageRepository(tt.ImageOverrides, tt.ImageRepo), tt.Expected) {
			t.Fatalf("ImageRepository override failure")
		}
	}
}
