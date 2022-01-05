// Copyright (c) 2020 Red Hat, Inc.

package multiclusterhub

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	subrelv1 "github.com/open-cluster-management/multicloud-operators-subscription-release/pkg/apis/apps/v1"
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/pkg/apis/operator/v1"
	"github.com/stolostron/multiclusterhub-operator/version"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_allComponentsSuccessful(t *testing.T) {
	available := operatorsv1.StatusCondition{Type: "Available", Status: v1.ConditionTrue, Available: true}
	deployed := operatorsv1.StatusCondition{Type: "Available", Status: v1.ConditionTrue, Available: true}
	unavailable := operatorsv1.StatusCondition{Type: "Available", Status: v1.ConditionFalse}
	type args struct {
		components map[string]operatorsv1.StatusCondition
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Single available component",
			args: args{
				components: map[string]operatorsv1.StatusCondition{
					"foo": available,
					"bar": deployed,
				},
			},
			want: true,
		},
		{
			name: "Single unavailable component",
			args: args{
				components: map[string]operatorsv1.StatusCondition{
					"foo": unavailable,
					"bar": deployed,
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := allComponentsSuccessful(tt.args.components); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("allComponentsSuccessful() = %v, want %v", got, tt.want)
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
	b := `[
		{
			"lastTransitionTime": "2020-09-02T21:22:22Z",
			"lastUpdateTime": "2020-09-02T21:22:22Z",
			"message": "Deployment has minimum availability.",
			"reason": "MinimumReplicasAvailable",
			"status": "True",
			"type": "Available"
		},
		{
			"lastTransitionTime": "2020-09-03T14:23:47Z",
			"lastUpdateTime": "2020-09-03T14:23:47Z",
			"message": "ReplicaSet \"kui-web-terminal-78c4bc769\" has timed out progressing.",
			"reason": "ProgressDeadlineExceeded",
			"status": "False",
			"type": "Progressing"
		}
	]`
	var bs []appsv1.DeploymentCondition
	err := json.Unmarshal([]byte(b), &bs)
	if err != nil {
		t.Errorf(err.Error())
	}

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
		want appsv1.DeploymentConditionType
	}{
		{
			name: "Deployed after initialized",
			args: args{
				conditions: []appsv1.DeploymentCondition{
					second,
					first,
				},
			},
			want: appsv1.DeploymentAvailable,
		},
		{
			name: "Progressing after available",
			args: args{
				conditions: bs,
			},
			want: appsv1.DeploymentProgressing,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := latestDeployCondition(tt.args.conditions); got.Type != tt.want {
				t.Errorf("latestDeployCondition() = %v, want %v", got, tt.want)
			}
		})
	}
}

var (
	old = operatorsv1.HubCondition{
		Type:               operatorsv1.Progressing,
		Reason:             "Working",
		Status:             v1.ConditionTrue,
		LastTransitionTime: v1.NewTime(time.Date(2020, 5, 29, 0, 0, 0, 0, time.UTC)),
	}
	old2 = operatorsv1.HubCondition{
		Type:               operatorsv1.Complete,
		Reason:             "EverythingRunning",
		Status:             v1.ConditionTrue,
		LastTransitionTime: v1.NewTime(time.Date(2020, 5, 29, 0, 0, 0, 0, time.UTC)),
	}
	new = operatorsv1.HubCondition{
		Type:               operatorsv1.Progressing,
		Reason:             "NotWorking",
		Status:             v1.ConditionFalse,
		LastTransitionTime: v1.NewTime(time.Date(2020, 5, 29, 0, 1, 0, 0, time.UTC)),
	}
	new2 = operatorsv1.HubCondition{
		Type:               operatorsv1.Complete,
		Reason:             "EverythingRunning",
		Status:             v1.ConditionTrue,
		LastTransitionTime: v1.NewTime(time.Date(2020, 5, 29, 0, 1, 0, 0, time.UTC)),
	}
)

func TestSetHubCondition(t *testing.T) {
	t.Run("Add single hubcondition", func(t *testing.T) {
		m := &operatorsv1.MultiClusterHub{}
		SetHubCondition(&m.Status, old)
		if len(m.Status.HubConditions) < 1 {
			t.Errorf("AddCondition() failed to add a HubCondition")
		}
	})

	t.Run("No duplicate hubconditions", func(t *testing.T) {
		m := &operatorsv1.MultiClusterHub{}
		expected := 1
		for i := 0; i < 2; i++ {
			SetHubCondition(&m.Status, old)
		}
		if len(m.Status.HubConditions) != 1 {
			t.Errorf("AddCondition() added duplicate hub conditions; expected %d, got %d", expected, len(m.Status.HubConditions))
		}
	})

	t.Run("Retain last transition time", func(t *testing.T) {
		m := &operatorsv1.MultiClusterHub{}
		SetHubCondition(&m.Status, old2)
		SetHubCondition(&m.Status, new2)
		if len(m.Status.HubConditions) != 1 {
			t.Errorf("AddCondition() too many hub conditions; expected %d, got %d", 1, len(m.Status.HubConditions))
		}
		if ltt := &m.Status.HubConditions[0].LastTransitionTime; !ltt.Equal(&old2.LastTransitionTime) {
			t.Errorf("AddCondition() expected lastTransitionTime of %v, got %v", old2.LastTransitionTime, ltt)
		}
	})
}

func TestGetHubCondition(t *testing.T) {
	testStatus := operatorsv1.MultiClusterHubStatus{
		HubConditions: []operatorsv1.HubCondition{new},
	}
	tests := []struct {
		name     string
		status   operatorsv1.MultiClusterHubStatus
		condType operatorsv1.HubConditionType
		want     bool
	}{
		{
			name:     "present",
			status:   testStatus,
			condType: operatorsv1.Progressing,
			want:     true,
		},
		{
			name:     "absent",
			status:   testStatus,
			condType: operatorsv1.Terminating,
			want:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := GetHubCondition(tt.status, tt.condType)
			exists := cond != nil
			if exists != tt.want {
				t.Errorf("%s: expected condition to exist: %t, got: %t", tt.name, tt.want, exists)
			}
		})
	}
}

