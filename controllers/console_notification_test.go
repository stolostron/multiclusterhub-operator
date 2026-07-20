// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"testing"

	consolev1 "github.com/openshift/api/console/v1"
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestMCEComplianceBannerText_Ahead(t *testing.T) {
	got := mceComplianceBannerText("5.1.0", "stable-5.0")
	expected := "WARNING: ACM in unexpected configuration: MCE 5.1.0 is ahead of the expected stable-5.0 channel."
	if got != expected {
		t.Errorf("mceComplianceBannerText() = %q, want %q", got, expected)
	}
}

func TestMCEComplianceBannerText_Behind(t *testing.T) {
	got := mceComplianceBannerText("2.17.0", "stable-5.0")
	expected := "WARNING: ACM in unexpected configuration: MCE 2.17.0 is behind the expected stable-5.0 channel."
	if got != expected {
		t.Errorf("mceComplianceBannerText() = %q, want %q", got, expected)
	}
}

func TestEnsureMCEComplianceBanner_NonCompliant(t *testing.T) {
	registerScheme()

	hub := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multiclusterhub",
			Namespace: "open-cluster-management",
		},
	}

	compliance := &operatorsv1.MCEVersionComplianceStatus{
		RequiredChannel: "stable-5.0",
		CurrentVersion:  "5.1.0",
		IsCompliant:     false,
		Message:         "MCE version 5.1.0 does not meet channel stable-5.0 requirements",
	}

	ctx := context.TODO()

	err := recon.ensureMCEComplianceBanner(ctx, hub, compliance)
	if err != nil {
		t.Fatalf("ensureMCEComplianceBanner() error = %v", err)
	}

	notification := &consolev1.ConsoleNotification{}
	err = recon.Client.Get(ctx, types.NamespacedName{Name: mceComplianceBannerName}, notification)
	if err != nil {
		t.Fatalf("failed to get ConsoleNotification: %v", err)
	}
	t.Cleanup(func() {
		_ = recon.Client.Delete(ctx, notification)
	})

	expectedText := mceComplianceBannerText("5.1.0", "stable-5.0")
	if notification.Spec.Text != expectedText {
		t.Errorf("banner text = %q, want %q", notification.Spec.Text, expectedText)
	}
	if notification.Spec.Location != consolev1.BannerTop {
		t.Errorf("banner location = %q, want %q", notification.Spec.Location, consolev1.BannerTop)
	}
	if notification.Spec.BackgroundColor != bannerBackgroundColor {
		t.Errorf("banner backgroundColor = %q, want %q", notification.Spec.BackgroundColor, bannerBackgroundColor)
	}
	if notification.Spec.Color != bannerTextColor {
		t.Errorf("banner color = %q, want %q", notification.Spec.Color, bannerTextColor)
	}
	if notification.Labels["installer.name"] != "multiclusterhub" {
		t.Errorf("label installer.name = %q, want %q", notification.Labels["installer.name"], "multiclusterhub")
	}
	if notification.Labels["installer.namespace"] != "open-cluster-management" {
		t.Errorf("label installer.namespace = %q, want %q", notification.Labels["installer.namespace"], "open-cluster-management")
	}
}

func TestEnsureMCEComplianceBanner_Compliant_RemovesBanner(t *testing.T) {
	registerScheme()
	ctx := context.TODO()

	hub := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multiclusterhub",
			Namespace: "open-cluster-management",
		},
	}

	existing := &consolev1.ConsoleNotification{
		ObjectMeta: metav1.ObjectMeta{
			Name: mceComplianceBannerName,
			Labels: map[string]string{
				"installer.name":      "multiclusterhub",
				"installer.namespace": "open-cluster-management",
			},
		},
		Spec: consolev1.ConsoleNotificationSpec{
			Text:            "old warning text",
			Location:        consolev1.BannerTop,
			BackgroundColor: bannerBackgroundColor,
			Color:           bannerTextColor,
		},
	}
	if err := recon.Client.Create(ctx, existing); err != nil {
		t.Fatalf("failed to create existing banner: %v", err)
	}
	t.Cleanup(func() {
		_ = recon.Client.Delete(ctx, &consolev1.ConsoleNotification{
			ObjectMeta: metav1.ObjectMeta{Name: mceComplianceBannerName},
		})
	})

	compliance := &operatorsv1.MCEVersionComplianceStatus{
		RequiredChannel: "stable-5.0",
		CurrentVersion:  "5.0.0",
		IsCompliant:     true,
		Message:         "MCE version 5.0.0 meets channel stable-5.0 requirements",
	}

	err := recon.ensureMCEComplianceBanner(ctx, hub, compliance)
	if err != nil {
		t.Fatalf("ensureMCEComplianceBanner() error = %v", err)
	}

	notification := &consolev1.ConsoleNotification{}
	err = recon.Client.Get(ctx, types.NamespacedName{Name: mceComplianceBannerName}, notification)
	if err == nil {
		t.Error("expected ConsoleNotification to be deleted when compliant, but it still exists")
	}
}

