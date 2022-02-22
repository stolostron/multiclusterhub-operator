// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	olmv1 "github.com/operator-framework/api/pkg/operators/v1"

	mcev1 "github.com/stolostron/backplane-operator/api/v1"

	subrelv1 "github.com/open-cluster-management/multicloud-operators-subscription-release/pkg/apis/apps/v1"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"

	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/channel"
	"github.com/stolostron/multiclusterhub-operator/pkg/helmrepo"
	"github.com/stolostron/multiclusterhub-operator/pkg/manifest"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengine"
	"github.com/stolostron/multiclusterhub-operator/pkg/subscription"
	"github.com/stolostron/multiclusterhub-operator/pkg/version"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// CacheSpec ...
type CacheSpec struct {
	IngressDomain    string
	ImageOverrides   map[string]string
	ImageRepository  string
	ManifestVersion  string
	ImageOverridesCM string
}

func (r *MultiClusterHubReconciler) ensureDeployment(m *operatorv1.MultiClusterHub, dep *appsv1.Deployment) (ctrl.Result, error) {
	r.Log.Info("Reconciling MultiClusterHub")

	if utils.ProxyEnvVarsAreSet() {
		dep = addProxyEnvVarsToDeployment(dep)
	}

	// See if deployment already exists and create if it doesn't
	found := &appsv1.Deployment{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      dep.Name,
		Namespace: m.Namespace,
	}, found)
	if err != nil && errors.IsNotFound(err) {

		// Create the deployment
		err = r.Client.Create(context.TODO(), dep)
		if err != nil {
			// Deployment failed
			r.Log.Error(err, "Failed to create new Deployment")
			return ctrl.Result{}, err
		}

		// Deployment was successful
		r.Log.Info("Created a new Deployment")
		condition := NewHubCondition(operatorv1.Progressing, metav1.ConditionTrue, NewComponentReason, "Created new resource")
		SetHubCondition(&m.Status, *condition)
		return ctrl.Result{}, nil

	} else if err != nil {
		// Error that isn't due to the deployment not existing
		r.Log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	// Validate object based on name
	var desired *appsv1.Deployment
	var needsUpdate bool

	switch found.Name {
	case helmrepo.HelmRepoName:
		desired, needsUpdate = helmrepo.ValidateDeployment(m, r.CacheSpec.ImageOverrides, dep, found)
	default:
		r.Log.Info("Could not validate deployment; unknown name")
		return ctrl.Result{}, nil
	}

	if needsUpdate {
		err = r.Client.Update(context.TODO(), desired)
		if err != nil {
			r.Log.Error(err, "Failed to update Deployment.")
			return ctrl.Result{}, err
		}
		// Spec updated - return
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) ensureService(m *operatorv1.MultiClusterHub, s *corev1.Service) (ctrl.Result, error) {
	svlog := r.Log.WithValues("Service.Namespace", s.Namespace, "Service.Name", s.Name)

	found := &corev1.Service{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      s.Name,
		Namespace: m.Namespace,
	}, found)
	if err != nil && errors.IsNotFound(err) {

		// Create the service
		err = r.Client.Create(context.TODO(), s)

		if err != nil {
			// Creation failed
			svlog.Error(err, "Failed to create new Service")
			return ctrl.Result{}, err
		}

		// Creation was successful
		svlog.Info("Created a new Service")
		condition := NewHubCondition(operatorv1.Progressing, metav1.ConditionTrue, NewComponentReason, "Created new resource")
		SetHubCondition(&m.Status, *condition)
		return ctrl.Result{}, nil

	} else if err != nil {
		// Error that isn't due to the service not existing
		svlog.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}

	modified := resourcemerge.BoolPtr(false)
	existingCopy := found.DeepCopy()
	resourcemerge.EnsureObjectMeta(modified, &existingCopy.ObjectMeta, s.ObjectMeta)
	selectorSame := equality.Semantic.DeepEqual(existingCopy.Spec.Selector, s.Spec.Selector)

	typeSame := false
	requiredIsEmpty := len(s.Spec.Type) == 0
	existingCopyIsCluster := existingCopy.Spec.Type == corev1.ServiceTypeClusterIP
	if (requiredIsEmpty && existingCopyIsCluster) || equality.Semantic.DeepEqual(existingCopy.Spec.Type, s.Spec.Type) {
		typeSame = true
	}

	if selectorSame && typeSame && !*modified {
		return ctrl.Result{}, nil
	}

	existingCopy.Spec.Selector = s.Spec.Selector
	existingCopy.Spec.Type = s.Spec.Type
	err = r.Client.Update(context.TODO(), existingCopy)
	if err != nil {
		svlog.Error(err, "Failed to update Service")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) ensureChannel(m *operatorv1.MultiClusterHub, u *unstructured.Unstructured) (ctrl.Result, error) {
	selog := r.Log.WithValues("Channel.Namespace", u.GetNamespace(), "Channel.Name", u.GetName())

	found := &unstructured.Unstructured{}
	found.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps.open-cluster-management.io",
		Kind:    "Channel",
		Version: "v1",
	})
	err := r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      u.GetName(),
		Namespace: m.Namespace,
	}, found)
	if err != nil && errors.IsNotFound(err) {
		// Create the Channel
		err = r.Client.Create(context.TODO(), u)
		if err != nil {
			// Creation failed
			selog.Error(err, "Failed to create new Channel")
			return ctrl.Result{}, err
		}

		// Creation was successful
		selog.Info("Created a new Channel")
		condition := NewHubCondition(operatorv1.Progressing, metav1.ConditionTrue, NewComponentReason, "Created new resource")
		SetHubCondition(&m.Status, *condition)
		return ctrl.Result{}, nil

	} else if err != nil {
		// Error that isn't due to the Channel not existing
		selog.Error(err, "Failed to get Channel")
		return ctrl.Result{}, err
	}

	updated, needsUpdate := channel.Validate(m, found)
	if needsUpdate {
		selog.Info("Updating channel")
		err = r.Client.Update(context.TODO(), updated)
		if err != nil {
			// Update failed
			selog.Error(err, "Failed to update channel")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) ensureSubscription(m *operatorv1.MultiClusterHub, u *unstructured.Unstructured) (ctrl.Result, error) {
	obLog := r.Log.WithValues("Namespace", u.GetNamespace(), "Name", u.GetName(), "Kind", u.GetKind())

	if utils.ProxyEnvVarsAreSet() {
		u = addProxyEnvVarsToSub(u)
	}

	found := &unstructured.Unstructured{}
	found.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps.open-cluster-management.io",
		Kind:    "Subscription",
		Version: "v1",
	})
	// Try to get API group instance
	err := r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      u.GetName(),
		Namespace: u.GetNamespace(),
	}, found)
	if err != nil && errors.IsNotFound(err) {

		err := r.Client.Create(context.TODO(), u)
		if err != nil {
			// Creation failed
			obLog.Error(err, "Failed to create new instance")
			return ctrl.Result{}, err
		}

		// Creation was successful
		obLog.Info("Created new object")
		condition := NewHubCondition(operatorv1.Progressing, metav1.ConditionTrue, NewComponentReason, "Created new resource")
		SetHubCondition(&m.Status, *condition)
		return ctrl.Result{}, nil

	} else if err != nil {
		// Error that isn't due to the resource not existing
		obLog.Error(err, "Failed to get subscription")
		return ctrl.Result{}, err
	}

	// Validate object based on type
	updated, needsUpdate := subscription.Validate(found, u)
	if needsUpdate {
		obLog.Info("Updating subscription")
		// Update the resource. Skip on unit test
		err = r.Client.Update(context.TODO(), updated)
		if err != nil {
			// Update failed
			obLog.Error(err, "Failed to update object")
			return ctrl.Result{}, err
		}

		// Spec updated - return
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) ensureNoSubscription(m *operatorv1.MultiClusterHub, u *unstructured.Unstructured) (ctrl.Result, error) {
	subLog := r.Log.WithValues("Namespace", u.GetNamespace(), "Name", u.GetName(), "Kind", u.GetKind())
	_, err := r.uninstall(m, u)
	if err != nil {
		subLog.Error(err, "Failed to uninstall subscription")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) ensureUnstructuredResource(m *operatorv1.MultiClusterHub, u *unstructured.Unstructured) (ctrl.Result, error) {
	obLog := r.Log.WithValues("Namespace", u.GetNamespace(), "Name", u.GetName(), "Kind", u.GetKind())

	found := &unstructured.Unstructured{}
	found.SetGroupVersionKind(u.GroupVersionKind())

	// Try to get API group instance
	err := r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      u.GetName(),
		Namespace: u.GetNamespace(),
	}, found)
	if err != nil && errors.IsNotFound(err) {
		// Resource doesn't exist so create it
		err := r.Client.Create(context.TODO(), u)
		if err != nil {
			// Creation failed
			obLog.Error(err, "Failed to create new instance")
			return ctrl.Result{}, err
		}
		// Creation was successful
		obLog.Info("Created new resource")
		condition := NewHubCondition(operatorv1.Progressing, metav1.ConditionTrue, NewComponentReason, "Created new resource")
		SetHubCondition(&m.Status, *condition)
		return ctrl.Result{}, nil

	} else if err != nil {
		// Error that isn't due to the resource not existing
		obLog.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) ensureNamespace(m *operatorv1.MultiClusterHub, ns *corev1.Namespace) (ctrl.Result, error) {
	ctx := context.Background()

	r.Log.Info(fmt.Sprintf("Ensuring namespace: %s", ns.GetName()))

	existingNS := &corev1.Namespace{}
	err := r.Client.Get(ctx, types.NamespacedName{
		Name: ns.GetName(),
	}, existingNS)
	if err != nil && errors.IsNotFound(err) {
		err = r.Client.Create(ctx, ns)
		if err != nil {
			r.Log.Info(fmt.Sprintf("Error creating namespace: %s", err.Error()))
			return ctrl.Result{Requeue: true}, nil
		}
	} else if err != nil {
		r.Log.Info(fmt.Sprintf("error locating namespace: %s. Error: %s", ns.GetName(), err.Error()))
		return ctrl.Result{Requeue: true}, nil
	}

	condition := NewHubCondition(operatorv1.Progressing, metav1.ConditionTrue, NewComponentReason, "Created new resource")
	SetHubCondition(&m.Status, *condition)

	if existingNS.Status.Phase == corev1.NamespaceActive {
		return ctrl.Result{}, nil
	}
	r.Log.Info(fmt.Sprintf("namespace '%s' is not in an active state", ns.GetName()))
	return ctrl.Result{RequeueAfter: resyncPeriod}, nil
}