func TestRemoveHubCondition(t *testing.T) {
	tests := []struct {
		name     string
		status   *operatorsv1.MultiClusterHubStatus
		condType operatorsv1.HubConditionType
		want     *operatorsv1.MultiClusterHubStatus
	}{
		{
			name: "empty status",

			status:   &operatorsv1.MultiClusterHubStatus{},
			condType: operatorsv1.Progressing,

			want: &operatorsv1.MultiClusterHubStatus{},
		},
		{
			name: "remove conditions",

			status:   &operatorsv1.MultiClusterHubStatus{HubConditions: []operatorsv1.HubCondition{new}},
			condType: operatorsv1.Progressing,

			want: &operatorsv1.MultiClusterHubStatus{},
		},
		{
			name: "don't remove condition",

			status:   &operatorsv1.MultiClusterHubStatus{HubConditions: []operatorsv1.HubCondition{new}},
			condType: operatorsv1.Complete,

			want: &operatorsv1.MultiClusterHubStatus{HubConditions: []operatorsv1.HubCondition{new}},
		}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RemoveHubCondition(tt.status, tt.condType)
			if !reflect.DeepEqual(tt.status, tt.want) {
				t.Errorf("latestHelmReleaseCondition() = %v, want %v", tt.status, tt.want)
			}
		})
	}
}

