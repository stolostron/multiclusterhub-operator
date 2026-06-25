// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package multiclusterengine

import (
	"context"
	"reflect"
	"testing"

	"github.com/onsi/gomega"
	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNewSubscription(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	mch := &operatorv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mch",
			Namespace: "mch-ns",
		},
	}
	config := &subv1alpha1.SubscriptionConfig{
		NodeSelector: map[string]string{"test": "test"},
	}
	overrides := &subv1alpha1.SubscriptionSpec{
		CatalogSource:          "custom",
		CatalogSourceNamespace: "custom",
		Package:                "custom",
		Channel:                "custom",
		StartingCSV:            "custom",
		InstallPlanApproval:    subv1alpha1.ApprovalManual,
	}

	got := NewSubscription(mch, config, nil, false)
	g.Expect(got.Labels["installer.name"]).To(gomega.Not(gomega.Equal("")), "New MCE subscription should have installer labels")
	g.Expect(got.Labels["installer.namespace"]).To(gomega.Not(gomega.Equal("")), "New MCE subscription should have installer labels")

	got = NewSubscription(mch, config, nil, true)
	g.Expect(got.Spec.Channel).To(gomega.Equal(communityChannel), "Use community values when in community mode")

	got = NewSubscription(mch, config, overrides, false)
	g.Expect(got.Spec.CatalogSource).To(gomega.Equal("custom"), "Overrides values should take priority")
	g.Expect(got.Spec.CatalogSourceNamespace).To(gomega.Equal("custom"), "Overrides values should take priority")
	g.Expect(got.Spec.Package).To(gomega.Equal("custom"), "Overrides values should take priority")
	g.Expect(got.Spec.Channel).To(gomega.Equal("custom"), "Overrides values should take priority")
	g.Expect(got.Spec.StartingCSV).To(gomega.Equal("custom"), "Overrides values should take priority")
	g.Expect(got.Spec.InstallPlanApproval).To(gomega.Equal(subv1alpha1.ApprovalManual), "Overrides values should take priority")
}

func TestRenderSubscription(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	mch := &operatorv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mch",
			Namespace: "mch-ns",
		},
	}
	config := &subv1alpha1.SubscriptionConfig{
		NodeSelector: map[string]string{"test": "test"},
	}
	overrides := &subv1alpha1.SubscriptionSpec{
		CatalogSource:          "custom",
		CatalogSourceNamespace: "custom",
		Package:                "custom",
		Channel:                "custom",
		StartingCSV:            "custom",
		InstallPlanApproval:    subv1alpha1.ApprovalManual,
	}

	existing := NewSubscription(mch, config, nil, false)
	existing.Spec.StartingCSV = "0.0.1"

	got := RenderSubscription(existing, config, nil, types.NamespacedName{}, true)
	g.Expect(existing.Labels).To(gomega.Equal(got.Labels), "RenderSubscription should not change metadata of subscription")
	g.Expect(got.Spec.Channel).To(gomega.Equal(communityChannel), "Community values replace existing ones")
	g.Expect(got.Spec.StartingCSV).To(gomega.Equal(""), "Changing the channel should scrub the startingCSV")

	got = RenderSubscription(existing, config, nil, types.NamespacedName{Name: "test", Namespace: "test"}, true)
	g.Expect(got.Spec.CatalogSource).To(gomega.Equal("test"), "catalogSource values are set")
	g.Expect(got.Spec.CatalogSourceNamespace).To(gomega.Equal("test"), "catalogSource values are set")

	got = RenderSubscription(existing, config, overrides, types.NamespacedName{Name: "test", Namespace: "test"}, true)
	g.Expect(got.Spec.CatalogSource).To(gomega.Equal("custom"), "Overrides values should take priority")
	g.Expect(got.Spec.CatalogSourceNamespace).To(gomega.Equal("custom"), "Overrides values should take priority")
	g.Expect(got.Spec.Package).To(gomega.Equal("custom"), "Overrides values should take priority")
	g.Expect(got.Spec.Channel).To(gomega.Equal("custom"), "Overrides values should take priority")
	g.Expect(got.Spec.StartingCSV).To(gomega.Equal("custom"), "Overrides values should take priority")
	g.Expect(got.Spec.InstallPlanApproval).To(gomega.Equal(subv1alpha1.ApprovalManual), "Overrides values should take priority")

}

