// Copyright (c) 2020 Red Hat, Inc.

package utils

import (
	"reflect"
	"testing"

	operatorsv1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operator/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
	validMCH := &operatorsv1.MultiClusterHub{
		TypeMeta:   metav1.TypeMeta{Kind: "MultiClusterHub"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: operatorsv1.MultiClusterHubSpec{
			ImagePullSecret: "test",
			Mongo: operatorsv1.Mongo{
				Storage:      "mongoStorage",
				StorageClass: "mongoStorageClass",
			},
			Etcd: operatorsv1.Etcd{
				Storage:      "etcdStorage",
				StorageClass: "etcdStorageClass",
			},
			Ingress: operatorsv1.IngressSpec{
				SSLCiphers: []string{"foo", "bar", "baz"},
			},
			AvailabilityConfig: operatorsv1.HAHigh,
		},
	}

	type args struct {
		m *operatorsv1.MultiClusterHub
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
			"Empty object",
			args{&operatorsv1.MultiClusterHub{}},
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

func TestDistributePods(t *testing.T) {
	t.Run("Returns pod affinity", func(t *testing.T) {
		if got := DistributePods("app", "testapp"); reflect.TypeOf(got) != reflect.TypeOf((*corev1.Affinity)(nil)) {
			t.Errorf("DistributePods() did not return an affinity type")
		}
	})
}

func TestGetImagePullPolicy(t *testing.T) {
	noPullPolicyMCH := &operatorsv1.MultiClusterHub{}
	pullPolicyMCH := &operatorsv1.MultiClusterHub{
		Spec: operatorsv1.MultiClusterHubSpec{
			Overrides: operatorsv1.Overrides{ImagePullPolicy: v1.PullIfNotPresent},
		},
	}

	t.Run("No pull policy set", func(t *testing.T) {
		want := v1.PullAlways
		if got := GetImagePullPolicy(noPullPolicyMCH); got != want {
			t.Errorf("GetImagePullPolicy() = %v, want %v", got, want)
		}
	})
	t.Run("Pull policy set", func(t *testing.T) {
		want := v1.PullIfNotPresent
		if got := GetImagePullPolicy(pullPolicyMCH); got != want {
			t.Errorf("GetImagePullPolicy() = %v, want %v", got, want)
		}
	})
}

func TestDefaultReplicaCount(t *testing.T) {
	mchDefault := &operatorsv1.MultiClusterHub{}
	mchNonHA := &operatorsv1.MultiClusterHub{
		Spec: operatorsv1.MultiClusterHubSpec{
			AvailabilityConfig: operatorsv1.HABasic,
		},
	}
	mchHA := &operatorsv1.MultiClusterHub{
		Spec: operatorsv1.MultiClusterHubSpec{
			AvailabilityConfig: operatorsv1.HAHigh,
		},
	}

	t.Run("HA (by default)", func(t *testing.T) {
		if got := DefaultReplicaCount(mchDefault); got != 2 {
			t.Errorf("DefaultReplicaCount() = %v, want %v", got, 2)
		}
	})
	t.Run("Non-HA", func(t *testing.T) {
		if got := DefaultReplicaCount(mchNonHA); got != 1 {
			t.Errorf("DefaultReplicaCount() = %v, want %v", got, 1)
		}
	})
	t.Run("HA-mode replicas", func(t *testing.T) {
		if got := DefaultReplicaCount(mchHA); got <= 1 {
			t.Errorf("DefaultReplicaCount() = %v, but should return multiple replicas", got)
		}
	})
}

func TestFormatSSLCiphers(t *testing.T) {
	type args struct {
		ciphers []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			"Default cipher list",
			args{[]string{"ECDHE-ECDSA-AES256-GCM-SHA384", "ECDHE-RSA-AES256-GCM-SHA384"}},
			"ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384",
		},
		{"Empty slice", args{[]string{}}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatSSLCiphers(tt.args.ciphers); got != tt.want {
				t.Errorf("FormatSSLCiphers() = %v, want %v", got, tt.want)
			}
		})
	}
}
