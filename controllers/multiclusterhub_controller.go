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
	"path"
	"path/filepath"
	"reflect"
	"time"

	"github.com/Masterminds/semver/v3"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/deploying"
	"github.com/stolostron/multiclusterhub-operator/pkg/overrides"
	"github.com/stolostron/multiclusterhub-operator/pkg/predicate"
	renderer "github.com/stolostron/multiclusterhub-operator/pkg/rendering"
	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"
	"github.com/stolostron/multiclusterhub-operator/pkg/version"
	ctrlpredicate "sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/yaml"

	configv1 "github.com/openshift/api/config/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/intstr"

	ocopv1 "github.com/openshift/api/operator/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	pkgerrors "github.com/pkg/errors"
)

// MultiClusterHubReconciler reconciles a MultiClusterHub object
type MultiClusterHubReconciler struct {
	Client           client.Client
	UncachedClient   client.Client
	CacheSpec        CacheSpec
	Scheme           *runtime.Scheme
	Log              logr.Logger
	UpgradeableCond  utils.Condition
	DeprecatedFields map[string]bool
}

const (
	resyncPeriod = time.Second * 20

	crdPathEnvVar       = "CRDS_PATH"
	templatesPathEnvVar = "TEMPLATES_PATH"
	templatesKind       = "multiclusterhub"
	hubFinalizer        = "finalizer.operator.open-cluster-management.io"

	trustBundleNameEnvVar  = "TRUSTED_CA_BUNDLE"
	defaultTrustBundleName = "trusted-ca-bundle"
)

var (
	log              = logf.Log.WithName("reconcile")
	STSEnabledStatus = false
)

