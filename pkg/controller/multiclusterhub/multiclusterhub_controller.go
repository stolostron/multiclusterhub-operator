// Copyright (c) 2020 Red Hat, Inc.

package multiclusterhub

import (
	"context"
	"fmt"
	"time"

	hive "github.com/openshift/hive/pkg/apis/hive/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	appsubv1 "github.com/open-cluster-management/multicloud-operators-subscription/pkg/apis/apps/v1"
	operatorsv1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operator/v1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/channel"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/deploying"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/foundation"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/helmrepo"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/manifest"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/predicate"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/rendering"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/subscription"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	"github.com/open-cluster-management/multicloudhub-operator/version"
	netv1 "github.com/openshift/api/config/v1"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

const hubFinalizer = "finalizer.operator.open-cluster-management.io"

var log = logf.Log.WithName("controller_multiclusterhub")
var resyncPeriod = time.Second * 20

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new MultiClusterHub Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileMultiClusterHub{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("multiclusterhub-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource MultiClusterHub
	err = c.Watch(&source.Kind{Type: &operatorsv1.MultiClusterHub{}}, &handler.EnqueueRequestForObject{}, predicate.GenerationChangedPredicate{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner MultiClusterHub
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &operatorsv1.MultiClusterHub{},
	})
	if err != nil {
		return err
	}

	// Watch application subscriptions
	err = c.Watch(&source.Kind{Type: &appsubv1.Subscription{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &operatorsv1.MultiClusterHub{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &operatorsv1.MultiClusterHub{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{Type: &apiregistrationv1.APIService{}},
		handler.Funcs{
			DeleteFunc: func(e event.DeleteEvent, q workqueue.RateLimitingInterface) {
				labels := e.Meta.GetLabels()
				q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
					Name:      labels["installer.name"],
					Namespace: labels["installer.namespace"],
				}})
			},
		},
		predicate.DeletePredicate{},
	)
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{Type: &hive.HiveConfig{}},
		&handler.Funcs{
			DeleteFunc: func(e event.DeleteEvent, q workqueue.RateLimitingInterface) {
				labels := e.Meta.GetLabels()
				q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
					Name:      labels["installer.name"],
					Namespace: labels["installer.namespace"],
				}})
			},
		},
		predicate.InstallerLabelPredicate{},
	)
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{Type: &appsv1.Deployment{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
				return []reconcile.Request{
					{NamespacedName: types.NamespacedName{
						Name:      a.Meta.GetLabels()["installer.name"],
						Namespace: a.Meta.GetLabels()["installer.namespace"],
					}},
				}
			}),
		},
		predicate.InstallerLabelPredicate{},
	)
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileMultiClusterHub implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileMultiClusterHub{}

// ReconcileMultiClusterHub reconciles a MultiClusterHub object
type ReconcileMultiClusterHub struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client    client.Client
	CacheSpec CacheSpec
	scheme    *runtime.Scheme
}

