// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"testing"

	consolev1 "github.com/openshift/api/console/v1"
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/version"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// --- MCE banner text tests ---

func TestMCEComplianceBannerText_Ahead(t *testing.T) {
	got := mceComplianceBannerText("2.18.0", "stable-2.17")
	expected := "WARNING: ACM in unexpected configuration: MCE 2.18.0 is ahead of the expected stable-2.17 channel."
	if got != expected {
		t.Errorf("mceComplianceBannerText() = %q, want %q", got, expected)
	}
}

func TestMCEComplianceBannerText_Behind(t *testing.T) {
	got := mceComplianceBannerText("2.16.0", "stable-2.17")
	expected := "WARNING: ACM in unexpected configuration: MCE 2.16.0 is behind the expected stable-2.17 channel."
	if got != expected {
		t.Errorf("mceComplianceBannerText() = %q, want %q", got, expected)
	}
}

// --- OCP banner text tests ---

func TestOCPComplianceBannerText(t *testing.T) {
	got := ocpComplianceBannerText("4.18.0", "4.19.0")
	expected := "WARNING: ACM in unexpected configuration: OCP 4.18.0 is below the minimum supported version 4.19.0."
	if got != expected {
		t.Errorf("ocpComplianceBannerText() = %q, want %q", got, expected)
	}
}

// --- MCE banner lifecycle tests ---

func TestEnsureMCEComplianceBanner_NonCompliant(t *testing.T) {
	registerScheme()

	hub := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multiclusterhub",
			Namespace: "open-cluster-management",
		},
	}

	compliance := &operatorsv1.MCEVersionComplianceStatus{
		RequiredChannel: "stable-2.17",
		CurrentVersion:  "2.18.0",
		IsCompliant:     false,
		Message:         "MCE version 2.18.0 does not meet channel stable-2.17 requirements",
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

	expectedText := mceComplianceBannerText("2.18.0", "stable-2.17")
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
		RequiredChannel: "stable-2.17",
		CurrentVersion:  "2.17.0",
		IsCompliant:     true,
		Message:         "MCE version 2.17.0 meets channel stable-2.17 requirements",
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
		RequiredChannel: "stable-2.17",
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
			Text:            mceComplianceBannerText("2.18.0", "stable-2.17"),
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
		RequiredChannel: "stable-2.17",
		CurrentVersion:  "2.19.0",
		IsCompliant:     false,
		Message:         "MCE version 2.19.0 does not meet channel stable-2.17 requirements",
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

	expectedText := mceComplianceBannerText("2.19.0", "stable-2.17")
	if notification.Spec.Text != expectedText {
		t.Errorf("updated banner text = %q, want %q", notification.Spec.Text, expectedText)
	}
}

func TestRemoveBanner_NoExistingBanner(t *testing.T) {
	registerScheme()
	ctx := context.TODO()

	err := recon.removeBanner(ctx, mceComplianceBannerName)
	if err != nil {
		t.Fatalf("removeBanner() error = %v, expected nil for non-existent banner", err)
	}
}

// --- OCP banner lifecycle tests ---

func TestEnsureOCPComplianceBanner_BelowMinimum(t *testing.T) {
	registerScheme()
	ctx := context.TODO()

	hub := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multiclusterhub",
			Namespace: "open-cluster-management",
		},
	}

	err := recon.ensureOCPComplianceBanner(ctx, hub, "4.18.0")
	if err != nil {
		t.Fatalf("ensureOCPComplianceBanner() error = %v", err)
	}

	notification := &consolev1.ConsoleNotification{}
	err = recon.Client.Get(ctx, types.NamespacedName{Name: ocpComplianceBannerName}, notification)
	if err != nil {
		t.Fatalf("failed to get ConsoleNotification: %v", err)
	}
	t.Cleanup(func() {
		_ = recon.Client.Delete(ctx, notification)
	})

	expectedText := ocpComplianceBannerText("4.18.0", version.MinimumOCPVersion)
	if notification.Spec.Text != expectedText {
		t.Errorf("banner text = %q, want %q", notification.Spec.Text, expectedText)
	}
	if notification.Spec.Location != consolev1.BannerTop {
		t.Errorf("banner location = %q, want %q", notification.Spec.Location, consolev1.BannerTop)
	}
	if notification.Spec.BackgroundColor != bannerBackgroundColor {
		t.Errorf("banner backgroundColor = %q, want %q", notification.Spec.BackgroundColor, bannerBackgroundColor)
	}
	if notification.Labels["installer.name"] != "multiclusterhub" {
		t.Errorf("label installer.name = %q, want %q", notification.Labels["installer.name"], "multiclusterhub")
	}
}

