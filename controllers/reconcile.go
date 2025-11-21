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
	"github.com/stolostron/multiclusterhub-operator/pkg/overrides"
	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"
	"github.com/stolostron/multiclusterhub-operator/pkg/version"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MultiClusterHub object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *MultiClusterHubReconciler) Reconcile(ctx context.Context, req ctrl.Request) (retQueue ctrl.Result, retError error) {
	r.Log = log
	r.Log.Info("Reconciling MultiClusterHub")

	// Fetch the MultiClusterHub instance
	multiClusterHub := &operatorv1.MultiClusterHub{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, multiClusterHub)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			r.Log.Info("MultiClusterHub resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		r.Log.Error(err, "Failed to get MultiClusterHub CR")
		return ctrl.Result{}, err
	}

	multiClusterHub.Status.HubConditions = filterOutConditionWithSubstring(multiClusterHub.Status.HubConditions,
		string(operatorv1.ComponentFailure))

	// Check if any deprecated fields are present within the multiClusterHub spec.
	r.CheckDeprecatedFieldUsage(multiClusterHub)

	// Check to see if upgradeable
	upgrade, err := r.setOperatorUpgradeableStatus(ctx, multiClusterHub)
	if err != nil {
		r.Log.Error(err, "Unable to set operator condition")
		return ctrl.Result{}, err
	}

	trackedNamespaces := utils.TrackedNamespaces(multiClusterHub)

	allDeploys, err := r.listDeployments(trackedNamespaces)
	if err != nil {
		return ctrl.Result{}, err
	}

	allCRs, err := r.listCustomResources(multiClusterHub)
	if err != nil {
		return ctrl.Result{}, err
	}

	ocpConsole, err := r.CheckConsole(ctx)
	if err != nil {
		r.Log.Error(err, "error finding OCP Console")
		return ctrl.Result{}, err
	}

	/*
	   In ACM 2.11, we need to determine if the operator is running in a STS enabled environment.
	*/
	stsEnabled, err := r.isSTSEnabled(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	originalStatus := multiClusterHub.Status.DeepCopy()
	defer func() {
		statusQueue, statusError := r.syncHubStatus(ctx, multiClusterHub, originalStatus, allDeploys, allCRs, ocpConsole, stsEnabled)
		if statusError != nil {
			r.Log.Error(retError, "Error updating status")
		}
		if empty := (reconcile.Result{}); retQueue == empty {
			retQueue = statusQueue
		}
		if retError == nil {
			retError = statusError
		}
	}()

	// Attempt to retrieve image overrides from environmental variables.
	imageOverrides := overrides.GetOverridesFromEnv(overrides.OperandImagePrefix)

	// If no overrides found using OperandImagePrefix, attempt to retrieve using OSBSImagePrefix.
	if len(imageOverrides) == 0 {
		imageOverrides = overrides.GetOverridesFromEnv(overrides.OSBSImagePrefix)
	}

	// Check if no image overrides were found using either prefix.
	if len(imageOverrides) == 0 {
		r.Log.Error(err, "Could not get map of image overrides")
		return ctrl.Result{}, nil
	}

	// Apply image repository override from annotation if present.
	if imageRepo := utils.GetImageRepository(multiClusterHub); imageRepo != "" {
		r.Log.Info(fmt.Sprintf("Overriding Image Repository from annotation: %s", imageRepo))
		imageOverrides = utils.OverrideImageRepository(imageOverrides, imageRepo)
	}

	// Check for developer overrides in configmap.
	if ioConfigmapName := utils.GetImageOverridesConfigmapName(multiClusterHub); ioConfigmapName != "" {
		imageOverrides, err = overrides.GetOverridesFromConfigmap(r.Client, imageOverrides,
			multiClusterHub.GetNamespace(), ioConfigmapName, false)
		if err != nil {
			r.Log.Error(err, fmt.Sprintf("Failed to find image override configmap: %s/%s",
				multiClusterHub.GetNamespace(), ioConfigmapName))

			return ctrl.Result{}, err
		}
	}

	// Update cache with image overrides and related information.
	r.CacheSpec.ImageOverrides = imageOverrides
	r.CacheSpec.ManifestVersion = version.Version
	r.CacheSpec.ImageRepository = utils.GetImageRepository(multiClusterHub)
	r.CacheSpec.ImageOverridesCM = utils.GetImageOverridesConfigmapName(multiClusterHub)

	// Attempt to retrieve template overrides from environmental variables.
	templateOverrides := overrides.GetOverridesFromEnv(overrides.TemplateOverridePrefix)

	// Check for developer overrides in configmap
	if toConfigmapName := utils.GetTemplateOverridesConfigmapName(multiClusterHub); toConfigmapName != "" {
		templateOverrides, err = overrides.GetOverridesFromConfigmap(r.Client, templateOverrides,
			multiClusterHub.GetNamespace(), toConfigmapName, true)
		if err != nil {
			r.Log.Error(err, fmt.Sprintf("Failed to find template override configmap: %s/%s",
				multiClusterHub.GetNamespace(), toConfigmapName))

			return ctrl.Result{}, err
		}
	}

	// Update cache with template overrides and related information.
	r.CacheSpec.TemplateOverrides = templateOverrides
	r.CacheSpec.TemplateOverridesCM = utils.GetTemplateOverridesConfigmapName(multiClusterHub)

	var result ctrl.Result
	result, err = r.setDefaults(multiClusterHub, ocpConsole)
	if result != (ctrl.Result{}) {
		return ctrl.Result{}, err
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	/*
		In ACM 2.13, we are required to get the default storage class name for the Edge Manager (aka Flight-Control)
		component. To ensure that we can pass the default storage class, we will store it as an environment variable.
	*/
	if result, err = r.SetDefaultStorageClassName(ctx, multiClusterHub); err != nil {
		r.Log.Error(err, "failed to set the default StorageClass name")
		return ctrl.Result{}, err
	}

	// Check if the multiClusterHub instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isHubMarkedToBeDeleted := multiClusterHub.GetDeletionTimestamp() != nil
	if isHubMarkedToBeDeleted {
		terminating := NewHubCondition(operatorv1.Terminating, metav1.ConditionTrue, DeleteTimestampReason, "Multiclusterhub is being cleaned up.")
		SetHubCondition(&multiClusterHub.Status, *terminating)

		if controllerutil.ContainsFinalizer(multiClusterHub, hubFinalizer) {
			// Run finalization logic. If the finalization
			// logic fails, don't remove the finalizer so
			// that we can retry during the next reconciliation.
			if err := r.finalizeHub(r.Log, multiClusterHub, ocpConsole, stsEnabled); err != nil {
				// Logging err and returning nil to ensure 45 second wait
				r.Log.Info(fmt.Sprintf("Finalizing: %s", err.Error()))
				return ctrl.Result{RequeueAfter: resyncPeriod}, nil
			}

			// Remove hubFinalizer. Once all finalizers have been
			// removed, the object will be deleted.
			controllerutil.RemoveFinalizer(multiClusterHub, hubFinalizer)

			err := r.Client.Update(context.TODO(), multiClusterHub)
			if err != nil {
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	/*
	   In ACM 2.9, we need to ensure that the openshift.io/cluster-monitoring is added to the same namespace as the
	   MultiClusterHub to avoid conflicts with the openshift-* namespace when deploying PrometheusRules and
	   ServiceMonitors in ACM.
	*/
	_, err = r.ensureOpenShiftNamespaceLabel(ctx, multiClusterHub)
	if err != nil {
		r.Log.Error(err, "Failed to add to %s label to namespace: %s", utils.OpenShiftClusterMonitoringLabel,
			multiClusterHub.GetNamespace())
		return ctrl.Result{}, err
	}

	err = r.maintainImageManifestConfigmap(multiClusterHub)
	if err != nil {
		r.Log.Error(err, "Error storing image manifests in configmap")
		return ctrl.Result{}, err
	}

	// Do not reconcile objects if this instance of mch is labeled "paused"
	updatePausedCondition(multiClusterHub)
	if utils.IsPaused(multiClusterHub) {
		r.Log.Info("MultiClusterHub reconciliation is paused. Nothing more to do.")
		return ctrl.Result{}, nil
	}

	if !utils.ShouldIgnoreOCPVersion(multiClusterHub) {
		currentOCPVersion, err := r.getClusterVersion(ctx)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to detect clusterversion: %w", err)
		}
		if err := version.ValidOCPVersion(currentOCPVersion); err != nil {
			condition := NewHubCondition(operatorv1.Progressing, metav1.ConditionFalse, RequirementsNotMetReason,
				fmt.Sprintf("OCP version requirement not met: %s", err.Error()))

			SetHubCondition(&multiClusterHub.Status, *condition)
			return ctrl.Result{}, err
		}
	}

	// Install CRDs
	var reason string
	reason, err = r.installCRDs(r.Log, multiClusterHub)
	if err != nil {
		condition := NewHubCondition(
			operatorv1.Progressing,
			metav1.ConditionFalse,
			reason,
			fmt.Sprintf("Error installing CRDs: %s", err),
		)
		SetHubCondition(&multiClusterHub.Status, *condition)
		return ctrl.Result{}, err
	}

	// 2.6 -> 2.7 upgrade logic
	// There are ClusterManagementAddOns in the GRC appsub that need to be preserved when deleting the helmrelease
	// To stop helm from removing them we will remove the finalizer on the GRC helmrelease, delete the appsub,
	// and clean things up ourselves
	err = r.cleanupGRCAppsub(multiClusterHub)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Deploy appsub operator component
	if multiClusterHub.Enabled(operatorv1.Appsub) {
		result, err = r.ensureComponent(ctx, multiClusterHub, operatorv1.Appsub, r.CacheSpec, stsEnabled)
	} else {
		result, err = r.ensureNoComponent(ctx, multiClusterHub, operatorv1.Appsub, r.CacheSpec, stsEnabled)
	}
	if result != (ctrl.Result{}) {
		return result, err
	}

	// Remove existing appsubs and helmreleases if present from upgrade
	result, err = r.ensureAppsubsGone(multiClusterHub)
	if result != (ctrl.Result{}) {
		return result, err
	}

	/*
	   Remove existing service and servicemonitor configurations, if present from upgrade. In ACM 2.2, operator-sdk
	   generated configurations for the MCH operator to be collecting metrics. In later releases, these resources are
	   no longer available; therefore, we need to explicitly remove them from the upgrade configuration.
	*/
	for _, kind := range operatorv1.GetLegacyConfigKind() {
		_ = r.removeLegacyConfigurations(ctx, "openshift-monitoring", kind)
	}

	if utils.ProxyEnvVarsAreSet() {
		r.Log.Info(
			fmt.Sprintf("Proxy configuration environment variables are set. HTTP_PROXY: %s, HTTPS_PROXY: %s, NO_PROXY: %s",
				os.Getenv("HTTP_PROXY"), os.Getenv("HTTPS_PROXY"), os.Getenv("NO_PROXY"),
			),
		)
	}

	result, err = r.ensurePullSecretCreated(multiClusterHub, multiClusterHub.GetNamespace())
	if err != nil {
		condition := NewHubCondition(
			operatorv1.Progressing,
			metav1.ConditionFalse,
			err.Error(),
			fmt.Sprintf("Error fetching Pull Secret: %s", err),
		)
		SetHubCondition(&multiClusterHub.Status, *condition)
		return result, fmt.Errorf("failed to find pullsecret: %s", err)
	}

	result, err = r.ensureMultiClusterEngine(ctx, multiClusterHub)
	if result != (ctrl.Result{}) {
		return result, err
	}

	result, err = r.ingressDomain(ctx, multiClusterHub)
	if result != (ctrl.Result{}) {
		return result, err
	}

	result, err = r.openShiftApiUrl(ctx, multiClusterHub)
	if err != nil {
		return result, err
	}

	result, err = r.createTrustBundleConfigmap(ctx, multiClusterHub)
	if err != nil {
		return result, err
	}

	result, err = r.createMetricsService(ctx, multiClusterHub)
	if err != nil {
		return result, err
	}

	result, err = r.createMetricsServiceMonitor(ctx, multiClusterHub)
	if err != nil {
		return result, err
	}

	// Install CRDs
	reason, err = r.deployResources(r.Log, multiClusterHub)
	if err != nil {
		condition := NewHubCondition(
			operatorv1.Progressing,
			metav1.ConditionFalse,
			reason,
			fmt.Sprintf("Error deploying resources: %s", err),
		)
		SetHubCondition(&multiClusterHub.Status, *condition)
		return ctrl.Result{}, err
	}

	result, err = r.waitForMCEReady(ctx)
	if result != (ctrl.Result{}) || err != nil {
		return result, err
	}

	if !multiClusterHub.Spec.DisableHubSelfManagement {
		result, err = r.ensureKlusterletAddonConfig(multiClusterHub)
		if result != (ctrl.Result{}) || err != nil {
			return result, err
		}
	}
	// iam-policy-controller was removed in 2.11
	result, err = r.ensureNoClusterManagementAddOn(multiClusterHub, operatorv1.IamPolicyController)
	if err != nil {
		return result, err
	}

	// Install the rest of the subscriptions in no particular order
	for _, c := range operatorv1.MCHComponents {
		result, err = r.ensureComponentOrNoComponent(ctx, multiClusterHub, c, r.CacheSpec, ocpConsole, stsEnabled)

		if result != (ctrl.Result{}) {
			return result, err
		}
	}

	result, err := r.ensureNoComponent(ctx, m, operatorv1.EdgeManagerPreview, cachespec, isSTSEnabled)
	if result != (ctrl.Result{}) || err != nil {
		return result, err
	}
	result, err := r.deleteEdgeManagerResources(ctx, m)
	if result != (ctrl.Result{}) || err != nil {
		return result, err
	}

	// Cleanup unused resources once components up-to-date
	if r.ComponentsAreRunning(multiClusterHub, ocpConsole, stsEnabled) {
		result, err = r.ensureRemovalsGone(multiClusterHub)
		if result != (ctrl.Result{}) {
			return result, err
		}
	}
	if upgrade {
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}

	logf.Log.Info("Reconcile completed. Requeuing after " + utils.ShortRefreshInterval.String())
	return ctrl.Result{RequeueAfter: utils.ShortRefreshInterval}, nil
}