//+kubebuilder:rbac:groups="";"admissionregistration.k8s.io";"apiextensions.k8s.io";"apiregistration.k8s.io";"apps";"apps.open-cluster-management.io";"authorization.k8s.io";"hive.openshift.io";"mcm.ibm.com";"proxy.open-cluster-management.io";"rbac.authorization.k8s.io";"security.openshift.io";"clusterview.open-cluster-management.io";"discovery.open-cluster-management.io";"wgpolicyk8s.io",resources=apiservices;channels;clusterjoinrequests;clusterrolebindings;clusterstatuses/log;configmaps;customresourcedefinitions;deployments;discoveryconfigs;hiveconfigs;mutatingwebhookconfigurations;validatingwebhookconfigurations;namespaces;pods;policyreports;replicasets;rolebindings;secrets;serviceaccounts;services;subjectaccessreviews;subscriptions;helmreleases;managedclusters;managedclustersets,verbs=get
//+kubebuilder:rbac:groups="";"admissionregistration.k8s.io";"apiextensions.k8s.io";"apiregistration.k8s.io";"apps";"apps.open-cluster-management.io";"authorization.k8s.io";"hive.openshift.io";"monitoring.coreos.com";"rbac.authorization.k8s.io";"mcm.ibm.com";"security.openshift.io",resources=apiservices;channels;clusterjoinrequests;clusterrolebindings;clusterroles;configmaps;customresourcedefinitions;deployments;hiveconfigs;mutatingwebhookconfigurations;validatingwebhookconfigurations;namespaces;rolebindings;secrets;serviceaccounts;services;servicemonitors;subjectaccessreviews;subscriptions;validatingwebhookconfigurations,verbs=create;update
//+kubebuilder:rbac:groups="";"apps";"apps.open-cluster-management.io";"admissionregistration.k8s.io";"apiregistration.k8s.io";"authorization.k8s.io";"config.openshift.io";"inventory.open-cluster-management.io";"mcm.ibm.com";"observability.open-cluster-management.io";"operator.open-cluster-management.io";"rbac.authorization.k8s.io";"hive.openshift.io";"clusterview.open-cluster-management.io";"discovery.open-cluster-management.io";"wgpolicyk8s.io",resources=apiservices;clusterjoinrequests;configmaps;deployments;discoveryconfigs;helmreleases;ingresses;multiclusterhubs;multiclusterobservabilities;namespaces;hiveconfigs;rolebindings;servicemonitors;secrets;services;subjectaccessreviews;subscriptions;validatingwebhookconfigurations;pods;policyreports;managedclusters;managedclustersets,verbs=list
//+kubebuilder:rbac:groups="";"admissionregistration.k8s.io";"apiregistration.k8s.io";"apps";"authorization.k8s.io";"config.openshift.io";"mcm.ibm.com";"operator.open-cluster-management.io";"rbac.authorization.k8s.io";"storage.k8s.io";"apps.open-cluster-management.io";"hive.openshift.io";"clusterview.open-cluster-management.io";"wgpolicyk8s.io",resources=apiservices;helmreleases;hiveconfigs;configmaps;clusterjoinrequests;deployments;ingresses;multiclusterhubs;namespaces;rolebindings;secrets;services;subjectaccessreviews;validatingwebhookconfigurations;pods;policyreports;managedclusters;managedclustersets,verbs=watch;list
//+kubebuilder:rbac:groups="";"admissionregistration.k8s.io";"apps";"apps.open-cluster-management.io";"mcm.ibm.com";"monitoring.coreos.com";"operator.open-cluster-management.io";,resources=deployments;deployments/finalizers;helmreleases;services;services/finalizers;servicemonitors;servicemonitors/finalizers;validatingwebhookconfigurations;multiclusterhubs;multiclusterhubs/finalizers;multiclusterhubs/status,verbs=update
//+kubebuilder:rbac:groups="admissionregistration.k8s.io";"apiextensions.k8s.io";"apiregistration.k8s.io";"hive.openshift.io";"mcm.ibm.com";"rbac.authorization.k8s.io";,resources=apiservices;clusterroles;clusterrolebindings;customresourcedefinitions;hiveconfigs;mutatingwebhookconfigurations;validatingwebhookconfigurations,verbs=delete;deletecollection;list;watch;patch
//+kubebuilder:rbac:groups="";"apps";"apiregistration.k8s.io";"apps.open-cluster-management.io";"apiextensions.k8s.io";,resources=deployments;services;channels;customresourcedefinitions;apiservices,verbs=delete
//+kubebuilder:rbac:groups="";"action.open-cluster-management.io";"addon.open-cluster-management.io";"agent.open-cluster-management.io";"argoproj.io";"cluster.open-cluster-management.io";"work.open-cluster-management.io";"app.k8s.io";"apps.open-cluster-management.io";"authorization.k8s.io";"certificates.k8s.io";"clusterregistry.k8s.io";"config.openshift.io";"compliance.mcm.ibm.com";"hive.openshift.io";"hiveinternal.openshift.io";"internal.open-cluster-management.io";"inventory.open-cluster-management.io";"mcm.ibm.com";"multicloud.ibm.com";"policy.open-cluster-management.io";"proxy.open-cluster-management.io";"rbac.authorization.k8s.io";"view.open-cluster-management.io";"operator.open-cluster-management.io";"register.open-cluster-management.io";"coordination.k8s.io";"search.open-cluster-management.io";"submarineraddon.open-cluster-management.io";"discovery.open-cluster-management.io";"imageregistry.open-cluster-management.io",resources=applications;applications/status;applicationrelationships;applicationrelationships/status;certificatesigningrequests;certificatesigningrequests/approval;channels;channels/status;clustermanagementaddons;managedclusteractions;managedclusteractions/status;clusterdeployments;clusterpools;clusterclaims;discoveryconfigs;discoveredclusters;managedclusteraddons;managedclusteraddons/status;managedclusterinfos;managedclusterinfos/status;managedclustersets;managedclustersets/bind;managedclustersets/join;managedclustersets/status;managedclustersetbindings;managedclusters;managedclusters/accept;managedclusters/status;managedclusterviews;managedclusterviews/status;manifestworks;manifestworks/status;clustercurators;clustermanagers;clusterroles;clusterrolebindings;clusterstatuses/aggregator;clusterversions;compliances;configmaps;deployables;deployables/status;deployableoverrides;deployableoverrides/status;endpoints;endpointconfigs;events;helmrepos;helmrepos/status;klusterletaddonconfigs;machinepools;namespaces;placements;placementrules/status;placementdecisions;placementdecisions/status;placementrules;placementrules/status;pods;pods/log;policies;policies/status;placementbindings;policyautomations;policysets;policysets/status;roles;rolebindings;secrets;signers;subscriptions;subscriptions/status;subjectaccessreviews;submarinerconfigs;submarinerconfigs/status;syncsets;clustersyncs;leases;searchcustomizations;managedclusterimageregistries;managedclusterimageregistries/status,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="operators.coreos.com",resources=catalogsources;subscriptions;clusterserviceversions;operatorgroups;operatorconditions,verbs=create;get;list;patch;update;delete;watch
//+kubebuilder:rbac:groups="multicluster.openshift.io",resources=multiclusterengines,verbs=create;get;list;patch;update;delete;watch
//+kubebuilder:rbac:groups=console.openshift.io;search.open-cluster-management.io,resources=consoleplugins;consolelinks;searches,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.openshift.io,resources=cloudcredentials;consoles,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=config.openshift.io,resources=authentications;infrastructures,verbs=get;list;watch
//+kubebuilder:rbac:groups="";"apps",resources=deployments;services;serviceaccounts,verbs=patch;delete;get;deletecollection
//+kubebuilder:rbac:groups=packages.operators.coreos.com,resources=packagemanifests,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules;servicemonitors,verbs=create;delete;get;list;watch;update;patch;deletecollection

//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=create;delete;get;list;watch;update;patch

