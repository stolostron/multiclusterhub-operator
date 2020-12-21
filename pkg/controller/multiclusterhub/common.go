// Copyright (c) 2020 Red Hat, Inc.

package multiclusterhub

import (
	"context"
	"encoding/json"
	e "errors"
	"fmt"
	"reflect"

	"time"

	subrelv1 "github.com/open-cluster-management/multicloud-operators-subscription-release/pkg/apis/apps/v1"
	operatorsv1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operator/v1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/foundation"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/helmrepo"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/manifest"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/subscription"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	"github.com/open-cluster-management/multicloudhub-operator/version"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// CacheSpec ...
type CacheSpec struct {
	IngressDomain     string
	ImageOverrides    map[string]string
	ImageOverrideType manifest.OverrideType
	ImageRepository   string
	ImageSuffix       string
	ManifestVersion   string
	ImageOverridesCM  string
}

func (r *ReconcileMultiClusterHub) ensureDeployment(m *operatorsv1.MultiClusterHub, dep *appsv1.Deployment) (*reconcile.Result, error) {
	dplog := log.WithValues("Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)

	// See if deployment already exists and create if it doesn't
	found := &appsv1.Deployment{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      dep.Name,
		Namespace: m.Namespace,
	}, found)
	if err != nil && errors.IsNotFound(err) {

		// Create the deployment
		err = r.client.Create(context.TODO(), dep)
		if err != nil {
			// Deployment failed
			dplog.Error(err, "Failed to create new Deployment")
			return &reconcile.Result{}, err
		}

		// Deployment was successful
		dplog.Info("Created a new Deployment")
		condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionTrue, NewComponentReason, "Created new resource")
		SetHubCondition(&m.Status, *condition)
		return nil, nil

	} else if err != nil {
		// Error that isn't due to the deployment not existing
		dplog.Error(err, "Failed to get Deployment")
		return &reconcile.Result{}, err
	}

	// Validate object based on name
	var desired *appsv1.Deployment
	var needsUpdate bool

	switch found.Name {
	case helmrepo.HelmRepoName:
		desired, needsUpdate = helmrepo.ValidateDeployment(m, r.CacheSpec.ImageOverrides, dep, found)
	case foundation.OCMControllerName, foundation.OCMProxyServerName, foundation.WebhookName:
		desired, needsUpdate = foundation.ValidateDeployment(m, r.CacheSpec.ImageOverrides, dep, found)
	default:
		dplog.Info("Could not validate deployment; unknown name")
		return nil, nil
	}

	if needsUpdate {
		err = r.client.Update(context.TODO(), desired)
		if err != nil {
			dplog.Error(err, "Failed to update Deployment.")
			return &reconcile.Result{}, err
		}
		// Spec updated - return
		return nil, nil
	}
	return nil, nil
}

func (r *ReconcileMultiClusterHub) ensureService(m *operatorsv1.MultiClusterHub, s *corev1.Service) (*reconcile.Result, error) {
	svlog := log.WithValues("Service.Namespace", s.Namespace, "Service.Name", s.Name)

	found := &corev1.Service{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      s.Name,
		Namespace: m.Namespace,
	}, found)
	if err != nil && errors.IsNotFound(err) {

		// Create the service
		err = r.client.Create(context.TODO(), s)

		if err != nil {
			// Creation failed
			svlog.Error(err, "Failed to create new Service")
			return &reconcile.Result{}, err
		}

		// Creation was successful
		svlog.Info("Created a new Service")
		condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionTrue, NewComponentReason, "Created new resource")
		SetHubCondition(&m.Status, *condition)
		return nil, nil

	} else if err != nil {
		// Error that isn't due to the service not existing
		svlog.Error(err, "Failed to get Service")
		return &reconcile.Result{}, err
	}

	return nil, nil
}