// Reconcile reads that state of the cluster for a MultiClusterHub object and makes changes based on the state read
// and what is in the MultiClusterHub.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileMultiClusterHub) Reconcile(request reconcile.Request) (retQueue reconcile.Result, retError error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling MultiClusterHub")

	// Fetch the MultiClusterHub instance
	multiClusterHub := &operatorsv1.MultiClusterHub{}
	err := r.client.Get(context.TODO(), request.NamespacedName, multiClusterHub)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("MultiClusterHub resource not found. Ignoring since object must be deleted")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Failed to get MultiClusterHub CR")
		return reconcile.Result{}, err
	}

	trackedNamespaces := utils.TrackedNamespaces(multiClusterHub)

	allDeploys, err := r.listDeployments(trackedNamespaces)
	if err != nil {
		return reconcile.Result{}, err
	}
	allHRs, err := r.listHelmReleases(trackedNamespaces)
	if err != nil {
		return reconcile.Result{}, err
	}
	allCRs, err := r.listCustomResources()
	if err != nil {
		return reconcile.Result{}, err
	}

	originalStatus := multiClusterHub.Status.DeepCopy()
	defer func() {
		statusQueue, statusError := r.syncHubStatus(multiClusterHub, originalStatus, allDeploys, allHRs, allCRs)
		if statusError != nil {
			log.Error(retError, "Error updating status")
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
		terminating := NewHubCondition(operatorsv1.Terminating, metav1.ConditionTrue, DeleteTimestampReason, "Multiclusterhub is being cleaned up.")
		SetHubCondition(&multiClusterHub.Status, *terminating)

		if contains(multiClusterHub.GetFinalizers(), hubFinalizer) {
			// Run finalization logic. If the finalization
			// logic fails, don't remove the finalizer so
			// that we can retry during the next reconciliation.
			if err := r.finalizeHub(reqLogger, multiClusterHub); err != nil {
				// Logging err and returning nil to ensure 45 second wait
				log.Info(fmt.Sprintf("Finalizing: %s", err.Error()))
				return reconcile.Result{RequeueAfter: resyncPeriod}, nil
			}

			// Remove hubFinalizer. Once all finalizers have been
			// removed, the object will be deleted.
			multiClusterHub.SetFinalizers(remove(multiClusterHub.GetFinalizers(), hubFinalizer))

			err := r.client.Update(context.TODO(), multiClusterHub)
			if err != nil {
				return reconcile.Result{}, err
			}
		}

		return reconcile.Result{}, nil
	}

	// Add finalizer for this CR
	if !contains(multiClusterHub.GetFinalizers(), hubFinalizer) {
		if err := r.addFinalizer(reqLogger, multiClusterHub); err != nil {
			return reconcile.Result{}, err
		}
	}

	var result *reconcile.Result
	result, err = r.setDefaults(multiClusterHub)
	if result != nil {
		return *result, err
	}

	// Read image overrides
	imageOverrides, err := manifest.GetImageOverrides(multiClusterHub)
	if err != nil {
		reqLogger.Error(err, "Could not get map of image overrides")
		return reconcile.Result{}, err
	}

	// Check for developer overrides
	if imageOverridesConfigmap := utils.GetImageOverridesConfigmap(multiClusterHub); imageOverridesConfigmap != "" {
		imageOverrides, err = r.OverrideImagesFromConfigmap(imageOverrides, multiClusterHub.GetNamespace(), imageOverridesConfigmap)
		if err != nil {
			reqLogger.Error(err, fmt.Sprintf("Could not find image override configmap: %s/%s", multiClusterHub.GetNamespace(), imageOverridesConfigmap))
			return reconcile.Result{}, err
		}
	}
	r.CacheSpec.ImageOverrides = imageOverrides
	r.CacheSpec.ManifestVersion = version.Version
	r.CacheSpec.ImageOverrideType = manifest.GetImageOverrideType(multiClusterHub)
	r.CacheSpec.ImageRepository = utils.GetImageRepository(multiClusterHub)
	r.CacheSpec.ImageSuffix = utils.GetImageSuffix(multiClusterHub)
	r.CacheSpec.ImageOverridesCM = utils.GetImageOverridesConfigmap(multiClusterHub)

	err = r.maintainImageManifestConfigmap(multiClusterHub)
	if err != nil {
		reqLogger.Error(err, "Error storing image manifests in configmap")
		return reconcile.Result{}, err
	}

	UpgradeHackRequired, err := r.UpgradeHubSelfMgmtHackRequired(multiClusterHub)
	if err != nil {
		reqLogger.Error(err, "Error determining if upgrade specific logic is required")
		return reconcile.Result{}, err
	}

	if UpgradeHackRequired {
		result, err = r.BeginEnsuringHubIsUpgradeable(multiClusterHub)
		if err != nil {
			log.Info(fmt.Sprintf("Error starting to ensure local-cluster hub is upgradeable: %s", err.Error()))
			return reconcile.Result{RequeueAfter: resyncPeriod}, nil
		}
	}

	// Add installer labels to Helm-owned deployments
	myHelmReleases := getAppSubOwnedHelmReleases(allHRs, getAppsubs(multiClusterHub))
	myHRDeployments := getHelmReleaseOwnedDeployments(allDeploys, myHelmReleases)
	if err := r.labelDeployments(multiClusterHub, myHRDeployments); err != nil {
		return reconcile.Result{}, nil
	}

	// Do not reconcile objects if this instance of mch is labeled "paused"
	updatePausedCondition(multiClusterHub)
	if utils.IsPaused(multiClusterHub) {
		reqLogger.Info("MultiClusterHub reconciliation is paused. Nothing more to do.")
		return reconcile.Result{}, nil
	}

	result, err = r.ensureSubscriptionOperatorIsRunning(multiClusterHub, allDeploys)
	if result != nil {
		return *result, err
	}

	// Render CRD templates
	crdRenderer, err := rendering.NewCRDRenderer(multiClusterHub)
	if err != nil {
		reqLogger.Error(err, "Failed to read CRD templates")
		return reconcile.Result{}, err
	}
	crdResources, err := crdRenderer.Render()
	if err != nil {
		reqLogger.Error(err, "Failed to render CRD templates")
		return reconcile.Result{}, err
	}
	for _, crd := range crdResources {
		err, ok := deploying.Deploy(r.client, crd)
		if err != nil {
			reqLogger.Error(err, fmt.Sprintf("Failed to deploy %s %s", crd.GetKind(), crd.GetName()))
			return reconcile.Result{}, err
		}
		if ok {
			condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionTrue, NewComponentReason, "Created new resource")
			SetHubCondition(&multiClusterHub.Status, *condition)
		}
	}

	result, err = r.ensureDeployment(multiClusterHub, helmrepo.Deployment(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != nil {
		return *result, err
	}

	result, err = r.ensureService(multiClusterHub, helmrepo.Service(multiClusterHub))
	if result != nil {
		return *result, err
	}

	result, err = r.ensureChannel(multiClusterHub, channel.Channel(multiClusterHub))
	if result != nil {
		return *result, err
	}

	if multiClusterHub.Spec.SeparateCertificateManagement && multiClusterHub.Spec.ImagePullSecret != "" {
		result, err = r.copyPullSecret(multiClusterHub, utils.CertManagerNamespace)
		if result != nil {
			return *result, err
		}
	}

	result, err = r.ensureSubscription(multiClusterHub, subscription.CertManager(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != nil {
		return *result, err
	}

	certGV := schema.GroupVersion{Group: "certmanager.k8s.io", Version: "v1alpha1"}
	// Skip wait for API to be ready on unit test
	if !utils.IsUnitTest() {
		// condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionTrue, CertManagerReason, "Waiting for cert manager CRDs")
		// SetHubCondition(&multiClusterHub.Status, *condition)
		result, err = r.apiReady(certGV)
		if result != nil {
			return *result, err
		}
	}

	result, err = r.ensureSubscription(multiClusterHub, subscription.CertWebhook(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != nil {
		return *result, err
	}

	result, _ = r.ensureWebhookIsAvailable(multiClusterHub)
	if result != nil {
		return *result, nil
	}

	result, err = r.ensureSubscription(multiClusterHub, subscription.ConfigWatcher(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != nil {
		return *result, err
	}

	result, err = r.ingressDomain(multiClusterHub)
	if result != nil {
		return *result, err
	}

	//Render the templates with a specified CR
	renderer := rendering.NewRenderer(multiClusterHub)
	toDeploy, err := renderer.Render(r.client)
	if err != nil {
		reqLogger.Error(err, "Failed to render MultiClusterHub templates")
		return reconcile.Result{}, err
	}
	//Deploy the resources
	for _, res := range toDeploy {
		if res.GetNamespace() == multiClusterHub.Namespace {
			if err := controllerutil.SetControllerReference(multiClusterHub, res, r.scheme); err != nil {
				reqLogger.Error(err, "Failed to set controller reference")
			}
		}
		err, ok := deploying.Deploy(r.client, res)
		if err != nil {
			reqLogger.Error(err, fmt.Sprintf("Failed to deploy %s %s/%s", res.GetKind(), multiClusterHub.Namespace, res.GetName()))
			return reconcile.Result{}, err
		}
		if ok {
			condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionTrue, NewComponentReason, "Created new resource")
			SetHubCondition(&multiClusterHub.Status, *condition)
		}
	}

	result, err = r.ensureDeployment(multiClusterHub, foundation.WebhookDeployment(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != nil {
		return *result, err
	}

	result, err = r.ensureService(multiClusterHub, foundation.WebhookService(multiClusterHub))
	if result != nil {
		return *result, err
	}

	// Wait for ocm-webhook to be fully available before applying rest of subscriptions
	if !(multiClusterHub.Status.Components["ocm-webhook"].Type == "Available" && multiClusterHub.Status.Components["ocm-webhook"].Status == metav1.ConditionTrue) {
		reqLogger.Info(fmt.Sprintf("Waiting for component 'ocm-webhook' to be available"))
		return reconcile.Result{RequeueAfter: resyncPeriod}, nil
	}

	// Install the rest of the subscriptions in no particular order
	result, err = r.ensureSubscription(multiClusterHub, subscription.ManagementIngress(multiClusterHub, r.CacheSpec.ImageOverrides, r.CacheSpec.IngressDomain))
	if result != nil {
		return *result, err
	}
	result, err = r.ensureSubscription(multiClusterHub, subscription.ApplicationUI(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != nil {
		return *result, err
	}
	result, err = r.ensureSubscription(multiClusterHub, subscription.Console(multiClusterHub, r.CacheSpec.ImageOverrides, r.CacheSpec.IngressDomain))
	if result != nil {
		return *result, err
	}
	result, err = r.ensureSubscription(multiClusterHub, subscription.GRC(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != nil {
		return *result, err
	}
	result, err = r.ensureSubscription(multiClusterHub, subscription.KUIWebTerminal(multiClusterHub, r.CacheSpec.ImageOverrides, r.CacheSpec.IngressDomain))
	if result != nil {
		return *result, err
	}
	result, err = r.ensureSubscription(multiClusterHub, subscription.RCM(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != nil {
		return *result, err
	}
	result, err = r.ensureSubscription(multiClusterHub, subscription.Search(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != nil {
		return *result, err
	}

	//OCM proxy server deployment
	result, err = r.ensureDeployment(multiClusterHub, foundation.OCMProxyServerDeployment(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != nil {
		return *result, err
	}

	//OCM proxy server service
	result, err = r.ensureService(multiClusterHub, foundation.OCMProxyServerService(multiClusterHub))
	if result != nil {
		return *result, err
	}

	//OCM controller deployment
	result, err = r.ensureDeployment(multiClusterHub, foundation.OCMControllerDeployment(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != nil {
		return *result, err
	}

	result, err = r.ensureUnstructuredResource(multiClusterHub, foundation.ClusterManager(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != nil {
		return *result, err
	}

	if !multiClusterHub.Spec.DisableHubSelfManagement {
		result, err = r.ensureHubIsImported(multiClusterHub)
		if result != nil {
			return *result, err
		}
	} else {
		result, err = r.ensureHubIsExported(multiClusterHub)
		if result != nil {
			return *result, err
		}
	}

	return retQueue, retError
}

// setDefaults updates MultiClusterHub resource with proper defaults
func (r *ReconcileMultiClusterHub) setDefaults(m *operatorsv1.MultiClusterHub) (*reconcile.Result, error) {
	if utils.MchIsValid(m) {
		return nil, nil
	}
	log.Info("MultiClusterHub is Invalid. Updating with proper defaults")

	if len(m.Spec.Ingress.SSLCiphers) == 0 {
		m.Spec.Ingress.SSLCiphers = utils.DefaultSSLCiphers
	}

	if !utils.AvailabilityConfigIsValid(m.Spec.AvailabilityConfig) {
		m.Spec.AvailabilityConfig = operatorsv1.HAHigh
	}

	// Apply defaults to server
	err := r.client.Update(context.TODO(), m)
	if err != nil {
		log.Error(err, "Failed to update MultiClusterHub", "MultiClusterHub.Namespace", m.Namespace, "MultiClusterHub.Name", m.Name)
		return &reconcile.Result{}, err
	}

	log.Info("MultiClusterHub successfully updated")
	return &reconcile.Result{Requeue: true}, nil
}

// ingressDomain is discovered from Openshift cluster configuration resources
func (r *ReconcileMultiClusterHub) ingressDomain(m *operatorsv1.MultiClusterHub) (*reconcile.Result, error) {
	if r.CacheSpec.IngressDomain != "" {
		return nil, nil
	}

	ingress := &netv1.Ingress{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name: "cluster",
	}, ingress)
	// Don't fail on a unit test (Fake client won't find "cluster" Ingress)
	if err != nil && !utils.IsUnitTest() {
		log.Error(err, "Failed to get Ingress")
		return &reconcile.Result{}, err
	}

	r.CacheSpec.IngressDomain = ingress.Spec.Domain
	return nil, nil
}

func (r *ReconcileMultiClusterHub) finalizeHub(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {
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

func (r *ReconcileMultiClusterHub) addFinalizer(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {
	reqLogger.Info("Adding Finalizer for the multiClusterHub")
	m.SetFinalizers(append(m.GetFinalizers(), hubFinalizer))

	// Update CR
	err := r.client.Update(context.TODO(), m)
	if err != nil {
		reqLogger.Error(err, "Failed to update MultiClusterHub with finalizer")
		return err
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

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func remove(list []string, s string) []string {
	for i, v := range list {
		if v == s {
			list = append(list[:i], list[i+1:]...)
		}
	}
	return list
}