func TestEnsureOCPComplianceBanner_MeetsMinimum_RemovesBanner(t *testing.T) {
	registerScheme()
	ctx := context.TODO()

	hub := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multiclusterhub",
			Namespace: "open-cluster-management",
		},
	}

	// Pre-create a banner
	existing := &consolev1.ConsoleNotification{
		ObjectMeta: metav1.ObjectMeta{
			Name: ocpComplianceBannerName,
			Labels: map[string]string{
				"installer.name":      "multiclusterhub",
				"installer.namespace": "open-cluster-management",
			},
		},
		Spec: consolev1.ConsoleNotificationSpec{
			Text:            "old OCP warning",
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
			ObjectMeta: metav1.ObjectMeta{Name: ocpComplianceBannerName},
		})
	})

	// OCP version meets minimum — banner should be removed
	err := recon.ensureOCPComplianceBanner(ctx, hub, "4.19.0")
	if err != nil {
		t.Fatalf("ensureOCPComplianceBanner() error = %v", err)
	}

	notification := &consolev1.ConsoleNotification{}
	err = recon.Client.Get(ctx, types.NamespacedName{Name: ocpComplianceBannerName}, notification)
	if err == nil {
		t.Error("expected ConsoleNotification to be deleted when OCP meets minimum, but it still exists")
	}
}

func TestEnsureOCPComplianceBanner_EmptyVersion_NoBanner(t *testing.T) {
	registerScheme()
	ctx := context.TODO()

	hub := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multiclusterhub",
			Namespace: "open-cluster-management",
		},
	}

	err := recon.ensureOCPComplianceBanner(ctx, hub, "")
	if err != nil {
		t.Fatalf("ensureOCPComplianceBanner() error = %v", err)
	}

	notification := &consolev1.ConsoleNotification{}
	err = recon.Client.Get(ctx, types.NamespacedName{Name: ocpComplianceBannerName}, notification)
	if err == nil {
		t.Error("expected no ConsoleNotification when OCP version is empty")
	}
}

func TestEnsureOCPComplianceBanner_UpdatesExistingBanner(t *testing.T) {
	registerScheme()
	ctx := context.TODO()

	hub := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multiclusterhub",
			Namespace: "open-cluster-management",
		},
	}

	// Create banner with old version
	existing := &consolev1.ConsoleNotification{
		ObjectMeta: metav1.ObjectMeta{
			Name: ocpComplianceBannerName,
			Labels: map[string]string{
				"installer.name":      "multiclusterhub",
				"installer.namespace": "open-cluster-management",
			},
		},
		Spec: consolev1.ConsoleNotificationSpec{
			Text:            ocpComplianceBannerText("4.17.0", version.MinimumOCPVersion),
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
			ObjectMeta: metav1.ObjectMeta{Name: ocpComplianceBannerName},
		})
	})

	// Update with different OCP version (still below minimum)
	err := recon.ensureOCPComplianceBanner(ctx, hub, "4.18.0")
	if err != nil {
		t.Fatalf("ensureOCPComplianceBanner() error = %v", err)
	}

	notification := &consolev1.ConsoleNotification{}
	err = recon.Client.Get(ctx, types.NamespacedName{Name: ocpComplianceBannerName}, notification)
	if err != nil {
		t.Fatalf("failed to get ConsoleNotification: %v", err)
	}

	expectedText := ocpComplianceBannerText("4.18.0", version.MinimumOCPVersion)
	if notification.Spec.Text != expectedText {
		t.Errorf("updated banner text = %q, want %q", notification.Spec.Text, expectedText)
	}
}