func Test_filterDuplicateHRs(t *testing.T) {
	tests := []struct {
		name     string
		allHRs   []*subrelv1.HelmRelease
		count    int
		excludes []string
	}{
		{
			name: "All helmrelease owner references unique",
			allHRs: []*subrelv1.HelmRelease{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "topology-foo",
						CreationTimestamp: v1.NewTime(time.Date(2020, 5, 29, 0, 0, 0, 0, time.UTC)),
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "apps.open-cluster-management.io/v1",
								Kind:       "Subscription",
								Name:       "topology-sub",
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "kui-web-terminal-foo",
						CreationTimestamp: v1.NewTime(time.Date(2020, 5, 29, 0, 1, 0, 0, time.UTC)),
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "apps.open-cluster-management.io/v1",
								Kind:       "Subscription",
								Name:       "kui-web-terminal-sub",
							},
						},
					},
				},
			},
			count:    2,
			excludes: []string{},
		},
		{
			name: "Two helmreleases with same owning appsub",
			allHRs: []*subrelv1.HelmRelease{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "topology-foo",
						CreationTimestamp: v1.NewTime(time.Date(2020, 5, 29, 0, 0, 0, 0, time.UTC)),
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "apps.open-cluster-management.io/v1",
								Kind:       "Subscription",
								Name:       "topology-sub",
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "topology-bar",
						CreationTimestamp: v1.NewTime(time.Date(2020, 5, 29, 0, 1, 0, 0, time.UTC)),
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "apps.open-cluster-management.io/v1",
								Kind:       "Subscription",
								Name:       "topology-sub",
							},
						},
					},
				},
			},
			count:    1,
			excludes: []string{"topology-foo"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterDuplicateHRs(tt.allHRs)
			if len(got) != tt.count {
				t.Errorf("filterDuplicateHRs() returned %d helmreleases, want %d", len(got), tt.count)
			}
			for i := range got {
				for _, name := range tt.excludes {
					if got[i].Name == name {
						t.Errorf("filterDuplicateHRs() should have filtered helmrelease %s", name)
					}
				}
			}
		})
	}
}

func Test_aggregatePhase(t *testing.T) {
	available := operatorsv1.StatusCondition{Type: "Available", Status: v1.ConditionTrue, Available: true}
	unavailable := operatorsv1.StatusCondition{Type: "Available", Status: v1.ConditionFalse}

	tests := []struct {
		name   string
		status operatorsv1.MultiClusterHubStatus
		want   operatorsv1.HubPhaseType
	}{
		{
			name: "Running hub with previous version",
			status: operatorsv1.MultiClusterHubStatus{
				CurrentVersion: "1.0.0",
				Components: map[string]operatorsv1.StatusCondition{
					"foo": available,
				},
			},
			want: operatorsv1.HubRunning,
		},
		{
			name: "Running hub with current version",
			status: operatorsv1.MultiClusterHubStatus{
				CurrentVersion: version.Version,
				Components: map[string]operatorsv1.StatusCondition{
					"foo": available,
				},
			},
			want: operatorsv1.HubRunning,
		},
		{
			name: "Progressing hub with previous version",
			status: operatorsv1.MultiClusterHubStatus{
				CurrentVersion: "1.0.0",
				Components: map[string]operatorsv1.StatusCondition{
					"foo": unavailable,
				},
			},
			want: operatorsv1.HubUpdating,
		},
		{
			name: "Progressing hub with current version",
			status: operatorsv1.MultiClusterHubStatus{
				CurrentVersion: version.Version,
				Components: map[string]operatorsv1.StatusCondition{
					"foo": unavailable,
				},
			},
			want: operatorsv1.HubPending,
		},
		{
			name: "Progressing hub with no version",
			status: operatorsv1.MultiClusterHubStatus{
				CurrentVersion: "",
				Components: map[string]operatorsv1.StatusCondition{
					"foo": unavailable,
				},
			},
			want: operatorsv1.HubInstalling,
		},
		{
			name: "Running hub with no version",
			status: operatorsv1.MultiClusterHubStatus{
				CurrentVersion: "",
				Components: map[string]operatorsv1.StatusCondition{
					"foo": available,
				},
			},
			want: operatorsv1.HubRunning,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := aggregatePhase(tt.status); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("aggregatePhase() = %v, want %v", got, tt.want)
			}
		})
	}
}
