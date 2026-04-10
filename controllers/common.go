// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	e "errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	consolev1 "github.com/openshift/api/operator/v1"

	"github.com/Masterminds/semver/v3"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"

	configv1 "github.com/openshift/api/config/v1"
	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	searchv2v1alpha1 "github.com/stolostron/search-v2-operator/api/v1alpha1"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	ocmapi "open-cluster-management.io/api/addon/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengineutils"
	renderer "github.com/stolostron/multiclusterhub-operator/pkg/rendering"
	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"

	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengine"
	"github.com/stolostron/multiclusterhub-operator/pkg/version"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CacheSpec ...
type CacheSpec struct {
	IngressDomain       string
	ImageOverrides      map[string]string
	ImageOverridesCM    string
	ImageRepository     string
	ManifestVersion     string
	TemplateOverrides   map[string]string
	TemplateOverridesCM string
}

var migratedComponentDeployments = map[string]string{
	operatorv1.ClusterPermission: "cluster-permission",
}

func (r *MultiClusterHubReconciler) ensureNoNamespace(m *operatorv1.MultiClusterHub, u *unstructured.Unstructured) (ctrl.Result, error) {
	subLog := r.Log.WithValues("Kind", u.GetKind(), "Name", u.GetName())
	gone, err := r.uninstall(m, u)
	if err != nil {
		subLog.Error(err, "Failed to uninstall namespace")
		return ctrl.Result{}, err
	}
	if gone {
		return ctrl.Result{}, nil
	} else {
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}
}