func (r *MultiClusterHubReconciler) ensureOperatorGroup(m *operatorv1.MultiClusterHub, og *olmv1.OperatorGroup) (ctrl.Result, error) {
	ctx := context.Background()

	r.Log.Info(fmt.Sprintf("Ensuring operator group exists in ns: %s", og.GetNamespace()))

	operatorGroupList := &olmv1.OperatorGroupList{}
	err := r.Client.List(ctx, operatorGroupList, client.InNamespace(og.GetNamespace()))
	if err != nil {
		r.Log.Info(fmt.Sprintf("error listing operatorgroups in ns: %s. Error: %s", og.GetNamespace(), err.Error()))
		return ctrl.Result{Requeue: true}, nil
	}

	if len(operatorGroupList.Items) > 1 {
		r.Log.Error(fmt.Errorf("found more than one operator group in namespace %s", og.GetNamespace()), "fatal error")
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	} else if len(operatorGroupList.Items) == 1 {
		return ctrl.Result{}, nil
	}

	force := true
	err = r.Client.Patch(ctx, og, client.Apply, &client.PatchOptions{Force: &force, FieldManager: "multiclusterhub-operator"})
	if err != nil {
		r.Log.Info(fmt.Sprintf("Error: %s", err.Error()))
		return ctrl.Result{Requeue: true}, nil
	}
	condition := NewHubCondition(operatorv1.Progressing, metav1.ConditionTrue, NewComponentReason, "Created new resource")
	SetHubCondition(&m.Status, *condition)

	existingOperatorGroup := &olmv1.OperatorGroup{}
	err = r.Client.Get(ctx, types.NamespacedName{
		Name: og.GetName(),
	}, existingOperatorGroup)
	if err != nil {
		r.Log.Info(fmt.Sprintf("error locating operatorgroup: %s/%s. Error: %s", og.GetNamespace(), og.GetName(), err.Error()))
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil

}

func (r *MultiClusterHubReconciler) ensureMultiClusterEngineCR(m *operatorv1.MultiClusterHub, mce *mcev1.MultiClusterEngine) (ctrl.Result, error) {
	ctx := context.Background()

	r.Log.Info(fmt.Sprintf("Ensuring Multicluster Engine custom resource: %s", mce.GetName()))
	force := true
	err := r.Client.Patch(ctx, mce, client.Apply, &client.PatchOptions{Force: &force, FieldManager: "multiclusterhub-operator"})
	if err != nil {
		r.Log.Info(fmt.Sprintf("Error: %s", err.Error()))
		return ctrl.Result{Requeue: true}, nil
	}
	condition := NewHubCondition(operatorv1.Progressing, metav1.ConditionTrue, NewComponentReason, "Created new resource")
	SetHubCondition(&m.Status, *condition)

	existingMCE := &mcev1.MultiClusterEngine{}
	err = r.Client.Get(ctx, types.NamespacedName{
		Name: mce.GetName(),
	}, existingMCE)
	if err != nil {
		r.Log.Info(fmt.Sprintf("error locating MCE: %s. Error: %s", mce.GetName(), err.Error()))
		return ctrl.Result{Requeue: true}, nil
	}

	if existingMCE.Status.Phase == mcev1.MultiClusterEnginePhaseAvailable || utils.IsUnitTest() {
		return ctrl.Result{}, nil
	}
	r.Log.Info(fmt.Sprintf("Multiclusterengine: %s is not yet available", mce.GetName()))
	return ctrl.Result{RequeueAfter: resyncPeriod}, nil

}

func (r *MultiClusterHubReconciler) ensureOLMSubscription(m *operatorv1.MultiClusterHub, sub *subv1alpha1.Subscription) (ctrl.Result, error) {
	ctx := context.Background()

	r.Log.Info(fmt.Sprintf("Ensuring OLM %s/%s subscription", sub.GetNamespace(), sub.GetName()))
	force := true

	err := r.Client.Patch(ctx, sub, client.Apply, &client.PatchOptions{Force: &force, FieldManager: "multiclusterhub-operator"})
	if err != nil {
		r.Log.Info(fmt.Sprintf("Error: %s", err.Error()))
		return ctrl.Result{Requeue: true}, nil
	}

	condition := NewHubCondition(operatorv1.Progressing, metav1.ConditionTrue, NewComponentReason, "Created new resource")
	SetHubCondition(&m.Status, *condition)

	existingSub := &subv1alpha1.Subscription{}
	err = r.Client.Get(ctx, types.NamespacedName{
		Name:      sub.GetName(),
		Namespace: sub.GetNamespace(),
	}, existingSub)
	if err != nil {
		r.Log.Info(fmt.Sprintf("error locating subscription: %s/%s. Error: %s", sub.GetNamespace(), sub.GetName(), err.Error()))
		return ctrl.Result{Requeue: true}, nil
	}

	if utils.IsUnitTest() {
		r.Log.Info("Skipping CSV check in unit test mode")
		return ctrl.Result{}, nil
	}

	currentCSV := existingSub.Status.CurrentCSV
	if currentCSV == "" {
		return ctrl.Result{RequeueAfter: resyncPeriod}, fmt.Errorf("CSV not located for subscription: %s/%s", sub.GetNamespace(), sub.GetName())
	}

	existingCSV := &subv1alpha1.ClusterServiceVersion{}
	err = r.Client.Get(ctx, types.NamespacedName{
		Name:      currentCSV,
		Namespace: sub.GetNamespace()}, existingCSV)
	if err != nil {
		r.Log.Info(fmt.Sprintf("CSV not located for subscription: %s/%s", sub.GetNamespace(), sub.GetName()))
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}
	if existingCSV.Status.Phase == "Succeeded" {
		return ctrl.Result{}, nil
	}

	r.Log.Info(fmt.Sprintf("%s/%s CSV is not yet in a successful state", sub.GetNamespace(), sub.GetName()))
	return ctrl.Result{RequeueAfter: resyncPeriod}, nil

}

// copies the imagepullsecret from mch to the newNS namespace
func (r *MultiClusterHubReconciler) ensurePullSecret(m *operatorv1.MultiClusterHub, newNS string) (ctrl.Result, error) {
	if m.Spec.ImagePullSecret == "" {
		// Delete imagepullsecret in MCE namespace if present
		secretList := &corev1.SecretList{}
		err := r.Client.List(
			context.TODO(),
			secretList,
			client.MatchingLabels{
				"installer.name":      m.GetName(),
				"installer.namespace": m.GetNamespace(),
			},
			client.InNamespace(newNS),
		)
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		for i, secret := range secretList.Items {
			r.Log.Info("Deleting imagePullSecret", "Name", secret.Name, "Namespace", secret.Namespace)
			err = r.Client.Delete(context.TODO(), &secretList.Items[i])
			if err != nil {
				r.Log.Error(err, fmt.Sprintf("Error deleting imagepullsecret: %s", secret.GetName()))
				return ctrl.Result{Requeue: true}, err
			}
		}

		return ctrl.Result{}, nil
	}

	pullSecret := &corev1.Secret{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      m.Spec.ImagePullSecret,
		Namespace: m.Namespace,
	}, pullSecret)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	mceSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: pullSecret.APIVersion,
			Kind:       pullSecret.Kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pullSecret.Name,
			Namespace: newNS,
			Labels:    pullSecret.Labels,
		},
		Data: pullSecret.Data,
	}
	mceSecret.SetName(m.Spec.ImagePullSecret)
	mceSecret.SetNamespace(newNS)
	mceSecret.SetLabels(pullSecret.Labels)
	addInstallerLabelSecret(mceSecret, m.Name, m.Namespace)

	force := true
	err = r.Client.Patch(context.TODO(), mceSecret, client.Apply, &client.PatchOptions{Force: &force, FieldManager: "multiclusterhub-operator"})
	if err != nil {
		r.Log.Info(fmt.Sprintf("Error applying pullSecret to mce namespace: %s", err.Error()))
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

// OverrideImagesFromConfigmap ...
func (r *MultiClusterHubReconciler) OverrideImagesFromConfigmap(imageOverrides map[string]string, namespace, configmapName string) (map[string]string, error) {
	r.Log.Info(fmt.Sprintf("Overriding images from configmap: %s/%s", namespace, configmapName))

	configmap := &corev1.ConfigMap{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{
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

func (r *MultiClusterHubReconciler) maintainImageManifestConfigmap(mch *operatorv1.MultiClusterHub) error {
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
	err := r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      configmap.Name,
		Namespace: configmap.Namespace,
	}, configmap)
	if err != nil && errors.IsNotFound(err) {
		// If configmap does not exist, create and return
		configmap.Data = r.CacheSpec.ImageOverrides
		err = r.Client.Create(context.TODO(), configmap)
		if err != nil {
			return err
		}
		return nil
	}

	// If cached image overrides are not equal to the configmap data, update configmap and return
	if !reflect.DeepEqual(configmap.Data, r.CacheSpec.ImageOverrides) {
		configmap.Data = r.CacheSpec.ImageOverrides
		err = r.Client.Update(context.TODO(), configmap)
		if err != nil {
			return err
		}
	}

	return nil
}

// listDeployments gets all deployments in the given namespaces
func (r *MultiClusterHubReconciler) listDeployments(namespaces []string) ([]*appsv1.Deployment, error) {
	var ret []*appsv1.Deployment

	for _, n := range namespaces {
		deployList := &appsv1.DeploymentList{}
		err := r.Client.List(context.TODO(), deployList, client.InNamespace(n))
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
func (r *MultiClusterHubReconciler) listHelmReleases(namespaces []string) ([]*subrelv1.HelmRelease, error) {
	var ret []*subrelv1.HelmRelease

	for _, n := range namespaces {
		hrList := &subrelv1.HelmReleaseList{}
		err := r.Client.List(context.TODO(), hrList, client.InNamespace(n))
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
func (r *MultiClusterHubReconciler) listCustomResources(m *operatorv1.MultiClusterHub) ([]*unstructured.Unstructured, error) {
	var ret []*unstructured.Unstructured
	subConfig := &subv1alpha1.SubscriptionConfig{}
	subConfig, err := r.GetSubConfig()
	if err != nil {
		return nil, err
	}
	mceSub, err := r.GetSubscription(multiclusterengine.Subscription(m, subConfig))
	if err != nil {
		mceSub = nil
	}

	mceCSV, err := r.GetCSVFromSubscription(multiclusterengine.Subscription(m, subConfig))
	if err != nil {
		mceCSV = nil
	}

	mce, err := r.GetMultiClusterEngine(multiclusterengine.MultiClusterEngine(m))
	if err != nil {
		mce = nil
	}

	ret = append(ret, mceSub, mceCSV, mce)
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

// addInstallerLabelSecret adds the installer name and namespace to a secret's labels
// so it can be watched. Returns false if the labels are already present.
func addInstallerLabelSecret(d *corev1.Secret, name string, ns string) bool {
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
func (r *MultiClusterHubReconciler) labelDeployments(hub *operatorv1.MultiClusterHub, dList []*appsv1.Deployment) error {
	for _, d := range dList {
		// Attach installer labels so we can keep our eyes on the deployment
		if addInstallerLabel(d, hub.Name, hub.Namespace) {
			r.Log.Info("Adding installer labels to deployment", "Name", d.Name)
			err := r.Client.Update(context.TODO(), d)
			if err != nil {
				r.Log.Error(err, "Failed to update Deployment", "Name", d.Name)
				return err
			}
		}
	}
	return nil
}

// ensureSubscriptionOperatorIsRunning verifies that the subscription operator that manages helm subscriptions exists and
// is running. This validation is only intended to run during upgrade and when run as an OLM managed deployment
func (r *MultiClusterHubReconciler) ensureSubscriptionOperatorIsRunning(mch *operatorv1.MultiClusterHub, allDeps []*appsv1.Deployment) (ctrl.Result, error) {
	// skip check if not upgrading
	if mch.Status.CurrentVersion == version.Version {
		return ctrl.Result{}, nil
	}

	selfDeployment, exists := getDeploymentByName(allDeps, utils.MCHOperatorName)
	if !exists {
		// Deployment doesn't exist so this is either being run locally or with unit tests
		return ctrl.Result{}, nil
	}

	// skip check if not deployed by OLM
	if !isACMManaged(selfDeployment) {
		return ctrl.Result{}, nil
	}

	subscriptionDeploy, exists := getDeploymentByName(allDeps, utils.SubscriptionOperatorName)
	if !exists {
		err := fmt.Errorf("Standalone subscription deployment not found")
		return ctrl.Result{}, err
	}

	// Check that the owning CSV version of the deployments match
	if selfDeployment.GetLabels() == nil || subscriptionDeploy.GetLabels() == nil {
		r.Log.Info("Missing labels on either MCH operator or subscription operator deployment")
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}
	if selfDeployment.Labels["olm.owner"] != subscriptionDeploy.Labels["olm.owner"] {
		r.Log.Info("OLM owner labels do not match. Requeuing.", "MCH operator label", selfDeployment.Labels["olm.owner"], "Subscription operator label", subscriptionDeploy.Labels["olm.owner"])
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	// Check that the standalone subscription deployment is available
	if successfulDeploy(subscriptionDeploy) {
		return ctrl.Result{}, nil
	} else {
		r.Log.Info("Standalone subscription deployment is not running. Requeuing.")
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}
}

// GetSubscriptionIPInformation retrieves the current subscriptions installplan information for status
func (r *MultiClusterHubReconciler) GetSubscription(sub *subv1alpha1.Subscription) (*unstructured.Unstructured, error) {
	mceSubscription := &subv1alpha1.Subscription{}
	err := r.Client.Get(context.Background(), types.NamespacedName{
		Name:      sub.GetName(),
		Namespace: sub.GetNamespace(),
	}, mceSubscription)
	if err != nil {
		return nil, err
	}
	unstructuredSub, err := runtime.DefaultUnstructuredConverter.ToUnstructured(mceSubscription)
	if err != nil {
		r.Log.Error(err, "Failed to unmarshal subscription")
		return nil, err
	}
	return &unstructured.Unstructured{Object: unstructuredSub}, nil
}

func (r *MultiClusterHubReconciler) GetMultiClusterEngine(mce *mcev1.MultiClusterEngine) (*unstructured.Unstructured, error) {
	multiClusterEngine, err := r.ManagedByMCEExists()
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	// Detect if preexisint MCE exists, if so update MCE spec. Otherwise, install all MCE resources
	if multiClusterEngine == nil {
		multiClusterEngine = &mcev1.MultiClusterEngine{}
		err := r.Client.Get(context.Background(), types.NamespacedName{
			Name: mce.GetName(),
		}, multiClusterEngine)
		if err != nil {
			return nil, err
		}
	}
	unstructuredMCE, err := runtime.DefaultUnstructuredConverter.ToUnstructured(multiClusterEngine)
	if err != nil {
		r.Log.Error(err, "Failed to unmarshal multiclusterengine")
		return nil, err
	}
	return &unstructured.Unstructured{Object: unstructuredMCE}, nil
}

func (r *MultiClusterHubReconciler) ensureMultiClusterEngine(multiClusterHub *operatorv1.MultiClusterHub) (ctrl.Result, error) {

	ctx := context.Background()
	subConfig, err := r.GetSubConfig()
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	// Deletes conflicting components and adds managedby label to MCE if necessary
	result, err := r.prepareForMultiClusterEngineInstall(multiClusterHub)
	if result != (ctrl.Result{}) {
		return result, err
	}

	existingMCE, err := r.ManagedByMCEExists()
	if err != nil {
		return ctrl.Result{}, err
	}

	// Detect if preexisting MCE exists, if so update MCE spec. Otherwise, install all MCE resources
	if existingMCE != nil {
		r.Log.Info("Updating preexisting MCE")
		existingMCE.Spec.AvailabilityConfig = mcev1.AvailabilityType(multiClusterHub.Spec.AvailabilityConfig)
		existingMCE.Spec.NodeSelector = multiClusterHub.Spec.NodeSelector
		if (multiClusterHub.ComponentEnabled(operatorv1.ManagedServiceAccount)){
			existingMCE.Spec.ComponentConfig = multiclusterengine.GetComponentConfig(multiClusterHub)
		}
		existingMCE.Spec.ComponentConfig = multiclusterengine.GetComponentConfig(multiClusterHub)
		if err := r.Client.Update(ctx, existingMCE); err != nil {
			r.Log.Error(err, "Failed to update preexisting MCE with MCH spec")
			return ctrl.Result{}, err
		}
		r.Log.Info("Preexisting MCE successfully updated")
		return ctrl.Result{}, nil
	}

	result, err = r.ensureNamespace(multiClusterHub, multiclusterengine.Namespace())
	if result != (ctrl.Result{}) {
		return result, err
	}

	result, err = r.ensurePullSecret(multiClusterHub, multiclusterengine.Namespace().Name)
	if result != (ctrl.Result{}) {
		return result, err
	}

	result, err = r.ensureOperatorGroup(multiClusterHub, multiclusterengine.OperatorGroup())
	if result != (ctrl.Result{}) {
		return result, err
	}

	if mceAnnotationOverrides := utils.GetMCEAnnotationOverrides(multiClusterHub); mceAnnotationOverrides != "" {
		r.Log.Info(fmt.Sprintf("Overridding MultiClusterEngine Subscription: %s", mceAnnotationOverrides))
	}

	result, err = r.ensureOLMSubscription(multiClusterHub, multiclusterengine.Subscription(multiClusterHub, subConfig))
	if result != (ctrl.Result{}) {
		return result, err
	}

	result, err = r.ensureMultiClusterEngineCR(multiClusterHub, multiclusterengine.MultiClusterEngine(multiClusterHub))
	if result != (ctrl.Result{}) {
		return result, err
	}

	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) prepareForMultiClusterEngineInstall(multiClusterHub *operatorv1.MultiClusterHub) (ctrl.Result, error) {
	ctx := context.Background()

	existingMCEList := &mcev1.MultiClusterEngineList{}
	err := r.Client.List(ctx, existingMCEList)
	if err != nil && !errors.IsNotFound(err) {
		if !strings.Contains(err.Error(), "no matches for kind \"MultiClusterEngine\"") {
			r.Log.Info(fmt.Sprintf("error locating MCE: %s. Error: %s", multiclusterengine.MulticlusterengineName, err.Error()))
			return ctrl.Result{Requeue: true}, nil
		}
	} else if err == nil {
		// Detect status of MCE
		// If 1 exists, add label if preexisting, return if created by us
		// If none exist, ensure conflicting resources are removed
		// if > 1 exists, error
		if len(existingMCEList.Items) == 1 {
			existingMCE := &existingMCEList.Items[0]
			labels := existingMCE.GetLabels()
			if labels == nil {
				labels = make(map[string]string)
			}
			if name, ok := labels["installer.name"]; ok && name == multiClusterHub.GetName() {
				if namespace, ok := labels["installer.namespace"]; ok && namespace == multiClusterHub.GetNamespace() {
					// MCE is installed by the MCH, no need to manage. Return
					return ctrl.Result{}, nil
				}
			}

			if _, ok := labels[utils.MCEManagedByLabel]; ok {
				// MCE is managed by the MCH, return
				return ctrl.Result{}, nil
			}

			r.Log.Info(fmt.Sprintf("Preexisting MCE exists: %s. Managing multiclusterengine resource", existingMCE.GetName()))
			labels[utils.MCEManagedByLabel] = "true"
			existingMCE.SetLabels(labels)

			if !controllerutil.ContainsFinalizer(existingMCE, hubFinalizer) {
				controllerutil.AddFinalizer(existingMCE, hubFinalizer)
			}

			err = r.Client.Update(ctx, existingMCE)
			if err != nil {
				r.Log.Info(fmt.Sprintf("Error: %s", err.Error()))
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{Requeue: true}, nil
		} else if len(existingMCEList.Items) > 1 {
			r.Log.Error(err, "multiple MCEs found. Only one is allowed")
			return ctrl.Result{}, fmt.Errorf("multiple preexisting MCEs found. Only one is allowed")
		}
	}

	if len(existingMCEList.Items) == 0 {
		r.Log.Info("Preparing for MCE installation. Removing existing resources that will be recreated by the MCE")
		result, err := r.ensureConflictingMCEComponentsGone(multiClusterHub)
		if result != (ctrl.Result{}) {
			return result, err
		}
		r.Log.Info("Conflicting resources removed. Proceeding with MCE installation")
	}

	return ctrl.Result{}, nil
}

// GetCSVFromSubscription retrieves CSV status information from the related subscription for status
func (r *MultiClusterHubReconciler) GetCSVFromSubscription(sub *subv1alpha1.Subscription) (*unstructured.Unstructured, error) {
	mceSubscription := &subv1alpha1.Subscription{}
	err := r.Client.Get(context.Background(), types.NamespacedName{
		Name:      sub.GetName(),
		Namespace: sub.GetNamespace(),
	}, mceSubscription)
	if err != nil {
		return nil, err
	}

	currentCSV := mceSubscription.Status.CurrentCSV
	if currentCSV == "" {
		return nil, fmt.Errorf("currentCSV is empty")
	}

	mceCSV := &subv1alpha1.ClusterServiceVersion{}
	err = r.Client.Get(context.Background(), types.NamespacedName{
		Name:      currentCSV,
		Namespace: sub.GetNamespace(),
	}, mceCSV)
	if err != nil {
		return nil, err
	}
	csv, err := runtime.DefaultUnstructuredConverter.ToUnstructured(mceCSV)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: csv}, nil
}

// isACMManaged returns whether this application is managed by OLM via an ACM subscription
func isACMManaged(deploy *appsv1.Deployment) bool {
	labels := deploy.GetLabels()
	if labels == nil {
		return false
	}
	if owner, ok := labels["olm.owner"]; ok {
		if strings.Contains(owner, "advanced-cluster-management") {
			return true
		}
	}
	return false
}

// getDeploymentByName returns the deployment with the matching name found from a list
func getDeploymentByName(allDeps []*appsv1.Deployment, desiredDeploy string) (*appsv1.Deployment, bool) {
	for i := range allDeps {
		if allDeps[i].Name == desiredDeploy {
			return allDeps[i], true
		}
	}
	return nil, false
}

func addProxyEnvVarsToDeployment(dep *appsv1.Deployment) *appsv1.Deployment {
	proxyEnvVars := []corev1.EnvVar{
		{
			Name:  "HTTP_PROXY",
			Value: os.Getenv("HTTP_PROXY"),
		},
		{
			Name:  "HTTPS_PROXY",
			Value: os.Getenv("HTTPS_PROXY"),
		},
		{
			Name:  "NO_PROXY",
			Value: os.Getenv("NO_PROXY"),
		},
	}
	for i := 0; i < len(dep.Spec.Template.Spec.Containers); i++ {
		dep.Spec.Template.Spec.Containers[i].Env = append(dep.Spec.Template.Spec.Containers[i].Env, proxyEnvVars...)
	}

	return dep
}

func addProxyEnvVarsToSub(u *unstructured.Unstructured) *unstructured.Unstructured {
	path := "spec.packageOverrides[].packageOverrides[].value.hubconfig."

	sub, err := injectMapIntoUnstructured(u.Object, path, map[string]interface{}{
		"proxyConfigs": map[string]interface{}{
			"HTTP_PROXY":  os.Getenv("HTTP_PROXY"),
			"HTTPS_PROXY": os.Getenv("HTTPS_PROXY"),
			"NO_PROXY":    os.Getenv("NO_PROXY"),
		},
	})
	if err != nil {
		// log.Info(fmt.Sprintf("Error inject proxy environmental variables into '%s' subscription: %s", u.GetName(), err.Error()))
	}
	u.Object = sub

	return u
}

// injects map into an unstructured objects map based off a given path
func injectMapIntoUnstructured(u map[string]interface{}, path string, mapToInject map[string]interface{}) (map[string]interface{}, error) {

	currentKey := strings.Split(path, ".")[0]
	isArr := false
	if strings.HasSuffix(currentKey, "[]") {
		isArr = true
	}
	currentKey = strings.TrimSuffix(currentKey, "[]")
	if i := strings.Index(path, "."); i >= 0 {
		// Determine remaining path
		path = path[(i + 1):]
		// If Arr, loop through each element of array
		if isArr {
			nextMap, ok := u[currentKey].([]map[string]interface{})
			if !ok {
				return u, fmt.Errorf("Failed to find key: %s", currentKey)
			}
			for i := 0; i < len(nextMap); i++ {
				_, err := injectMapIntoUnstructured(nextMap[i], path, mapToInject)
				if err != nil {
					return u, err
				}
			}
			return u, nil
		} else {
			nextMap, ok := u[currentKey].(map[string]interface{})
			if !ok {
				return u, fmt.Errorf("Failed to find key: %s", currentKey)
			}
			_, err := injectMapIntoUnstructured(nextMap, path, mapToInject)
			if err != nil {
				return u, err
			}
			return u, nil
		}
	} else {
		for key, val := range mapToInject {
			u[key] = val
			break
		}
		return u, nil
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

// mergeErrors combines errors into a single string
func mergeErrors(errs []error) string {
	errStrings := []string{}
	for _, e := range errs {
		errStrings = append(errStrings, e.Error())
	}
	return strings.Join(errStrings, " ; ")
}

func (r *MultiClusterHubReconciler) ManagedByMCEExists() (*mcev1.MultiClusterEngine, error) {
	ctx := context.Background()
	// First check if MCE is preexisting
	mceList := &mcev1.MultiClusterEngineList{}
	err := r.Client.List(ctx, mceList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			utils.MCEManagedByLabel: "true",
		}),
	})
	if err != nil && !errors.IsNotFound(err) {
		if !strings.Contains(err.Error(), "no matches for kind \"MultiClusterEngine\"") {
			r.Log.Info(fmt.Sprintf("error locating MCE: %s. Error: %s", multiclusterengine.MulticlusterengineName, err.Error()))
			return nil, err
		}
	} else if err == nil && len(mceList.Items) == 1 {
		// Preexisting MCE exists, no need to terminate resources
		return &mceList.Items[0], nil
	} else if len(mceList.Items) > 1 {
		return nil, fmt.Errorf("multiple MCEs found. Only one MCE is allowed per cluster")
	}
	// No MCE exists, return
	return nil, nil
}

func (r *MultiClusterHubReconciler) GetSubConfig() (*subv1alpha1.SubscriptionConfig, error) {
	configEnvVars := []corev1.EnvVar{}
	found := &appsv1.Deployment{}

	namespace, err := utils.FindNamespace()
	if err != nil {
		return nil, err
	}

	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      utils.MCHOperatorName,
		Namespace: namespace,
	}, found)

	if err != nil {
		return nil, err
	}

	foundSubscription := &subv1alpha1.Subscription{}

	proxyEnv := []corev1.EnvVar{}
	if utils.ProxyEnvVarsAreSet() {
		proxyEnv = []corev1.EnvVar{
			corev1.EnvVar{
				Name:  "HTTP_PROXY",
				Value: os.Getenv("HTTP_PROXY"),
			},
			corev1.EnvVar{
				Name:  "HTTPS_PROXY",
				Value: os.Getenv("HTTPS_PROXY"),
			},
			corev1.EnvVar{
				Name:  "NO_PROXY",
				Value: os.Getenv("NO_PROXY"),
			},
		}
		err = r.Client.Get(context.TODO(), types.NamespacedName{
			Name:      "multicluster-engine",
			Namespace: utils.MCESubscriptionNamespace,
		}, foundSubscription)
		if err != nil && errors.IsNotFound(err) {
			configEnvVars = proxyEnv
		} else if err != nil {
			return nil, err
		} else {
			configEnvVars = utils.AppendProxyVariables(foundSubscription.Spec.Config.Env, proxyEnv)
		}

	}
	return &subv1alpha1.SubscriptionConfig{
		NodeSelector: found.Spec.Template.Spec.NodeSelector,
		Tolerations:  found.Spec.Template.Spec.Tolerations,
		Env:          configEnvVars,
	}, nil

}
