// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"testing"

	consolev1 "github.com/openshift/api/console/v1"
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newConsoleNotificationReconciler() *MultiClusterHubReconciler {
	_ = consolev1.AddToScheme(k8sscheme.Scheme)
	return &MultiClusterHubReconciler{
		Client: fake.NewClientBuilder().WithScheme(k8sscheme.Scheme).Build(),
		Scheme: k8sscheme.Scheme,
	}
}

func testHub() *operatorsv1.MultiClusterHub {
	return &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multiclusterhub",
			Namespace: "open-cluster-management",
		},
	}
}

func TestMCEComplianceBannerText(t *testing.T) {
	tests := []struct {
		name           string
		currentVersion string
		expected       string
	}{
		{
			name:           "ahead",
			currentVersion: "5.0.0",
			expected:       "WARNING: ACM in unexpected configuration: MCE 5.0.0 is ahead of the expected stable-2.9 channel.",
		},
		{
			name:           "behind",
			currentVersion: "2.8.0",
			expected:       "WARNING: ACM in unexpected configuration: MCE 2.8.0 is behind the expected stable-2.9 channel.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mceComplianceBannerText(tt.currentVersion, "stable-2.9"); got != tt.expected {
				t.Errorf("mceComplianceBannerText() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestEnsureMCEComplianceBanner_CreatesBanner(t *testing.T) {
	reconciler := newConsoleNotificationReconciler()
	hub := testHub()
	compliance := &operatorsv1.MCEVersionComplianceStatus{
		RequiredChannel: "stable-2.9",
		CurrentVersion:  "5.0.0",
	}

	if err := reconciler.ensureMCEComplianceBanner(context.Background(), hub, compliance); err != nil {
		t.Fatalf("ensureMCEComplianceBanner() error = %v", err)
	}

	notification := &consolev1.ConsoleNotification{}
	if err := reconciler.Client.Get(context.Background(), types.NamespacedName{Name: mceComplianceBannerName}, notification); err != nil {
		t.Fatalf("failed to get ConsoleNotification: %v", err)
	}

	if notification.Spec.Text != mceComplianceBannerText("5.0.0", "stable-2.9") {
		t.Errorf("banner text = %q, want mismatch warning", notification.Spec.Text)
	}
	if notification.Spec.Location != consolev1.BannerTop {
		t.Errorf("banner location = %q, want %q", notification.Spec.Location, consolev1.BannerTop)
	}
	if notification.Spec.BackgroundColor != bannerBackgroundColor {
		t.Errorf("banner background color = %q, want %q", notification.Spec.BackgroundColor, bannerBackgroundColor)
	}
	if notification.Spec.Color != bannerTextColor {
		t.Errorf("banner text color = %q, want %q", notification.Spec.Color, bannerTextColor)
	}
	if notification.Spec.Link == nil || notification.Spec.Link.Text != bannerSupportLinkText || notification.Spec.Link.Href != bannerSupportLinkHref {
		t.Errorf("banner support link = %#v, want %q (%s)", notification.Spec.Link, bannerSupportLinkText, bannerSupportLinkHref)
	}
	if notification.Labels["installer.name"] != hub.Name || notification.Labels["installer.namespace"] != hub.Namespace {
		t.Errorf("banner labels = %#v, want installer labels for %s/%s", notification.Labels, hub.Namespace, hub.Name)
	}
}

func TestEnsureMCEComplianceBanner_CompliantRemovesBanner(t *testing.T) {
	reconciler := newConsoleNotificationReconciler()
	ctx := context.Background()
	hub := testHub()

	if err := reconciler.ensureMCEComplianceBanner(ctx, hub, &operatorsv1.MCEVersionComplianceStatus{
		RequiredChannel: "stable-2.9",
		CurrentVersion:  "5.0.0",
	}); err != nil {
		t.Fatalf("failed to create initial banner: %v", err)
	}

	if err := reconciler.ensureMCEComplianceBanner(ctx, hub, &operatorsv1.MCEVersionComplianceStatus{
		RequiredChannel: "stable-2.9",
		CurrentVersion:  "2.14.0",
		IsCompliant:     true,
	}); err != nil {
		t.Fatalf("ensureMCEComplianceBanner() error = %v", err)
	}

	err := reconciler.Client.Get(ctx, types.NamespacedName{Name: mceComplianceBannerName}, &consolev1.ConsoleNotification{})
	if !apierrors.IsNotFound(err) {
		t.Errorf("expected banner to be deleted, got error %v", err)
	}
}

func TestEnsureMCEComplianceBanner_EmptyVersionRemovesBanner(t *testing.T) {
	reconciler := newConsoleNotificationReconciler()
	ctx := context.Background()
	hub := testHub()

	if err := reconciler.ensureMCEComplianceBanner(ctx, hub, &operatorsv1.MCEVersionComplianceStatus{
		RequiredChannel: "stable-2.9",
		CurrentVersion:  "5.0.0",
	}); err != nil {
		t.Fatalf("failed to create initial banner: %v", err)
	}

	if err := reconciler.ensureMCEComplianceBanner(ctx, hub, &operatorsv1.MCEVersionComplianceStatus{
		RequiredChannel: "stable-2.9",
	}); err != nil {
		t.Fatalf("ensureMCEComplianceBanner() error = %v", err)
	}

	err := reconciler.Client.Get(ctx, types.NamespacedName{Name: mceComplianceBannerName}, &consolev1.ConsoleNotification{})
	if !apierrors.IsNotFound(err) {
		t.Errorf("expected banner to be deleted, got error %v", err)
	}
}

func TestEnsureMCEComplianceBanner_UpdatesBanner(t *testing.T) {
	reconciler := newConsoleNotificationReconciler()
	ctx := context.Background()
	hub := testHub()

	if err := reconciler.ensureMCEComplianceBanner(ctx, hub, &operatorsv1.MCEVersionComplianceStatus{
		RequiredChannel: "stable-2.9",
		CurrentVersion:  "5.0.0",
	}); err != nil {
		t.Fatalf("failed to create initial banner: %v", err)
	}

	hub.Name = "other-hub"
	hub.Namespace = "other-namespace"
	if err := reconciler.ensureMCEComplianceBanner(ctx, hub, &operatorsv1.MCEVersionComplianceStatus{
		RequiredChannel: "stable-2.9",
		CurrentVersion:  "2.8.0",
	}); err != nil {
		t.Fatalf("ensureMCEComplianceBanner() error = %v", err)
	}

	notification := &consolev1.ConsoleNotification{}
	if err := reconciler.Client.Get(ctx, types.NamespacedName{Name: mceComplianceBannerName}, notification); err != nil {
		t.Fatalf("failed to get updated ConsoleNotification: %v", err)
	}
	if notification.Spec.Text != mceComplianceBannerText("2.8.0", "stable-2.9") {
		t.Errorf("updated banner text = %q, want mismatch warning", notification.Spec.Text)
	}
	if notification.Labels["installer.name"] != hub.Name || notification.Labels["installer.namespace"] != hub.Namespace {
		t.Errorf("updated banner labels = %#v, want installer labels for %s/%s", notification.Labels, hub.Namespace, hub.Name)
	}
}

func TestRemoveMCEComplianceBanner_NoBanner(t *testing.T) {
	reconciler := newConsoleNotificationReconciler()
	if err := reconciler.removeMCEComplianceBanner(context.Background()); err != nil {
		t.Fatalf("removeMCEComplianceBanner() error = %v", err)
	}
}
