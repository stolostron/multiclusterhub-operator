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
	"time"

	clustermanager "github.com/open-cluster-management/api/operator/v1"
	hive "github.com/openshift/hive/apis/hive/v1"

	appsubv1 "github.com/open-cluster-management/multicloud-operators-subscription/pkg/apis/apps/v1"
	"github.com/open-cluster-management/multiclusterhub-operator/pkg/channel"
	"github.com/open-cluster-management/multiclusterhub-operator/pkg/deploying"
	"github.com/open-cluster-management/multiclusterhub-operator/pkg/foundation"
	"github.com/open-cluster-management/multiclusterhub-operator/pkg/helmrepo"
	"github.com/open-cluster-management/multiclusterhub-operator/pkg/imageoverrides"
	"github.com/open-cluster-management/multiclusterhub-operator/pkg/manifest"
	"github.com/open-cluster-management/multiclusterhub-operator/pkg/predicate"
	"github.com/open-cluster-management/multiclusterhub-operator/pkg/rendering"
	"github.com/open-cluster-management/multiclusterhub-operator/pkg/subscription"
	"github.com/open-cluster-management/multiclusterhub-operator/pkg/version"
	netv1 "github.com/openshift/api/config/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	operatorsv1 "github.com/open-cluster-management/multiclusterhub-operator/api/v1"
	operatorv1 "github.com/open-cluster-management/multiclusterhub-operator/api/v1"
	utils "github.com/open-cluster-management/multiclusterhub-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
)

// MultiClusterHubReconciler reconciles a MultiClusterHub object
type MultiClusterHubReconciler struct {
	Client    client.Client
	CacheSpec CacheSpec
	Scheme    *runtime.Scheme
	log       logr.Logger
}

var resyncPeriod = time.Second * 20

const hubFinalizer = "finalizer.operator.open-cluster-management.io"

