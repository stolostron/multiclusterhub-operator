// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package multiclusterengine

import (
	"reflect"
	"testing"

	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewSubscription(t *testing.T) {
	mch := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mch",
			Namespace: "mch-ns",
		},
	}
	type args struct {
		m            *operatorsv1.MultiClusterHub
		c            *subv1alpha1.SubscriptionConfig
		subOverrides *subv1alpha1.SubscriptionSpec
		community    bool
	}
	tests := []struct {
		name string
		args args
		want *subv1alpha1.Subscription
	}{
		{
			name: "Prod subscription",
			args: args{
				m:            mch,
				c:            nil,
				subOverrides: nil,
				community:    false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewSubscription(tt.args.m, tt.args.c, tt.args.subOverrides, tt.args.community); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewSubscription() = %v, want %v", got, tt.want)
			}
		})
	}
}
