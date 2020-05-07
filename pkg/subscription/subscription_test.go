// Copyright (c) 2020 Red Hat, Inc.

package subscription

import (
	"reflect"
	"testing"

	subalpha1 "github.com/open-cluster-management/multicloud-operators-subscription/pkg/apis/apps/v1"
	operatorsv1beta1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1beta1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func TestValidate(t *testing.T) {
	mch := &operatorsv1beta1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: operatorsv1beta1.MultiClusterHubSpec{
			ImagePullSecret: "test",
			Mongo:           operatorsv1beta1.Mongo{},
		},
	}

	cs := utils.CacheSpec{
		IngressDomain:  "testIngress",
		ImageOverrides: map[string]string{},
	}
	// 1. Valid mch
	sub := KUIWebTerminal(mch, cs)

	// 2. Modified ImagePullSecret
	mch1 := mch.DeepCopy()
	mch1.Spec.ImagePullSecret = "notTest"
	sub1 := KUIWebTerminal(mch1, cs)

	// 3. Modified ImagePullPolicy
	mch2 := mch.DeepCopy()
	mch2.Spec.Overrides.ImagePullPolicy = corev1.PullNever
	sub2 := KUIWebTerminal(mch2, cs)

	// 4. Modified ImageRepository
	mch3 := mch.DeepCopy()
	mch3.Spec.Overrides.ImageRepository = "notquay.io/closed-cluster-management"
	sub3 := KUIWebTerminal(mch3, cs)

	// 5. Activate HA mode
	mch4 := mch.DeepCopy()
	mch4.Spec.Failover = true
	sub4 := KUIWebTerminal(mch4, cs)

	type args struct {
		found *subalpha1.Subscription
		want  *subalpha1.Subscription
	}
	tests := []struct {
		name  string
		args  args
		want  *subalpha1.Subscription
		want1 bool
	}{
		{
			name:  "Valid subscription",
			args:  args{sub, sub},
			want:  nil,
			want1: false,
		},
		{
			name:  "Modified ImagePullSecret",
			args:  args{sub, sub1},
			want:  sub1,
			want1: true,
		},
		{
			name:  "Modified ImagePullPolicy",
			args:  args{sub, sub2},
			want:  sub2,
			want1: true,
		},
		{
			name:  "Modified ImageRepository",
			args:  args{sub, sub3},
			want:  sub3,
			want1: true,
		},
		{
			name:  "Activate failover mode",
			args:  args{sub, sub4},
			want:  sub4,
			want1: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := Validate(tt.args.found, tt.args.want)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Validate() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("Validate() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestSubscriptions(t *testing.T) {
	mch := &operatorsv1beta1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: operatorsv1beta1.MultiClusterHubSpec{
			ImagePullSecret: "test",
			Mongo:           operatorsv1beta1.Mongo{},
		},
	}

	cache := utils.CacheSpec{
		IngressDomain:  "testIngress",
		ImageOverrides: map[string]string{},
	}

	tests := []struct {
		name string
		got  *subalpha1.Subscription
	}{
		{"ApplicationUI subscription", ApplicationUI(mch, cache)},
		{"CertManager subscription", CertManager(mch, cache)},
		{"CertWebhook subscription", CertWebhook(mch, cache)},
		{"ConfigWatcher subscription", ConfigWatcher(mch, cache)},
		{"Console subscription", Console(mch, cache)},
		{"GRC subscription", GRC(mch, cache)},
		{"KUIWebTerminal subscription", KUIWebTerminal(mch, cache)},
		{"ManagementIngress subscription", ManagementIngress(mch, cache)},
		{"MongoDB subscription", MongoDB(mch, cache)},
		{"RCM subscription", RCM(mch, cache)},
		{"Search subscription", Search(mch, cache)},
		{"Topology subscription", Topology(mch, cache)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := yaml.Marshal(tt.got.Spec)
			if err != nil {
				t.Error("Issue parsing subscription values")
			}
		})
	}
}
