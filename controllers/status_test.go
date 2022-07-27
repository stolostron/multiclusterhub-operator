// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/version"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	subhelmv1 "open-cluster-management.io/multicloud-operators-subscription/pkg/apis/apps/helmrelease/v1"
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
	first := subhelmv1.HelmAppCondition{
		Type:               subhelmv1.ConditionInitialized,
		LastTransitionTime: v1.NewTime(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)),
	}
	second := subhelmv1.HelmAppCondition{
		Type:               subhelmv1.ConditionDeployed,
		LastTransitionTime: v1.NewTime(time.Date(3000, 1, 1, 0, 0, 0, 0, time.UTC)),
	}
	type args struct {
		conditions []subhelmv1.HelmAppCondition
	}
	tests := []struct {
		name string
		args args
		want subhelmv1.HelmAppCondition
	}{
		{
			name: "Deployed after initialized",
			args: args{
				conditions: []subhelmv1.HelmAppCondition{
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
			"message": "ReplicaSet \"cluster-lifecycle-78c4bc769\" has timed out progressing.",
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
		allHRs   []*subhelmv1.HelmRelease
		count    int
		excludes []string
	}{
		{
			name: "All helmrelease owner references unique",
			allHRs: []*subhelmv1.HelmRelease{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "console-foo",
						CreationTimestamp: v1.NewTime(time.Date(2020, 5, 29, 0, 0, 0, 0, time.UTC)),
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "apps.open-cluster-management.io/v1",
								Kind:       "Subscription",
								Name:       "console-sub",
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "cluster-lifecycle-foo",
						CreationTimestamp: v1.NewTime(time.Date(2020, 5, 29, 0, 1, 0, 0, time.UTC)),
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "apps.open-cluster-management.io/v1",
								Kind:       "Subscription",
								Name:       "cluster-lifecycle-sub",
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
			allHRs: []*subhelmv1.HelmRelease{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "console-foo",
						CreationTimestamp: v1.NewTime(time.Date(2020, 5, 29, 0, 0, 0, 0, time.UTC)),
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "apps.open-cluster-management.io/v1",
								Kind:       "Subscription",
								Name:       "console-sub",
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "console-bar",
						CreationTimestamp: v1.NewTime(time.Date(2020, 5, 29, 0, 1, 0, 0, time.UTC)),
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "apps.open-cluster-management.io/v1",
								Kind:       "Subscription",
								Name:       "console-sub",
							},
						},
					},
				},
			},
			count:    1,
			excludes: []string{"console-foo"},
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

func Test_mapCSV(t *testing.T) {
	tests := []struct {
		name string
		csv  *olmv1alpha1.ClusterServiceVersion
		want operatorsv1.StatusCondition
	}{
		{
			name: "Successful install",
			csv: &olmv1alpha1.ClusterServiceVersion{
				Status: olmv1alpha1.ClusterServiceVersionStatus{
					Conditions: []olmv1alpha1.ClusterServiceVersionCondition{
						{
							Phase:   olmv1alpha1.CSVPhaseSucceeded,
							Message: "Success",
							Reason:  olmv1alpha1.CSVReasonInstallSuccessful,
						},
					},
				},
			},
			want: operatorsv1.StatusCondition{
				Kind:      "ClusterServiceVersion",
				Status:    metav1.ConditionTrue,
				Reason:    "InstallSucceeded",
				Message:   "Success",
				Type:      "Available",
				Available: true,
			},
		},
		{
			name: "No status reported",
			csv: &olmv1alpha1.ClusterServiceVersion{
				Status: olmv1alpha1.ClusterServiceVersionStatus{},
			},
			want: unknownStatus,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			csv, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tt.csv)
			if err != nil {
				t.Errorf("error converting csv: %v", err)
			}
			u := &unstructured.Unstructured{Object: csv}
			got := mapCSV(u)

			if got.Kind != tt.want.Kind {
				t.Errorf("mapCSV() = %v, want %v", got, tt.want)
			}
			if string(got.Status) != string(tt.want.Status) {
				t.Errorf("mapCSV() = %v, want %v", got, tt.want)
			}
			if got.Reason != tt.want.Reason {
				t.Errorf("mapCSV() = %v, want %v", got, tt.want)
			}
			if got.Message != tt.want.Message {
				t.Errorf("mapCSV() = %v, want %v", got, tt.want)
			}
			if got.Available != tt.want.Available {
				t.Errorf("mapCSV() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_mapMCE(t *testing.T) {
	tests := []struct {
		name string
		mce  *mcev1.MultiClusterEngine
		want operatorsv1.StatusCondition
	}{
		{
			name: "Successful install",
			mce: &mcev1.MultiClusterEngine{
				Status: mcev1.MultiClusterEngineStatus{
					Conditions: []mcev1.MultiClusterEngineCondition{
						{
							Type:    mcev1.MultiClusterEngineAvailable,
							Status:  metav1.ConditionTrue,
							Reason:  "Available",
							Message: "",
						},
					},
				},
			},
			want: operatorsv1.StatusCondition{
				Kind:      "ClusterServiceVersion",
				Status:    metav1.ConditionTrue,
				Reason:    "Available",
				Message:   "",
				Type:      "Available",
				Available: true,
			},
		},
		{
			name: "No status reported",
			mce: &mcev1.MultiClusterEngine{
				Status: mcev1.MultiClusterEngineStatus{
					Conditions: []mcev1.MultiClusterEngineCondition{},
				},
			},
			want: unknownStatus,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mce, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tt.mce)
			if err != nil {
				t.Errorf("error converting mce: %v", err)
			}
			u := &unstructured.Unstructured{Object: mce}
			got := mapMultiClusterEngine(u)

			if got.Kind != tt.want.Kind {
				t.Errorf("mapMultiClusterEngine() = %v, want %v", got, tt.want)
			}
			if string(got.Status) != string(tt.want.Status) {
				t.Errorf("mapMultiClusterEngine() = %v, want %v", got, tt.want)
			}
			if got.Reason != tt.want.Reason {
				t.Errorf("mapMultiClusterEngine() = %v, want %v", got, tt.want)
			}
			if got.Message != tt.want.Message {
				t.Errorf("mapMultiClusterEngine() = %v, want %v", got, tt.want)
			}
			if got.Available != tt.want.Available {
				t.Errorf("mapMultiClusterEngine() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_mapSubscription(t *testing.T) {
	tests := []struct {
		name string
		sub  *olmv1alpha1.Subscription
		want operatorsv1.StatusCondition
	}{
		{
			name: "Has installPlanRef",
			sub: &olmv1alpha1.Subscription{
				Spec: &olmv1alpha1.SubscriptionSpec{
					InstallPlanApproval: olmv1alpha1.ApprovalManual,
				},
				Status: olmv1alpha1.SubscriptionStatus{
					InstallPlanRef: &corev1.ObjectReference{
						Kind:      "InstallPlan",
						Name:      "test",
						Namespace: "test",
					},
					State: olmv1alpha1.SubscriptionState("AtLatestKnown"),
				},
			},
			want: operatorsv1.StatusCondition{
				Kind:      "Subscription",
				Status:    metav1.ConditionTrue,
				Reason:    "AtLatestKnown",
				Message:   "installPlanApproval: Manual. installPlan: test/test",
				Type:      "Available",
				Available: true,
			},
		},
		{
			name: "No status reported",
			sub: &olmv1alpha1.Subscription{
				Status: olmv1alpha1.SubscriptionStatus{},
			},
			want: unknownStatus,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tt.sub)
			if err != nil {
				t.Errorf("error converting subscription: %v", err)
			}
			u := &unstructured.Unstructured{Object: sub}
			got := mapSubscription(u)

			if got.Kind != tt.want.Kind {
				t.Errorf("mapSubscription() = %v, want %v", got, tt.want)
			}
			if string(got.Status) != string(tt.want.Status) {
				t.Errorf("mapSubscription() = %v, want %v", got, tt.want)
			}
			if got.Reason != tt.want.Reason {
				t.Errorf("mapSubscription() = %v, want %v", got, tt.want)
			}
			if got.Message != tt.want.Message {
				t.Errorf("mapSubscription() = %v, want %v", got, tt.want)
			}
			if got.Available != tt.want.Available {
				t.Errorf("mapSubscription() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_mapHelmRelease(t *testing.T) {
	tests := []struct {
		name string
		hr   *subhelmv1.HelmRelease
		want operatorsv1.StatusCondition
	}{
		{
			name: "Has installPlanRef",
			hr: &subhelmv1.HelmRelease{
				Repo: subhelmv1.HelmReleaseRepo{
					Version: version.Version,
				},
				Status: subhelmv1.HelmAppStatus{
					Conditions: []subhelmv1.HelmAppCondition{
						{
							Type:   subhelmv1.ConditionDeployed,
							Status: subhelmv1.StatusTrue,
							Reason: subhelmv1.ReasonInstallSuccessful,
						},
					},
				},
			},
			want: operatorsv1.StatusCondition{
				Kind:      "HelmRelease",
				Status:    metav1.ConditionTrue,
				Reason:    "InstallSuccessful",
				Message:   "",
				Type:      "Deployed",
				Available: true,
			},
		},
		{
			name: "No status reported",
			hr: &subhelmv1.HelmRelease{
				Status: subhelmv1.HelmAppStatus{},
			},
			want: unknownStatus,
		},
		{
			name: "Wrong version",
			hr: &subhelmv1.HelmRelease{
				Repo: subhelmv1.HelmReleaseRepo{
					Version: "0.0.0",
				},
				Status: subhelmv1.HelmAppStatus{
					Conditions: []subhelmv1.HelmAppCondition{
						{
							Type:   subhelmv1.ConditionDeployed,
							Status: subhelmv1.StatusTrue,
							Reason: subhelmv1.ReasonInstallSuccessful,
						},
					},
				},
			},
			want: wrongVersionStatus,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapHelmRelease(tt.hr)

			if got.Kind != tt.want.Kind {
				t.Errorf("mapSubscription() = %v, want %v", got, tt.want)
			}
			if string(got.Status) != string(tt.want.Status) {
				t.Errorf("mapSubscription() = %v, want %v", got, tt.want)
			}
			if got.Reason != tt.want.Reason {
				t.Errorf("mapSubscription() = %v, want %v", got, tt.want)
			}
			if got.Available != tt.want.Available {
				t.Errorf("mapSubscription() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getComponentStatuses(t *testing.T) {
	mce := &mcev1.MultiClusterEngine{
		Status: mcev1.MultiClusterEngineStatus{
			Conditions: []mcev1.MultiClusterEngineCondition{
				{
					Type:    mcev1.MultiClusterEngineAvailable,
					Status:  metav1.ConditionTrue,
					Reason:  "Available",
					Message: "",
				},
			},
		},
	}
	mceU, err := runtime.DefaultUnstructuredConverter.ToUnstructured(mce)
	if err != nil {
		t.Errorf("error converting mce: %v", err)
	}
	mceUnstructured := &unstructured.Unstructured{Object: mceU}

	type args struct {
		hub                 *operatorsv1.MultiClusterHub
		allHRs              []*subhelmv1.HelmRelease
		allDeps             []*appsv1.Deployment
		allCRs              []*unstructured.Unstructured
		importClusterStatus []interface{}
	}
	tests := []struct {
		name string
		args args
		want map[string]operatorsv1.StatusCondition
	}{
		{
			name: "1",
			args: args{
				hub: &operatorsv1.MultiClusterHub{
					Spec: operatorsv1.MultiClusterHubSpec{
						Overrides: &operatorsv1.Overrides{
							Components: []operatorsv1.ComponentConfig{
								{
									Name:    "console",
									Enabled: true,
								},
							},
						},
					},
				},
				allHRs: []*subhelmv1.HelmRelease{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "console-hr",
							OwnerReferences: []v1.OwnerReference{
								{
									Name: "console-chart-sub",
								},
							},
						},
						Repo: subhelmv1.HelmReleaseRepo{
							Version: version.Version,
						},
						Status: subhelmv1.HelmAppStatus{
							Conditions: []subhelmv1.HelmAppCondition{
								{
									Type:   subhelmv1.ConditionDeployed,
									Status: subhelmv1.StatusTrue,
									Reason: subhelmv1.ReasonInstallSuccessful,
								},
							},
						},
					},
				},
				allDeps: []*appsv1.Deployment{
					{
						ObjectMeta: v1.ObjectMeta{
							Annotations: map[string]string{
								"meta.helm.sh/release-name": "console-hr",
							},
						},
						Status: appsv1.DeploymentStatus{
							Conditions: []appsv1.DeploymentCondition{
								{
									Type:   appsv1.DeploymentAvailable,
									Status: corev1.ConditionFalse,
									Reason: "Available",
								},
							},
						},
					},
				},
				allCRs:              []*unstructured.Unstructured{mceUnstructured},
				importClusterStatus: nil,
			},
			want: map[string]operatorsv1.StatusCondition{
				"local-cluster": unknownStatus,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getComponentStatuses(tt.args.hub, tt.args.allHRs, tt.args.allDeps, tt.args.allCRs, tt.args.importClusterStatus); len(got) == 0 {
				t.Errorf("getComponentStatuses() = %v, want %v", got, tt.want)
			}
		})
	}
}
