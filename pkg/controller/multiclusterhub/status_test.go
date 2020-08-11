// Copyright (c) 2020 Red Hat, Inc.

package multiclusterhub

import (
	"reflect"
	"testing"
	"time"

	subrelv1 "github.com/open-cluster-management/multicloud-operators-subscription-release/pkg/apis/apps/v1"
	operatorsv1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operator/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_aggregateStatus(t *testing.T) {
	available := operatorsv1.StatusCondition{Type: "Available", Status: v1.ConditionTrue}
	deployed := operatorsv1.StatusCondition{Type: "Available", Status: v1.ConditionTrue}
	unavailable := operatorsv1.StatusCondition{Type: "Available", Status: v1.ConditionFalse}
	type args struct {
		components map[string]operatorsv1.StatusCondition
	}
	tests := []struct {
		name string
		args args
		want operatorsv1.HubPhaseType
	}{
		{
			name: "Single available component",
			args: args{
				components: map[string]operatorsv1.StatusCondition{
					"foo": available,
					"bar": deployed,
				},
			},
			want: operatorsv1.HubRunning,
		},
		{
			name: "Single unavailable component",
			args: args{
				components: map[string]operatorsv1.StatusCondition{
					"foo": unavailable,
					"bar": deployed,
				},
			},
			want: operatorsv1.HubPending,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := aggregateStatus(tt.args.components); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("aggregateStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_latestHelmReleaseCondition(t *testing.T) {
	first := subrelv1.HelmAppCondition{
		Type:               subrelv1.ConditionInitialized,
		LastTransitionTime: v1.NewTime(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)),
	}
	second := subrelv1.HelmAppCondition{
		Type:               subrelv1.ConditionDeployed,
		LastTransitionTime: v1.NewTime(time.Date(3000, 1, 1, 0, 0, 0, 0, time.UTC)),
	}
	type args struct {
		conditions []subrelv1.HelmAppCondition
	}
	tests := []struct {
		name string
		args args
		want subrelv1.HelmAppCondition
	}{
		{
			name: "Deployed after initialized",
			args: args{
				conditions: []subrelv1.HelmAppCondition{
					first,
					second,
				},
			},
			want: second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := latestHelmReleaseCondition(tt.args.conditions); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("latestHelmReleaseCondition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_latestDeployCondition(t *testing.T) {
	first := appsv1.DeploymentCondition{
		Type:               appsv1.DeploymentProgressing,
		LastTransitionTime: v1.NewTime(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)),
	}
	second := appsv1.DeploymentCondition{
		Type:               appsv1.DeploymentAvailable,
		LastTransitionTime: v1.NewTime(time.Date(3000, 1, 1, 0, 0, 0, 0, time.UTC)),
	}
	type args struct {
		conditions []appsv1.DeploymentCondition
	}
	tests := []struct {
		name string
		args args
		want appsv1.DeploymentCondition
	}{
		{
			name: "Deployed after initialized",
			args: args{
				conditions: []appsv1.DeploymentCondition{
					first,
					second,
				},
			},
			want: second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := latestDeployCondition(tt.args.conditions); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("latestDeployCondition() = %v, want %v", got, tt.want)
			}
		})
	}
}