// AgentServiceConfig webhook delete check
//+kubebuilder:rbac:groups=agent-install.openshift.io,resources=agentserviceconfigs,verbs=get;list;watch

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
		return ctrl.Result{RequeueAfter: utils.WarningRefreshInterval}, err
	}

	originalStatus := multiClusterHub.Status.DeepCopy()
	defer func() {
		statusQueue, statusError := r.syncHubStatus(multiClusterHub, originalStatus, allDeploys, allCRs, ocpConsole, stsEnabled)
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

	imageOverrides, err = r.overrideOauthImage(ctx, imageOverrides)
	if err != nil {
		r.Log.Error(err, "Could not override oauth image")
		return ctrl.Result{}, err
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

	var result ctrl.Result
	result, err = r.setDefaults(multiClusterHub, ocpConsole)
	if result != (ctrl.Result{}) {
		return ctrl.Result{}, err
	}
	if err != nil {
		return ctrl.Result{Requeue: true}, err
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
		return ctrl.Result{Requeue: true}, err
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

	result, err = r.ingressDomain(multiClusterHub)
	if result != (ctrl.Result{}) {
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
	if result != (ctrl.Result{}) {
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

/*
ensureAuthenticationIssuerNotEmpty ensures that the Authentication ServiceAccountIssuer is not empty.
*/
func (r *MultiClusterHubReconciler) ensureAuthenticationIssuerNotEmpty(ctx context.Context) (ctrl.Result, bool, error) {
	auth := &configv1.Authentication{}
	exists, err := r.ensureObjectExistsAndNotDeleted(ctx, auth, "cluster")

	if err != nil || !exists {
		return ctrl.Result{RequeueAfter: utils.WarningRefreshInterval}, false, err
	}

	stsEnabled := auth.Spec.ServiceAccountIssuer != "" // Determine STS enabled status

	if STSEnabledStatus && !stsEnabled {
		r.Log.Info("Cluster is no longer STS enabled due to empty Authentication ServiceAccountIssuer",
			"Name", auth.GetName())
	}

	return ctrl.Result{}, stsEnabled, nil
}

/*
ensureCloudCredentialModeManual ensures that the CloudCredential CredentialMode is set to Manual.
*/
func (r *MultiClusterHubReconciler) ensureCloudCredentialModeManual(ctx context.Context) (ctrl.Result, bool, error) {
	cloudCred := &ocopv1.CloudCredential{}
	exists, err := r.ensureObjectExistsAndNotDeleted(ctx, cloudCred, "cluster")

	if err != nil || !exists {
		return ctrl.Result{RequeueAfter: utils.WarningRefreshInterval}, false, err
	}

	stsEnabled := cloudCred.Spec.CredentialsMode == "Manual" // Determine STS enabled status

	if STSEnabledStatus && !stsEnabled {
		r.Log.Info("Cluster is no longer STS enabled due to CloudCredential CredentialMode not set to Manual.", "Name",
			cloudCred.GetName())
	}

	return ctrl.Result{}, stsEnabled, nil
}

/*
ensureInfrastructureAWS ensures that the infrastructure platform type is AWS.
*/
func (r *MultiClusterHubReconciler) ensureInfrastructureAWS(ctx context.Context) (ctrl.Result, bool, error) {
	infra := &configv1.Infrastructure{}
	exists, err := r.ensureObjectExistsAndNotDeleted(ctx, infra, "cluster")

	if err != nil || !exists {
		return ctrl.Result{RequeueAfter: utils.WarningRefreshInterval}, false, err
	}

	stsEnabled := infra.Spec.PlatformSpec.Type == "AWS"

	if STSEnabledStatus && !stsEnabled {
		r.Log.Info("Infrastructure platform type is not AWS. Cluster is not STS enabled", "Name", infra.GetName(),
			"Type", infra.Spec.PlatformSpec.Type)
	}
	return ctrl.Result{}, stsEnabled, nil
}

/*
ensureObjectExistsAndNotDeleted ensures the existence of the specified object and that it has not been deleted.
*/
func (r *MultiClusterHubReconciler) ensureObjectExistsAndNotDeleted(ctx context.Context, obj client.Object,
	name string,
) (bool, error) {
	if err := r.Client.Get(ctx, types.NamespacedName{Name: name}, obj); err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info(
				fmt.Sprintf("%s was not found. Ignoring since object must be deleted",
					reflect.TypeOf(obj).Elem().Name()), "Name", name)
			return false, nil
		}

		r.Log.Error(err, fmt.Sprintf("failed to get %s", reflect.TypeOf(obj).Elem().Name()), "Name", name)
		return false, err
	}

	return true, nil
}

/*
isSTSEnabled checks if STS (Security Token Service) is enabled by verifying that all required conditions are met.
*/
func (r *MultiClusterHubReconciler) isSTSEnabled(ctx context.Context) (bool, error) {
	_, authOK, err := r.ensureAuthenticationIssuerNotEmpty(ctx)
	if err != nil {
		return false, err
	}

	_, cloudCredOK, err := r.ensureCloudCredentialModeManual(ctx)
	if err != nil {
		return false, err
	}

	_, infraOK, err := r.ensureInfrastructureAWS(ctx)
	if err != nil {
		return false, err
	}

	// Check if all conditions are met
	allConditionsMet := authOK && cloudCredOK && infraOK

	// Check if the status has changed, and log the message if it has changed
	if allConditionsMet != STSEnabledStatus {
		STSEnabledStatus = allConditionsMet

		if STSEnabledStatus {
			r.Log.Info("STS is enabled.")
		} else {
			r.Log.Info("STS is not enabled.")
		}
	}

	// Return the combined result of all conditions
	return allConditionsMet, nil
}

func (r *MultiClusterHubReconciler) setOperatorUpgradeableStatus(ctx context.Context, m *operatorv1.MultiClusterHub) (bool, error) {
	// Temporary variable
	var upgradeable bool

	// Checking to see if the current version of the MCH matches the desired to determine if we are in an upgrade scenario
	// If the current version doesn't exist, we are currently in a install which will also not allow it to upgrade

	if m.Status.CurrentVersion != m.Status.DesiredVersion {
		upgradeable = false
	} else {
		upgradeable = true
	}
	// These messages are drawn from operator condition
	// Right now, they just indicate between upgrading and not
	msg := utils.UpgradeableAllowMessage
	status := metav1.ConditionTrue
	reason := utils.UpgradeableAllowReason

	// The condition is the only field that affects whether or not we can upgrade
	// The rest are just status info
	if !upgradeable {
		status = metav1.ConditionFalse
		reason = utils.UpgradeableUpgradingReason
		msg = utils.UpgradeableUpgradingMessage

	} else {

		msg = utils.UpgradeableAllowMessage
		status = metav1.ConditionTrue
		reason = utils.UpgradeableAllowReason

	}
	// This error should only occur if the operator condition does not exist for some reason
	if err := r.UpgradeableCond.Set(ctx, status, reason, msg); err != nil {
		return true, err
	}

	if !upgradeable {
		return true, nil
	} else {
		return false, nil
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *MultiClusterHubReconciler) SetupWithManager(mgr ctrl.Manager) (controller.Controller, error) {
	return ctrl.NewControllerManagedBy(mgr).
		For(
			&operatorv1.MultiClusterHub{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Watches(
			&appsv1.Deployment{},
			handler.EnqueueRequestForOwner(
				mgr.GetScheme(), mgr.GetRESTMapper(), &operatorv1.MultiClusterHub{}, handler.OnlyControllerOwner(),
			),
			builder.WithPredicates(
				ctrlpredicate.Or(
					ctrlpredicate.GenerationChangedPredicate{},
					ctrlpredicate.LabelChangedPredicate{},
					ctrlpredicate.AnnotationChangedPredicate{},
				),
			),
		).
		Watches(
			&apiregistrationv1.APIService{},
			handler.Funcs{
				DeleteFunc: func(ctx context.Context, e event.DeleteEvent, q workqueue.RateLimitingInterface) {
					labels := e.Object.GetLabels()
					q.Add(
						reconcile.Request{
							NamespacedName: types.NamespacedName{
								Name:      labels["installer.name"],
								Namespace: labels["installer.namespace"],
							},
						},
					)
				},
			},
			builder.WithPredicates(predicate.DeletePredicate{}),
		).
		Watches(&appsv1.Deployment{},
			handler.EnqueueRequestsFromMapFunc(
				func(ctx context.Context, a client.Object) []reconcile.Request {
					return []reconcile.Request{
						{
							NamespacedName: types.NamespacedName{
								Name:      a.GetLabels()["installer.name"],
								Namespace: a.GetLabels()["installer.namespace"],
							},
						},
					}
				},
			),
			builder.WithPredicates(
				ctrlpredicate.And(
					predicate.InstallerLabelPredicate{},
					ctrlpredicate.Or(
						ctrlpredicate.GenerationChangedPredicate{},
						ctrlpredicate.LabelChangedPredicate{},
						ctrlpredicate.AnnotationChangedPredicate{},
					),
				),
			),
		).
		Watches(
			&configv1.ClusterVersion{},
			handler.EnqueueRequestsFromMapFunc(
				func(ctx context.Context, a client.Object) []reconcile.Request {
					multiClusterHubList := &operatorv1.MultiClusterHubList{}
					if err := r.Client.List(context.TODO(), multiClusterHubList); err == nil && len(multiClusterHubList.Items) > 0 {
						mch := multiClusterHubList.Items[0]
						return []reconcile.Request{
							{
								NamespacedName: types.NamespacedName{
									Name:      mch.GetName(),
									Namespace: mch.GetNamespace(),
								},
							},
						}
					}
					return []reconcile.Request{}
				},
			),
		).
		Build(r)
}

func (r *MultiClusterHubReconciler) applyTemplate(ctx context.Context, m *operatorv1.MultiClusterHub, template *unstructured.Unstructured) (ctrl.Result, error) {
	// Set owner reference.
	if (template.GetKind() == "ClusterRole") || (template.GetKind() == "ClusterRoleBinding") || (template.GetKind() == "ServiceMonitor") || (template.GetKind() == "CustomResourceDefinition") {
		utils.AddInstallerLabel(template, m.Name, m.Namespace)
	}

	if template.GetKind() == "APIService" {
		result, err := r.ensureUnstructuredResource(m, template)
		if err != nil {
			log.Info(err.Error())
			return result, err
		}
	} else {
		// Check if the resource exists before creating it.
		for _, gvk := range operatorv1.MCECRDs {
			if template.GroupVersionKind().Group == gvk.Group && template.GetKind() == gvk.Kind && template.GroupVersionKind().Version == gvk.Version {
				crd := &apixv1.CustomResourceDefinition{}

				if err := r.Client.Get(ctx, types.NamespacedName{Name: gvk.Name}, crd); errors.IsNotFound(err) {
					log.Info("CustomResourceDefinition does not exist. Skipping resource creation",
						"Group", gvk.Group, "Version", gvk.Version, "Kind", gvk.Kind)

					return ctrl.Result{RequeueAfter: utils.WarningRefreshInterval}, nil
				} else if err != nil {
					log.Error(err, "failed to get CustomResourceDefinition", "Resource", gvk)
					return ctrl.Result{RequeueAfter: utils.WarningRefreshInterval}, err
				}
			}
		}

		// Apply the object data.
		force := true
		err := r.Client.Patch(ctx, template, client.Apply, &client.PatchOptions{Force: &force, FieldManager: "multiclusterhub-operator"})
		if err != nil {
			log.Info(err.Error())
			wrappedError := pkgerrors.Wrapf(err, "error applying object Name: %s Kind: %s", template.GetName(), template.GetKind())
			SetHubCondition(&m.Status, *NewHubCondition(operatorv1.ComponentFailure+": "+operatorv1.HubConditionType(template.GetName())+"(Kind:)"+operatorv1.HubConditionType(template.GetKind()), metav1.ConditionTrue, FailedApplyingComponent, wrappedError.Error()))
			return ctrl.Result{}, wrappedError
		}
	}
	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) fetchChartLocation(component string) string {
	switch component {
	case operatorv1.Appsub:
		return utils.AppsubChartLocation

	case operatorv1.ClusterBackup:
		return utils.ClusterBackupChartLocation

	case operatorv1.ClusterLifecycle:
		return utils.CLCChartLocation

	case operatorv1.ClusterPermission:
		return utils.ClusterPermissionChartLocation

	case operatorv1.Console:
		return utils.ConsoleChartLocation

	case operatorv1.GRC:
		return utils.GRCChartLocation

	case operatorv1.Insights:
		return utils.InsightsChartLocation

	case operatorv1.MCH:
		return ""

	case operatorv1.MultiClusterObservability:
		return utils.MCOChartLocation

	case operatorv1.Search:
		return utils.SearchV2ChartLocation

	case operatorv1.SubmarinerAddon:
		return utils.SubmarinerAddonChartLocation

	case operatorv1.Volsync:
		return utils.VolsyncChartLocation

	default:
		log.Info(fmt.Sprintf("Unregistered component detected: %v", component))
		return fmt.Sprintf("/chart/toggle/%v", component)
	}
}

func (r *MultiClusterHubReconciler) ensureComponentOrNoComponent(ctx context.Context, m *operatorv1.MultiClusterHub,
	component string, cachespec CacheSpec, ocpConsole, isSTSEnabled bool,
) (ctrl.Result, error) {
	var result ctrl.Result
	var err error

	if !m.Enabled(component) {
		if component == operatorv1.ClusterBackup {
			result, err = r.ensureNoComponent(ctx, m, component, cachespec, isSTSEnabled)
			if result != (ctrl.Result{}) || err != nil {
				return result, err
			}
			return r.ensureNoNamespace(m, BackupNamespaceUnstructured())
		}
		return r.ensureNoComponent(ctx, m, component, cachespec, isSTSEnabled)

	} else {
		if component == operatorv1.ClusterBackup {
			result, err = r.ensureNamespaceAndPullSecret(m, BackupNamespace())
			if result != (ctrl.Result{}) || err != nil {
				return result, err
			}
		}

		if component == operatorv1.Console && !ocpConsole {
			log.Info("OCP console is not enabled")
			return r.ensureNoComponent(ctx, m, component, cachespec, isSTSEnabled)
		}

		return r.ensureComponent(ctx, m, component, cachespec, isSTSEnabled)
	}
}

func (r *MultiClusterHubReconciler) ensureNamespaceAndPullSecret(m *operatorv1.MultiClusterHub, ns *corev1.Namespace) (
	ctrl.Result, error,
) {
	var result ctrl.Result
	var err error

	result, err = r.ensureNamespace(m, ns)
	if result != (ctrl.Result{}) {
		return result, err
	}

	result, err = r.ensurePullSecret(m, ns.Name)
	if result != (ctrl.Result{}) {
		return result, err
	}

	return result, err
}

func (r *MultiClusterHubReconciler) ensureComponent(ctx context.Context, m *operatorv1.MultiClusterHub, component string,
	cachespec CacheSpec, isSTSEnabled bool,
) (ctrl.Result, error) {
	/*
	   If the component is detected to be MCH, we can simply return successfully. MCH is only listed in the components
	   list for cleanup purposes.
	*/
	if component == operatorv1.MCH || component == operatorv1.MultiClusterEngine {
		return ctrl.Result{}, nil
	}

	chartLocation := r.fetchChartLocation(component)

	// Renders all templates from charts
	templates, errs := renderer.RenderChart(chartLocation, m, cachespec.ImageOverrides, cachespec.TemplateOverrides,
		isSTSEnabled)

	if len(errs) > 0 {
		for _, err := range errs {
			log.Info(err.Error())
		}
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}

	// Applies all templates
	for _, template := range templates {
		annotations := template.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[utils.AnnotationReleaseVersion] = version.Version
		template.SetAnnotations(annotations)
		result, err := r.applyTemplate(ctx, m, template)
		if err != nil {
			return result, err
		}
	}

	switch component {
	case operatorv1.Console:
		return r.addPluginToConsole(m)

	case operatorv1.Search:
		return r.ensureSearchCR(m)

	default:
		return ctrl.Result{}, nil
	}
}

func (r *MultiClusterHubReconciler) ensureNoComponent(ctx context.Context, m *operatorv1.MultiClusterHub,
	component string, cachespec CacheSpec, isSTSEnabled bool,
) (result ctrl.Result, err error) {
	/*
	   If the component is detected to be MCH, we can simply return successfully. MCH is only listed in the components
	   list for cleanup purposes. If the component is detected to be MCE, we can simply return successfully.
	   MCE is only listed in the components list for webhook validation purposes.
	*/
	if component == operatorv1.MCH || component == operatorv1.MultiClusterEngine {
		return ctrl.Result{}, nil
	}

	chartLocation := r.fetchChartLocation(component)

	switch component {
	case operatorv1.Console:
		ocpConsole, err := r.CheckConsole(ctx)
		if err != nil {
			r.Log.Error(err, "error finding OCP Console")
			return ctrl.Result{}, err
		}
		if !ocpConsole {
			// If Openshift console is disabled then no cleanup to be done, because MCH console cannot be installed
			return ctrl.Result{}, nil
		}

		result, err := r.removePluginFromConsole(m)
		if result != (ctrl.Result{}) {
			return result, err
		}

	// SearchV2
	case operatorv1.Search:
		result, err := r.ensureNoSearchCR(m)
		if err != nil {
			return result, err
		}

	/*
	   In ACM 2.9 we need to ensure that the submariner ClusterManagementAddOn is removed before
	   removing the submariner-addon component.
	*/
	case operatorv1.SubmarinerAddon:
		result, err := r.ensureNoClusterManagementAddOn(m, component)
		if err != nil {
			return result, err
		}
	}

	// Renders all templates from charts
	templates, errs := renderer.RenderChart(chartLocation, m, cachespec.ImageOverrides, cachespec.TemplateOverrides,
		isSTSEnabled)

	if len(errs) > 0 {
		for _, err := range errs {
			log.Info(err.Error())
		}
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}

	// Deletes all templates
	for _, template := range templates {
		result, err := r.deleteTemplate(ctx, m, template)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to delete template: %s", template.GetName()))
			return result, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) ensureOpenShiftNamespaceLabel(ctx context.Context, m *operatorv1.MultiClusterHub) (
	ctrl.Result, error,
) {
	existingNs := &corev1.Namespace{}

	err := r.Client.Get(ctx, types.NamespacedName{Name: m.GetNamespace()}, existingNs)
	if err != nil || errors.IsNotFound(err) {
		log.Error(err, fmt.Sprintf("Failed to find namespace for MultiClusterHub: %s", m.GetNamespace()))
		return ctrl.Result{Requeue: true}, err
	}

	if existingNs.Labels == nil || len(existingNs.Labels) == 0 {
		existingNs.Labels = make(map[string]string)
	}

	if _, ok := existingNs.Labels[utils.OpenShiftClusterMonitoringLabel]; !ok {
		r.Log.Info(fmt.Sprintf("Adding label: %s to namespace: %s", utils.OpenShiftClusterMonitoringLabel,
			m.GetNamespace()))
		existingNs.Labels[utils.OpenShiftClusterMonitoringLabel] = "true"

		err = r.Client.Update(ctx, existingNs)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to update namespace for MultiClusterHub: %s with the label: %s",
				m.GetNamespace(), utils.OpenShiftClusterMonitoringLabel))
			return ctrl.Result{Requeue: true}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) deleteTemplate(ctx context.Context, m *operatorv1.MultiClusterHub,
	template *unstructured.Unstructured,
) (ctrl.Result, error) {
	err := r.Client.Get(ctx, types.NamespacedName{Name: template.GetName(), Namespace: template.GetNamespace()}, template)

	if err != nil && (errors.IsNotFound(err) || apimeta.IsNoMatchError(err)) {
		return ctrl.Result{}, nil
	}

	// set status progressing condition
	if err != nil {
		log.Error(err, "Odd error delete template")
		return ctrl.Result{}, err
	}

	log.Info(fmt.Sprintf("finalizing template: %s\n", template.GetName()))
	err = r.Client.Delete(ctx, template)
	if err != nil {
		log.Error(err, "Failed to delete template")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// createCAconfigmap creates a configmap that will be injected with the
// trusted CA bundle for use with the OCP cluster wide proxy
func (r *MultiClusterHubReconciler) createTrustBundleConfigmap(ctx context.Context, mch *operatorv1.MultiClusterHub) (
	ctrl.Result, error,
) {
	// Get Trusted Bundle configmap name
	trustBundleName := defaultTrustBundleName
	trustBundleNamespace := mch.Namespace
	if name, ok := os.LookupEnv(trustBundleNameEnvVar); ok && name != "" {
		trustBundleName = name
	}
	namespacedName := types.NamespacedName{
		Name:      trustBundleName,
		Namespace: trustBundleNamespace,
	}
	log.Info(fmt.Sprintf("using trust bundle configmap %s/%s", trustBundleNamespace, trustBundleName))

	// Check if configmap exists
	cm := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, namespacedName, cm)
	if err != nil && !errors.IsNotFound(err) {
		// Unknown error. Requeue
		msg := fmt.Sprintf("error while getting trust bundle configmap %s/%s", trustBundleNamespace, trustBundleName)
		log.Error(err, msg)
		return ctrl.Result{RequeueAfter: resyncPeriod}, err
	} else if err == nil {
		// configmap exists
		return ctrl.Result{}, nil
	}

	// Create configmap
	cm = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      trustBundleName,
			Namespace: trustBundleNamespace,
			Labels: map[string]string{
				"config.openshift.io/inject-trusted-cabundle": "true",
			},
		},
	}
	err = ctrl.SetControllerReference(mch, cm, r.Scheme)
	if err != nil {
		return ctrl.Result{}, pkgerrors.Wrapf(
			err, "Error setting controller reference on trust bundle configmap %s",
			trustBundleName,
		)
	}
	err = r.Client.Create(ctx, cm)
	if err != nil {
		// Error creating configmap
		log.Info(fmt.Sprintf("error creating trust bundle configmap %s: %s", trustBundleName, err))
		return ctrl.Result{RequeueAfter: resyncPeriod}, err
	}
	// Configmap created successfully
	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) createMetricsService(ctx context.Context, m *operatorv1.MultiClusterHub) (
	ctrl.Result, error,
) {
	const Port = 8383

	sName := utils.MCHOperatorMetricsServiceName
	sNamespace := m.GetNamespace()

	namespacedName := types.NamespacedName{
		Name:      sName,
		Namespace: sNamespace,
	}

	// Check if service exists
	if err := r.Client.Get(ctx, namespacedName, &corev1.Service{}); err != nil {
		if !errors.IsNotFound(err) {
			// Unknown error. Requeue
			log.Error(err, fmt.Sprintf("error while getting multiclusterhub metrics service: %s/%s", sNamespace, sName))
			return ctrl.Result{RequeueAfter: resyncPeriod}, err
		}

		// Create metrics service
		s := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sName,
				Namespace: sNamespace,
				Labels: map[string]string{
					"name": operatorv1.MCH,
				},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:       "metrics",
						Port:       int32(Port),
						Protocol:   "TCP",
						TargetPort: intstr.FromInt(Port),
					},
				},
				Selector: map[string]string{
					"name": operatorv1.MCH,
				},
			},
		}

		if err = ctrl.SetControllerReference(m, s, r.Scheme); err != nil {
			return ctrl.Result{}, pkgerrors.Wrapf(
				err, "error setting controller reference on metrics service: %s", sName,
			)
		}

		if err = r.Client.Create(ctx, s); err != nil {
			// Error creating metrics service
			log.Error(err, fmt.Sprintf("error creating multiclusterhub metrics service: %s", sName))
			return ctrl.Result{RequeueAfter: resyncPeriod}, err
		}

		log.Info(fmt.Sprintf("Created multiclusterhub metrics service: %s", sName))
	}

	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) createMetricsServiceMonitor(ctx context.Context, m *operatorv1.MultiClusterHub) (
	ctrl.Result, error,
) {
	smName := utils.MCHOperatorMetricsServiceMonitorName
	smNamespace := m.GetNamespace()

	namespacedName := types.NamespacedName{
		Name:      smName,
		Namespace: smNamespace,
	}

	// Check if service exists
	if err := r.Client.Get(ctx, namespacedName, &promv1.ServiceMonitor{}); err != nil {
		if !errors.IsNotFound(err) {
			// Unknown error. Requeue
			log.Error(err, fmt.Sprintf("error while getting multiclusterhub metrics service: %s/%s", smNamespace, smName))
			return ctrl.Result{RequeueAfter: resyncPeriod}, err
		}

		// Create metrics service
		sm := &promv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      smName,
				Namespace: smNamespace,
				Labels: map[string]string{
					"name": operatorv1.MCH,
				},
			},
			Spec: promv1.ServiceMonitorSpec{
				Endpoints: []promv1.Endpoint{
					{
						BearerTokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
						BearerTokenSecret: &corev1.SecretKeySelector{
							Key: "",
						},
						Port: "metrics",
					},
				},
				NamespaceSelector: promv1.NamespaceSelector{
					MatchNames: []string{
						m.GetNamespace(),
					},
				},
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"name": operatorv1.MCH,
					},
				},
			},
		}

		if err = ctrl.SetControllerReference(m, sm, r.Scheme); err != nil {
			return ctrl.Result{}, pkgerrors.Wrapf(
				err, "error setting controller reference on multiclusterhub metrics servicemonitor: %s", smName)
		}

		if err = r.Client.Create(ctx, sm); err != nil {
			// Error creating metrics servicemonitor
			log.Error(err, fmt.Sprintf("error creating metrics servicemonitor: %s", smName))
			return ctrl.Result{RequeueAfter: resyncPeriod}, err
		}

		log.Info(fmt.Sprintf("Created multiclusterhub metrics servicemonitor: %s", smName))
	}

	return ctrl.Result{}, nil
}