func (r *MultiClusterHubReconciler) ensureUnstructuredResource(m *operatorv1.MultiClusterHub, u *unstructured.Unstructured) (ctrl.Result, error) {
	obLog := r.Log.WithValues("Namespace", u.GetNamespace(), "Kind", u.GetKind(), "Name", u.GetName())

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

func (r *MultiClusterHubReconciler) ensureNamespace(m *operatorv1.MultiClusterHub, ns *corev1.Namespace) (
	ctrl.Result, error) {
	ctx := context.Background()

	existingNS := &corev1.Namespace{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: ns.GetName()}, existingNS); err != nil {
		if errors.IsNotFound(err) {
			if err = r.Client.Create(ctx, ns); err != nil {
				r.Log.Info(fmt.Sprintf("Error creating namespace: %s", err.Error()))
				return ctrl.Result{Requeue: true}, nil
			}
		} else {
			r.Log.Info(fmt.Sprintf("error locating namespace: %s. Error: %s", ns.GetName(), err.Error()))
			return ctrl.Result{Requeue: true}, nil
		}
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

	r.Log.Info(fmt.Sprintf("Ensuring operator group exists in ns: %s", og.GetNamespace()))

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

func (r *MultiClusterHubReconciler) ensureMultiClusterEngineCR(ctx context.Context, m *operatorv1.MultiClusterHub) (ctrl.Result, error) {
	mce, err := multiclusterengine.FindAndManageMCE(ctx, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	if mce == nil {
		mce = multiclusterengine.NewMultiClusterEngine(m)
		err = r.Client.Create(ctx, mce)
		if err != nil {
			return ctrl.Result{Requeue: true}, fmt.Errorf("error creating new MCE: %w", err)
		}
		return ctrl.Result{}, nil
	}

	mceannotations := mce.GetAnnotations()
	if mceannotations == nil {
		mceannotations = map[string]string{}
	}

	mce.SetAnnotations(mceannotations)

	// secret should be delivered to targetNamespace
	if mce.Spec.TargetNamespace == "" {
		return ctrl.Result{Requeue: true}, fmt.Errorf("MCE %s does not have a target namespace to apply pullsecret", mce.Name)
	}
	result, err := r.ensurePullSecret(m, mce.Spec.TargetNamespace)
	if result != (ctrl.Result{}) {
		return result, err
	}

	calcMCE := multiclusterengine.RenderMultiClusterEngine(mce, m)
	err = r.Client.Update(ctx, calcMCE)
	if err != nil {
		return ctrl.Result{Requeue: true}, fmt.Errorf("error updating MCE %s: %w", mce.Name, err)
	}
	return ctrl.Result{}, nil
}

// copies the imagepullsecret from mch to the newNS namespace
func (r *MultiClusterHubReconciler) ensurePullSecret(m *operatorv1.MultiClusterHub, newNS string) (ctrl.Result, error) {
	if m.Spec.ImagePullSecret == "" {
		// Delete imagepullsecret in MCE namespace if present
		secretList := &corev1.SecretList{}
		if err := r.Client.List(context.TODO(), secretList, client.MatchingLabels{"installer.name": m.GetName(),
			"installer.namespace": m.GetNamespace()}, client.InNamespace(newNS)); err != nil {

			return ctrl.Result{}, err
		}

		for i, secret := range secretList.Items {
			r.Log.Info("Deleting imagePullSecret", "Name", secret.Name, "Namespace", secret.Namespace)
			if err := r.Client.Delete(context.TODO(), &secretList.Items[i]); err != nil {
				r.Log.Error(err, fmt.Sprintf("Error deleting imagepullsecret: %s", secret.GetName()))
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	pullSecret := &corev1.Secret{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: m.Spec.ImagePullSecret, Namespace: m.Namespace},
		pullSecret); err != nil {
		return ctrl.Result{}, err
	}

	mceSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pullSecret.Name,
			Namespace: newNS,
			Labels:    pullSecret.Labels,
		},
		Data: pullSecret.Data,
		Type: corev1.SecretTypeDockerConfigJson,
	}
	addInstallerLabelSecret(mceSecret, m.Name, m.Namespace)

	force := true
	if err := r.Client.Patch(context.TODO(), mceSecret, client.Apply,
		&client.PatchOptions{Force: &force, FieldManager: "multiclusterhub-operator"}); err != nil {
		r.Log.Info(fmt.Sprintf("Error applying pullSecret to mce namespace: %s", err.Error()))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// checks if imagepullsecret was created in mch namespace
func (r *MultiClusterHubReconciler) ensurePullSecretCreated(m *operatorv1.MultiClusterHub, namespace string) (ctrl.Result, error) {
	if m.Spec.ImagePullSecret == "" {
		// No imagepullsecret set, continuing
		return ctrl.Result{}, nil
	}

	pullSecret := &corev1.Secret{}

	err := r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      m.Spec.ImagePullSecret,
		Namespace: m.Namespace,
	}, pullSecret)
	if err != nil {
		return ctrl.Result{}, err
	}
	if pullSecret.Namespace == "" || pullSecret.Namespace != namespace {
		return ctrl.Result{}, fmt.Errorf("pullsecret doest not exist in namespace: %s", namespace)
	}

	return ctrl.Result{}, nil
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

// listCustomResources gets custom resources the installer observes
func (r *MultiClusterHubReconciler) listCustomResources(m *operatorv1.MultiClusterHub) (map[string]*unstructured.Unstructured, error) {
	ret := make(map[string]*unstructured.Unstructured)

	var mceSub *unstructured.Unstructured
	gotSub, err := multiclusterengine.GetManagedMCESubscription(context.Background(), r.Client)
	if err != nil || gotSub == nil {
		mceSub = nil
	} else {
		unstructuredSub, err := runtime.DefaultUnstructuredConverter.ToUnstructured(gotSub)
		if err != nil {
			r.Log.Error(err, "Failed to unmarshal subscription")
		}
		mceSub = &unstructured.Unstructured{Object: unstructuredSub}
	}

	var mceCSV *unstructured.Unstructured
	if gotSub == nil {
		mceCSV = nil
	} else {
		mceCSV, err = r.GetCSVFromSubscription(gotSub)
		if err != nil {
			mceCSV = nil
		}
	}

	var mce *unstructured.Unstructured
	gotMCE, err := multiclusterengineutils.GetManagedMCE(context.Background(), r.Client)
	if err != nil || gotMCE == nil {
		mce = nil
	} else {
		unstructuredMCE, err := runtime.DefaultUnstructuredConverter.ToUnstructured(gotMCE)
		if err != nil {
			r.Log.Error(err, "Failed to unmarshal subscription")
		}
		mce = &unstructured.Unstructured{Object: unstructuredMCE}
	}

	ret["mce-sub"] = mceSub
	ret["mce-csv"] = mceCSV
	ret["mce"] = mce
	return ret, nil
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

// ensureMCESubscription verifies resources needed for MCE are created
func (r *MultiClusterHubReconciler) ensureMCESubscription(ctx context.Context, multiClusterHub *operatorv1.MultiClusterHub) (ctrl.Result, error) {
	mceSub, err := multiclusterengine.FindAndManageMCESubscription(ctx, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Get sub config, catalogsource, and annotation overrides
	subConfig, err := r.GetSubConfig()
	if err != nil {
		return ctrl.Result{}, err
	}
	overrides, err := multiclusterengine.GetAnnotationOverrides(multiClusterHub)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Get InstallPlan approval from MCH operator subscription
	var installPlanApproval subv1alpha1.Approval = subv1alpha1.ApprovalAutomatic
	mchOperatorSub, err := r.FindMultiClusterHubOperatorSubscription(ctx)
	if err != nil {
		r.Log.Info("Unable to find MultiClusterHub operator subscription, defaulting to automatic InstallPlan approval", "error", err)
	} else {
		installPlanApproval = r.GetInstallPlanApprovalFromSubscription(mchOperatorSub)
		r.Log.Info("Using InstallPlan approval from MCH operator subscription", "approval", installPlanApproval)
	}

	// Apply InstallPlan approval to overrides if not already set
	if overrides == nil {
		overrides = &subv1alpha1.SubscriptionSpec{}
	}
	if overrides.InstallPlanApproval == "" {
		overrides.InstallPlanApproval = installPlanApproval
	}
	ctlSrc := types.NamespacedName{}
	// Search for catalogsource if not defined in overrides
	if overrides == nil || overrides.CatalogSource == "" {
		ctlSrc, err = multiclusterengine.GetCatalogSource(r.Client)
		if err != nil {
			r.Log.Info("Failed to find a suitable catalogsource.", "error", err)
			return ctrl.Result{}, err
		}
	}

	createSub := false
	if mceSub == nil {
		result, err := r.ensureNamespace(multiClusterHub, multiclusterengine.Namespace())
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
		// Sub is nil so create a new one
		mceSub = multiclusterengine.NewSubscription(multiClusterHub, subConfig, overrides, utils.IsCommunityMode())
		createSub = true
	} else if multiclusterengine.CreatedByMCH(mceSub, multiClusterHub) {
		result, err := r.ensurePullSecret(multiClusterHub, multiclusterengine.Namespace().Name)
		if result != (ctrl.Result{}) {
			return result, err
		}
		result, err = r.ensureOperatorGroup(multiClusterHub, multiclusterengine.OperatorGroup())
		if result != (ctrl.Result{}) {
			return result, err
		}
	}

	// Apply MCE sub
	calcSub := multiclusterengine.RenderSubscription(mceSub, subConfig, overrides, ctlSrc, utils.IsCommunityMode())
	if createSub {
		err = r.Client.Create(ctx, calcSub)
	} else {
		err = r.Client.Update(ctx, calcSub)
	}
	if err != nil {
		return ctrl.Result{Requeue: true}, fmt.Errorf("error updating subscription %s: %w", calcSub.Name, err)
	}

	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) ensureMultiClusterEngine(ctx context.Context, multiClusterHub *operatorv1.MultiClusterHub) (ctrl.Result, error) {
	// confirm subscription and reqs exist and are configured correctly
	result, err := r.ensureMCESubscription(ctx, multiClusterHub)
	if result != (ctrl.Result{}) {
		return result, err
	}

	result, err = r.ensureMultiClusterEngineCR(ctx, multiClusterHub)
	if result != (ctrl.Result{}) {
		return result, err
	}

	return ctrl.Result{}, nil
}

// waitForMCE checks that MCE is in a running state and at the expected version.
func (r *MultiClusterHubReconciler) waitForMCEReady(ctx context.Context) (ctrl.Result, error) {
	// Wait for MCE to be ready
	existingMCE, err := multiclusterengineutils.GetManagedMCE(ctx, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	if existingMCE == nil {
		r.Log.Info("Multiclusterengine is not yet present")
		return ctrl.Result{Requeue: true}, nil
	}
	if utils.IsUnitTest() {
		return ctrl.Result{}, nil
	}

	if existingMCE.Status.CurrentVersion == "" {
		r.Log.Info(fmt.Sprintf("Multiclusterengine: %s is not yet available", existingMCE.GetName()))
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}

	// MCE version depends on mode
	if utils.IsCommunityMode() {
		err = version.ValidCommunityMCEVersion(existingMCE.Status.CurrentVersion)
	} else {
		err = version.ValidMCEVersion(existingMCE.Status.CurrentVersion)
	}
	if err != nil {
		r.Log.Info("Waiting for MCE upgrade to complete", "CurrentVersion", existingMCE.Status.CurrentVersion, "Reason", err.Error())
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}
	r.Log.Info("MCE is ready", "Version", existingMCE.Status.CurrentVersion)
	return ctrl.Result{}, nil
}

// GetCSVFromSubscription retrieves CSV status information from the related subscription for status
func (r *MultiClusterHubReconciler) GetCSVFromSubscription(sub *subv1alpha1.Subscription) (*unstructured.Unstructured, error) {
	if sub == nil {
		return nil, fmt.Errorf("cannot find CSV from nil Subscription")
	}
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

	csv := &unstructured.Unstructured{}
	if err := r.Scheme.Convert(mceCSV, csv, nil); err != nil {
		return nil, err
	}

	return csv, nil
}

// mergeErrors combines errors into a single string
func mergeErrors(errs []error) string {
	errStrings := []string{}
	for _, e := range errs {
		errStrings = append(errStrings, e.Error())
	}
	return strings.Join(errStrings, " ; ")
}

// GetSubConfig returns a SubscriptionConfig based on proxy variables and the mch operator configuration
func (r *MultiClusterHubReconciler) GetSubConfig() (*subv1alpha1.SubscriptionConfig, error) {
	found := &appsv1.Deployment{}
	mchOperatorNS, err := utils.OperatorNamespace()
	if err != nil {
		return nil, err
	}

	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      utils.MCHOperatorName,
		Namespace: mchOperatorNS,
	}, found)
	if err != nil {
		return nil, err
	}

	proxyEnv := []corev1.EnvVar{}
	if utils.ProxyEnvVarsAreSet() {
		proxyEnv = []corev1.EnvVar{
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
	}
	return &subv1alpha1.SubscriptionConfig{
		NodeSelector: found.Spec.Template.Spec.NodeSelector,
		Tolerations:  found.Spec.Template.Spec.Tolerations,
		Env:          proxyEnv,
	}, nil
}

func (r *MultiClusterHubReconciler) addPluginToConsole(multiClusterHub *operatorv1.MultiClusterHub) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log
	console := &consolev1.Console{}
	// If trying to check this resource from the CLI run - `oc get consoles.operator.openshift.io cluster`.
	// The default `console` is not the correct resource
	err := r.Client.Get(ctx, types.NamespacedName{Name: "cluster"}, console)
	if err != nil {
		log.Info("Failed to find console: cluster")
		return ctrl.Result{}, err
	}

	if console.Spec.Plugins == nil {
		console.Spec.Plugins = []string{}
	}

	// Add acm to the plugins list if it is not already there
	if !utils.Contains(console.Spec.Plugins, "acm") {
		log.Info("Ready to add plugin")
		console.Spec.Plugins = append(console.Spec.Plugins, "acm")
		err = r.Client.Update(ctx, console)
		if err != nil {
			log.Info("Failed to add acm consoleplugin to console")
			return ctrl.Result{}, err
		} else {
			log.Info("Added acm consoleplugin to console")
		}
	}

	return ctrl.Result{}, nil
}

// removePluginFromConsoleResource ...
func (r *MultiClusterHubReconciler) removePluginFromConsole(multiClusterHub *operatorv1.MultiClusterHub) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log
	console := &consolev1.Console{}
	// If trying to check this resource from the CLI run - `oc get consoles.operator.openshift.io cluster`.
	// The default `console` is not the correct resource
	err := r.Client.Get(ctx, types.NamespacedName{Name: "cluster"}, console)
	if err != nil {
		log.Info("Failed to find console: cluster")
		return ctrl.Result{}, err
	}

	// If No plugins, it is already removed
	if console.Spec.Plugins == nil {
		return ctrl.Result{}, nil
	}

	// Remove mce to the plugins list if it is not already there
	if utils.Contains(console.Spec.Plugins, "acm") {
		console.Spec.Plugins = utils.RemoveString(console.Spec.Plugins, "acm")
		err = r.Client.Update(ctx, console)
		if err != nil {
			log.Info("Failed to remove acm consoleplugin to console")
			return ctrl.Result{}, err
		} else {
			log.Info("Removed acm consoleplugin to console")
		}
	}

	return ctrl.Result{}, nil
}

// return current OCP version from clusterversion resource
// equivalent to `oc get clusterversion version -o=jsonpath='{.status.history[0].version}'`
func (r *MultiClusterHubReconciler) getClusterVersion(ctx context.Context) (string, error) {
	if utils.IsUnitTest() {
		// If unit test pass along a version, Can't set status in unit test
		return "4.99.99", nil
	}

	clusterVersion := &configv1.ClusterVersion{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: "version"}, clusterVersion)
	if err != nil {
		return "", err
	}

	if len(clusterVersion.Status.History) == 0 {
		return "", e.New("failed to detect status in clusterversion.status.history")
	}
	return clusterVersion.Status.History[0].Version, nil
}

func (r *MultiClusterHubReconciler) ensureSearchCR(m *operatorv1.MultiClusterHub) (ctrl.Result, error) {
	ctx := context.Background()

	searchCR := &searchv2v1alpha1.Search{
		TypeMeta: metav1.TypeMeta{
			APIVersion: searchv2v1alpha1.GroupVersion.String(),
			Kind:       "Search",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "search-v2-operator",
			Namespace: m.Namespace,
			Labels:    map[string]string{"cluster.open-cluster-management.io/backup": ""},
			Annotations: map[string]string{
				utils.AnnotationFineGrainedRbac: strconv.FormatBool(
					m.Enabled(operatorv1.FineGrainedRbac)),
			},
		},
		Spec: searchv2v1alpha1.SearchSpec{
			NodeSelector: m.Spec.NodeSelector,
			Tolerations:  utils.GetTolerations(m),
		},
	}

	force := true
	err := r.Client.Patch(ctx, searchCR, client.Apply, &client.PatchOptions{Force: &force, FieldManager: "multiclusterhub-operator"})
	if err != nil {
		r.Log.Info(fmt.Sprintf("error applying Search CR. Error: %s", err.Error()))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) ensureNoClusterManagementAddOn(m *operatorv1.MultiClusterHub, component string) (
	ctrl.Result, error,
) {
	ctx := context.Background()

	addonName, err := operatorv1.GetClusterManagementAddonName(component)
	if err != nil {
		r.Log.Info(fmt.Sprintf("Detected unregistered ClusterManagementAddon component: %s", err.Error()))
		return ctrl.Result{}, err
	}

	// if CRD doesn't exist, return
	cmaCRD := &apixv1.CustomResourceDefinition{}
	err = r.Client.Get(context.Background(), types.NamespacedName{Name: "clustermanagementaddons.addon.open-cluster-management.io"}, cmaCRD)
	if errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}

	clusterMgmtAddon := &ocmapi.ClusterManagementAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name: addonName,
		},
	}

	foreground := metav1.DeletePropagationForeground
	err = r.Client.Delete(context.TODO(), clusterMgmtAddon, &client.DeleteOptions{
		PropagationPolicy: &foreground,
	})

	if errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}

	if err != nil {
		r.Log.Error(err, "Error deleting ClusterManagementAddOn CR")

		return ctrl.Result{}, err
	}

	r.Log.Info("Deleting the ClusterManagementAddOn CR", "name", addonName)

	err = r.Client.Get(ctx, types.NamespacedName{Name: clusterMgmtAddon.GetName()}, clusterMgmtAddon)
	if errors.IsNotFound(err) {
		r.Log.Info("Successfully deleted the ClusterManagementAddOn CR", "name", addonName)

		return ctrl.Result{}, nil
	}

	if err != nil {
		r.Log.Error(err, "Failed to get the ClusterManagementAddOn CR", "name", addonName)

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, errors.NewBadRequest("ClusterManagementAddOn CR has not been deleted")
}

func (r *MultiClusterHubReconciler) ensureNoSearchCR(m *operatorv1.MultiClusterHub) (ctrl.Result, error) {
	ctx := context.Background()

	searchList := &searchv2v1alpha1.SearchList{}
	err := r.Client.List(ctx, searchList, client.InNamespace(m.GetNamespace()))
	if err != nil {
		r.Log.Info(fmt.Sprintf("error locating Search CR. Error: %s", err.Error()))
		return ctrl.Result{}, err
	}

	if len(searchList.Items) != 0 {
		err = r.Client.Delete(context.TODO(), &searchList.Items[0])
		if err != nil {
			r.Log.Error(err, "Error deleting Search CR")
			return ctrl.Result{}, err
		}

	}
	err = r.Client.List(ctx, searchList, client.InNamespace(m.GetNamespace()))
	if err != nil {
		r.Log.Info(fmt.Sprintf("error locating Search CR. Error: %s", err.Error()))
		return ctrl.Result{}, err
	}
	if len(searchList.Items) != 0 {
		r.Log.Info("Waiting for Search CR to be deleted")
		return ctrl.Result{}, errors.NewBadRequest("Search CR has not been deleted")
	}
	return ctrl.Result{}, nil
}

// Checks if OCP Console is enabled and return true if so. If <OCP v4.12, always return true
// Otherwise check in the EnabledCapabilities spec for OCP console
func (r *MultiClusterHubReconciler) CheckConsole(ctx context.Context) (bool, error) {
	versionStatus := &configv1.ClusterVersion{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: "version"}, versionStatus)
	if err != nil {
		return false, err
	}
	ocpVersion, err := r.getClusterVersion(ctx)
	if err != nil {
		return false, err
	}
	if hubOCPVersion, ok := os.LookupEnv("ACM_HUB_OCP_VERSION"); ok {
		ocpVersion = hubOCPVersion
	}
	semverVersion, err := semver.NewVersion(ocpVersion)
	if err != nil {
		return false, fmt.Errorf("failed to convert ocp version to semver compatible value: %w", err)
	}
	// -0 allows for prerelease builds to pass the validation.
	// If -0 is removed, developer/rc builds will not pass this check
	// OCP Console can only be disabled in OCP 4.12+
	constraint, err := semver.NewConstraint(">= 4.12.0-0")
	if err != nil {
		return false, fmt.Errorf("failed to set ocp version constraint: %w", err)
	}
	if !constraint.Check(semverVersion) {
		return true, nil
	}
	if utils.IsUnitTest() {
		return true, nil
	}
	for _, v := range versionStatus.Status.Capabilities.EnabledCapabilities {
		if v == "Console" {
			return true, nil
		}
	}
	return false, nil
}

