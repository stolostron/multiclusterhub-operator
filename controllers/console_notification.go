// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"fmt"

	semver "github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	consolev1 "github.com/openshift/api/console/v1"
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/version"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	mceComplianceBannerName = "acm-mce-version-compliance"
	ocpComplianceBannerName = "acm-ocp-version-compliance"
	bannerBackgroundColor   = "#880808"
	bannerTextColor         = "#ffffff"
)

func mceComplianceBannerText(currentVersion, requiredChannel string) string {
	direction := "does not match"
	current, errCur := semver.NewVersion(currentVersion)
	required, errReq := semver.NewVersion(version.RequiredMCEVersion)
	if errCur == nil && errReq == nil {
		if current.GreaterThan(required) {
			direction = "is ahead of"
		} else {
			direction = "is behind"
		}
	}
	return fmt.Sprintf(
		"WARNING: ACM in unexpected configuration: MCE %s %s the expected %s channel.",
		currentVersion, direction, requiredChannel,
	)
}

func ocpComplianceBannerText(currentVersion, minimumVersion string) string {
	return fmt.Sprintf(
		"WARNING: ACM in unexpected configuration: OCP %s is below the minimum supported version %s.",
		currentVersion, minimumVersion,
	)
}

func (r *MultiClusterHubReconciler) ensureMCEComplianceBanner(ctx context.Context,
	hub *operatorsv1.MultiClusterHub,
	compliance *operatorsv1.MCEVersionComplianceStatus) error {

	if compliance == nil || compliance.IsCompliant {
		return r.removeBanner(ctx, mceComplianceBannerName)
	}

	if compliance.CurrentVersion == "" {
		return r.removeBanner(ctx, mceComplianceBannerName)
	}

	desired := &consolev1.ConsoleNotification{
		ObjectMeta: metav1.ObjectMeta{
			Name: mceComplianceBannerName,
			Labels: map[string]string{
				"installer.name":      hub.GetName(),
				"installer.namespace": hub.GetNamespace(),
			},
		},
		Spec: consolev1.ConsoleNotificationSpec{
			Text:            mceComplianceBannerText(compliance.CurrentVersion, compliance.RequiredChannel),
			Location:        consolev1.BannerTop,
			BackgroundColor: bannerBackgroundColor,
			Color:           bannerTextColor,
		},
	}

	return r.ensureBanner(ctx, mceComplianceBannerName, desired)
}

func (r *MultiClusterHubReconciler) ensureOCPComplianceBanner(ctx context.Context,
	hub *operatorsv1.MultiClusterHub,
	ocpVersion string) error {

	if ocpVersion == "" {
		return r.removeBanner(ctx, ocpComplianceBannerName)
	}

	// Check if OCP version meets minimum requirement
	validationErr := version.ValidOCPVersion(ocpVersion)
	if validationErr == nil {
		// OCP version is valid, remove banner if it exists
		return r.removeBanner(ctx, ocpComplianceBannerName)
	}

	desired := &consolev1.ConsoleNotification{
		ObjectMeta: metav1.ObjectMeta{
			Name: ocpComplianceBannerName,
			Labels: map[string]string{
				"installer.name":      hub.GetName(),
				"installer.namespace": hub.GetNamespace(),
			},
		},
		Spec: consolev1.ConsoleNotificationSpec{
			Text:            ocpComplianceBannerText(ocpVersion, version.MinimumOCPVersion),
			Location:        consolev1.BannerTop,
			BackgroundColor: bannerBackgroundColor,
			Color:           bannerTextColor,
		},
	}

	return r.ensureBanner(ctx, ocpComplianceBannerName, desired)
}

// ensureBanner creates or updates a ConsoleNotification banner
func (r *MultiClusterHubReconciler) ensureBanner(ctx context.Context, name string, desired *consolev1.ConsoleNotification) error {
	existing := &consolev1.ConsoleNotification{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: name}, existing)
	if errors.IsNotFound(err) {
		log.Info("Creating ConsoleNotification banner", "name", name)
		if err := r.Client.Create(ctx, desired); err != nil {
			return fmt.Errorf("failed to create ConsoleNotification %s: %w", name, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get ConsoleNotification %s: %w", name, err)
	}

	if existing.Spec.Text != desired.Spec.Text ||
		existing.Spec.BackgroundColor != desired.Spec.BackgroundColor ||
		existing.Spec.Color != desired.Spec.Color ||
		existing.Spec.Location != desired.Spec.Location {
		patch := client.MergeFrom(existing.DeepCopy())
		existing.Spec = desired.Spec
		existing.Labels = desired.Labels
		log.Info("Updating ConsoleNotification banner", "name", name)
		if err := r.Client.Patch(ctx, existing, patch); err != nil {
			return fmt.Errorf("failed to patch ConsoleNotification %s: %w", name, err)
		}
	}

	return nil
}

// removeBanner removes a ConsoleNotification banner if it exists
func (r *MultiClusterHubReconciler) removeBanner(ctx context.Context, name string) error {
	notification := &consolev1.ConsoleNotification{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: name}, notification)
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get ConsoleNotification %s: %w", name, err)
	}

	log.Info("Removing ConsoleNotification banner", "name", name)
	return r.Client.Delete(ctx, notification)
}

func (r *MultiClusterHubReconciler) cleanupConsoleNotifications(_ logr.Logger, m *operatorsv1.MultiClusterHub) error {
	return r.Client.DeleteAllOf(context.TODO(), &consolev1.ConsoleNotification{}, client.MatchingLabels{
		"installer.name":      m.GetName(),
		"installer.namespace": m.GetNamespace(),
	})
}
