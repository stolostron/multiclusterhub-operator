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
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"

	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/deploying"
	"github.com/stolostron/multiclusterhub-operator/pkg/imageoverrides"
	"github.com/stolostron/multiclusterhub-operator/pkg/predicate"
	renderer "github.com/stolostron/multiclusterhub-operator/pkg/rendering"
	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"
	"github.com/stolostron/multiclusterhub-operator/pkg/version"
	ctrlpredicate "sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/yaml"

	configv1 "github.com/openshift/api/config/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"

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
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	pkgerrors "github.com/pkg/errors"
)

// MultiClusterHubReconciler reconciles a MultiClusterHub object
type MultiClusterHubReconciler struct {
	Client          client.Client
	UncachedClient  client.Client
	CacheSpec       CacheSpec
	Scheme          *runtime.Scheme
	Log             logr.Logger
	UpgradeableCond utils.Condition
}

const (
	resyncPeriod = time.Second * 20

	crdPathEnvVar       = "CRDS_PATH"
	templatesPathEnvVar = "TEMPLATES_PATH"
	templatesKind       = "multiclusterhub"
	hubFinalizer        = "finalizer.operator.open-cluster-management.io"

	trustBundleNameEnvVar  = "TRUSTED_CA_BUNDLE"
	defaultTrustBundleName = "trusted-ca-bundle"

	mceUpgradeDuration = 10 * time.Minute
)

var (
	mceUpgradeStartTime = time.Time{}
)