func (r *ReconcileMultiClusterHub) ensureChannel(m *operatorsv1.MultiClusterHub, u *unstructured.Unstructured) (*reconcile.Result, error) {
	selog := log.WithValues("Channel.Namespace", u.GetNamespace(), "Channel.Name", u.GetName())

	found := &unstructured.Unstructured{}
	found.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps.open-cluster-management.io",
		Kind:    "Channel",
		Version: "v1",
	})
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      u.GetName(),
		Namespace: m.Namespace,
	}, found)
	if err != nil && errors.IsNotFound(err) {

		// Create the Channel
		err = r.client.Create(context.TODO(), u)
		if err != nil {
			// Creation failed
			selog.Error(err, "Failed to create new Channel")
			return &reconcile.Result{}, err
		}

		// Creation was successful
		selog.Info("Created a new Channel")
		condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionTrue, NewComponentReason, "Created new resource")
		SetHubCondition(&m.Status, *condition)
		return nil, nil

	} else if err != nil {
		// Error that isn't due to the Channel not existing
		selog.Error(err, "Failed to get Channel")
		return &reconcile.Result{}, err
	}

	return nil, nil
}

func (r *ReconcileMultiClusterHub) ensureSubscription(m *operatorsv1.MultiClusterHub, u *unstructured.Unstructured) (*reconcile.Result, error) {
	obLog := log.WithValues("Namespace", u.GetNamespace(), "Name", u.GetName(), "Kind", u.GetKind())

	found := &unstructured.Unstructured{}
	found.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps.open-cluster-management.io",
		Kind:    "Subscription",
		Version: "v1",
	})
	// Try to get API group instance
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      u.GetName(),
		Namespace: u.GetNamespace(),
	}, found)
	if err != nil && errors.IsNotFound(err) {

		// Create the resource. Skip on unit test
		if !utils.IsUnitTest() {
			err := r.client.Create(context.TODO(), u)
			if err != nil {
				// Creation failed
				obLog.Error(err, "Failed to create new instance")
				return &reconcile.Result{}, err
			}
		}

		// Creation was successful
		obLog.Info("Created new object")
		condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionTrue, NewComponentReason, "Created new resource")
		SetHubCondition(&m.Status, *condition)
		return nil, nil

	} else if err != nil {
		// Error that isn't due to the resource not existing
		obLog.Error(err, "Failed to get subscription")
		return &reconcile.Result{}, err
	}

	// Validate object based on type
	updated, needsUpdate := subscription.Validate(found, u)
	if needsUpdate {
		obLog.Info("Updating subscription")
		// Update the resource. Skip on unit test
		err = r.client.Update(context.TODO(), updated)
		if err != nil {
			// Update failed
			obLog.Error(err, "Failed to update object")
			return &reconcile.Result{}, err
		}

		// Spec updated - return
		return nil, nil
	}

	return nil, nil
}

func (r *ReconcileMultiClusterHub) ensureUnstructuredResource(m *operatorsv1.MultiClusterHub, u *unstructured.Unstructured) (*reconcile.Result, error) {
	obLog := log.WithValues("Namespace", u.GetNamespace(), "Name", u.GetName(), "Kind", u.GetKind())

	found := &unstructured.Unstructured{}
	found.SetGroupVersionKind(u.GroupVersionKind())

	// Try to get API group instance
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      u.GetName(),
		Namespace: u.GetNamespace(),
	}, found)
	if err != nil && errors.IsNotFound(err) {
		// Resource doesn't exist so create it
		err := r.client.Create(context.TODO(), u)
		if err != nil {
			// Creation failed
			obLog.Error(err, "Failed to create new instance")
			return &reconcile.Result{}, err
		}
		// Creation was successful
		obLog.Info("Created new resource")
		condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionTrue, NewComponentReason, "Created new resource")
		SetHubCondition(&m.Status, *condition)
		return nil, nil

	} else if err != nil {
		// Error that isn't due to the resource not existing
		obLog.Error(err, "Failed to get resource")
		return &reconcile.Result{}, err
	}

	// Validate object based on name
	var desired *unstructured.Unstructured
	var needsUpdate bool

	switch found.GetKind() {
	case "ClusterManager":
		desired, needsUpdate = foundation.ValidateClusterManager(found, u)
	default:
		obLog.Info("Could not validate unstrucuted resource with type.", "Type", found.GetKind())
		return nil, nil
	}

	if needsUpdate {
		obLog.Info("Updating resource")
		err = r.client.Update(context.TODO(), desired)
		if err != nil {
			obLog.Error(err, "Failed to update resource.")
			return &reconcile.Result{}, err
		}
	}
	return nil, nil
}

