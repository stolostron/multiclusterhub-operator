package multiclusterhub

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storv1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/deploying"
	subscription "github.com/open-cluster-management/multicloudhub-operator/pkg/deploying/subscription"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/rendering"
)

var log = logf.Log.WithName("controller_multiclusterhub")

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
	err = c.Watch(&source.Kind{Type: &operatorsv1alpha1.MultiClusterHub{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner MultiClusterHub
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &operatorsv1alpha1.MultiClusterHub{},
	})
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
	client client.Client
	scheme *runtime.Scheme
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
	multiClusterHub := &operatorsv1alpha1.MultiClusterHub{}
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

	var result *reconcile.Result
	result, err = r.ensureDeployment(request, multiClusterHub, r.helmRepoDeployment(multiClusterHub))
	if result != nil {
		return *result, err
	}

	result, err = r.ensureService(request, multiClusterHub, r.repoService(multiClusterHub))
	if result != nil {
		return *result, err
	}

	chClient, config, err := createDynamicClient()
	if err != nil {
		return reconcile.Result{}, nil
	}
	result, err = r.ensureChannel(multiClusterHub, *chClient)
	if result != nil {
		return *result, err
	}

	subClient, config, err := createDynamicClient()
	if err != nil {
		return reconcile.Result{}, nil
	}

	result, err = r.ensureSubscription(multiClusterHub, *subClient, subscription.CertManager(multiClusterHub))
	if result != nil {
		return *result, err
	}

	certGV := schema.GroupVersion{Group: "certmanager.k8s.io", Version: "v1alpha1"}
	result, err = r.apiReady(config, certGV)
	if result != nil {
		return *result, err
	}

	result, err = r.ensureSubscription(multiClusterHub, *dynClient, subscription.CertWebhook(multiClusterHub))
	if result != nil {
		return *result, err
	}

	result, err = r.ensureSubscription(multiClusterHub, *dynClient, subscription.ConfigWatcher(multiClusterHub))
	if result != nil {
		return *result, err
	}

	result, err = r.ensureSecret(request, multiClusterHub, r.mongoAuthSecret(multiClusterHub))
	if result != nil {
		return *result, err
	}

	result, err = r.storageClass(multiClusterHub)
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

	// Update the CR status
	multiClusterHub.Status.Phase = "Failed"
	ready, deployments, err := deploying.ListDeployments(r.client, multiClusterHub.Namespace)
	if err != nil {
		return reconcile.Result{}, err
	}
	if ready {
		multiClusterHub.Status.Phase = "Running"
	}
	statedDeployments := []operatorsv1alpha1.DeploymentResult{}
	for _, deploy := range deployments {
		statedDeployments = append(statedDeployments, operatorsv1alpha1.DeploymentResult{
			Name:   deploy.Name,
			Status: deploy.Status,
		})
	}
	multiClusterHub.Status.Deployments = statedDeployments
	err = r.client.Status().Update(context.TODO(), multiClusterHub)
	if err != nil {
		reqLogger.Error(err, fmt.Sprintf("Failed to update %s/%s status ", multiClusterHub.Namespace, multiClusterHub.Name))
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *ReconcileMultiClusterHub) mongoAuthSecret(v *operatorsv1alpha1.MultiClusterHub) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mongodb-admin",
			Namespace: v.Namespace,
		},
		Type: "Opaque",
		StringData: map[string]string{
			"user":     "some@example.com",
			"password": generatePass(16),
		},
	}

	if err := controllerutil.SetControllerReference(v, secret, r.scheme); err != nil {
		log.Error(err, "Failed to set controller reference", "Secret.Namespace", v.Namespace, "Secret.Name", v.Name)
	}
	return secret
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

func (r *ReconcileMultiClusterHub) storageClass(m *operatorsv1alpha1.MultiClusterHub) (*reconcile.Result, error) {
	storageClass := m.Spec.StorageClass
	if storageClass == "" {
		scList := &storv1.StorageClassList{}
		if err := r.client.List(context.TODO(), scList); err != nil {
			return nil, err
		}
		for _, sc := range scList.Items {
			if sc.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
				m.Spec.StorageClass = sc.GetName()
				break
			}
		}
	}
	// edge case (hopefully)
	if m.Spec.StorageClass == "" {
		return &reconcile.Result{}, fmt.Errorf("failed to find storage class")
	}
	return nil, nil
}

// ingressDomain is discovered from Openshift cluster configuration resources
func (r *ReconcileMultiClusterHub) ingressDomain(m *operatorsv1alpha1.MultiClusterHub) (*reconcile.Result, error) {
	if m.Spec.IngressDomain != "" {
		return nil, nil
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Error(err, "Failed to get cluster config for API host discovery/authentication")
		return &reconcile.Result{}, err
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Error(err, "Failed to create dynamic client from cluster config")
		return &reconcile.Result{}, err
	}

	schema := schema.GroupVersionResource{Group: "config.openshift.io", Version: "v1", Resource: "ingresses"}
	crdClient := dynClient.Resource(schema)

	crd, err := crdClient.Get("cluster", metav1.GetOptions{})
	if err != nil {
		log.Error(err, "Failed to get resource", "resource", schema.GroupResource().String())
		return &reconcile.Result{}, err
	}

	domain, ok, err := unstructured.NestedString(crd.UnstructuredContent(), "spec", "domain")
	if err != nil {
		log.Error(err, "Error parsing resource", "resource", schema.GroupResource().String(), "value", "spec.domain")
		return &reconcile.Result{}, err
	}
	if !ok {
		err = fmt.Errorf("field not found")
		log.Error(err, "Ingress config did not contain expected value", "resource", schema.GroupResource().String(), "value", "spec.domain")
		return &reconcile.Result{}, err
	}

	log.Info("Ingress domain not set, updating value in spec", "MultiClusterHub.Namespace", m.Namespace, "MultiClusterHub.Name", m.Name, "ingressDomain", domain)
	m.Spec.IngressDomain = domain
	err = r.client.Update(context.TODO(), m)
	if err != nil {
		log.Error(err, "Failed to update MultiClusterHub", "MultiClusterHub.Namespace", m.Namespace, "MultiClusterHub.Name", m.Name)
		return &reconcile.Result{}, err
	}

	return nil, nil
}