// ingressDomain is discovered from Openshift cluster configuration resources
func (r *MultiClusterHubReconciler) ingressDomain(m *operatorv1.MultiClusterHub) (ctrl.Result, error) {
	if r.CacheSpec.IngressDomain != "" || utils.IsUnitTest() {
		err := os.Setenv("INGRESS_DOMAIN", "dev01.red-chesterfield.com")
		return ctrl.Result{}, err
	}
	ingress := &configv1.Ingress{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{
		Name: "cluster",
	}, ingress)
	// Don't fail on a unit test (Fake client won't find "cluster" Ingress)
	if err != nil {
		r.Log.Error(err, "Failed to get Ingress")
		return ctrl.Result{}, err
	}

	r.CacheSpec.IngressDomain = ingress.Spec.Domain
	// Set OCP version as env var, so that charts can render this value
	err = os.Setenv("INGRESS_DOMAIN", ingress.Spec.Domain)
	if err != nil {
		r.Log.Error(err, "Failed to set INGRESS_DOMAIN environment variable")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) finalizeHub(reqLogger logr.Logger, m *operatorv1.MultiClusterHub, ocpConsole,
	isSTSEnabled bool,
) error {
	if err := r.cleanupAppSubscriptions(reqLogger, m); err != nil {
		return err
	}

	for _, c := range operatorv1.MCHComponents {
		if _, err := r.ensureNoComponent(context.TODO(), m, c, r.CacheSpec, isSTSEnabled); err != nil {
			return err
		}
	}

	cleanupFunctions := []func(reqLogger logr.Logger, m *operatorv1.MultiClusterHub) error{
		r.cleanupNamespaces, r.cleanupClusterRoles, r.cleanupClusterRoleBindings,
		r.cleanupMultiClusterEngine, r.orphanOwnedMultiClusterEngine,
	}

	for _, cleanupFn := range cleanupFunctions {
		if err := cleanupFn(reqLogger, m); err != nil {
			return err
		}
	}

	reqLogger.Info("Successfully finalized multiClusterHub")
	return nil
}

func (r *MultiClusterHubReconciler) installCRDs(reqLogger logr.Logger, m *operatorv1.MultiClusterHub) (string, error) {
	crdDir, ok := os.LookupEnv(crdPathEnvVar)
	if !ok {
		err := fmt.Errorf("%s environment variable is required", crdPathEnvVar)
		reqLogger.Error(err, err.Error())
		return CRDRenderReason, err
	}

	crds, errs := renderer.RenderCRDs(crdDir, m)
	if len(errs) > 0 {
		message := mergeErrors(errs)
		err := fmt.Errorf("failed to render CRD templates: %s", message)
		reqLogger.Error(err, err.Error())
		return CRDRenderReason, err
	}

	for _, crd := range crds {
		utils.AddInstallerLabel(crd, m.GetName(), m.GetNamespace())
		err, ok := deploying.Deploy(r.Client, crd)
		if err != nil {
			err := fmt.Errorf("failed to deploy %s %s", crd.GetKind(), crd.GetName())
			reqLogger.Error(err, err.Error())
			return DeployFailedReason, err
		}
		if ok {
			message := fmt.Sprintf("created new resource: %s %s", crd.GetKind(), crd.GetName())
			condition := NewHubCondition(operatorv1.Progressing, metav1.ConditionTrue, NewComponentReason, message)
			SetHubCondition(&m.Status, *condition)
		}
	}
	return "", nil
}

func (r *MultiClusterHubReconciler) deployResources(reqLogger logr.Logger, m *operatorv1.MultiClusterHub) (string, error) {
	resourceDir, ok := os.LookupEnv(templatesPathEnvVar)
	if !ok {
		err := fmt.Errorf("%s environment variable is required", templatesPathEnvVar)
		reqLogger.Error(err, err.Error())
		return ResourceRenderReason, err
	}

	resourceDir = path.Join(resourceDir, templatesKind, "base")
	files, err := os.ReadDir(resourceDir)
	if err != nil {
		err := fmt.Errorf("unable to read resource files from %s : %s", resourceDir, err)
		reqLogger.Error(err, err.Error())
		return ResourceRenderReason, err
	}

	resources := make([]*unstructured.Unstructured, 0, len(files))
	errs := make([]error, 0, len(files))
	for _, file := range files {
		fileName := file.Name()
		if filepath.Ext(fileName) != ".yaml" {
			continue
		}

		path := path.Join(resourceDir, fileName)
		src, err := os.ReadFile(filepath.Clean(path)) // #nosec G304 (filepath cleaned)
		if err != nil {
			errs = append(errs, fmt.Errorf("error reading file %s : %s", fileName, err))
			continue
		}

		resource := &unstructured.Unstructured{}
		if err = yaml.Unmarshal(src, resource); err != nil {
			errs = append(errs, fmt.Errorf("error unmarshalling file %s to unstructured: %s", fileName, err))
			continue
		}

		resources = append(resources, resource)
	}

	if len(errs) > 0 {
		message := mergeErrors(errs)
		err := fmt.Errorf("failed to render resources: %s", message)
		reqLogger.Error(err, err.Error())
		return CRDRenderReason, err
	}

	for _, res := range resources {
		if res.GetNamespace() == m.Namespace {
			err := controllerutil.SetControllerReference(m, res, r.Scheme)
			if err != nil {
				r.Log.Error(
					err,
					fmt.Sprintf(
						"Failed to set controller reference on %s %s/%s",
						res.GetKind(), m.Namespace, res.GetName(),
					),
				)
			}
		}
		err, ok := deploying.Deploy(r.Client, res)
		if err != nil {
			err := fmt.Errorf("failed to deploy %s %s", res.GetKind(), res.GetName())
			reqLogger.Error(err, err.Error())
			return DeployFailedReason, err
		}
		if ok {
			message := fmt.Sprintf("created new resource: %s %s", res.GetKind(), res.GetName())
			condition := NewHubCondition(operatorv1.Progressing, metav1.ConditionTrue, NewComponentReason, message)
			SetHubCondition(&m.Status, *condition)
		}
	}

	return "", nil
}

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

	for _, c := range operatorv1.MCEComponents {
		if m.Prune(c) {
			log.Info(fmt.Sprintf("Removing MultiClusterEngine component: %v from existing MultiClusterHub", c))
			updateNecessary = true
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
		{"ingress", !reflect.DeepEqual(m.Spec.Ingress, operatorv1.IngressSpec{})},
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

/*
overrideOauthImage select the oauth image to use for the given build.
Select oauth proxy image to use. If OCP 4.15 use old version. If OCP 4.16+ use new version. Set with key oauth_proxy
before applying overrides.
*/
func (r *MultiClusterHubReconciler) overrideOauthImage(ctx context.Context, imageOverrides map[string]string) (
	map[string]string, error) {
	ocpVersion, err := r.getClusterVersion(ctx)
	if err != nil {
		return nil, err
	}

	semverVersion, err := semver.NewVersion(ocpVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to convert ocp version to semver compatible version: %v", err)
	}

	constraint, err := semver.NewConstraint(">= 4.16.0-0")
	if err != nil {
		return nil, fmt.Errorf("failed to set ocp version constraint: %v", err)
	}

	oauthKey := "oauth_proxy"
	oauthKeyOld := "oauth_proxy_415_and_below"
	oauthKeyNew := "oauth_proxy_416_and_up"

	if constraint.Check(semverVersion) { // use newer ouath image
		imageOverrides[oauthKey] = imageOverrides[oauthKeyNew]

	} else { // use older ouath image
		imageOverrides[oauthKey] = imageOverrides[oauthKeyOld]
	}

	return imageOverrides, nil
}