func (r *ReconcileMultiClusterHub) apiReady(gv schema.GroupVersion) (*reconcile.Result, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "Failed to create rest config")
		return &reconcile.Result{}, err
	}

	c, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		log.Error(err, "Failed to create discovery client")
		return &reconcile.Result{}, err
	}

	err = discovery.ServerSupportsVersion(c, gv)
	if err != nil {
		// Wait a little and try again
		log.Info("Waiting for API group to be available", "API group", gv)
		// condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionTrue, NewComponentReason, "Waiting for cert manager CRD availability")
		// SetHubCondition(&m.Status, *condition)
		return &reconcile.Result{RequeueAfter: time.Second * 10}, nil
	}
	return nil, nil
}

func (r *ReconcileMultiClusterHub) copyPullSecret(m *operatorsv1.MultiClusterHub, newNS string) (*reconcile.Result, error) {
	sublog := log.WithValues("Copying Secret to cert-manager namespace", m.Spec.ImagePullSecret, "Namespace.Name", utils.CertManagerNamespace)

	if m.Spec.ImagePullSecret == "" {
		err := e.New("imagePullSecret is empty")
		sublog.Error(err, "copyPullSecret requires a valid secret to copy")
		return &reconcile.Result{}, err
	}

	pullSecret := &v1.Secret{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      m.Spec.ImagePullSecret,
		Namespace: m.Namespace,
	}, pullSecret)
	if err != nil {
		sublog.Error(err, "Failed to get secret")
		return &reconcile.Result{}, err
	}

	pullSecret.SetNamespace(newNS)
	pullSecret.SetSelfLink("")
	pullSecret.SetResourceVersion("")
	pullSecret.SetUID("")

	unstructuredPullSecret, err := utils.CoreToUnstructured(pullSecret)
	if err != nil {
		sublog.Error(err, "Failed to unmarshal into unstructured object")
		return &reconcile.Result{}, err
	}
	utils.AddInstallerLabel(unstructuredPullSecret, m.Name, m.Namespace)

	err = r.client.Get(context.TODO(), types.NamespacedName{
		Name:      unstructuredPullSecret.GetName(),
		Namespace: newNS,
	}, unstructuredPullSecret)

	if err != nil && errors.IsNotFound(err) {
		sublog.Info(fmt.Sprintf("Creating secret %s in namespace %s", unstructuredPullSecret.GetName(), utils.CertManagerNamespace))
		err = r.client.Create(context.TODO(), unstructuredPullSecret)
		if err != nil {
			sublog.Error(err, "Failed to create secret")
			return &reconcile.Result{}, err
		}
	}
	return nil, nil
}

// OverrideImagesFromConfigmap ...
func (r *ReconcileMultiClusterHub) OverrideImagesFromConfigmap(imageOverrides map[string]string, namespace, configmapName string) (map[string]string, error) {
	log.Info(fmt.Sprintf("Overriding images from configmap: %s/%s", namespace, configmapName))

	configmap := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      configmapName,
		Namespace: namespace,
	}, configmap)
	if err != nil && errors.IsNotFound(err) {
		return imageOverrides, err
	}

	if len(configmap.Data) != 1 {
		return imageOverrides, fmt.Errorf(fmt.Sprintf("Unexpected number of keys in configmap: %s", configmapName))
	}

	for _, v := range configmap.Data {

		var manifestImages []manifest.ManifestImage
		err = json.Unmarshal([]byte(v), &manifestImages)
		if err != nil {
			return nil, err
		}

		for _, manifestImage := range manifestImages {
			if manifestImage.ImageDigest != "" {
				imageOverrides[manifestImage.ImageKey] = fmt.Sprintf("%s/%s@%s", manifestImage.ImageRemote, manifestImage.ImageName, manifestImage.ImageDigest)
			} else if manifestImage.ImageTag != "" {
				imageOverrides[manifestImage.ImageKey] = fmt.Sprintf("%s/%s:%s", manifestImage.ImageRemote, manifestImage.ImageName, manifestImage.ImageTag)
			}

		}
	}

	return imageOverrides, nil
}

