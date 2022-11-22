// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package subscription

import (
	"fmt"
	"reflect"
	"testing"

	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func TestValidate(t *testing.T) {
	mch := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: operatorsv1.MultiClusterHubSpec{
			ImagePullSecret:   "test",
			CustomCAConfigmap: "test-config",
		},
	}
	ovr := map[string]string{}

	// 1. Valid mch
	sub := ClusterLifecycle(mch, ovr)

	// 2. Modified ImagePullSecret
	mch1 := mch.DeepCopy()
	mch1.Spec.ImagePullSecret = "notTest"
	sub1 := ClusterLifecycle(mch1, ovr)

	// 3. Modified ImagePullPolicy
	mch2 := mch.DeepCopy()
	mch2.Spec.Overrides = &operatorsv1.Overrides{
		ImagePullPolicy: corev1.PullNever,
	}
	sub2 := ClusterLifecycle(mch2, ovr)

	// 4. Modified ImageRepository
	mch3 := mch.DeepCopy()
	mch3.SetAnnotations(map[string]string{utils.AnnotationImageRepo: "notquay.io/closed-cluster-management"})
	sub3 := ClusterLifecycle(mch3, ovr)

	// 5. Activate HA mode
	mch4 := mch.DeepCopy()
	mch4.Spec.AvailabilityConfig = operatorsv1.HABasic
	sub4 := ClusterLifecycle(mch4, ovr)

	// 6. Modified CustomCAConfigmap
	mch6 := mch.DeepCopy()
	mch6.Spec.CustomCAConfigmap = ""
	sub5 := ClusterLifecycle(mch6, ovr)

	type args struct {
		found *unstructured.Unstructured
		want  *unstructured.Unstructured
	}
	tests := []struct {
		name  string
		args  args
		want  *unstructured.Unstructured
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
			name:  "Deactivate HighAvailabilityConfig mode",
			args:  args{sub, sub4},
			want:  sub4,
			want1: true,
		},
		{
			name:  "Modified CustomCAConfigmap",
			args:  args{sub, sub5},
			want:  sub5,
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

func TestOADPAnnotation(t *testing.T) {
	oadp := `{"channel": "stable-1.0", "installPlanApproval": "Manual", "name": "redhat-oadp-operator2", "source": "redhat-operators2", "sourceNamespace": "openshift-marketplace2"}`
	mch := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Annotations: map[string]string{
				"installer.open-cluster-management.io/oadp-subscription-spec": oadp,
			},
		},
	}

	test1, test2, test3, test4, test5 := GetOADPConfig(mch)

	if test1 != "redhat-oadp-operator2" {
		t.Error(fmt.Sprintf("Cluster Backup missing OADP overrides for name"))
	}

	if test2 != "stable-1.0" {
		t.Error(fmt.Sprintf("Cluster Backup missing OADP overrides for channel"))
	}

	if test3 != "Manual" {
		t.Error(fmt.Sprintf("Cluster Backup missing OADP overrides for install plan"))
	}

	if test4 != "redhat-operators2" {
		t.Error(fmt.Sprintf("Cluster Backup missing OADP overrides for source"))
	}

	if test5 != "openshift-marketplace2" {
		t.Error(fmt.Sprintf("Cluster Backup missing OADP overrides for source namespace"))
	}

	mch = &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
		},
	}

	test1, test2, test3, test4, test5 = GetOADPConfig(mch)

	if test1 != "redhat-oadp-operator" {
		t.Error(fmt.Sprintf("Cluster Backup missing OADP overrides for name"))
	}

	if test2 != "stable-1.1" {
		t.Error(fmt.Sprintf("Cluster Backup missing OADP overrides for channel"))
	}

	if test3 != "Automatic" {
		t.Error(fmt.Sprintf("Cluster Backup missing OADP overrides for install plan"))
	}

	if test4 != "redhat-operators" {
		t.Error(fmt.Sprintf("Cluster Backup missing OADP overrides for source"))
	}

	if test5 != "openshift-marketplace" {
		t.Error(fmt.Sprintf("Cluster Backup missing OADP overrides for source namespace"))
	}
}

func TestSubscriptions(t *testing.T) {
	mch := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: operatorsv1.MultiClusterHubSpec{
			ImagePullSecret: "test",
		},
	}
	ovr := map[string]string{}

	tests := []struct {
		name string
		got  *unstructured.Unstructured
	}{
		{"Console subscription", Console(mch, ovr, "")},
		{"GRC subscription", GRC(mch, ovr)},
		{"Insights subscription", Insights(mch, ovr)},
		{"cluster-lifecycle subscription", ClusterLifecycle(mch, ovr)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.got.MarshalJSON(); err != nil {
				t.Error("Subscription does not marshal properly")
			}
			_, err := yaml.Marshal(tt.got.Object["spec"])
			if err != nil {
				t.Error("Issue parsing subscription values")
			}
		})
	}
}
