// Copyright (c) 2020 Red Hat, Inc.

package utils

import (
	"testing"

	operatorsv1beta1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsPaused(t *testing.T) {
	t.Run("Unpaused MCH", func(t *testing.T) {
		mch := &operatorsv1beta1.MultiClusterHub{}
		want := false
		if got := IsPaused(mch); got != want {
			t.Errorf("IsPaused() = %v, want %v", got, want)
		}
	})
	t.Run("Paused MCH", func(t *testing.T) {
		mch := &operatorsv1beta1.MultiClusterHub{
			ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{AnnotationMCHPause: "true"}},
		}
		want := true
		if got := IsPaused(mch); got != want {
			t.Errorf("IsPaused() = %v, want %v", got, want)
		}
	})
	t.Run("Pause label false MCH", func(t *testing.T) {
		mch := &operatorsv1beta1.MultiClusterHub{
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
		instance *operatorsv1beta1.MultiClusterHub
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
				instance: &operatorsv1beta1.MultiClusterHub{},
				key:      "",
			},
			want: "",
		},
		{
			name: "Annotation exists",
			args: args{
				instance: &operatorsv1beta1.MultiClusterHub{
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