func (r *ReconcileMultiClusterHub) maintainImageManifestConfigmap(mch *operatorsv1.MultiClusterHub) error {
	// Define configmap
	configmap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("mch-image-manifest-%s", r.CacheSpec.ManifestVersion),
			Namespace: mch.Namespace,
		},
	}
	configmap.SetOwnerReferences([]metav1.OwnerReference{
		*metav1.NewControllerRef(mch, mch.GetObjectKind().GroupVersionKind()),
	})

	labels := make(map[string]string)
	labels["ocm-configmap-type"] = "image-manifest"
	labels["ocm-release-version"] = r.CacheSpec.ManifestVersion

	configmap.SetLabels(labels)

	// Get Configmap if it exists
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      configmap.Name,
		Namespace: configmap.Namespace,
	}, configmap)
	if err != nil && errors.IsNotFound(err) {
		// If configmap does not exist, create and return
		configmap.Data = r.CacheSpec.ImageOverrides
		err = r.client.Create(context.TODO(), configmap)
		if err != nil {
			return err
		}
		return nil
	}

	// If cached image overrides are not equal to the configmap data, update configmap and return
	if !reflect.DeepEqual(configmap.Data, r.CacheSpec.ImageOverrides) {
		configmap.Data = r.CacheSpec.ImageOverrides
		err = r.client.Update(context.TODO(), configmap)
		if err != nil {
			return err
		}
	}

	return nil
}

// listDeployments gets all deployments in the given namespaces
func (r *ReconcileMultiClusterHub) listDeployments(namespaces []string) ([]*appsv1.Deployment, error) {
	var ret []*appsv1.Deployment

	for _, n := range namespaces {
		deployList := &appsv1.DeploymentList{}
		err := r.client.List(context.TODO(), deployList, client.InNamespace(n))
		if err != nil && !errors.IsNotFound(err) {
			return nil, err
		}

		for i := 0; i < len(deployList.Items); i++ {
			ret = append(ret, &deployList.Items[i])
		}
	}
	return ret, nil
}

// listHelmReleases gets all helmreleases in the given namespaces
func (r *ReconcileMultiClusterHub) listHelmReleases(namespaces []string) ([]*subrelv1.HelmRelease, error) {
	var ret []*subrelv1.HelmRelease

	for _, n := range namespaces {
		hrList := &subrelv1.HelmReleaseList{}
		err := r.client.List(context.TODO(), hrList, client.InNamespace(n))
		if err != nil && !errors.IsNotFound(err) {
			return nil, err
		}

		for i := 0; i < len(hrList.Items); i++ {
			ret = append(ret, &hrList.Items[i])
		}
	}

	return ret, nil
}

// listCustomResources gets custom resources the installer observes
func (r *ReconcileMultiClusterHub) listCustomResources() ([]*unstructured.Unstructured, error) {
	var ret []*unstructured.Unstructured

	cr, err := foundation.GetClusterManager(r.client)
	if err != nil {
		// Return nil on error to prevent blocking status updates
		cr = nil
	}
	ret = append(ret, cr)
	return ret, nil
}

// filterDeploymentsByRelease returns a subset of deployments with the release label value
func filterDeploymentsByRelease(deploys []*appsv1.Deployment, releaseLabel string) []*appsv1.Deployment {
	var labeledDeps []*appsv1.Deployment
	for i := range deploys {
		anno := deploys[i].GetAnnotations()
		if anno["meta.helm.sh/release-name"] == releaseLabel {
			labeledDeps = append(labeledDeps, deploys[i])
		}
	}
	return labeledDeps
}

// addInstallerLabel adds the installer name and namespace to a deployment's labels
// so it can be watched. Returns false if the labels are already present.
func addInstallerLabel(d *appsv1.Deployment, name string, ns string) bool {
	updated := false
	if d.Labels == nil {
		d.Labels = map[string]string{}
	}
	if d.Labels["installer.name"] != name {
		d.Labels["installer.name"] = name
		updated = true
	}
	if d.Labels["installer.namespace"] != ns {
		d.Labels["installer.namespace"] = ns
		updated = true
	}
	return updated
}

// getAppSubOwnedHelmReleases gets a subset of helmreleases created by the appsubs
func getAppSubOwnedHelmReleases(allHRs []*subrelv1.HelmRelease, appsubs []types.NamespacedName) []*subrelv1.HelmRelease {
	subMap := make(map[string]bool)
	for _, s := range appsubs {
		subMap[s.Name] = true
	}

	var ownedHRs []*subrelv1.HelmRelease
	for _, hr := range allHRs {
		// check if this is one of our helmreleases
		owner := hr.OwnerReferences[0].Name
		if subMap[owner] {
			ownedHRs = append(ownedHRs, hr)

		}
	}
	return ownedHRs
}