func TestGetAnnotationOverrides(t *testing.T) {
	mch := &operatorv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec:       operatorv1.MultiClusterHubSpec{},
	}

	tests := []struct {
		name        string
		annotations map[string]string
		want        *subv1alpha1.SubscriptionSpec
		wantErr     bool
	}{
		{
			name: "All MCE annotations",
			annotations: map[string]string{
				"installer.open-cluster-management.io/mce-subscription-spec": `{"channel": "channel-1.0","installPlanApproval": "Manual","name":
				  "package","source": "catalogsource","sourceNamespace": "catalogsourcenamespace","startingCSV":
				  "csv-1.0"}`,
			},
			want: &subv1alpha1.SubscriptionSpec{
				Channel:                "channel-1.0",
				InstallPlanApproval:    subv1alpha1.ApprovalManual,
				Package:                "package",
				CatalogSource:          "catalogsource",
				CatalogSourceNamespace: "catalogsourcenamespace",
				StartingCSV:            "csv-1.0",
			},
			wantErr: false,
		},
		{
			name: "Invalid annotation",
			annotations: map[string]string{
				"installer.open-cluster-management.io/mce-subscription-spec": `{"channel-1.0"}`,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "No annotation",
			annotations: map[string]string{
				"installer.name": "test",
			},
			want:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mch.SetAnnotations(tt.annotations)
			got, err := GetAnnotationOverrides(mch)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAnnotationOverrides() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetAnnotationOverrides() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindAndManageMCESubscription(t *testing.T) {

	managedSub1 := &subv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mce-sub",
			Namespace: "mce",
			Labels: map[string]string{
				utils.MCEManagedByLabel: "true",
			},
		},
	}
	managedSub2 := &subv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mce-sub2",
			Namespace: "mce2",
			Labels: map[string]string{
				utils.MCEManagedByLabel: "true",
			},
		},
	}
	unmanagedSub1 := &subv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mce-unsub",
			Namespace: "unmce",
		},
		Spec: &subv1alpha1.SubscriptionSpec{
			Package: communityPackageName,
		},
	}

	scheme := runtime.NewScheme()
	err := subv1alpha1.AddToScheme(scheme)
	if err != nil {
		t.Fatalf("Couldn't set up scheme")
	}

	// One good subscription
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithLists(&subv1alpha1.SubscriptionList{Items: []subv1alpha1.Subscription{*managedSub1}}).
		Build()

	got, err := FindAndManageMCESubscription(context.Background(), cl)
	if err != nil {
		t.Errorf("FindAndManageMCESubscription() should have found subscription by label. Got %v", err)
	}
	if got.Name != managedSub1.Name || got.Namespace != managedSub1.Namespace {
		t.Errorf("FindAndManageMCESubscription() return subscription %s, want %s", got.Name, managedSub1.Name)
	}

	// Conflicting subscriptions
	cl = fake.NewClientBuilder().
		WithScheme(scheme).
		WithLists(&subv1alpha1.SubscriptionList{Items: []subv1alpha1.Subscription{*managedSub1, *managedSub2}}).
		Build()

	_, err = FindAndManageMCESubscription(context.Background(), cl)
	if err == nil {
		t.Errorf("FindAndManageMCESubscription() should have errored due to multiple subscriptions")
	}

	// Eligible subscription without label
	cl = fake.NewClientBuilder().
		WithScheme(scheme).
		WithLists(&subv1alpha1.SubscriptionList{Items: []subv1alpha1.Subscription{*unmanagedSub1}}).
		Build()

	got, err = FindAndManageMCESubscription(context.Background(), cl)
	if err != nil {
		t.Errorf("FindAndManageMCESubscription() should have found subscription and labeled it. Got error %v", err)
	}
	if got.Name != unmanagedSub1.Name || got.Namespace != unmanagedSub1.Namespace {
		t.Errorf("FindAndManageMCESubscription() return subscription %s, want %s", got.Name, managedSub1.Name)
	}
	if got.Labels[utils.MCEManagedByLabel] != "true" {
		t.Errorf("FindAndManageMCESubscription() should have set the managed label on the subscription")
	}
	gotSub := &subv1alpha1.Subscription{}
	key := types.NamespacedName{Name: unmanagedSub1.Name, Namespace: unmanagedSub1.Namespace}
	err = cl.Get(context.Background(), key, gotSub)
	if err != nil {
		t.Errorf("Got error from mock client %v", err)
	}
	if gotSub.Labels[utils.MCEManagedByLabel] != "true" {
		t.Errorf("FindAndManageMCESubscription() should have updated the managed label on the subscription")
	}

}

func TestCreatedByMCH(t *testing.T) {
	mch := &operatorv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mch",
			Namespace: "mch-ns",
		},
	}
	tests := []struct {
		name string
		sub  *subv1alpha1.Subscription
		m    *operatorv1.MultiClusterHub
		want bool
	}{
		{
			name: "Created by MCH",
			sub: &subv1alpha1.Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"installer.name":      "mch",
						"installer.namespace": "mch-ns",
					},
				},
			},
			m:    mch,
			want: true,
		},
		{
			name: "Adopted by MCH",
			sub: &subv1alpha1.Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						utils.MCEManagedByLabel: "true",
					},
				},
			},
			m:    mch,
			want: false,
		},
		{
			name: "Unlabeled",
			sub: &subv1alpha1.Subscription{
				ObjectMeta: metav1.ObjectMeta{},
			},
			m:    mch,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CreatedByMCH(tt.sub, tt.m); got != tt.want {
				t.Errorf("CreatedByMCH() = %v, want %v", got, tt.want)
			}
		})
	}
}