func TestEnsureMCEComplianceBanner_NilCompliance(t *testing.T) {
	registerScheme()
	ctx := context.TODO()

	hub := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multiclusterhub",
			Namespace: "open-cluster-management",
		},
	}

	err := recon.ensureMCEComplianceBanner(ctx, hub, nil)
	if err != nil {
		t.Fatalf("ensureMCEComplianceBanner(nil) error = %v", err)
	}
}

func TestEnsureMCEComplianceBanner_NoVersion_NoBanner(t *testing.T) {
	registerScheme()
	ctx := context.TODO()

	hub := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multiclusterhub",
			Namespace: "open-cluster-management",
		},
	}

	compliance := &operatorsv1.MCEVersionComplianceStatus{
		RequiredChannel: "stable-5.0",
		CurrentVersion:  "",
		IsCompliant:     false,
		Message:         "MCE not yet installed",
	}

	err := recon.ensureMCEComplianceBanner(ctx, hub, compliance)
	if err != nil {
		t.Fatalf("ensureMCEComplianceBanner() error = %v", err)
	}

	notification := &consolev1.ConsoleNotification{}
	err = recon.Client.Get(ctx, types.NamespacedName{Name: mceComplianceBannerName}, notification)
	if err == nil {
		t.Error("expected no ConsoleNotification when MCE version is empty")
	}
}

func TestEnsureMCEComplianceBanner_UpdatesExistingBanner(t *testing.T) {
	registerScheme()
	ctx := context.TODO()

	hub := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multiclusterhub",
			Namespace: "open-cluster-management",
		},
	}

	existing := &consolev1.ConsoleNotification{
		ObjectMeta: metav1.ObjectMeta{
			Name: mceComplianceBannerName,
			Labels: map[string]string{
				"installer.name":      "multiclusterhub",
				"installer.namespace": "open-cluster-management",
			},
		},
		Spec: consolev1.ConsoleNotificationSpec{
			Text:            mceComplianceBannerText("5.1.0", "stable-5.0"),
			Location:        consolev1.BannerTop,
			BackgroundColor: bannerBackgroundColor,
			Color:           bannerTextColor,
		},
	}
	if err := recon.Client.Create(ctx, existing); err != nil {
		t.Fatalf("failed to create existing banner: %v", err)
	}
	t.Cleanup(func() {
		_ = recon.Client.Delete(ctx, &consolev1.ConsoleNotification{
			ObjectMeta: metav1.ObjectMeta{Name: mceComplianceBannerName},
		})
	})

	newCompliance := &operatorsv1.MCEVersionComplianceStatus{
		RequiredChannel: "stable-5.0",
		CurrentVersion:  "5.2.0",
		IsCompliant:     false,
		Message:         "MCE version 5.2.0 does not meet channel stable-5.0 requirements",
	}

	err := recon.ensureMCEComplianceBanner(ctx, hub, newCompliance)
	if err != nil {
		t.Fatalf("ensureMCEComplianceBanner() error = %v", err)
	}

	notification := &consolev1.ConsoleNotification{}
	err = recon.Client.Get(ctx, types.NamespacedName{Name: mceComplianceBannerName}, notification)
	if err != nil {
		t.Fatalf("failed to get ConsoleNotification: %v", err)
	}

	expectedText := mceComplianceBannerText("5.2.0", "stable-5.0")
	if notification.Spec.Text != expectedText {
		t.Errorf("updated banner text = %q, want %q", notification.Spec.Text, expectedText)
	}
}

func TestRemoveMCEComplianceBanner_NoExistingBanner(t *testing.T) {
	registerScheme()
	ctx := context.TODO()

	err := recon.removeMCEComplianceBanner(ctx)
	if err != nil {
		t.Fatalf("removeMCEComplianceBanner() error = %v, expected nil for non-existent banner", err)
	}
}