// getHelmReleaseOwnedDeployments gets a subset of deployments created by the helmreleases
func getHelmReleaseOwnedDeployments(allDeps []*appsv1.Deployment, hrList []*subrelv1.HelmRelease) []*appsv1.Deployment {
	var mchDeps []*appsv1.Deployment
	for _, hr := range hrList {
		hrDeployments := filterDeploymentsByRelease(allDeps, hr.Name)
		mchDeps = append(mchDeps, hrDeployments...)
	}
	return mchDeps
}

// labelDeployments updates deployments with installer labels if not already present
func (r *ReconcileMultiClusterHub) labelDeployments(hub *operatorsv1.MultiClusterHub, dList []*appsv1.Deployment) error {
	for _, d := range dList {
		// Attach installer labels so we can keep our eyes on the deployment
		if addInstallerLabel(d, hub.Name, hub.Namespace) {
			log.Info("Adding installer labels to deployment", "Name", d.Name)
			err := r.client.Update(context.TODO(), d)
			if err != nil {
				log.Error(err, "Failed to update Deployment", "Name", d.Name)
				return err
			}
		}
	}
	return nil
}

func (r *ReconcileMultiClusterHub) ensureWebhookIsAvailable(mch *operatorsv1.MultiClusterHub) (*reconcile.Result, error) {
	duration := time.Second * 5
	if _, ok := mch.Status.Components["cert-manager-webhook-sub"]; !ok {
		log.Info("Waiting for cert-manager-webhook status")
		return &reconcile.Result{RequeueAfter: duration}, fmt.Errorf("Waiting for cert-manager-webhook status")
	}

	component := mch.Status.Components["cert-manager-webhook-sub"]
	if !((component.Type == "Deployed" || component.Type == "DeployedRelease") && component.Status == "True") {
		log.Info("Waiting for cert-manager-webhook to be available")
		return &reconcile.Result{RequeueAfter: duration}, fmt.Errorf("Waiting for cert-manager-webhook to be available")
	}

	return nil, nil
}

// ensureSubscriptionOperatorIsRunning verifies that the subscription operator that manages helm subscriptions exists and
// is running. This validation is only intended to run during upgrade and when run as a bundled product
func (r *ReconcileMultiClusterHub) ensureSubscriptionOperatorIsRunning(mch *operatorsv1.MultiClusterHub, allDeps []*appsv1.Deployment) (*reconcile.Result, error) {
	// skip check if not upgrading
	if mch.Status.CurrentVersion == version.Version {
		log.Info("Not upgrading, skipping check")
		return nil, nil
	}
	// skip check if not in bundle
	if !isACMBundle(allDeps) {
		log.Info("Not running in ACM bundle, skipping check")
		return nil, nil
	}

	subscriptionDeploy, exists := getSubscriptionOperator(allDeps)
	if !exists {
		return &reconcile.Result{RequeueAfter: time.Second * 10}, fmt.Errorf("Standalone subscription deployment not found")
	}

	if successfulDeploy(subscriptionDeploy) {
		log.Info("Subscription operator is running")
		return nil, nil
	} else {
		log.Info("Standalone subscription deployment is not running")
		return &reconcile.Result{RequeueAfter: time.Second * 10}, nil
	}
}

// isACMBundle returns whether this application was deployment by the bundled ACM csv
func isACMBundle(allDeps []*appsv1.Deployment) bool {
	selfDeployment, exists := getSelfDeployment(allDeps)
	if !exists {
		return false
	}

	labels := selfDeployment.GetLabels()
	if labels == nil {
		return false
	}
	if _, ok := labels["olm.owner"]; ok {
		return true
	}
	return false
}

// getSubscriptionOperator returns the deployment that operates helm subscriptions based on name
func getSubscriptionOperator(allDeps []*appsv1.Deployment) (*appsv1.Deployment, bool) {
	for i := range allDeps {
		if allDeps[i].Name == utils.SubscriptionOperatorName {
			return allDeps[i], true
		}
	}
	return nil, false
}

// getSelfDeployment returns the multiclusterhub-operator deployment based on name
func getSelfDeployment(allDeps []*appsv1.Deployment) (*appsv1.Deployment, bool) {
	for i := range allDeps {
		if allDeps[i].Name == utils.MCHOperatorName {
			return allDeps[i], true
		}
	}
	return nil, false
}
