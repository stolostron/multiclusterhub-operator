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
	bannerBackgroundColor   = "#880808"
	bannerTextColor         = "#ffffff"
	bannerSupportLinkHref   = "https://access.redhat.com/support"
	bannerSupportLinkText   = "Contact Red Hat Support"
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

func (r *MultiClusterHubReconciler) ensureMCEComplianceBanner(ctx context.Context,
	hub *operatorsv1.MultiClusterHub,
	compliance *operatorsv1.MCEVersionComplianceStatus) error {

	if compliance == nil || compliance.IsCompliant {
		return r.removeMCEComplianceBanner(ctx)
	}

	if compliance.CurrentVersion == "" {
		return r.removeMCEComplianceBanner(ctx)
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
			Link: &consolev1.Link{
				Text: bannerSupportLinkText,
				Href: bannerSupportLinkHref,
			},
		},
	}

	existing := &consolev1.ConsoleNotification{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: mceComplianceBannerName}, existing)
	if errors.IsNotFound(err) {
		log.Info("Creating MCE compliance ConsoleNotification banner")
		if err := r.Client.Create(ctx, desired); err != nil {
			return fmt.Errorf("failed to create ConsoleNotification %s: %w", mceComplianceBannerName, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get ConsoleNotification %s: %w", mceComplianceBannerName, err)
	}

	if existing.Spec.Text != desired.Spec.Text ||
		existing.Spec.BackgroundColor != desired.Spec.BackgroundColor ||
		existing.Spec.Color != desired.Spec.Color ||
		existing.Spec.Location != desired.Spec.Location {
		patch := client.MergeFrom(existing.DeepCopy())
		existing.Spec = desired.Spec
		existing.Labels = desired.Labels
		log.Info("Updating MCE compliance ConsoleNotification banner")
		if err := r.Client.Patch(ctx, existing, patch); err != nil {
			return fmt.Errorf("failed to patch ConsoleNotification %s: %w", mceComplianceBannerName, err)
		}
	}

	return nil
}

func (r *MultiClusterHubReconciler) removeMCEComplianceBanner(ctx context.Context) error {
	notification := &consolev1.ConsoleNotification{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: mceComplianceBannerName}, notification)
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get ConsoleNotification %s: %w", mceComplianceBannerName, err)
	}

	log.Info("Removing MCE compliance ConsoleNotification banner")
	return r.Client.Delete(ctx, notification)
}

func (r *MultiClusterHubReconciler) cleanupConsoleNotifications(_ logr.Logger, m *operatorsv1.MultiClusterHub) error {
	return r.Client.DeleteAllOf(context.TODO(), &consolev1.ConsoleNotification{}, client.MatchingLabels{
		"installer.name":      m.GetName(),
		"installer.namespace": m.GetNamespace(),
	})
}
