// Copyright (c) 2020 Red Hat, Inc.

package multiclusterhub

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
	"sigs.k8s.io/controller-runtime/pkg/predicate"
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

// GenerationChangedPredicate implements a default update predicate function on Generation change.
//
// This predicate will skip update events that have no change in the object's metadata.generation field.
// The metadata.generation field of an object is incremented by the API server when writes are made to the spec field of an object.
// This allows a controller to ignore update events where the spec is unchanged, and only the metadata and/or status fields are changed.
type GenerationChangedPredicate struct {
	predicate.Funcs
}

// Update implements default UpdateEvent filter for validating generation change
func (GenerationChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.MetaOld == nil {
		log.Error(nil, "Update event has no old metadata", "event", e)
		return false
	}
	if e.ObjectOld == nil {
		log.Error(nil, "Update event has no old runtime object to update", "event", e)
		return false
	}
	if e.ObjectNew == nil {
		log.Error(nil, "Update event has no new runtime object for update", "event", e)
		return false
	}
	if e.MetaNew == nil {
		log.Error(nil, "Update event has no new metadata", "event", e)
		return false
	}

	if !utils.AnnotationsMatch(e.MetaOld.GetAnnotations(), e.MetaNew.GetAnnotations()) {
		log.Info("Metadata annotations have changed")
		return true
	}

	return e.MetaNew.GetGeneration() != e.MetaOld.GetGeneration()
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("multiclusterhub-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for apiService deletions
	pred := predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc:  func(e event.UpdateEvent) bool { return false },
		DeleteFunc: func(e event.DeleteEvent) bool {
			labels := e.Meta.GetLabels()
			_, nameExists := labels["installer.name"]
			_, namespaceExists := labels["installer.namespace"]
			return nameExists && namespaceExists
		},
	}

	// Watch for changes to primary resource MultiClusterHub
	err = c.Watch(&source.Kind{Type: &operatorsv1.MultiClusterHub{}}, &handler.EnqueueRequestForObject{}, GenerationChangedPredicate{})
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
		pred,
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
func (r *ReconcileMultiClusterHub) Reconcile(request reconcile.Request) (reconcile.Result, error) {
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

	// Check if the multiClusterHub instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isHubMarkedToBeDeleted := multiClusterHub.GetDeletionTimestamp() != nil
	if isHubMarkedToBeDeleted {
		if contains(multiClusterHub.GetFinalizers(), hubFinalizer) {
			// Run finalization logic. If the finalization
			// logic fails, don't remove the finalizer so
			// that we can retry during the next reconciliation.
			if err := r.finalizeHub(reqLogger, multiClusterHub); err != nil {
				return reconcile.Result{}, err
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

	// Revalidate cache. Rebuild image overrides if invalid
	if r.CacheSpec.isStale(multiClusterHub) {
		log.Info("Refreshing image cache")
		// Need to update image format
		imageOverrides, err := manifest.GetImageOverrides(multiClusterHub)
		if err != nil {
			reqLogger.Error(err, "Could not get map of image overrides")
			return reconcile.Result{}, err
		}
		r.CacheSpec.ImageOverrides = imageOverrides
		r.CacheSpec.ManifestVersion = version.Version
		r.CacheSpec.ImageOverrideType = manifest.GetImageOverrideType(multiClusterHub)
		r.CacheSpec.ImageRepository = utils.GetImageRepository(multiClusterHub)
		r.CacheSpec.ImageSuffix = utils.GetImageSuffix(multiClusterHub)
	}

	// Do not reconcile objects if this instance of mch is labeled "paused"
	if utils.IsPaused(multiClusterHub) {
		reqLogger.Info("MultiClusterHub reconciliation is paused. Nothing more to do.")
		return reconcile.Result{}, nil
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

	if multiClusterHub.Spec.SeparateCertificateManagement {
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
		result, err = r.apiReady(certGV)
		if result != nil {
			return *result, err
		}
	}

	result, err = r.ensureSubscription(multiClusterHub, subscription.CertWebhook(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != nil {
		return *result, err
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
		if err := deploying.Deploy(r.client, res); err != nil {
			reqLogger.Error(err, fmt.Sprintf("Failed to deploy %s %s/%s", res.GetKind(), multiClusterHub.Namespace, res.GetName()))
			return reconcile.Result{}, err
		}
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
	result, err = r.ensureSubscription(multiClusterHub, subscription.KUIWebTerminal(multiClusterHub, r.CacheSpec.ImageOverrides))
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
	result, err = r.ensureSubscription(multiClusterHub, subscription.Topology(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != nil {
		return *result, err
	}
	result, err = r.ensureDeployment(multiClusterHub, foundation.WebhookDeployment(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != nil {
		return *result, err
	}

	result, err = r.ensureService(multiClusterHub, foundation.WebhookService(multiClusterHub))
	if result != nil {
		return *result, err
	}

	//ACM proxy server deployment
	result, err = r.ensureDeployment(multiClusterHub, foundation.ACMProxyServerDeployment(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != nil {
		return *result, err
	}

	//ACM proxy server service
	result, err = r.ensureService(multiClusterHub, foundation.ACMProxyServerService(multiClusterHub))
	if result != nil {
		return *result, err
	}

	//ACM controller deployment
	result, err = r.ensureDeployment(multiClusterHub, foundation.ACMControllerDeployment(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != nil {
		return *result, err
	}

	result, err = r.ensureClusterManager(multiClusterHub, foundation.ClusterManager(multiClusterHub, r.CacheSpec.ImageOverrides))
	if result != nil {
		return *result, err
	}

	// Update the CR status
	multiClusterHub.Status.Phase = "Pending"
	multiClusterHub.Status.DesiredVersion = version.Version
	ready, _, err := deploying.ListDeployments(r.client, multiClusterHub.Namespace)
	if err != nil {
		reqLogger.Error(err, "Failed to list deployments")
		return reconcile.Result{}, err
	}
	if ready {
		multiClusterHub.Status.Phase = "Running"
		multiClusterHub.Status.CurrentVersion = version.Version
	}

	result, err = r.UpdateStatus(multiClusterHub)
	if result != nil {
		return *result, err
	}

	if !ready {
		// Keep reconciling while install is not complete
		return reconcile.Result{RequeueAfter: resyncPeriod}, nil
	}
	return reconcile.Result{}, nil
}

func (r *ReconcileMultiClusterHub) UpdateStatus(m *operatorsv1.MultiClusterHub) (*reconcile.Result, error) {
	err := r.client.Status().Update(context.TODO(), m)
	if err != nil {
		if errors.IsConflict(err) {
			// Error from object being modified is normal behavior and should not be treated like an error
			log.Info("Failed to update status", "Reason", "Object has been modified")
			return &reconcile.Result{RequeueAfter: resyncPeriod}, nil
		}

		log.Error(err, fmt.Sprintf("Failed to update %s/%s status ", m.Namespace, m.Name))
		return &reconcile.Result{}, err
	}
	return nil, nil
}

func generatePass(length int) string {
	chars := "ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz" +
		"0123456789"

	buf := make([]byte, length)
	for i := 0; i < length; i++ {
		nBig, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		buf[i] = chars[nBig.Int64()]
	}
	return string(buf)
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
