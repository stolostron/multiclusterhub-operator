// Copyright Contributors to the Open Cluster Management project

/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"os"

	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"

	configv1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func updatePausedCondition(m *operatorv1.MultiClusterHub) {
	c := GetHubCondition(m.Status, operatorv1.Progressing)

	if utils.IsPaused(m) {
		// Pause condition needs to go on
		if c == nil || c.Reason != PausedReason {
			condition := NewHubCondition(operatorv1.Progressing, metav1.ConditionUnknown, PausedReason, "Multiclusterhub is paused")
			SetHubCondition(&m.Status, *condition)
		}
	} else {
		// Pause condition needs to come off
		if c != nil && c.Reason == PausedReason {
			condition := NewHubCondition(operatorv1.Progressing, metav1.ConditionTrue, ResumedReason, "Multiclusterhub is resumed")
			SetHubCondition(&m.Status, *condition)
		}
	}
}

func (r *MultiClusterHubReconciler) setDefaults(m *operatorv1.MultiClusterHub, ocpConsole bool) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log

	updateNecessary := false

	defaultUpdate, err := utils.SetDefaultComponents(m)
	if err != nil {
		log.Error(err, "OPERATOR_CATALOG is an illegal value")
		return ctrl.Result{}, err
	}
	if defaultUpdate {
		updateNecessary = true
	}

	// Add finalizer for this CR
	if controllerutil.AddFinalizer(m, hubFinalizer) {
		updateNecessary = true
	}

	if utils.DeduplicateComponents(m) {
		updateNecessary = true
	}

	// management-ingress component removed in 2.7.0
	if m.Prune(operatorv1.ManagementIngress) {
		updateNecessary = true
	}

	// helm-repo component removed in 2.7.0
	if m.Prune(operatorv1.Repo) {
		updateNecessary = true
	}

	if m.Enabled(operatorv1.MTVIntegrationsPreview) {
		m.Enable(operatorv1.MTVIntegrations)
		m.Prune(operatorv1.MTVIntegrationsPreview)
		updateNecessary = true
	}

	if m.Enabled(operatorv1.FineGrainedRbacPreview) {
		m.Enable(operatorv1.FineGrainedRbac)
		m.Prune(operatorv1.FineGrainedRbacPreview)
		updateNecessary = true
	}

	for _, c := range m.Spec.Overrides.Components {
		if !operatorv1.ValidComponent(c, operatorv1.MCHComponents) {
			if m.Prune(c.Name) {
				log.Info(fmt.Sprintf("Removing invalid component: %v from existing MultiClusterHub", c.Name))
				updateNecessary = true
			}
		}
	}

	if utils.MchIsValid(m) && os.Getenv("ACM_HUB_OCP_VERSION") != "" && !updateNecessary {
		return ctrl.Result{}, nil
	}

	if !operatorv1.AvailabilityConfigIsValid(m.Spec.AvailabilityConfig) {
		m.Spec.AvailabilityConfig = operatorv1.HAHigh
		updateNecessary = true
	}

	// If OCP 4.10+ then set then enable the MCE console. Else ensure it is disabled
	clusterVersion := &configv1.ClusterVersion{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: "version"}, clusterVersion)
	if err != nil {
		log.Error(err, "Failed to detect clusterversion")
		return ctrl.Result{}, err
	}
	currentClusterVersion := ""
	if len(clusterVersion.Status.History) == 0 {
		if !utils.IsUnitTest() {
			log.Error(err, "Failed to detect status in clusterversion.status.history")
			return ctrl.Result{}, err
		}
	}

	if utils.IsUnitTest() {
		// If unit test pass along a version, Can't set status in unit test
		currentClusterVersion = "4.99.99"
	} else {
		currentClusterVersion = clusterVersion.Status.History[0].Version
	}

	// Set OCP version as env var, so that charts can render this value
	err = os.Setenv("ACM_HUB_OCP_VERSION", currentClusterVersion)
	if err != nil {
		log.Error(err, "Failed to set ACM_HUB_OCP_VERSION environment variable")
		return ctrl.Result{}, err
	}

	if updateNecessary {
		// Apply defaults to server
		err = r.Client.Update(ctx, m)
		if err != nil {
			r.Log.Error(err, "Failed to update MultiClusterHub", "MultiClusterHub.Namespace", m.Namespace, "MultiClusterHub.Name", m.Name)
			return ctrl.Result{}, err
		}
		r.Log.Info("MultiClusterHub successfully updated")
		return ctrl.Result{Requeue: true}, nil

	}
	log.Info("No updates to defaults detected")
	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) CheckDeprecatedFieldUsage(m *operatorv1.MultiClusterHub) {
	a := m.GetAnnotations()
	df := []struct {
		name      string
		isPresent bool
	}{
		{"hive", m.Spec.Hive != nil},
		{"ingress", m.Spec.Ingress != nil},
		{"customCAConfigmap", m.Spec.CustomCAConfigmap != ""},
		{"enableClusterBackup", m.Spec.EnableClusterBackup},
		{"enableClusterProxyAddon", m.Spec.EnableClusterProxyAddon},
		{"separateCertificateManagement", m.Spec.SeparateCertificateManagement},
		{utils.DeprecatedAnnotationIgnoreOCPVersion, a[utils.DeprecatedAnnotationIgnoreOCPVersion] != ""},
		{utils.DeprecatedAnnotationImageOverridesCM, a[utils.DeprecatedAnnotationImageOverridesCM] != ""},
		{utils.DeprecatedAnnotationImageRepo, a[utils.DeprecatedAnnotationImageRepo] != ""},
		{utils.DeprecatedAnnotationKubeconfig, a[utils.DeprecatedAnnotationKubeconfig] != ""},
		{utils.DeprecatedAnnotationMCHPause, a[utils.DeprecatedAnnotationMCHPause] != ""},
	}

	if r.DeprecatedFields == nil {
		r.DeprecatedFields = make(map[string]bool)
	}

	for _, f := range df {
		if f.isPresent && !r.DeprecatedFields[f.name] {
			r.Log.Info(fmt.Sprintf("Warning: %s field usage is deprecated in operator.", f.name))
			r.DeprecatedFields[f.name] = true
		}
	}
}