//+kubebuilder:rbac:groups="";"admissionregistration.k8s.io";"apiextensions.k8s.io";"apiregistration.k8s.io";"apps";"apps.open-cluster-management.io";"authorization.k8s.io";"hive.openshift.io";"mcm.ibm.com";"proxy.open-cluster-management.io";"rbac.authorization.k8s.io";"security.openshift.io";"clusterview.open-cluster-management.io";"discovery.open-cluster-management.io";"wgpolicyk8s.io",resources=apiservices;channels;clusterjoinrequests;clusterrolebindings;clusterstatuses/log;configmaps;customresourcedefinitions;deployments;discoveryconfigs;hiveconfigs;mutatingwebhookconfigurations;validatingwebhookconfigurations;namespaces;pods;policyreports;replicasets;rolebindings;secrets;serviceaccounts;services;subjectaccessreviews;subscriptions;helmreleases;managedclusters;managedclustersets,verbs=get
//+kubebuilder:rbac:groups="";"admissionregistration.k8s.io";"apiextensions.k8s.io";"apiregistration.k8s.io";"apps";"apps.open-cluster-management.io";"authorization.k8s.io";"hive.openshift.io";"monitoring.coreos.com";"rbac.authorization.k8s.io";"mcm.ibm.com";"security.openshift.io",resources=apiservices;channels;clusterjoinrequests;clusterrolebindings;clusterroles;configmaps;customresourcedefinitions;deployments;hiveconfigs;mutatingwebhookconfigurations;validatingwebhookconfigurations;namespaces;rolebindings;secrets;serviceaccounts;services;servicemonitors;subjectaccessreviews;subscriptions;validatingwebhookconfigurations,verbs=create;update
//+kubebuilder:rbac:groups="";"apps";"apps.open-cluster-management.io";"admissionregistration.k8s.io";"apiregistration.k8s.io";"authorization.k8s.io";"config.openshift.io";"inventory.open-cluster-management.io";"mcm.ibm.com";"observability.open-cluster-management.io";"operator.open-cluster-management.io";"rbac.authorization.k8s.io";"hive.openshift.io";"clusterview.open-cluster-management.io";"discovery.open-cluster-management.io";"wgpolicyk8s.io",resources=apiservices;clusterjoinrequests;configmaps;deployments;discoveryconfigs;helmreleases;ingresses;multiclusterhubs;multiclusterobservabilities;namespaces;hiveconfigs;rolebindings;servicemonitors;secrets;services;subjectaccessreviews;subscriptions;validatingwebhookconfigurations;pods;policyreports;managedclusters;managedclustersets,verbs=list
//+kubebuilder:rbac:groups="";"admissionregistration.k8s.io";"apiregistration.k8s.io";"apps";"authorization.k8s.io";"config.openshift.io";"mcm.ibm.com";"operator.open-cluster-management.io";"rbac.authorization.k8s.io";"storage.k8s.io";"apps.open-cluster-management.io";"hive.openshift.io";"clusterview.open-cluster-management.io";"wgpolicyk8s.io",resources=apiservices;helmreleases;hiveconfigs;configmaps;clusterjoinrequests;deployments;ingresses;multiclusterhubs;namespaces;rolebindings;secrets;services;subjectaccessreviews;validatingwebhookconfigurations;pods;policyreports;managedclusters;managedclustersets,verbs=watch;list
//+kubebuilder:rbac:groups="";"admissionregistration.k8s.io";"apps";"apps.open-cluster-management.io";"mcm.ibm.com";"monitoring.coreos.com";"operator.open-cluster-management.io";,resources=deployments;deployments/finalizers;helmreleases;services;services/finalizers;servicemonitors;servicemonitors/finalizers;validatingwebhookconfigurations;multiclusterhubs;multiclusterhubs/finalizers;multiclusterhubs/status,verbs=update
//+kubebuilder:rbac:groups="admissionregistration.k8s.io";"apiextensions.k8s.io";"apiregistration.k8s.io";"hive.openshift.io";"mcm.ibm.com";"rbac.authorization.k8s.io";,resources=apiservices;clusterroles;clusterrolebindings;customresourcedefinitions;hiveconfigs;mutatingwebhookconfigurations;validatingwebhookconfigurations,verbs=delete;deletecollection;list;watch;patch
//+kubebuilder:rbac:groups="";"apps";"apiregistration.k8s.io";"apps.open-cluster-management.io";"apiextensions.k8s.io";,resources=deployments;services;channels;customresourcedefinitions;apiservices,verbs=delete
//+kubebuilder:rbac:groups="";"action.open-cluster-management.io";"addon.open-cluster-management.io";"agent.open-cluster-management.io";"argoproj.io";"cluster.open-cluster-management.io";"work.open-cluster-management.io";"app.k8s.io";"apps.open-cluster-management.io";"authorization.k8s.io";"certificates.k8s.io";"clusterregistry.k8s.io";"config.openshift.io";"compliance.mcm.ibm.com";"hive.openshift.io";"hiveinternal.openshift.io";"internal.open-cluster-management.io";"inventory.open-cluster-management.io";"mcm.ibm.com";"multicloud.ibm.com";"policy.open-cluster-management.io";"proxy.open-cluster-management.io";"rbac.authorization.k8s.io";"view.open-cluster-management.io";"operator.open-cluster-management.io";"register.open-cluster-management.io";"coordination.k8s.io";"search.open-cluster-management.io";"submarineraddon.open-cluster-management.io";"discovery.open-cluster-management.io";"imageregistry.open-cluster-management.io",resources=applications;applications/status;applicationrelationships;applicationrelationships/status;certificatesigningrequests;certificatesigningrequests/approval;channels;channels/status;clustermanagementaddons;managedclusteractions;managedclusteractions/status;clusterdeployments;clusterpools;clusterclaims;discoveryconfigs;discoveredclusters;managedclusteraddons;managedclusteraddons/status;managedclusterinfos;managedclusterinfos/status;managedclustersets;managedclustersets/bind;managedclustersets/join;managedclustersets/status;managedclustersetbindings;managedclusters;managedclusters/accept;managedclusters/status;managedclusterviews;managedclusterviews/status;manifestworks;manifestworks/status;clustercurators;clustermanagers;clusterroles;clusterrolebindings;clusterstatuses/aggregator;clusterversions;compliances;configmaps;deployables;deployables/status;deployableoverrides;deployableoverrides/status;endpoints;endpointconfigs;events;helmrepos;helmrepos/status;klusterletaddonconfigs;machinepools;namespaces;placements;placementrules/status;placementdecisions;placementdecisions/status;placementrules;placementrules/status;pods;pods/log;policies;policies/status;placementbindings;policyautomations;policysets;policysets/status;roles;rolebindings;secrets;signers;subscriptions;subscriptions/status;subjectaccessreviews;submarinerconfigs;submarinerconfigs/status;syncsets;clustersyncs;leases;searchcustomizations;managedclusterimageregistries;managedclusterimageregistries/status,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="operators.coreos.com",resources=subscriptions;clusterserviceversions;operatorgroups;operatorconditions,verbs=create;get;list;patch;update;delete;watch
//+kubebuilder:rbac:groups="multicluster.openshift.io",resources=multiclusterengines,verbs=create;get;list;patch;update;delete;watch
//+kubebuilder:rbac:groups=console.openshift.io;search.open-cluster-management.io,resources=consoleplugins;consolelinks;searches,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.openshift.io,resources=consoles,verbs=get;list;watch;update;patch
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
	r.Log = log.FromContext(ctx)

	r.Log.Info("Reconciling MultiClusterHub")

	// Fetch the MultiClusterHub instance
	multiClusterHub := &operatorv1.MultiClusterHub{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, multiClusterHub)
	if err != nil {
		if apierrors.IsNotFound(err) {
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

	if multiClusterHub.IsInHostedMode() {
		return r.HostedReconcile(ctx, multiClusterHub)
	}

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

	originalStatus := multiClusterHub.Status.DeepCopy()
	defer func() {
		statusQueue, statusError := r.syncHubStatus(multiClusterHub, originalStatus, allDeploys, allCRs, ocpConsole)
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

	// Read image overrides
	// First, attempt to read image overrides from environmental variables
	imageOverrides := imageoverrides.GetImageOverrides()
	if len(imageOverrides) == 0 {
		r.Log.Error(err, "Could not get map of image overrides")
		return ctrl.Result{}, nil
	}

	if imageRepo := utils.GetImageRepository(multiClusterHub); imageRepo != "" {
		r.Log.Info(fmt.Sprintf("Overriding Image Repository from annotation 'mch-imageRepository': %s", imageRepo))
		imageOverrides = utils.OverrideImageRepository(imageOverrides, imageRepo)
	}

	// Check for developer overrides
	if imageOverridesConfigmap := utils.GetImageOverridesConfigmap(multiClusterHub); imageOverridesConfigmap != "" {
		imageOverrides, err = r.OverrideImagesFromConfigmap(imageOverrides, multiClusterHub.GetNamespace(), imageOverridesConfigmap)
		if err != nil {
			r.Log.Error(err, fmt.Sprintf("Could not find image override configmap: %s/%s", multiClusterHub.GetNamespace(), imageOverridesConfigmap))
			return ctrl.Result{}, err
		}
	}
	r.CacheSpec.ImageOverrides = imageOverrides
	r.CacheSpec.ManifestVersion = version.Version
	r.CacheSpec.ImageRepository = utils.GetImageRepository(multiClusterHub)
	r.CacheSpec.ImageOverridesCM = utils.GetImageOverridesConfigmap(multiClusterHub)

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
			if err := r.finalizeHub(r.Log, multiClusterHub, ocpConsole); err != nil {
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
			condition := NewHubCondition(operatorv1.Progressing, metav1.ConditionFalse, RequirementsNotMetReason, fmt.Sprintf("OCP version requirement not met: %s", err.Error()))
			SetHubCondition(&multiClusterHub.Status, *condition)
			return ctrl.Result{}, err
		}
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
		result, err = r.ensureAppsub(ctx, multiClusterHub, r.CacheSpec.ImageOverrides)
	} else {
		result, err = r.ensureNoAppsub(ctx, multiClusterHub, r.CacheSpec.ImageOverrides)
	}
	if result != (ctrl.Result{}) {
		return result, err
	}

	// Remove existing appsubs and helmreleases if present from upgrade
	result, err = r.ensureAppsubsGone(multiClusterHub)
	if result != (ctrl.Result{}) {
		return result, err
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

	if utils.ProxyEnvVarsAreSet() {
		r.Log.Info(fmt.Sprintf("Proxy configuration environment variables are set. HTTP_PROXY: %s, HTTPS_PROXY: %s, NO_PROXY: %s", os.Getenv("HTTP_PROXY"), os.Getenv("HTTPS_PROXY"), os.Getenv("NO_PROXY")))
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

	// Install the rest of the subscriptions in no particular order

	if multiClusterHub.Enabled(operatorv1.Console) && ocpConsole {
		result, err = r.ensureConsole(ctx, multiClusterHub, r.CacheSpec.ImageOverrides)
	} else {
		result, err = r.ensureNoConsole(ctx, multiClusterHub, r.CacheSpec.ImageOverrides)
	}
	if result != (ctrl.Result{}) {
		return result, err
	}
	if multiClusterHub.Enabled(operatorv1.Insights) {
		result, err = r.ensureInsights(ctx, multiClusterHub, r.CacheSpec.ImageOverrides)
	} else {
		result, err = r.ensureNoInsights(ctx, multiClusterHub, r.CacheSpec.ImageOverrides)
	}
	if result != (ctrl.Result{}) {
		return result, err
	}
	if multiClusterHub.Enabled(operatorv1.Search) {
		result, err = r.ensureSearchV2(ctx, multiClusterHub, r.CacheSpec.ImageOverrides)
	} else {
		result, err = r.ensureNoSearchV2(ctx, multiClusterHub, r.CacheSpec.ImageOverrides)
	}
	if result != (ctrl.Result{}) {
		return result, err
	}
	if multiClusterHub.Enabled(operatorv1.GRC) {
		result, err = r.ensureGRC(ctx, multiClusterHub, r.CacheSpec.ImageOverrides)
	} else {
		result, err = r.ensureNoGRC(ctx, multiClusterHub, r.CacheSpec.ImageOverrides)
	}
	if result != (ctrl.Result{}) {
		return result, err
	}
	if multiClusterHub.Enabled(operatorv1.ClusterLifecycle) {
		result, err = r.ensureCLC(ctx, multiClusterHub, r.CacheSpec.ImageOverrides)
	} else {
		result, err = r.ensureNoCLC(ctx, multiClusterHub, r.CacheSpec.ImageOverrides)
	}
	if result != (ctrl.Result{}) {
		return result, err
	}
	if multiClusterHub.Enabled(operatorv1.Volsync) {
		result, err = r.ensureVolsync(ctx, multiClusterHub, r.CacheSpec.ImageOverrides)
	} else {
		result, err = r.ensureNoVolsync(ctx, multiClusterHub, r.CacheSpec.ImageOverrides)
	}
	if result != (ctrl.Result{}) {
		return result, err
	}
	if result != (ctrl.Result{}) {
		return result, err
	}
	if multiClusterHub.Enabled(operatorv1.ClusterBackup) {
		ns := BackupNamespace()
		result, err = r.ensureNamespace(multiClusterHub, ns)
		if result != (ctrl.Result{}) {
			return result, err
		}
		result, err = r.ensurePullSecret(multiClusterHub, ns.Name)
		if result != (ctrl.Result{}) {
			return result, err
		}
		result, err = r.ensureClusterBackup(ctx, multiClusterHub, r.CacheSpec.ImageOverrides)
		if result != (ctrl.Result{}) {
			return result, err
		}
	} else {
		result, err = r.ensureNoClusterBackup(ctx, multiClusterHub, r.CacheSpec.ImageOverrides)
		if result != (ctrl.Result{}) {
			return result, err
		}
		result, err = r.ensureNoNamespace(multiClusterHub, BackupNamespaceUnstructured())
		if result != (ctrl.Result{}) {
			return result, err
		}
	}
	if result != (ctrl.Result{}) {
		return result, err
	}

	if !multiClusterHub.Spec.DisableHubSelfManagement {
		result, err = r.ensureKlusterletAddonConfig(multiClusterHub)
		if result != (ctrl.Result{}) || err != nil {
			return result, err
		}
	}

	// Cleanup unused resources once components up-to-date
	if r.ComponentsAreRunning(multiClusterHub, ocpConsole) {
		result, err = r.ensureRemovalsGone(multiClusterHub)
		if result != (ctrl.Result{}) {
			return result, err
		}
	}
	if upgrade {
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}

	return retQueue, retError
	// return ctrl.Result{}, nil
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
	// 	These messages are drawn from operator condition
	// Right now, they just indicate between upgrading and not
	msg := utils.UpgradeableAllowMessage
	status := metav1.ConditionTrue
	reason := utils.UpgradeableAllowReason

	// 	The condition is the only field that affects whether or not we can upgrade
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
			&source.Kind{Type: &appsv1.Deployment{}},
			&handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    &operatorv1.MultiClusterHub{},
			},
			builder.WithPredicates(
				ctrlpredicate.Or(
					ctrlpredicate.GenerationChangedPredicate{},
					ctrlpredicate.LabelChangedPredicate{},
					ctrlpredicate.AnnotationChangedPredicate{},
				),
			),
		).
		Watches(
			&source.Kind{Type: &corev1.ConfigMap{}},
			&handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    &operatorv1.MultiClusterHub{},
			},
		).
		Watches(
			&source.Kind{Type: &apiregistrationv1.APIService{}},
			handler.Funcs{
				DeleteFunc: func(e event.DeleteEvent, q workqueue.RateLimitingInterface) {
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
		Watches(&source.Kind{Type: &appsv1.Deployment{}},
			handler.EnqueueRequestsFromMapFunc(
				func(a client.Object) []reconcile.Request {
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
			&source.Kind{Type: &configv1.ClusterVersion{}},
			handler.EnqueueRequestsFromMapFunc(
				func(a client.Object) []reconcile.Request {
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
	log := log.FromContext(ctx)
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
		// Apply the object data.
		force := true
		err := r.Client.Patch(ctx, template, client.Apply, &client.PatchOptions{Force: &force, FieldManager: "multiclusterhub-operator"})
		if err != nil {
			log.Info(err.Error())
			return ctrl.Result{}, pkgerrors.Wrapf(err, "error applying object Name: %s Kind: %s", template.GetName(), template.GetKind())
		}
	}
	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) ensureCLC(ctx context.Context, m *operatorv1.MultiClusterHub, images map[string]string) (ctrl.Result, error) {

	log := log.FromContext(ctx)

	templates, errs := renderer.RenderChart(utils.CLCChartLocation, m, images)
	if len(errs) > 0 {
		for _, err := range errs {
			log.Info(err.Error())
		}
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}

	// Applies all templates
	for _, template := range templates {
		result, err := r.applyTemplate(ctx, m, template)
		if err != nil {
			return result, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) ensureNoCLC(ctx context.Context, m *operatorv1.MultiClusterHub, images map[string]string) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Renders all templates from charts
	templates, errs := renderer.RenderChart(utils.CLCChartLocation, m, images)
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

func (r *MultiClusterHubReconciler) ensureConsole(ctx context.Context, m *operatorv1.MultiClusterHub, images map[string]string) (ctrl.Result, error) {

	log := log.FromContext(ctx)

	templates, errs := renderer.RenderChart(utils.ConsoleChartLocation, m, images)
	if len(errs) > 0 {
		for _, err := range errs {
			log.Info(err.Error())
		}
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}

	// Applies all templates
	for _, template := range templates {
		result, err := r.applyTemplate(ctx, m, template)
		if err != nil {
			return result, err
		}
	}

	return r.addPluginToConsole(m)
}

func (r *MultiClusterHubReconciler) ensureNoConsole(ctx context.Context, m *operatorv1.MultiClusterHub, images map[string]string) (ctrl.Result, error) {
	log := log.FromContext(ctx)

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

	// Renders all templates from charts
	templates, errs := renderer.RenderChart(utils.ConsoleChartLocation, m, images)
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

func (r *MultiClusterHubReconciler) ensureInsights(ctx context.Context, m *operatorv1.MultiClusterHub, images map[string]string) (ctrl.Result, error) {

	log := log.FromContext(ctx)

	templates, errs := renderer.RenderChart(utils.InsightsChartLocation, m, images)
	if len(errs) > 0 {
		for _, err := range errs {
			log.Info(err.Error())
		}
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}

	// Applies all templates
	for _, template := range templates {
		result, err := r.applyTemplate(ctx, m, template)
		if err != nil {
			return result, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) ensureNoInsights(ctx context.Context, m *operatorv1.MultiClusterHub, images map[string]string) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Renders all templates from charts
	templates, errs := renderer.RenderChart(utils.InsightsChartLocation, m, images)
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

func (r *MultiClusterHubReconciler) ensureAppsub(ctx context.Context, m *operatorv1.MultiClusterHub, images map[string]string) (ctrl.Result, error) {

	log := log.FromContext(ctx)

	templates, errs := renderer.RenderChart(utils.AppsubChartLocation, m, images)
	if len(errs) > 0 {
		for _, err := range errs {
			log.Info(err.Error())
		}
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}

	// Applies all templates
	for _, template := range templates {
		result, err := r.applyTemplate(ctx, m, template)
		if err != nil {
			return result, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) ensureNoAppsub(ctx context.Context, m *operatorv1.MultiClusterHub, images map[string]string) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Renders all templates from charts
	templates, errs := renderer.RenderChart(utils.AppsubChartLocation, m, images)
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

func (r *MultiClusterHubReconciler) ensureGRC(ctx context.Context, m *operatorv1.MultiClusterHub, images map[string]string) (ctrl.Result, error) {

	log := log.FromContext(ctx)

	templates, errs := renderer.RenderChart(utils.GRCChartLocation, m, images)
	if len(errs) > 0 {
		for _, err := range errs {
			log.Info(err.Error())
		}
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}

	// Applies all templates
	for _, template := range templates {
		result, err := r.applyTemplate(ctx, m, template)
		if err != nil {
			return result, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) ensureNoGRC(ctx context.Context, m *operatorv1.MultiClusterHub, images map[string]string) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Renders all templates from charts
	templates, errs := renderer.RenderChart(utils.GRCChartLocation, m, images)
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

func (r *MultiClusterHubReconciler) ensureSearchV2(ctx context.Context, m *operatorv1.MultiClusterHub, images map[string]string) (ctrl.Result, error) {

	log := log.FromContext(ctx)

	templates, errs := renderer.RenderChart(utils.SearchV2ChartLocation, m, images)
	if len(errs) > 0 {
		for _, err := range errs {
			log.Info(err.Error())
		}
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}

	// Applies all templates
	for _, template := range templates {
		result, err := r.applyTemplate(ctx, m, template)
		if err != nil {
			return result, err
		}
	}

	result, err := r.ensureSearchCR(m)
	return result, err
}

func (r *MultiClusterHubReconciler) ensureNoSearchV2(ctx context.Context, m *operatorv1.MultiClusterHub, images map[string]string) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	result, err := r.ensureNoSearchCR(m)
	if err != nil {
		return result, err
	}

	// Renders all templates from charts
	templates, errs := renderer.RenderChart(utils.SearchV2ChartLocation, m, images)
	if len(errs) > 0 {
		for _, err := range errs {
			log.Info(err.Error())
		}
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}

	// Deletes all templates
	for _, template := range templates {
		result, err = r.deleteTemplate(ctx, m, template)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to delete template: %s", template.GetName()))
			return result, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) ensureClusterBackup(ctx context.Context, m *operatorv1.MultiClusterHub, images map[string]string) (ctrl.Result, error) {

	log := log.FromContext(ctx)

	templates, errs := renderer.RenderChart(utils.ClusterBackupChartLocation, m, images)
	if len(errs) > 0 {
		for _, err := range errs {
			log.Info(err.Error())
		}
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}

	// Applies all templates
	for _, template := range templates {
		result, err := r.applyTemplate(ctx, m, template)
		if err != nil {
			return result, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) ensureNoClusterBackup(ctx context.Context, m *operatorv1.MultiClusterHub, images map[string]string) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Renders all templates from charts
	templates, errs := renderer.RenderChart(utils.ClusterBackupChartLocation, m, images)
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

func (r *MultiClusterHubReconciler) ensureVolsync(ctx context.Context, m *operatorv1.MultiClusterHub, images map[string]string) (ctrl.Result, error) {

	log := log.FromContext(ctx)

	templates, errs := renderer.RenderChart(utils.VolsyncChartLocation, m, images)
	if len(errs) > 0 {
		for _, err := range errs {
			log.Info(err.Error())
		}
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}

	// Applies all templates
	for _, template := range templates {
		result, err := r.applyTemplate(ctx, m, template)
		if err != nil {
			return result, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) ensureNoVolsync(ctx context.Context, m *operatorv1.MultiClusterHub, images map[string]string) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Renders all templates from charts
	templates, errs := renderer.RenderChart(utils.VolsyncChartLocation, m, images)
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

func (r *MultiClusterHubReconciler) updateSearchEnablement(ctx context.Context, m *operatorv1.MultiClusterHub) (ctrl.Result, error) {
	m.Disable(operatorv1.Search)
	err := r.Client.Update(ctx, m)
	if err != nil {
		r.Log.Error(err, "Failed to update MultiClusterHub", "MultiClusterHub.Namespace", m.Namespace, "MultiClusterHub.Name", m.Name)
		return ctrl.Result{}, err
	}
	r.Log.Info("MultiClusterHub successfully updated")
	return ctrl.Result{Requeue: true}, nil
}

func (r *MultiClusterHubReconciler) deleteTemplate(ctx context.Context, m *operatorv1.MultiClusterHub, template *unstructured.Unstructured) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	err := r.Client.Get(ctx, types.NamespacedName{Name: template.GetName(), Namespace: template.GetNamespace()}, template)

	if err != nil && (apierrors.IsNotFound(err) || apimeta.IsNoMatchError(err)) {
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
func (r *MultiClusterHubReconciler) createTrustBundleConfigmap(ctx context.Context, mch *operatorv1.MultiClusterHub) (ctrl.Result, error) {
	log := log.FromContext(ctx)

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

func (r *MultiClusterHubReconciler) finalizeHub(reqLogger logr.Logger, m *operatorv1.MultiClusterHub, ocpConsole bool) error {
	if err := r.cleanupAppSubscriptions(reqLogger, m); err != nil {
		return err
	}
	_, err := r.ensureNoClusterBackup(context.TODO(), m, r.CacheSpec.ImageOverrides)
	if err != nil {
		return err
	}
	if err := r.cleanupNamespaces(reqLogger); err != nil {
		return err
	}
	_, err = r.ensureNoAppsub(context.TODO(), m, r.CacheSpec.ImageOverrides)
	if err != nil {
		return err
	}
	_, err = r.ensureNoInsights(context.TODO(), m, r.CacheSpec.ImageOverrides)
	if err != nil {
		return err
	}
	_, err = r.ensureNoCLC(context.TODO(), m, r.CacheSpec.ImageOverrides)
	if err != nil {
		return err
	}
	_, err = r.ensureNoGRC(context.TODO(), m, r.CacheSpec.ImageOverrides)
	if err != nil {
		return err
	}
	_, err = r.ensureNoConsole(context.TODO(), m, r.CacheSpec.ImageOverrides)
	if err != nil {
		return err
	}
	_, err = r.ensureNoVolsync(context.TODO(), m, r.CacheSpec.ImageOverrides)
	if err != nil {
		return err
	}
	_, err = r.ensureNoSearchV2(context.TODO(), m, r.CacheSpec.ImageOverrides)
	if err != nil {
		return err
	}
	if err := r.cleanupClusterRoles(reqLogger, m); err != nil {
		return err
	}
	if err := r.cleanupClusterRoleBindings(reqLogger, m); err != nil {
		return err
	}

	if err := r.cleanupMultiClusterEngine(reqLogger, m); err != nil {
		return err
	}

	if err := r.orphanOwnedMultiClusterEngine(m); err != nil {
		return err
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

	crds, errs := renderer.RenderCRDs(crdDir)
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
			err := fmt.Errorf("Failed to deploy %s %s", crd.GetKind(), crd.GetName())
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
		src, err := ioutil.ReadFile(filepath.Clean(path)) // #nosec G304 (filepath cleaned)
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
			err := fmt.Errorf("Failed to deploy %s %s", res.GetKind(), res.GetName())
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

	if utils.MchIsValid(m) && os.Getenv("ACM_HUB_OCP_VERSION") != "" && !updateNecessary {
		return ctrl.Result{}, nil
	}
	log.Info("MultiClusterHub is Invalid. Updating with proper defaults")

	if len(m.Spec.Ingress.SSLCiphers) == 0 {
		m.Spec.Ingress.SSLCiphers = utils.DefaultSSLCiphers
		updateNecessary = true
	}

	if !utils.AvailabilityConfigIsValid(m.Spec.AvailabilityConfig) {
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