//+kubebuilder:rbac:groups="",resources=secrets;pods;pods/log;configmaps;endpoints;services;services/finalizers;serviceaccounts;namespaces;events,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="action.open-cluster-management.io",resources=managedclusteractions;managedclusteractions/status,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="addon.open-cluster-management.io",resources=clustermanagementaddons;managedclusteraddons;managedclusteraddons/status,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="admissionregistration.k8s.io",resources=mutatingwebhookconfigurations;validatingwebhookconfigurations,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="agent.open-cluster-management.io",resources=klusterletaddonconfigs,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="apiextensions.k8s.io",resources=customresourcedefinitions,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="apiregistration.k8s.io",resources=apiservices,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="app.k8s.io",resources=applications;applications/status,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="apps",resources=deployments;deployments/finalizers;replicasets,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="apps.open-cluster-management.io",resources=channels;channels/status;subscriptions;subscriptions/status;helmreleases;deployables;deployables/status;placementrules;placementrules/status,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="authorization.k8s.io",resources=subjectaccessreviews,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="certificates.k8s.io",resources=signers;certificatesigningrequests;certificatesigningrequests/approval,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="cluster.open-cluster-management.io",resources=clusterclaims;clustercurators;managedclusters;managedclustersetbindings;managedclustersets;managedclustersets/bind;managedclustersets/join;managedclustersets/status;placementdecisions;placementdecisions/status;placements,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="clusterview.open-cluster-management.io",resources=managedclusters;managedclusters/accept;managedclusters/status;managedclustersets;managedclustersets/bind;managedclustersets/join;managedclustersets/status,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="compliance.mcm.ibm.com",resources=compliances,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="config.openshift.io",resources=clusterversions;ingresses,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="coordination.k8s.io",resources=leases,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="discovery.open-cluster-management.io",resources=discoveredclusters;discoveryconfigs,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="hive.openshift.io",resources=clusterclaims;clusterdeployments;clusterpools;hiveconfigs;machinepools;syncsets,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="hiveinternal.openshift.io",resources=clustersyncs,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="internal.open-cluster-management.io",resources=managedclusterinfos;managedclusterinfos/status,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="inventory.open-cluster-management.io",resources=baremetalassets;baremetalassets/status;baremetalassets/finalizers,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="mcm.ibm.com",resources=helmrepos;helmrepos/status;clusterjoinrequests;deployables;deployables/status;deployableoverrides;deployableoverrides/status;applicationrelationships;applicationrelationships/status,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="monitoring.coreos.com",resources=servicemonitors;servicemonitors/finalizers,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="multicloud.ibm.com",resources=endpointconfigs,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="observability.open-cluster-management.io",resources=multiclusterobservabilities,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="operator.open-cluster-management.io",resources=multiclusterhubs;multiclusterhubs/status;multiclusterhubs/finalizers;clustermanagers,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="policy.open-cluster-management.io",resources=placementbindings;policies/status;policies;policyautomations,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="proxy.open-cluster-management.io",resources=clusterstatuses/log;clusterstatuses/aggregator,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="register.open-cluster-management.io",resources=managedclusters/accept,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterroles;clusterrolebindings;rolebindings;roles,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="search.open-cluster-management.io",resources=searchcustomizations,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="submarineraddon.open-cluster-management.io",resources=submarinerconfigs;submarinerconfigs/status,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="view.open-cluster-management.io",resources=managedclusterviews;managedclusterviews/status,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="wgpolicyk8s.io",resources=policyreports,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="work.open-cluster-management.io",resources=manifestworks;manifestworks/status,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind

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
	r.log = log.FromContext(ctx)

	r.log.Info("Reconciling MultiClusterHub")

	// Fetch the MultiClusterHub instance
	multiClusterHub := &operatorv1.MultiClusterHub{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, multiClusterHub)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			r.log.Info("MultiClusterHub resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		r.log.Error(err, "Failed to get MultiClusterHub CR")
		return ctrl.Result{}, err
	}

	trackedNamespaces := utils.TrackedNamespaces(multiClusterHub)

	allDeploys, err := r.listDeployments(trackedNamespaces)
	if err != nil {
		return ctrl.Result{}, err
	}

	allHRs, err := r.listHelmReleases(trackedNamespaces)
	if err != nil {
		return ctrl.Result{}, err
	}

	allCRs, err := r.listCustomResources()
	if err != nil {
		return ctrl.Result{}, err
	}

	originalStatus := multiClusterHub.Status.DeepCopy()
	defer func() {
		statusQueue, statusError := r.syncHubStatus(multiClusterHub, originalStatus, allDeploys, allHRs, allCRs)
		if statusError != nil {
			r.log.Error(retError, "Error updating status")
		}
		if empty := (reconcile.Result{}); retQueue == empty {
			retQueue = statusQueue
		}
		if retError == nil {
			retError = statusError
		}
	}()

	// Check if the multiClusterHub instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isHubMarkedToBeDeleted := multiClusterHub.GetDeletionTimestamp() != nil
	if isHubMarkedToBeDeleted {
		terminating := NewHubCondition(operatorv1.Terminating, metav1.ConditionTrue, DeleteTimestampReason, "Multiclusterhub is being cleaned up.")
		SetHubCondition(&multiClusterHub.Status, *terminating)

		if contains(multiClusterHub.GetFinalizers(), hubFinalizer) {
			// Run finalization logic. If the finalization
			// logic fails, don't remove the finalizer so
			// that we can retry during the next reconciliation.
			if err := r.finalizeHub(r.log, multiClusterHub); err != nil {
				// Logging err and returning nil to ensure 45 second wait
				r.log.Info(fmt.Sprintf("Finalizing: %s", err.Error()))
				return ctrl.Result{RequeueAfter: resyncPeriod}, nil
			}

			// Remove hubFinalizer. Once all finalizers have been
			// removed, the object will be deleted.
			multiClusterHub.SetFinalizers(remove(multiClusterHub.GetFinalizers(), hubFinalizer))

			err := r.Client.Update(context.TODO(), multiClusterHub)
			if err != nil {
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	// Add finalizer for this CR
	if !contains(multiClusterHub.GetFinalizers(), hubFinalizer) {
		if err := r.addFinalizer(r.log, multiClusterHub); err != nil {
			return ctrl.Result{}, err
		}
	}

	var result ctrl.Result
	result, err = r.setDefaults(multiClusterHub)
	if result != (ctrl.Result{}) {
		return ctrl.Result{}, err
	}

	// Read image overrides
	// First, attempt to read image overrides from environmental variables
	imageOverrides := imageoverrides.GetImageOverrides()
	if len(imageOverrides) == 0 {
		// If imageoverrides are not set from environmental variables, read from manifest
		r.log.Info("Image Overrides not set from environment variables. Checking for overrides in manifest")
		imageOverrides, err = manifest.GetImageOverrides(multiClusterHub)
		if err != nil {
			r.log.Error(err, "Could not get map of image overrides")
			return ctrl.Result{}, err
		}
	}

	if imageRepo := utils.GetImageRepository(multiClusterHub); imageRepo != "" {
		r.log.Info(fmt.Sprintf("Overriding Image Repository from annotation 'mch-imageRepository': %s", imageRepo))
		imageOverrides = utils.OverrideImageRepository(imageOverrides, imageRepo)
	}

	// Check for developer overrides
	if imageOverridesConfigmap := utils.GetImageOverridesConfigmap(multiClusterHub); imageOverridesConfigmap != "" {
		imageOverrides, err = r.OverrideImagesFromConfigmap(imageOverrides, multiClusterHub.GetNamespace(), imageOverridesConfigmap)
		if err != nil {
			r.log.Error(err, fmt.Sprintf("Could not find image override configmap: %s/%s", multiClusterHub.GetNamespace(), imageOverridesConfigmap))
			return ctrl.Result{}, err
		}
	}
	r.CacheSpec.ImageOverrides = imageOverrides
	r.CacheSpec.ManifestVersion = version.Version
	r.CacheSpec.ImageRepository = utils.GetImageRepository(multiClusterHub)
	r.CacheSpec.ImageOverridesCM = utils.GetImageOverridesConfigmap(multiClusterHub)

	err = r.maintainImageManifestConfigmap(multiClusterHub)
	if err != nil {
		r.log.Error(err, "Error storing image manifests in configmap")
		return ctrl.Result{}, err
	}

	CustomUpgradeRequired, err := r.CustomSelfMgmtHubUpgradeRequired(multiClusterHub)
	if err != nil {
		r.log.Error(err, "Error determining if upgrade specific logic is required")
		return ctrl.Result{}, err
	}

	if CustomUpgradeRequired {
		result, err = r.BeginEnsuringHubIsUpgradeable(multiClusterHub)
		if err != nil {
			r.log.Info(fmt.Sprintf("Error starting to ensure local-cluster hub is upgradeable: %s", err.Error()))
			return ctrl.Result{RequeueAfter: resyncPeriod}, nil
		}
	}

	// Add installer labels to Helm-owned deployments
	myHelmReleases := getAppSubOwnedHelmReleases(allHRs, getAppsubs(multiClusterHub))
	myHRDeployments := getHelmReleaseOwnedDeployments(allDeploys, myHelmReleases)
	if err := r.labelDeployments(multiClusterHub, myHRDeployments); err != nil {
		return ctrl.Result{}, nil
	}

	// Do not reconcile objects if this instance of mch is labeled "paused"
	updatePausedCondition(multiClusterHub)
	if utils.IsPaused(multiClusterHub) {
		r.log.Info("MultiClusterHub reconciliation is paused. Nothing more to do.")
		return ctrl.Result{}, nil
	}

	result, err = r.ensureSubscriptionOperatorIsRunning(multiClusterHub, allDeploys)
	if result != (ctrl.Result{}) {
		return result, err
	}

	// Render CRD templates
	err = r.installCRDs(r.log, multiClusterHub)
	if err != nil {
		return ctrl.Result{}, err
	}

	if utils.ProxyEnvVarsAreSet() {
		r.log.Info(fmt.Sprintf("Proxy configuration environment variables are set. HTTP_PROXY: %s, HTTPS_PROXY: %s, NO_PROXY: %s", os.Getenv("HTTP_PROXY"), os.Getenv("HTTPS_PROXY"), os.Getenv("NO_PROXY")))
	}

	result, err = r.ensureDeployment(multiClusterHub, helmrepo.Deployment(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != (ctrl.Result{}) {
		return result, err
	}

	result, err = r.ensureService(multiClusterHub, helmrepo.Service(multiClusterHub))
	if result != (ctrl.Result{}) {
		return result, err
	}

	result, err = r.ensureChannel(multiClusterHub, channel.Channel(multiClusterHub))
	if result != (ctrl.Result{}) {
		return result, err
	}

	result, err = r.ingressDomain(multiClusterHub)
	if result != (ctrl.Result{}) {
		return result, err
	}

	//Render the templates with a specified CR
	renderer := rendering.NewRenderer(multiClusterHub)
	toDeploy, err := renderer.Render(r.Client)
	if err != nil {
		r.log.Error(err, "Failed to render MultiClusterHub templates")
		return reconcile.Result{}, err
	}
	//Deploy the resources
	for _, res := range toDeploy {
		if res.GetNamespace() == multiClusterHub.Namespace {
			if err := controllerutil.SetControllerReference(multiClusterHub, res, r.Scheme); err != nil {
				r.log.Error(err, "Failed to set controller reference")
			}
		}
		err, ok := deploying.Deploy(r.Client, res)
		if err != nil {
			r.log.Error(err, fmt.Sprintf("Failed to deploy %s %s/%s", res.GetKind(), multiClusterHub.Namespace, res.GetName()))
			return reconcile.Result{}, err
		}
		if ok {
			condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionTrue, NewComponentReason, "Created new resource")
			SetHubCondition(&multiClusterHub.Status, *condition)
		}
	}

	result, err = r.ensureDeployment(multiClusterHub, foundation.WebhookDeployment(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != (ctrl.Result{}) {
		return result, err
	}

	result, err = r.ensureService(multiClusterHub, foundation.WebhookService(multiClusterHub))
	if result != (ctrl.Result{}) {
		return result, err
	}

	// Wait for ocm-webhook to be fully available before applying rest of subscriptions
	if !(multiClusterHub.Status.Components["ocm-webhook"].Type == "Available" && multiClusterHub.Status.Components["ocm-webhook"].Status == metav1.ConditionTrue) {
		r.log.Info(fmt.Sprintf("Waiting for component 'ocm-webhook' to be available"))
		return reconcile.Result{RequeueAfter: resyncPeriod}, nil
	}

	// Install the rest of the subscriptions in no particular order
	result, err = r.ensureSubscription(multiClusterHub, subscription.ManagementIngress(multiClusterHub, r.CacheSpec.ImageOverrides, r.CacheSpec.IngressDomain))
	if result != (ctrl.Result{}) {
		return result, err
	}
	result, err = r.ensureSubscription(multiClusterHub, subscription.ApplicationUI(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != (ctrl.Result{}) {
		return result, err
	}
	result, err = r.ensureSubscription(multiClusterHub, subscription.Console(multiClusterHub, r.CacheSpec.ImageOverrides, r.CacheSpec.IngressDomain))
	if result != (ctrl.Result{}) {
		return result, err
	}
	result, err = r.ensureSubscription(multiClusterHub, subscription.Insights(multiClusterHub, r.CacheSpec.ImageOverrides, r.CacheSpec.IngressDomain))
	if result != (ctrl.Result{}) {
		return result, err
	}
	result, err = r.ensureSubscription(multiClusterHub, subscription.Discovery(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != (ctrl.Result{}) {
		return result, err
	}
	result, err = r.ensureSubscription(multiClusterHub, subscription.GRC(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != (ctrl.Result{}) {
		return result, err
	}
	result, err = r.ensureSubscription(multiClusterHub, subscription.KUIWebTerminal(multiClusterHub, r.CacheSpec.ImageOverrides, r.CacheSpec.IngressDomain))
	if result != (ctrl.Result{}) {
		return result, err
	}
	result, err = r.ensureSubscription(multiClusterHub, subscription.ClusterLifecycle(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != (ctrl.Result{}) {
		return result, err
	}
	result, err = r.ensureSubscription(multiClusterHub, subscription.Search(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != (ctrl.Result{}) {
		return result, err
	}
	result, err = r.ensureSubscription(multiClusterHub, subscription.AssistedService(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != (ctrl.Result{}) {
		return result, err
	}

	//OCM proxy server deployment
	result, err = r.ensureDeployment(multiClusterHub, foundation.OCMProxyServerDeployment(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != (ctrl.Result{}) {
		return result, err
	}

	//OCM proxy server service
	result, err = r.ensureService(multiClusterHub, foundation.OCMProxyServerService(multiClusterHub))
	if result != (ctrl.Result{}) {
		return result, err
	}

	// OCM proxy apiService
	result, err = r.ensureAPIService(multiClusterHub, foundation.OCMProxyAPIService(multiClusterHub))
	if result != (ctrl.Result{}) {
		return result, err
	}

	// OCM clusterView v1 apiService
	result, err = r.ensureAPIService(multiClusterHub, foundation.OCMClusterViewV1APIService(multiClusterHub))
	if result != (ctrl.Result{}) {
		return result, err
	}

	// OCM clusterView v1alpha1 apiService
	result, err = r.ensureAPIService(multiClusterHub, foundation.OCMClusterViewV1alpha1APIService(multiClusterHub))
	if result != (ctrl.Result{}) {
		return result, err
	}

	//OCM controller deployment
	result, err = r.ensureDeployment(multiClusterHub, foundation.OCMControllerDeployment(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != (ctrl.Result{}) {
		return result, err
	}

	result, err = r.ensureUnstructuredResource(multiClusterHub, foundation.ClusterManager(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != (ctrl.Result{}) {
		return result, err
	}

	if !multiClusterHub.Spec.DisableHubSelfManagement {
		result, err = r.ensureHubIsImported(multiClusterHub)
		if result != (ctrl.Result{}) {
			return result, err
		}
	} else {
		result, err = r.ensureHubIsExported(multiClusterHub)
		if result != (ctrl.Result{}) {
			return result, err
		}
	}

	// Cleanup unused resources once components up-to-date
	if r.ComponentsAreRunning(multiClusterHub) {
		result, err = r.ensureRemovalsGone(multiClusterHub)
		if result != (ctrl.Result{}) {
			return result, err
		}
	}

	return retQueue, retError
	// return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MultiClusterHubReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1.MultiClusterHub{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &operatorsv1.MultiClusterHub{},
		}).
		Watches(&source.Kind{Type: &appsubv1.Subscription{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &operatorsv1.MultiClusterHub{},
		}).
		Watches(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &operatorsv1.MultiClusterHub{},
		}).
		Watches(&source.Kind{Type: &apiregistrationv1.APIService{}}, handler.Funcs{
			DeleteFunc: func(e event.DeleteEvent, q workqueue.RateLimitingInterface) {
				labels := e.Object.GetLabels()
				q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
					Name:      labels["installer.name"],
					Namespace: labels["installer.namespace"],
				}})
			},
		}, builder.WithPredicates(predicate.DeletePredicate{})).
		Watches(&source.Kind{Type: &hive.HiveConfig{}}, &handler.Funcs{
			DeleteFunc: func(e event.DeleteEvent, q workqueue.RateLimitingInterface) {
				labels := e.Object.GetLabels()
				q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
					Name:      labels["installer.name"],
					Namespace: labels["installer.namespace"],
				}})
			},
		}, builder.WithPredicates(predicate.InstallerLabelPredicate{})).
		Watches(&source.Kind{Type: &clustermanager.ClusterManager{}}, &handler.Funcs{
			DeleteFunc: func(e event.DeleteEvent, q workqueue.RateLimitingInterface) {
				labels := e.Object.GetLabels()
				q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
					Name:      labels["installer.name"],
					Namespace: labels["installer.namespace"],
				}})
			},
			UpdateFunc: func(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
				labels := e.ObjectOld.GetLabels()
				q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
					Name:      labels["installer.name"],
					Namespace: labels["installer.namespace"],
				}})
			},
		}, builder.WithPredicates(predicate.InstallerLabelPredicate{})).
		Watches(&source.Kind{Type: &appsv1.Deployment{}},
			handler.EnqueueRequestsFromMapFunc(func(a client.Object) []reconcile.Request {
				return []reconcile.Request{
					{NamespacedName: types.NamespacedName{
						Name:      a.GetLabels()["installer.name"],
						Namespace: a.GetLabels()["installer.namespace"],
					}},
				}
			}), builder.WithPredicates(predicate.InstallerLabelPredicate{})).
		Complete(r)
}

// ingressDomain is discovered from Openshift cluster configuration resources
func (r *MultiClusterHubReconciler) ingressDomain(m *operatorsv1.MultiClusterHub) (ctrl.Result, error) {
	if r.CacheSpec.IngressDomain != "" {
		return ctrl.Result{}, nil
	}

	ingress := &netv1.Ingress{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{
		Name: "cluster",
	}, ingress)
	// Don't fail on a unit test (Fake client won't find "cluster" Ingress)
	if err != nil && !utils.IsUnitTest() {
		r.log.Error(err, "Failed to get Ingress")
		return ctrl.Result{}, err
	}

	r.CacheSpec.IngressDomain = ingress.Spec.Domain
	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) finalizeHub(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {
	if _, err := r.ensureHubIsExported(m); err != nil {
		return err
	}
	if err := r.cleanupAppSubscriptions(reqLogger, m); err != nil {
		return err
	}
	if err := r.cleanupFoundation(reqLogger, m); err != nil {
		return err
	}
	if err := r.cleanupHiveConfigs(reqLogger, m); err != nil {
		return err
	}
	if err := r.cleanupAPIServices(reqLogger, m); err != nil {
		return err
	}
	if err := r.cleanupClusterRoles(reqLogger, m); err != nil {
		return err
	}
	if err := r.cleanupClusterRoleBindings(reqLogger, m); err != nil {
		return err
	}
	if err := r.cleanupMutatingWebhooks(reqLogger, m); err != nil {
		return err
	}
	if err := r.cleanupValidatingWebhooks(reqLogger, m); err != nil {
		return err
	}
	if err := r.cleanupCRDs(reqLogger, m); err != nil {
		return err
	}
	if err := r.cleanupClusterManagers(reqLogger, m); err != nil {
		return err
	}
	if m.Spec.SeparateCertificateManagement {
		if err := r.cleanupPullSecret(reqLogger, m); err != nil {
			return err
		}
	}

	reqLogger.Info("Successfully finalized multiClusterHub")
	return nil
}

func (r *MultiClusterHubReconciler) addFinalizer(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {
	reqLogger.Info("Adding Finalizer for the multiClusterHub")
	m.SetFinalizers(append(m.GetFinalizers(), hubFinalizer))

	// Update CR
	err := r.Client.Update(context.TODO(), m)
	if err != nil {
		reqLogger.Error(err, "Failed to update MultiClusterHub with finalizer")
		return err
	}
	return nil
}

func (r *MultiClusterHubReconciler) installCRDs(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {
	crdRenderer, err := rendering.NewCRDRenderer(m)
	if err != nil {
		condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionFalse, ResourceRenderReason, fmt.Sprintf("Error creating CRD renderer: %s", err.Error()))
		SetHubCondition(&m.Status, *condition)
		return fmt.Errorf("failed to setup CRD templates: %w", err)
	}
	crdResources, errs := crdRenderer.Render()
	if errs != nil && len(errs) > 0 {
		message := mergeErrors(errs)
		condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionFalse, ResourceRenderReason, fmt.Sprintf("Error rendering CRD templates: %s", message))
		SetHubCondition(&m.Status, *condition)
		return fmt.Errorf("failed to render CRD templates: %s", message)
	}

	for _, crd := range crdResources {
		err, ok := deploying.Deploy(r.Client, crd)
		if err != nil {
			message := fmt.Sprintf("Failed to deploy %s %s", crd.GetKind(), crd.GetName())
			reqLogger.Error(err, message)
			condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionFalse, DeployFailedReason, message)
			SetHubCondition(&m.Status, *condition)
			return err
		}
		if ok {
			condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionTrue, NewComponentReason, "Created new resource")
			SetHubCondition(&m.Status, *condition)
		}
	}
	return nil
}

func updatePausedCondition(m *operatorsv1.MultiClusterHub) {
	c := GetHubCondition(m.Status, operatorsv1.Progressing)

	if utils.IsPaused(m) {
		// Pause condition needs to go on
		if c == nil || c.Reason != PausedReason {
			condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionUnknown, PausedReason, "Multiclusterhub is paused")
			SetHubCondition(&m.Status, *condition)
		}
	} else {
		// Pause condition needs to come off
		if c != nil && c.Reason == PausedReason {
			condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionTrue, ResumedReason, "Multiclusterhub is resumed")
			SetHubCondition(&m.Status, *condition)
		}

	}
}

func (r *MultiClusterHubReconciler) setDefaults(m *operatorsv1.MultiClusterHub) (ctrl.Result, error) {
	if utils.MchIsValid(m) {
		return ctrl.Result{}, nil
	}
	r.log.Info("MultiClusterHub is Invalid. Updating with proper defaults")

	if len(m.Spec.Ingress.SSLCiphers) == 0 {
		m.Spec.Ingress.SSLCiphers = utils.DefaultSSLCiphers
	}

	if !utils.AvailabilityConfigIsValid(m.Spec.AvailabilityConfig) {
		m.Spec.AvailabilityConfig = operatorsv1.HAHigh
	}

	// Apply defaults to server
	err := r.Client.Update(context.TODO(), m)
	if err != nil {
		r.log.Error(err, "Failed to update MultiClusterHub", "MultiClusterHub.Namespace", m.Namespace, "MultiClusterHub.Name", m.Name)
		return ctrl.Result{}, err
	}

	r.log.Info("MultiClusterHub successfully updated")
	return ctrl.Result{Requeue: true}, nil
}