// FindMultiClusterHubOperatorSubscription finds the subscription that manages the multiclusterhub-operator
func (r *MultiClusterHubReconciler) FindMultiClusterHubOperatorSubscription(ctx context.Context) (*subv1alpha1.Subscription, error) {
	operatorNS, err := utils.OperatorNamespace()
	if err != nil {
		return nil, fmt.Errorf("failed to get operator namespace: %w", err)
	}

	// List all subscriptions in the operator namespace
	subList := &subv1alpha1.SubscriptionList{}
	err = r.Client.List(ctx, subList, client.InNamespace(operatorNS))
	if err != nil {
		return nil, fmt.Errorf("failed to list subscriptions in namespace %s: %w", operatorNS, err)
	}

	// Look for subscription that manages the multiclusterhub-operator
	for _, sub := range subList.Items {
		// Check if this subscription's CSV manages our operator deployment
		if sub.Status.CurrentCSV != "" {
			csv := &subv1alpha1.ClusterServiceVersion{}
			err = r.Client.Get(ctx, types.NamespacedName{
				Name:      sub.Status.CurrentCSV,
				Namespace: operatorNS,
			}, csv)
			if err != nil {
				continue // Skip if CSV not found
			}

			// Check if this CSV manages the multiclusterhub-operator deployment
			if csv.Spec.InstallStrategy.StrategyName == "deployment" {
				for _, deploymentSpec := range csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs {
					if deploymentSpec.Name == utils.MCHOperatorName {
						return &sub, nil
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("no subscription found that manages deployment %s in namespace %s", utils.MCHOperatorName, operatorNS)
}

// GetInstallPlanApprovalFromSubscription returns the InstallPlanApproval setting from a subscription
func (r *MultiClusterHubReconciler) GetInstallPlanApprovalFromSubscription(sub *subv1alpha1.Subscription) subv1alpha1.Approval {
	if sub == nil || sub.Spec == nil {
		return subv1alpha1.ApprovalAutomatic // Default fallback
	}
	return sub.Spec.InstallPlanApproval
}

/*
waitForMigratedComponentsAdopted verifies that MCE has successfully adopted and deployed
all components that have been migrated from MCH to MCE. This prevents a migration gap
where we delete MCH-owned resources before MCE has finished deploying its version.
*/
func (r *MultiClusterHubReconciler) waitForMigratedComponentsAdopted(ctx context.Context,
	m *operatorv1.MultiClusterHub) (bool, error) {

	// List of components migrated from MCH to MCE
	migratedComponents := migratedComponentDeployments

	// Get the managed MCE instance
	mce, err := multiclusterengineutils.GetManagedMCE(ctx, r.Client)
	if err != nil {
		return false, fmt.Errorf("failed to get managed MCE: %w", err)
	}
	if mce == nil {
		r.Log.Info("MCE not found, waiting for MCE to be created")
		return false, nil
	}

	// Check each migrated component to ensure its deployment is available in MCE
	for mchComponent, mceDeploymentName := range migratedComponents {
		// Skip if this component isn't enabled in MCH (nothing to adopt before cleanup)
		if !m.ComponentPresent(mchComponent) || !m.Enabled(mchComponent) {
			continue
		}

		// Look for the deployment in MCE status and verify it's available
		deploymentAvailable := false
		for _, comp := range mce.Status.Components {
			// Check for deployment with matching name, type Available, and status True
			if comp.Kind == "Deployment" &&
				comp.Name == mceDeploymentName &&
				comp.Type == "Available" &&
				comp.Status == "True" {
				deploymentAvailable = true
				r.Log.Info("Migrated component deployment is available in MCE",
					"MCHComponent", mchComponent,
					"MCEDeployment", mceDeploymentName,
					"Reason", comp.Reason)
				break
			}
		}

		if !deploymentAvailable {
			r.Log.Info("Waiting for migrated component deployment to be available in MCE",
				"MCHComponent", mchComponent,
				"MCEDeployment", mceDeploymentName)
			return false, nil
		}
	}

	return true, nil
}

// transferClusterResourcesToMCE relabels all cluster-scoped resources from MCH to MCE ownership.
// MCE will then update the resources to match its desired state.
func (r *MultiClusterHubReconciler) transferClusterResourcesToMCE(ctx context.Context, m *operatorv1.MultiClusterHub,
	component string, cachespec CacheSpec, isSTSEnabled bool) (ctrl.Result, error) {

	mce, err := multiclusterengineutils.GetManagedMCE(ctx, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	if mce == nil {
		r.Log.Info("MCE not found, cannot transfer cluster resources", "Component", component)
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}

	chartLocation := r.fetchChartLocation(component)
	templates, errs := renderer.RenderChart(chartLocation, m, cachespec.ImageOverrides, cachespec.TemplateOverrides, isSTSEnabled)
	if len(errs) > 0 {
		for _, err := range errs {
			r.Log.Info(fmt.Sprintf("Error rendering chart for resource transfer: %s", err.Error()), "Component", component)
		}
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}

	for _, template := range templates {
		// Only process cluster-scoped resources (no namespace)
		if template.GetNamespace() != "" {
			continue
		}

		kind := template.GetKind()
		name := template.GetName()
		existing := template.DeepCopy()
		err := r.Client.Get(ctx, types.NamespacedName{Name: name}, existing)
		if err != nil {
			if errors.IsNotFound(err) {
				r.Log.Info("Resource not found, skipping transfer", "Kind", kind, "Name", name)
				continue
			}
			return ctrl.Result{}, err
		}

		// Skip if already transferred to the current MCE
		labels := existing.GetLabels()
		if labels != nil && labels["backplaneconfig.name"] == mce.GetName() {
			r.Log.Info("Resource already transferred to current MCE, skipping",
				"Kind", kind,
				"Name", name,
				"MCEName", mce.GetName())
			continue
		}

		// Only relabel resources owned by this MCH or with stale MCE labels
		if labels == nil {
			labels = make(map[string]string)
		}
		mchOwned := labels["installer.name"] == m.GetName() && labels["installer.namespace"] == m.GetNamespace()
		staleMCELabel := labels["backplaneconfig.name"] != "" && labels["backplaneconfig.name"] != mce.GetName()

		if !mchOwned && !staleMCELabel {
			r.Log.Info("Resource not owned by this MCH, skipping transfer",
				"Kind", kind,
				"Name", name,
				"InstallerName", labels["installer.name"],
				"InstallerNamespace", labels["installer.namespace"],
				"BackplaneconfigName", labels["backplaneconfig.name"])
			continue
		}

		// Relabel: remove MCH labels, add MCE labels
		delete(labels, "installer.name")
		delete(labels, "installer.namespace")
		labels["backplaneconfig.name"] = mce.GetName()
		existing.SetLabels(labels)

		// Remove MCH annotations
		annotations := existing.GetAnnotations()
		if annotations != nil {
			for key := range annotations {
				if strings.HasPrefix(key, "installer.open-cluster-management.io/") {
					delete(annotations, key)
				}
			}
			existing.SetAnnotations(annotations)
		}

		if err := r.Client.Update(ctx, existing); err != nil {
			r.Log.Info("Failed to transfer resource to MCE", "Kind", kind, "Name", name, "Error", err)
			return ctrl.Result{}, err
		}

		r.Log.Info("Transferred resource to MCE ownership", "Kind", kind, "Name", name)
	}

	return ctrl.Result{}, nil
}

// deleteNamespaceScopedResources deletes namespace-scoped resources (Deployment, ServiceAccount, etc.) for a component.
// This is used after MCE has adopted cluster-scoped RBAC resources, to clean up remaining MCH resources.
func (r *MultiClusterHubReconciler) deleteNamespaceScopedResources(ctx context.Context, m *operatorv1.MultiClusterHub,
	component string, cachespec CacheSpec, isSTSEnabled bool) (ctrl.Result, error) {

	return r.deleteResourcesByScope(ctx, m, component, cachespec, isSTSEnabled, false)
}

// deleteResourcesByScope deletes resources for a component based on scope.
// If deleteClusterScoped is true, deletes only cluster-scoped RBAC resources (ClusterRole, ClusterRoleBinding).
// If deleteClusterScoped is false, deletes only namespace-scoped resources (Deployment, ServiceAccount, etc.).
// Note: For migrated components, cluster-scoped resources are now transferred (not deleted) via
// transferClusterResourcesToMCE, so this function is primarily used for namespace-scoped cleanup.
func (r *MultiClusterHubReconciler) deleteResourcesByScope(ctx context.Context, m *operatorv1.MultiClusterHub,
	component string, cachespec CacheSpec, isSTSEnabled bool, deleteClusterScoped bool) (ctrl.Result, error) {

	// Get chart location for this component
	chartLocation := r.fetchChartLocation(component)

	// Render templates to get all resources for this component
	templates, errs := renderer.RenderChart(chartLocation, m, cachespec.ImageOverrides, cachespec.TemplateOverrides, isSTSEnabled)
	if len(errs) > 0 {
		for _, err := range errs {
			r.Log.Info(fmt.Sprintf("Error rendering chart for resource deletion: %s", err.Error()), "Component", component)
		}
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}

	// Track if any resources are still present or terminating
	resourcesRemaining := false

	// Delete resources based on scope
	for _, template := range templates {
		isNamespaceScoped := template.GetNamespace() != ""

		// Skip based on what we're deleting
		if deleteClusterScoped && isNamespaceScoped {
			continue // We want cluster-scoped, skip namespace-scoped
		}
		if !deleteClusterScoped && !isNamespaceScoped {
			continue // We want namespace-scoped, skip cluster-scoped
		}

		// If deleting cluster-scoped, only delete RBAC resources
		if deleteClusterScoped {
			kind := template.GetKind()
			if kind != "ClusterRole" && kind != "ClusterRoleBinding" {
				continue
			}
		}

		// Try to get existing resource
		existing := template.DeepCopy()
		err := r.Client.Get(ctx, types.NamespacedName{Name: existing.GetName(), Namespace: existing.GetNamespace()}, existing)
		if err != nil {
			if errors.IsNotFound(err) {
				r.Log.V(1).Info("Resource already deleted",
					"Kind", template.GetKind(),
					"Name", template.GetName(),
					"Namespace", template.GetNamespace())
				continue
			}
			return ctrl.Result{}, fmt.Errorf("failed to get resource %s/%s: %w", template.GetKind(), template.GetName(), err)
		}

		// Skip cluster-scoped resources that have been transferred to MCE ownership
		if deleteClusterScoped {
			labels := existing.GetLabels()
			if labels != nil && labels["backplaneconfig.name"] != "" {
				r.Log.Info("Skipping resource with MCE ownership",
					"Kind", existing.GetKind(),
					"Name", existing.GetName(),
					"MCEName", labels["backplaneconfig.name"])
				continue
			}
		}

		// Check if resource is already being deleted
		if existing.GetDeletionTimestamp() != nil {
			r.Log.Info("Resource is terminating",
				"Kind", existing.GetKind(),
				"Name", existing.GetName(),
				"Namespace", existing.GetNamespace())
			resourcesRemaining = true
			continue
		}

		r.Log.Info("Deleting resource",
			"Kind", existing.GetKind(),
			"Name", existing.GetName(),
			"Namespace", existing.GetNamespace(),
			"Component", component)

		// Delete the resource
		if err := r.Client.Delete(ctx, existing); err != nil {
			if errors.IsNotFound(err) {
				// Already deleted between Get and Delete
				continue
			}
			return ctrl.Result{}, fmt.Errorf("failed to delete resource %s/%s: %w",
				existing.GetKind(), existing.GetName(), err)
		}
		resourcesRemaining = true
	}

	// Requeue if resources are still present or terminating
	if resourcesRemaining {
		r.Log.Info("Waiting for resources to finish deleting", "Component", component)
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}

	return ctrl.Result{}, nil
}

/*
ensureMigratedComponentsCleanup handles cleanup of namespace-scoped resources for components migrated
from MCH to MCE. This runs AFTER:
1. Cluster-scoped resources have been relabeled with MCE ownership
2. MCE has adopted those resources and shows the component as Available

This function deletes namespace-scoped resources (Deployment, ServiceAccount, etc.) from the MCH namespace
and prunes the component from the MCH CR.
*/
func (r *MultiClusterHubReconciler) ensureMigratedComponentsCleanup(ctx context.Context, m *operatorv1.MultiClusterHub,
	isSTSEnabled bool) (ctrl.Result, error) {

	updated := false
	for component := range migratedComponentDeployments {
		if m.ComponentPresent(component) {
			r.Log.Info("Cleaning up migrated component namespace-scoped resources", "Component", component)

			// Delete namespace-scoped resources (Deployment, ServiceAccount, etc.)
			// Note: Cluster-scoped resources were relabeled earlier and adopted by MCE
			result, err := r.deleteNamespaceScopedResources(ctx, m, component, r.CacheSpec, isSTSEnabled)
			if result != (ctrl.Result{}) || err != nil {
				return result, err
			}

			// Prune from MCH CR after successful cleanup
			if m.Prune(component) {
				r.Log.Info("Pruned migrated component from MCH CR", "Component", component)
				updated = true
			}
		}
	}

	// Update MCH CR if any components were pruned
	if updated {
		if err := r.Client.Update(ctx, m); err != nil {
			r.Log.Error(err, "Failed to update MCH CR after pruning migrated components")
			return ctrl.Result{}, err
		}
		r.Log.Info("Successfully updated MCH CR after pruning migrated components")
	}

	return ctrl.Result{}, nil
}
