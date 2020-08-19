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

func TestAddCondition(t *testing.T) {
	oldest := operatorsv1.StatusCondition{
		Reason:             "OldestReason",
		Status:             v1.ConditionTrue,
		LastTransitionTime: v1.NewTime(time.Date(1, 1, 0, 0, 1, 0, 0, time.UTC)),
	}
	older := operatorsv1.StatusCondition{
		Reason:             "OlderReason",
		Status:             v1.ConditionTrue,
		LastTransitionTime: v1.NewTime(time.Date(1, 1, 0, 0, 2, 0, 0, time.UTC)),
	}
	old := operatorsv1.StatusCondition{
		Reason:             "OldReason",
		Status:             v1.ConditionTrue,
		LastTransitionTime: v1.NewTime(time.Date(1, 1, 0, 0, 3, 0, 0, time.UTC)),
	}
	new := operatorsv1.StatusCondition{
		Reason:             "OldReason",
		Status:             v1.ConditionTrue,
		LastTransitionTime: v1.NewTime(time.Date(1, 1, 0, 0, 4, 0, 0, time.UTC)),
	}

	t.Run("Add single hubcondition", func(t *testing.T) {
		m := &operatorsv1.MultiClusterHub{}
		sc := unknownStatus
		AddCondition(m, sc)
		if len(m.Status.HubConditions) < 1 {
			t.Errorf("AddCondition() failed to add a HubCondition")
		}
	})

	t.Run("Add several hubconditions", func(t *testing.T) {
		m := &operatorsv1.MultiClusterHub{}
		expected := 3
		AddCondition(m, oldest)
		AddCondition(m, older)
		AddCondition(m, old)
		AddCondition(m, new)
		if len(m.Status.HubConditions) > expected {
			t.Errorf("AddCondition() added too many hub conditions; expected a max of %d, got %d", expected, len(m.Status.HubConditions))
		}
	})

	t.Run("No duplicate hubconditions", func(t *testing.T) {
		m := &operatorsv1.MultiClusterHub{}
		sc := unknownStatus
		expected := 1
		for i := 0; i < 2; i++ {
			AddCondition(m, sc)
		}
		if len(m.Status.HubConditions) != 1 {
			t.Errorf("AddCondition() added duplicate hub conditions; expected %d, got %d", expected, len(m.Status.HubConditions))
		}
	})

	t.Run("Retain last transition time", func(t *testing.T) {
		m := &operatorsv1.MultiClusterHub{}
		AddCondition(m, old)
		AddCondition(m, new)
		if len(m.Status.HubConditions) != 1 {
			t.Errorf("AddCondition() too many hub conditions; expected %d, got %d", 1, len(m.Status.HubConditions))
		}
		if ltt := &m.Status.HubConditions[0].LastTransitionTime; !ltt.Equal(&old.LastTransitionTime) {
			t.Errorf("AddCondition() expected lastTransitionTime of %v, got %v", old.LastTransitionTime, ltt)
		}
	})

	t.Run("Remove oldest conditions", func(t *testing.T) {
		m := &operatorsv1.MultiClusterHub{}
		AddCondition(m, oldest)
		AddCondition(m, older)
		AddCondition(m, old)
		for _, x := range m.Status.HubConditions {
			if x.Reason == "OldestReason" {
				t.Errorf("Expected oldest condition to be removed")
			}
		}
	})

	t.Run("Conditions should be in sorted order", func(t *testing.T) {
		m := &operatorsv1.MultiClusterHub{}
		AddCondition(m, oldest)
		AddCondition(m, older)
		first := &m.Status.HubConditions[0]
		second := &m.Status.HubConditions[1]
		if !first.LastTransitionTime.Time.After(second.LastTransitionTime.Time) {
			t.Errorf("AddCondition() expected first condition to be the most recent; got %v", m.Status.HubConditions)
		}
	})
}
