// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package utils

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/ghodss/yaml"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (

	// KubeClient ...
	KubeClient = NewKubeClient("", "", "")
	// DynamicKubeClient ...
	DynamicKubeClient = NewKubeClientDynamic("", "", "")

	// ImageOverridesCMBadImageName ...
	ImageOverridesCMBadImageName = "bad-image-ref"

	// GVRCustomResourceDefinition ...
	GVRCustomResourceDefinition = schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}

	// GVRClusterManager ...
	GVRClusterManager = schema.GroupVersionResource{
		Group:    "operator.open-cluster-management.io",
		Version:  "v1",
		Resource: "clustermanagers",
	}

	// GVRObservability ...
	GVRObservability = schema.GroupVersionResource{
		Group:    "observability.open-cluster-management.io",
		Version:  "v1beta2",
		Resource: "multiclusterobservabilities",
	}

	// GVRMultiClusterEngine ...
	GVRMultiClusterEngine = schema.GroupVersionResource{
		Group:    "multicluster.openshift.io",
		Version:  "v1",
		Resource: "multiclusterengines",
	}

	// GVRMultiClusterHub ...
	GVRMultiClusterHub = schema.GroupVersionResource{
		Group:    "operator.open-cluster-management.io",
		Version:  "v1",
		Resource: "multiclusterhubs",
	}

	// GVRAppSub ...
	GVRAppSub = schema.GroupVersionResource{
		Group:    "apps.open-cluster-management.io",
		Version:  "v1",
		Resource: "subscriptions",
	}

	// GVRHiveConfig ...
	GVRHiveConfig = schema.GroupVersionResource{
		Group:    "hive.openshift.io",
		Version:  "v1",
		Resource: "hiveconfigs",
	}
	// GVRNamespace ...
	GVRNamespace = schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "namespaces",
	}
	// GVRSub ...
	GVRSub = schema.GroupVersionResource{
		Group:    "operators.coreos.com",
		Version:  "v1alpha1",
		Resource: "subscriptions",
	}
	// GVROperatorGroup ...
	GVROperatorGroup = schema.GroupVersionResource{
		Group:    "operators.coreos.com",
		Version:  "v1",
		Resource: "operatorgroups",
	}
	// GVRCSV ...
	GVRCSV = schema.GroupVersionResource{
		Group:    "operators.coreos.com",
		Version:  "v1alpha1",
		Resource: "clusterserviceversions",
	}
	// GVRHelmRelease ...
	GVRHelmRelease = schema.GroupVersionResource{
		Group:    "apps.open-cluster-management.io",
		Version:  "v1",
		Resource: "helmreleases",
	}

	// GVRInstallPlan ...
	GVRInstallPlan = schema.GroupVersionResource{
		Group:    "operators.coreos.com",
		Version:  "v1alpha1",
		Resource: "installplans",
	}

	// GVRDeployment ...
	GVRDeployment = schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}

	// GVRManagedCluster
	GVRManagedCluster = schema.GroupVersionResource{
		Group:    "cluster.open-cluster-management.io",
		Version:  "v1",
		Resource: "managedclusters",
	}

	// GVRKlusterletAddonConfig
	GVRKlusterletAddonConfig = schema.GroupVersionResource{
		Group:    "agent.open-cluster-management.io",
		Version:  "v1",
		Resource: "klusterletaddonconfigs",
	}

	// GVRBareMetalAsset
	GVRBareMetalAsset = schema.GroupVersionResource{
		Group:    "inventory.open-cluster-management.io",
		Version:  "v1alpha1",
		Resource: "baremetalassets",
	}

	// GVRDiscoveryConfig
	GVRDiscoveryConfig = schema.GroupVersionResource{
		Group:    "discovery.open-cluster-management.io",
		Version:  "v1",
		Resource: "discoveryconfigs",
	}

	// GVRClusterVersion
	GVRClusterVersion = schema.GroupVersionResource{
		Group:    "config.openshift.io",
		Version:  "v1",
		Resource: "clusterversions",
	}

	// GVRConsole
	GVRConsole = schema.GroupVersionResource{
		Group:    "operator.openshift.io",
		Version:  "v1",
		Resource: "consoles",
	}

	// DefaultImageRegistry ...
	DefaultImageRegistry = "quay.io/stolostron"
	// DefaultImagePullSecretName ...
	DefaultImagePullSecretName = "multiclusterhub-operator-pull-secret"

	// MCHName ...
	MCHName = "multiclusterhub"
	// MCHNamespace ...
	MCHNamespace = "open-cluster-management"
	// MCHPullSecretName ...
	MCHPullSecretName = os.Getenv("pullSecret")

	// MCHRepoName ...
	MCHRepoName = "multiclusterhub-repo"
	// MCHOperatorName ...
	MCHOperatorName = "multiclusterhub-operator"

	// OCMSubscriptionName ...
	OCMSubscriptionName = os.Getenv("name")

	// HiveConfigName ...
	HiveConfigName = "hive"

	// AppSubName console-chart-sub where clusterset pause is set
	AppSubName = "console-chart-sub"

	// SubList contains the list of subscriptions to delete
	SubList = [...]string{
		OCMSubscriptionName,
		"hive-operator-alpha-community-operators-openshift-marketplace",
		"multicluster-operators-subscription-alpha-community-operators-openshift-marketplace",
	}

	// AppSubSlice ...
	AppSubSlice = [...]string{
		"cluster-lifecycle-sub",
		"console-chart-sub",
		"grc-sub",
		"management-ingress-sub",
		"policyreport-sub",
	}

	// AppMap ...
	AppMap = map[string]struct{}{
		"console-chart-v2": struct{}{},
		"grc":              struct{}{},
		"policyreport":     struct{}{},
		"search":           struct{}{},
		"search-prod":      struct{}{},
	}

	// CSVName ...
	CSVName = "advanced-cluster-management"

	// WaitInMinutesDefault ...
	WaitInMinutesDefault = 22

	// DisableHubSelfManagementString ...
	DisableHubSelfManagementString = "disableHubSelfManagement"
)

// GetWaitInMinutes...
func GetWaitInMinutes() int {
	waitInMinutesAsString := os.Getenv("waitInMinutes")
	if waitInMinutesAsString == "" {
		return WaitInMinutesDefault
	}
	waitInMinutesAsInt, err := strconv.Atoi(waitInMinutesAsString)
	if err != nil {
		return WaitInMinutesDefault
	}
	return waitInMinutesAsInt
}

func runCleanUpScript() bool {
	runCleanUpScript, _ := strconv.ParseBool(os.Getenv("runCleanUpScript"))
	return runCleanUpScript
}

// CreateNewUnstructured creates resources by using gvr & obj, will get object after create.
func CreateNewUnstructured(
	clientHubDynamic dynamic.Interface,
	gvr schema.GroupVersionResource,
	obj *unstructured.Unstructured,
	name, namespace string,
) {
	ns := clientHubDynamic.Resource(gvr).Namespace(namespace)
	Expect(ns.Create(context.TODO(), obj, metav1.CreateOptions{})).NotTo(BeNil())
	Expect(ns.Get(context.TODO(), name, metav1.GetOptions{})).NotTo(BeNil())
}

// CreateNewConfigMap ...
func CreateNewConfigMap(cm *corev1.ConfigMap, namespace string) error {
	_, err := KubeClient.CoreV1().ConfigMaps(namespace).Create(context.TODO(), cm, metav1.CreateOptions{})
	return err
}

// DeleteConfigMapIfExists ...
func DeleteConfigMapIfExists(cmName, namespace string) error {
	_, err := KubeClient.CoreV1().ConfigMaps(namespace).Get(context.TODO(), cmName, metav1.GetOptions{})
	if err == nil {
		return KubeClient.CoreV1().ConfigMaps(namespace).Delete(context.TODO(), cmName, metav1.DeleteOptions{})
	}
	return nil
}

// DeleteIfExists deletes resources by using gvr, name, and namespace.
// Will wait for deletion to complete by using eventually
func DeleteIfExists(clientHubDynamic dynamic.Interface, gvr schema.GroupVersionResource, name, namespace string, wait bool) {
	ns := clientHubDynamic.Resource(gvr).Namespace(namespace)
	if _, err := ns.Get(context.TODO(), name, metav1.GetOptions{}); err != nil {
		Expect(errors.IsNotFound(err)).To(Equal(true))
		return
	}
	Expect(func() error {
		// possibly already got deleted
		err := ns.Delete(context.TODO(), name, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		return nil
	}()).To(BeNil())

	By("Wait for deletion")
	Eventually(func() error {
		var err error
		_, err = ns.Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		if err == nil {
			if wait {
				return fmt.Errorf("found object %s in namespace %s after deletion", name, namespace)
			}
			return nil
		}
		return nil
	}, GetWaitInMinutes()*60, 1).Should(BeNil())
}

// NewKubeClient returns a kube client
func NewKubeClient(url, kubeconfig, context string) kubernetes.Interface {
	klog.V(5).Infof("Create kubeclient for url %s using kubeconfig path %s\n", url, kubeconfig)
	config, err := LoadConfig(url, kubeconfig, context)
	if err != nil {
		panic(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	return clientset
}

// NewKubeClientDynamic returns a dynamic kube client
func NewKubeClientDynamic(url, kubeconfig, context string) dynamic.Interface {
	klog.V(5).Infof(
		"Create kubeclient dynamic for url %s using kubeconfig path %s\n",
		url,
		kubeconfig,
	)
	config, err := LoadConfig(url, kubeconfig, context)
	if err != nil {
		panic(err)
	}

	clientset, err := dynamic.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	return clientset
}

// LoadConfig loads kubeconfig
func LoadConfig(url, kubeconfig, context string) (*rest.Config, error) {
	if kubeconfig == "" {
		kubeconfig = os.Getenv("KUBECONFIG")
	}
	klog.V(5).Infof("Kubeconfig path %s\n", kubeconfig)
	// If we have an explicit indication of where the kubernetes config lives, read that.
	if kubeconfig != "" {
		if context == "" {
			return clientcmd.BuildConfigFromFlags(url, kubeconfig)
		}
		return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
			&clientcmd.ConfigOverrides{
				CurrentContext: context,
			}).ClientConfig()
	}
	// If not, try the in-cluster config.
	if c, err := rest.InClusterConfig(); err == nil {
		return c, nil
	}
	// If no in-cluster config, try the default location in the user's home directory.
	if usr, err := user.Current(); err == nil {
		klog.V(5).Infof("clientcmd.BuildConfigFromFlags for url %s using %s\n", url, filepath.Join(usr.HomeDir, ".kube", "config"))
		if c, err := clientcmd.BuildConfigFromFlags("", filepath.Join(usr.HomeDir, ".kube", "config")); err == nil {
			return c, nil
		}
	}

	return nil, fmt.Errorf("could not create a valid kubeconfig")
}

// IsOwner checks if obj is owned by owner, obj can either be unstructured or ObjectMeta
func IsOwner(owner *unstructured.Unstructured, obj interface{}) bool {
	if obj == nil {
		return false
	}
	var owners []metav1.OwnerReference
	objMeta, ok := obj.(*metav1.ObjectMeta)
	if ok {
		owners = objMeta.GetOwnerReferences()
	} else {
		if objUnstructured, ok := obj.(*unstructured.Unstructured); ok {
			owners = objUnstructured.GetOwnerReferences()
		} else {
			klog.Error("Failed to get owners")
			return false
		}
	}

	for _, ownerRef := range owners {
		if _, ok := owner.Object["metadata"]; !ok {
			klog.Error("no meta")
			continue
		}
		meta, ok := owner.Object["metadata"].(map[string]interface{})
		if !ok || meta == nil {
			klog.Error("no meta map")
			continue
		}
		name, ok := meta["name"].(string)
		if !ok || name == "" {
			klog.Error("failed to get name")
			continue
		}
		if ownerRef.Kind == owner.Object["kind"] && ownerRef.Name == name {
			return true
		}
	}
	return false
}

// CreateMCHNotManaged ...
func CreateMCHNotManaged() *unstructured.Unstructured {
	mch := NewMultiClusterHub(MCHName, MCHNamespace, "", true)
	CreateNewUnstructured(DynamicKubeClient, GVRMultiClusterHub, mch, MCHName, MCHNamespace)
	return mch
}

// CreateMCHTolerations ...
func CreateMCHTolerations() *unstructured.Unstructured {
	mch := NewMCHTolerations(MCHName, MCHNamespace, "", true)
	CreateNewUnstructured(DynamicKubeClient, GVRMultiClusterHub, mch, MCHName, MCHNamespace)
	return mch
}

// CreateMCHImageOverridesAnnotation ...
func CreateMCHImageOverridesAnnotation(imageOverridesConfigmapName string) *unstructured.Unstructured {
	mch := NewMultiClusterHub(MCHName, MCHNamespace, imageOverridesConfigmapName, true)
	CreateNewUnstructured(DynamicKubeClient, GVRMultiClusterHub, mch, MCHName, MCHNamespace)
	return mch
}

func CreateDefaultMCH() *unstructured.Unstructured {
	mch := NewMultiClusterHub(MCHName, MCHNamespace, "", false)
	CreateNewUnstructured(DynamicKubeClient, GVRMultiClusterHub, mch, MCHName, MCHNamespace)
	return mch
}

// GetDeploymentLabels returns the labels on deployment d
func GetDeploymentLabels(d string) (map[string]string, error) {
	deploy, err := KubeClient.AppsV1().Deployments(MCHNamespace).Get(context.TODO(), d, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return deploy.GetLabels(), nil
}

// BrickMCHRepo modifies the multiclusterhub-repo deployment so it becomes unhealthy
func BrickMCHRepo() error {
	By("- Breaking mch repo")
	deploy, err := KubeClient.AppsV1().Deployments(MCHNamespace).Get(context.TODO(), MCHRepoName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	// Add non-existent nodeSelector so the pod isn't scheduled
	deploy.Spec.Template.Spec.NodeSelector = map[string]string{"schedule": "never"}

	_, err = KubeClient.AppsV1().Deployments(MCHNamespace).Update(context.TODO(), deploy, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	if err = waitForUnavailable(MCHRepoName, time.Duration(GetWaitInMinutes())*time.Minute); err != nil {
		return err
	}

	return nil
}

// FixMCHRepo deletes the multiclusterhub-repo deployment so it can be recreated by the installer
func FixMCHRepo() error {
	By("- Repairing mch-repo")
	return KubeClient.AppsV1().Deployments(MCHNamespace).Delete(context.TODO(), MCHRepoName, metav1.DeleteOptions{})
}

// DeleteMCHRepo deletes the multiclusterhub-repo deployment
func DeleteMCHRepo() error {
	return KubeClient.AppsV1().Deployments(MCHNamespace).Delete(context.TODO(), MCHRepoName, metav1.DeleteOptions{})
}

func PauseMCH() error {
	mch, err := DynamicKubeClient.Resource(GVRMultiClusterHub).Namespace(MCHNamespace).Get(context.TODO(), MCHName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	labels := mch.GetLabels()
	if labels == nil {
		labels = map[string]string{"mch-pause": "true"}
	} else {
		labels["mch-pause"] = "true"
	}
	mch.SetLabels(labels)
	_, err = DynamicKubeClient.Resource(GVRMultiClusterHub).Namespace(MCHNamespace).Update(context.TODO(), mch, metav1.UpdateOptions{})
	return err
}

func UnpauseMCH() error {
	mch, err := DynamicKubeClient.Resource(GVRMultiClusterHub).Namespace(MCHNamespace).Get(context.TODO(), MCHName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	labels := mch.GetLabels()
	if labels == nil {
		labels = map[string]string{"mch-pause": "false"}
	} else {
		labels["mch-pause"] = "false"
	}
	mch.SetLabels(labels)
	_, err = DynamicKubeClient.Resource(GVRMultiClusterHub).Namespace(MCHNamespace).Update(context.TODO(), mch, metav1.UpdateOptions{})
	return err
}

// BrickCLC modifies the multiclusterhub-repo deployment so it becomes unhealthy
func BrickCLC() (string, error) {
	By("- Breaking cluster-lifecycle")
	oldImage, err := UpdateDeploymentImage("cluster-lifecycle", "bad-image")
	if err != nil {
		return "", err
	}
	err = waitForUnavailable("cluster-lifecycle", time.Duration(GetWaitInMinutes())*time.Minute)

	return oldImage, err
}

// FiCLC deletes the multiclusterhub-repo deployment so it can be recreated by the installer
func FixCLC(image string) error {
	By("- Repairing cluster-lifecycle")
	_, err := UpdateDeploymentImage("cluster-lifecycle", image)
	if err != nil {
		return err
	}
	err = waitForAvailable("cluster-lifecycle", time.Duration(GetWaitInMinutes())*time.Minute)
	return err
}

// UpdateDeploymentImage updates the deployment image
func UpdateDeploymentImage(dName string, image string) (string, error) {
	deploy, err := KubeClient.AppsV1().Deployments(MCHNamespace).Get(context.TODO(), dName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	originalImage := deploy.Spec.Template.Spec.Containers[0].Image
	deploy.Spec.Template.Spec.Containers[0].Image = image

	_, err = KubeClient.AppsV1().Deployments(MCHNamespace).Update(context.TODO(), deploy, metav1.UpdateOptions{})
	return originalImage, err
}

// waitForUnavailable waits for the deployment to go unready, with timeout
func waitForUnavailable(dName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		deploy, err := KubeClient.AppsV1().Deployments(MCHNamespace).Get(context.TODO(), dName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if deploy.Status.UnavailableReplicas > 0 {
			time.Sleep(10 * time.Second)
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("Deploy failed to become unready after %s", timeout)
}

// waitForAvailable waits for the deployment to be available, with timeout
func waitForAvailable(dName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		deploy, err := KubeClient.AppsV1().Deployments(MCHNamespace).Get(context.TODO(), dName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if deploy.Status.UnavailableReplicas == 0 {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("Repo failed to become unready after %s", timeout)
}

// GetMCHStatus gets the mch object and parses its status
func GetMCHStatus() (map[string]interface{}, error) {
	mch, err := DynamicKubeClient.Resource(GVRMultiClusterHub).Namespace(MCHNamespace).Get(context.TODO(), MCHName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	status, ok := mch.Object["status"].(map[string]interface{})
	if !ok || status == nil {
		return nil, fmt.Errorf("MultiClusterHub: %s has no 'status' map", mch.GetName())
	}
	return status, nil
}

// IsMCHSelfManaged returns the opposite of `spec.disableHubSelfManagement`
func IsMCHSelfManaged() (bool, error) {
	mch, err := DynamicKubeClient.Resource(GVRMultiClusterHub).Namespace(MCHNamespace).Get(context.TODO(), MCHName, metav1.GetOptions{})
	if err != nil {
		return true, err
	}
	spec, ok := mch.Object["spec"].(map[string]interface{})
	if !ok || spec == nil {
		return true, fmt.Errorf("MultiClusterHub: %s has no 'spec' map", mch.GetName())
	}
	disableHubSelfManagement, ok := spec[DisableHubSelfManagementString]
	if !ok || disableHubSelfManagement == nil {
		return true, nil // if spec not set, default to managed
	}
	selfManaged := !(disableHubSelfManagement.(bool))
	return selfManaged, nil
}

// findPhase reports whether the hub status has the desired phase and returns an error if not
func findPhase(status map[string]interface{}, wantPhase string) error {
	if _, ok := status["phase"]; !ok {
		return fmt.Errorf("MCH status has no 'phase' field")
	}
	if phase := status["phase"]; phase != wantPhase {
		return fmt.Errorf("MCH phase equals `%s`, expected `%s`", phase, wantPhase)
	}
	return nil
}

// ValidateMCHDegraded validates the install operator responds appropriately when the install components
// go into a degraded state after a successful install
func ValidateMCHDegraded() error {
	status, err := GetMCHStatus()
	if err != nil {
		return err
	}

	// Ensuring MCH is in 'pending' phase
	if err := findPhase(status, "Pending"); err != nil {
		return err
	}

	// Ensuring hub condition shows installation as incomplete
	if err := FindCondition(status, "Complete", "False"); err != nil {
		return err
	}

	return nil
}

// ValidateDelete ...
func ValidateDelete(clientHubDynamic dynamic.Interface) error {
	By("Validating MCH has been successfully uninstalled.")

	labelSelector := fmt.Sprintf("installer.name=%s, installer.namespace=%s", MCHName, MCHNamespace)
	listOptions := metav1.ListOptions{
		LabelSelector: labelSelector,
		Limit:         100,
	}

	appSubLink := clientHubDynamic.Resource(GVRAppSub)
	appSubs, err := appSubLink.List(context.TODO(), listOptions)
	if err != nil {
		return err
	}

	helmReleaseLink := clientHubDynamic.Resource(GVRHelmRelease)
	helmReleases, err := helmReleaseLink.List(context.TODO(), listOptions)
	if err != nil {
		return err
	}

	By("- Ensuring Application Subscriptions have terminated")
	if len(appSubs.Items) != 0 {
		return fmt.Errorf("%d appsubs left to be uninstalled", len(appSubs.Items))
	}

	By("- Ensuring HelmReleases have terminated")
	if len(helmReleases.Items) != 0 {
		By(fmt.Sprintf("%d helmreleases left to be uninstalled", len(helmReleases.Items)))
		return fmt.Errorf("%d helmreleases left to be uninstalled", len(helmReleases.Items))
	}

	By("- Ensuring MCH Repo deployment has been terminated")
	deploymentLink := clientHubDynamic.Resource(GVRDeployment).Namespace(MCHNamespace)
	_, err = deploymentLink.Get(context.TODO(), "multiclusterhub-repo", metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	By("- Ensuring MCH image manifest configmap is terminated")
	labelSelector = fmt.Sprintf("ocm-configmap-type=%s", "image-manifest")
	listOptions = metav1.ListOptions{
		LabelSelector: labelSelector,
		Limit:         100,
	}

	Eventually(func() error {
		configmaps, err := KubeClient.CoreV1().ConfigMaps(MCHNamespace).List(context.TODO(), listOptions)
		Expect(err).Should(BeNil())
		if len(configmaps.Items) != 0 {
			return fmt.Errorf("Expecting configmaps to terminate")
		}
		return nil
	}, GetWaitInMinutes()*60, 1).Should(BeNil())

	By("- Validating CRDs were deleted")
	crds, err := getCRDs()
	if err != nil {
		return err
	}

	for _, crd := range crds {
		_, err = DynamicKubeClient.Resource(GVRCustomResourceDefinition).Get(context.TODO(), crd, metav1.GetOptions{})
		Expect(err).ToNot(BeNil())
	}

	By("- Validating ClusterManager was deleted")
	clusterManagerLink := clientHubDynamic.Resource(GVRClusterManager)
	_, err = clusterManagerLink.Get(context.TODO(), "cluster-manager", metav1.GetOptions{})
	Expect(err).ShouldNot(BeNil())

	By("- Validating HiveConfig was deleted")
	hiveConfigLink := clientHubDynamic.Resource(GVRHiveConfig)
	_, err = hiveConfigLink.Get(context.TODO(), HiveConfigName, metav1.GetOptions{})
	Expect(err).ShouldNot(BeNil())

	if runCleanUpScript() {
		By("- Running documented clean up script")
		workingDir, err := os.Getwd()
		if err != nil {
			log.Fatalf("failed to get working dir %v", err)
		}
		cleanupPath := path.Join(path.Dir(workingDir), "clean-up.sh")
		err = os.Setenv("ACM_NAMESPACE", MCHNamespace)
		if err != nil {
			log.Fatal(err)
		}
		out, err := exec.Command("/bin/sh", cleanupPath).Output()
		if err != nil {
			log.Fatal(err)
		}
		err = os.Unsetenv("ACM_NAMESPACE")
		if err != nil {
			log.Fatal(err)
		}
		log.Println(fmt.Sprintf("Resources cleaned up by clean-up script:\n %s\n", bytes.NewBuffer(out).String()))

	}
	return nil
}

// FindCondition reports whether a hub condition of type 't' exists and matches the status 's'
func FindCondition(status map[string]interface{}, t string, s string) error {
	conditions, ok := status["conditions"].([]interface{})
	if !ok || conditions == nil {
		return fmt.Errorf("no hubConditions found")
	}
	for i := range conditions {
		condition := conditions[i]
		if condition.(map[string]interface{})["type"].(string) == t {
			if got := condition.(map[string]interface{})["status"].(string); got == s {
				return nil
			} else {
				return fmt.Errorf("hubCondition `%s` status equals '%s', expected '%s'", t, got, s)
			}
		}
	}
	return fmt.Errorf("MCH does not have a hubcondition with type '%s'", t)
}

// ValidateMCHUnsuccessful ...
func ValidateMCHUnsuccessful() error {
	By("Validating MultiClusterHub Unsuccessful")
	By(fmt.Sprintf("- Waiting %d minutes", GetWaitInMinutes()), func() {
		time.Sleep(time.Duration(GetWaitInMinutes()) * time.Minute)
	})

	By("- Ensuring MCH is in 'Installing' phase")
	status, err := GetMCHStatus()
	if err != nil {
		return err
	}
	if err := findPhase(status, "Installing"); err != nil {
		return err
	}

	By("MCH Condition 'type' should be `Progressing` and 'status' should be 'true")
	Eventually(func() error {
		mch, err := DynamicKubeClient.Resource(GVRMultiClusterHub).Namespace(MCHNamespace).Get(context.TODO(), MCHName, metav1.GetOptions{})
		Expect(err).To(BeNil())
		status := mch.Object["status"].(map[string]interface{})
		return FindCondition(status, "Progressing", "True")
	}, 1, 1).Should(BeNil())

	return nil
}

// ValidateMCH validates MCH CR is running successfully
func ValidateMCH() error {
	By("Validating MultiClusterHub")

	By(fmt.Sprintf("- Ensuring MCH is in 'running' phase within %d minutes", GetWaitInMinutes()))
	Eventually(func() error {
		status, err := GetMCHStatus()
		if err != nil {
			return err
		}
		if err := findPhase(status, "Running"); err != nil {
			return err
		}
		return nil
	}, GetWaitInMinutes()*60, 1).Should(BeNil())

	By("- Ensuring MCH Repo Is available")
	var deploy *appsv1.Deployment
	deploy, err := KubeClient.AppsV1().Deployments(MCHNamespace).Get(context.TODO(), MCHRepoName, metav1.GetOptions{})
	Expect(err).Should(BeNil())
	mch, err := DynamicKubeClient.Resource(GVRMultiClusterHub).Namespace(MCHNamespace).Get(context.TODO(), MCHName, metav1.GetOptions{})
	Expect(err).To(BeNil())
	Expect(deploy.Status.AvailableReplicas).ShouldNot(Equal(0))
	Expect(IsOwner(mch, &deploy.ObjectMeta)).To(Equal(true))

	By("- Validating CRDs were created successfully")
	crds, err := getCRDs()
	Expect(err).Should(BeNil())

	for _, crd := range crds {
		_, err = DynamicKubeClient.Resource(GVRCustomResourceDefinition).Get(context.TODO(), crd, metav1.GetOptions{})
		Expect(err).To(BeNil())
	}

	By("- Ensuring components have status 'true' when MCH is in 'running' phase")
	mch, err = DynamicKubeClient.Resource(GVRMultiClusterHub).Namespace(MCHNamespace).Get(context.TODO(), MCHName, metav1.GetOptions{})
	Expect(err).To(BeNil())
	status := mch.Object["status"].(map[string]interface{})
	if findPhase(status, "Running") == nil {
		components, ok := mch.Object["status"].(map[string]interface{})["components"]
		if !ok || components == nil {
			return fmt.Errorf("MultiClusterHub: %s has no 'Components' map despite reporting 'running'", mch.GetName())
		}
		for k, v := range components.(map[string]interface{}) {
			compStatus := v.(map[string]interface{})["status"].(string)
			if compStatus != "True" {
				return fmt.Errorf("Component: %s does not have status of 'true'", k)
			}
		}
	}

	By("- Ensuring condition has status 'true' and type 'complete' when MCH is in 'running' phase")
	Eventually(func() error {
		mch, err := DynamicKubeClient.Resource(GVRMultiClusterHub).Namespace(MCHNamespace).Get(context.TODO(), MCHName, metav1.GetOptions{})
		Expect(err).To(BeNil())
		status := mch.Object["status"].(map[string]interface{})
		return FindCondition(status, "Complete", "True")
	}, 1, 1).Should(BeNil())

	By("- Checking Appsubs")
	unstructuredAppSubs := listByGVR(DynamicKubeClient, GVRAppSub, MCHNamespace, 1, len(AppSubSlice))
	for _, appsub := range unstructuredAppSubs.Items {
		if _, ok := appsub.Object["status"]; !ok {
			return fmt.Errorf("Appsub: %s has no 'status' field", appsub.GetName())
		}
		status, ok := appsub.Object["status"].(map[string]interface{})
		if !ok || status == nil {
			return fmt.Errorf("Appsub: %s has no 'status' map", appsub.GetName())
		}
		klog.V(5).Infof("Checking Appsub - %s", appsub.GetName())
		Expect(status["message"]).To(Equal("Active"))
		Expect(status["phase"]).To(Equal("Subscribed"))
	}

	By("- Checking HelmReleases")
	unstructuredHelmReleases := listByGVR(DynamicKubeClient, GVRHelmRelease, MCHNamespace, 1, len(AppSubSlice))
	for _, helmRelease := range unstructuredHelmReleases.Items {
		klog.V(5).Infof("Checking HelmRelease - %s", helmRelease.GetName())

		status, ok := helmRelease.Object["status"].(map[string]interface{})
		if !ok || status == nil {
			return fmt.Errorf("HelmRelease: %s has no 'status' map", helmRelease.GetName())
		}

		conditions, ok := status["deployedRelease"].(map[string]interface{})
		if !ok || conditions == nil {
			return fmt.Errorf("HelmRelease: %s has no 'deployedRelease' interface", helmRelease.GetName())
		}
	}

	By("- Checking Imported Hub Cluster")
	if os.Getenv("MOCK") != "true" {
		selfManaged, err := IsMCHSelfManaged()
		Expect(err).Should(BeNil())
		err = ValidateManagedCluster(selfManaged)
		Expect(err).Should(BeNil())
	}

	currentVersion, err := GetCurrentVersionFromMCH()
	Expect(err).Should(BeNil())
	v, err := semver.NewVersion(currentVersion)
	Expect(err).Should(BeNil())
	c, err := semver.NewConstraint(">= 2.5.0")
	Expect(err).Should(BeNil())
	if c.Check(v) {
		By("- Ensuring image manifest configmap is created")
		_, err = KubeClient.CoreV1().ConfigMaps(MCHNamespace).Get(context.TODO(), fmt.Sprintf("mch-image-manifest-%s", currentVersion), metav1.GetOptions{})
		Expect(err).Should(BeNil())
	}

	By("- Checking for Installer Labels on Deployments")
	l, err := GetDeploymentLabels("search-operator")
	if err != nil {
		return err
	}
	if l["installer.name"] != MCHName || l["installer.namespace"] != MCHNamespace {
		return fmt.Errorf("search-operator missing installer labels: `%s` != `%s`, `%s` != `%s`", l["installer.name"], MCHName, l["installer.namespace"], MCHNamespace)
	}

	clusterVersionKey := types.NamespacedName{Name: "version"}
	clusterVersion, err := DynamicKubeClient.Resource(GVRClusterVersion).Get(context.TODO(), clusterVersionKey.Name, metav1.GetOptions{})
	Expect(err).To(BeNil())
	status, ok := clusterVersion.Object["status"].(map[string]interface{})
	Expect(ok).To(BeTrue())
	history, ok := status["history"].([]interface{})
	Expect(ok).To(BeTrue())
	Expect(len(history)).To(BeNumerically(">", 0))
	latestHistory := history[0].(map[string]interface{})
	version := latestHistory["version"].(string)

	consoleConstraint, err := semver.NewConstraint(">= 4.10.0-0")
	Expect(err).To(BeNil(), "Error creating semver constraint")
	semverVersion, err := semver.NewVersion(version)
	Expect(err).To(BeNil(), "Error creating semver constraint")

	if consoleConstraint.Check(semverVersion) {
		By("OCP 4.10+ cluster detected. Checking MCE Console is installed")
		consoleKey := types.NamespacedName{Name: "cluster"}
		console, err := DynamicKubeClient.Resource(GVRConsole).Get(context.TODO(), consoleKey.Name, metav1.GetOptions{})
		Expect(err).To(BeNil())
		By("Ensuring mce plugin is enabled in openshift console")
		spec, ok := console.Object["spec"].(map[string]interface{})
		Expect(ok).To(BeTrue())
		plugins, ok := spec["plugins"].([]interface{})
		Expect(ok).To(BeTrue())
		pluginsSlice := make([]string, len(plugins))
		for i, v := range plugins {
			pluginsSlice[i] = fmt.Sprint(v)
		}
		Expect(utils.Contains(pluginsSlice, "acm")).To(BeTrue(), "Expected ACM plugin to be enabled in console resource")
	} else {
		By("OCP cluster below 4.10 detected. Skipping plugin check")
	}

	return nil
}

// ValidateMCHStatusExist check if mch status exists
func ValidateMCHStatusExist() error {
	Eventually(func() error {
		mch, err := DynamicKubeClient.Resource(GVRMultiClusterHub).Namespace(MCHNamespace).Get(context.TODO(), MCHName, metav1.GetOptions{})
		Expect(err).To(BeNil())
		status, ok := mch.Object["status"].(map[string]interface{})
		if !ok || status == nil {
			return fmt.Errorf("MultiClusterHub: %s has no 'status' map", mch.GetName())
		}
		return nil
	}, GetWaitInMinutes()*60, 1).Should(BeNil())
	return nil
}

// ValidateComponentStatusExist check if Component statuses exist immediately when MCH is created
func ValidateComponentStatusExist() error {
	Eventually(func() error {
		mch, err := DynamicKubeClient.Resource(GVRMultiClusterHub).Namespace(MCHNamespace).Get(context.TODO(), MCHName, metav1.GetOptions{})
		Expect(err).To(BeNil())
		status, ok := mch.Object["status"].(map[string]interface{})
		if !ok || status == nil {
			return fmt.Errorf("MultiClusterHub: %s has no 'status' map", mch.GetName())
		}
		if components, ok := status["components"]; !ok || components == nil {
			return fmt.Errorf("MultiClusterHub: %s has no 'Components' map in status", mch.GetName())
		} else {
			for k, v := range components.(map[string]interface{}) {
				if _, ok := v.(map[string]interface{})["status"].(string); !ok {
					return fmt.Errorf("Component: %s status does not exist", k)
				}
			}
		}
		return nil
	}, GetWaitInMinutes()*60, 1).Should(BeNil())
	return nil
}

// ValidateHubStatusExist checks if hub statuses exist immediately when MCH is created
func ValidateHubStatusExist() error {
	Eventually(func() error {
		mch, err := DynamicKubeClient.Resource(GVRMultiClusterHub).Namespace(MCHNamespace).Get(context.TODO(), MCHName, metav1.GetOptions{})
		Expect(err).To(BeNil())
		status, ok := mch.Object["status"].(map[string]interface{})
		if !ok || status == nil {
			return fmt.Errorf("MultiClusterHub: %s has no 'status' map", mch.GetName())
		}
		return FindCondition(status, "Progressing", "True")
	}, GetWaitInMinutes()*60, 1).Should(BeNil())
	return nil
}

// ValidateConditionDuringUninstall check if condition is terminating during uninstall of MCH
func ValidateConditionDuringUninstall() error {
	By("- Checking HubCondition type")
	Eventually(func() error {
		mch, err := DynamicKubeClient.Resource(GVRMultiClusterHub).Namespace(MCHNamespace).Get(context.TODO(), MCHName, metav1.GetOptions{})
		Expect(err).To(BeNil())
		status := mch.Object["status"].(map[string]interface{})
		return FindCondition(status, "Terminating", "True")
	}, GetWaitInMinutes()*60, 1).Should(BeNil())
	return nil
}

// ValidatePhase returns error if MCH phase does not match the provided phase
func ValidatePhase(phase string) error {
	By("- Checking HubCondition type")
	status, err := GetMCHStatus()
	if err != nil {
		return err
	}
	return findPhase(status, phase)
}

// ValidateStatusesExist Confirms existence of both overall MCH and Component statuses immediately after MCH creation
func ValidateStatusesExist() error {
	By("Validating Statuses exist")

	By("- Ensuring MCH Status exists")
	if err := ValidateMCHStatusExist(); err != nil {
		return err
	}
	By("- Ensuring Component Status exist")
	if err := ValidateComponentStatusExist(); err != nil {
		return err
	}
	By("- Ensuring Hub Status exist")
	if err := ValidateHubStatusExist(); err != nil {
		return err
	}
	return nil
}

// ValidateImportHubResourcesExist confirms the existence of 3 resources that are created when importing hub as managed cluster
func ValidateImportHubResourcesExist(expected bool) error {
	//check created namespace exists
	_, nsErr := KubeClient.CoreV1().Namespaces().Get(context.TODO(), "local-cluster", metav1.GetOptions{})
	//check created ManagedCluster exists
	mc, mcErr := DynamicKubeClient.Resource(GVRManagedCluster).Get(context.TODO(), "local-cluster", metav1.GetOptions{})
	//check created KlusterletAddonConfig
	kac, kacErr := DynamicKubeClient.Resource(GVRKlusterletAddonConfig).Namespace("local-cluster").Get(context.TODO(), "local-cluster", metav1.GetOptions{})
	if expected {
		if mc != nil {
			if nsErr != nil || mcErr != nil || kacErr != nil {
				return fmt.Errorf("not all local-cluster resources created")
			}
			return nil
		} else {
			return fmt.Errorf("local-cluster resources exist")
		}
	} else {
		if mc != nil || kac != nil {
			return fmt.Errorf("local-cluster resources exist")
		}
		return nil
	}
}

// ValidateManagedCluster ...
func ValidateManagedCluster(importResourcesShouldExist bool) error {
	By("- Checking imported hub resources exist or not")
	By("- Confirming Necessary Resources")
	// mc, _ := DynamicKubeClient.Resource(GVRManagedCluster).Get(context.TODO(), "local-cluster", metav1.GetOptions{})
	if err := ValidateImportHubResourcesExist(importResourcesShouldExist); err != nil {
		return fmt.Errorf("Resources are as they shouldn't")
	}
	if importResourcesShouldExist {
		if val := validateManagedClusterConditions(); val != nil {
			return fmt.Errorf("cluster conditions")
		}
		return nil
	}
	return nil
}

// validateManagedClusterConditions
func validateManagedClusterConditions() error {
	By("- Checking ManagedClusterConditions type true")
	mc, _ := DynamicKubeClient.Resource(GVRManagedCluster).Get(context.TODO(), "local-cluster", metav1.GetOptions{})
	status, ok := mc.Object["status"].(map[string]interface{})
	if ok {
		joinErr := FindCondition(status, "ManagedClusterJoined", "True")
		avaiErr := FindCondition(status, "ManagedClusterConditionAvailable", "True")
		accpErr := FindCondition(status, "HubAcceptedManagedCluster", "True")
		if joinErr != nil || avaiErr != nil || accpErr != nil {
			return fmt.Errorf("managedcluster conditions not all true")
		}
		return nil
	} else {
		return fmt.Errorf("no status")
	}
}

func ValidateDeploymentPolicies() error {
	unstructuredDeployments := listByGVR(DynamicKubeClient, GVRDeployment, MCHNamespace, 60, 3)

	for _, deployment := range unstructuredDeployments.Items {
		deploymentName := deployment.GetName()
		if deploymentName != "multicluster-operators-application" && deploymentName != "hive-operator" && deploymentName != "multicluster-operators-channel" && deploymentName != "multicluster-operators-hub-subscription" && deploymentName != "multicluster-operators-standalone-subscription" {
			policy := deployment.Object["spec"].(map[string]interface{})["template"].(map[string]interface{})["spec"].(map[string]interface{})["containers"].([]interface{})[0].(map[string]interface{})["imagePullPolicy"]
			fmt.Println(fmt.Sprintf(deploymentName))
			Expect(policy).To(BeEquivalentTo("IfNotPresent"))
		}
	}
	return nil
}

func ValidateMCESub() error {

	expectedNodeSelector := map[string]interface{}{"beta.kubernetes.io/os": "linux"}
	expectedTolerations := []interface{}{map[string]interface{}{"operator": "Exists"}}
	expectedEnv := make([]interface{}, 3, 4)

	expectedEnv[0] = map[string]interface{}{"name": "HTTP_PROXY", "value": "test"}
	expectedEnv[1] = map[string]interface{}{"name": "HTTPS_PROXY"}
	expectedEnv[2] = map[string]interface{}{"name": "NO_PROXY"}
	Eventually(func() error {
		sub, err := DynamicKubeClient.Resource(GVRSub).Namespace("multicluster-engine").Get(context.TODO(), "multicluster-engine", metav1.GetOptions{})
		if err != nil {
			return err
		}

		config := sub.Object["spec"].(map[string]interface{})["config"].(map[string]interface{})
		Expect(config["nodeSelector"].(map[string]interface{})).To(BeEquivalentTo(expectedNodeSelector))
		Expect(config["tolerations"].([]interface{})).To(BeEquivalentTo(expectedTolerations))
		Expect(config["env"].([]interface{})).To(BeEquivalentTo(expectedEnv))
		return err
	}, 3, 1).Should(BeNil())

	return nil

}

// ValidateMCHTolerations ...
func ValidateMCHTolerations() error {
	ns := DynamicKubeClient.Resource(GVRDeployment).Namespace(MCHNamespace)
	labelSelector := fmt.Sprintf("installer.name=%s", MCHName)
	listOptions := metav1.ListOptions{
		LabelSelector: labelSelector,
	}
	deployments, err := ns.List(context.Background(), listOptions)
	if err != nil {
		return err
	}

	for _, deployment := range deployments.Items {
		var d appsv1.Deployment
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(deployment.Object, &d)
		if err != nil {
			return fmt.Errorf("Could not convert from unstructured to deployment for %s: %s", deployment.GetName(), err)
		}
		if _, ok := AppMap[d.Labels["app"]]; !ok {
			continue
		}
		tolerations := d.Spec.Template.Spec.Tolerations
		for _, testToleration := range TestTolerations() {
			found := false
			for _, toleration := range tolerations {
				if testToleration.MatchToleration(&toleration) {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("Test toleration not found on deployment %s: %#v", d.Name, testToleration)
			}
		}
	}
	return nil
}

// ToggleDisableHubSelfManagement toggles the value of spec.disableHubSelfManagement from true to false or false to true
func ToggleDisableHubSelfManagement(disableHubSelfImport bool) error {
	mch, err := DynamicKubeClient.Resource(GVRMultiClusterHub).Namespace(MCHNamespace).Get(context.TODO(), MCHName, metav1.GetOptions{})
	Expect(err).To(BeNil())
	mch.Object["spec"].(map[string]interface{})[DisableHubSelfManagementString] = disableHubSelfImport
	mch, err = DynamicKubeClient.Resource(GVRMultiClusterHub).Namespace(MCHNamespace).Update(context.TODO(), mch, metav1.UpdateOptions{})
	Expect(err).To(BeNil())
	mch, err = DynamicKubeClient.Resource(GVRMultiClusterHub).Namespace(MCHNamespace).Get(context.TODO(), MCHName, metav1.GetOptions{})
	if disableHubSelfManagement := mch.Object["spec"].(map[string]interface{})[DisableHubSelfManagementString].(bool); disableHubSelfManagement != disableHubSelfImport {
		return fmt.Errorf("Spec was not updated")
	}
	return nil
}

// ToggleDisableUpdateClusterImageSets toggles the value of spec.disableUpdateClusterImageSets from true to false or false to true
func ToggleDisableUpdateClusterImageSets(disableUpdateCIS bool) error {
	mch, err := DynamicKubeClient.Resource(GVRMultiClusterHub).Namespace(MCHNamespace).Get(context.TODO(), MCHName, metav1.GetOptions{})
	Expect(err).To(BeNil())
	disableUpdateClusterImageSetsString := "disableUpdateClusterImageSets"
	mch.Object["spec"].(map[string]interface{})[disableUpdateClusterImageSetsString] = disableUpdateCIS
	mch, err = DynamicKubeClient.Resource(GVRMultiClusterHub).Namespace(MCHNamespace).Update(context.TODO(), mch, metav1.UpdateOptions{})
	Expect(err).To(BeNil())
	mch, err = DynamicKubeClient.Resource(GVRMultiClusterHub).Namespace(MCHNamespace).Get(context.TODO(), MCHName, metav1.GetOptions{})
	if disableUpdateClusterImageSets := mch.Object["spec"].(map[string]interface{})[disableUpdateClusterImageSetsString].(bool); disableUpdateClusterImageSets != disableUpdateCIS {
		return fmt.Errorf("Spec was not updated")
	}
	return nil
}

// UpdateAnnotations
func UpdateAnnotations(annotations map[string]string) {
	mch, err := DynamicKubeClient.Resource(GVRMultiClusterHub).Namespace(MCHNamespace).Get(context.TODO(), MCHName, metav1.GetOptions{})
	Expect(err).To(BeNil())
	mch.SetAnnotations(annotations)
	mch, err = DynamicKubeClient.Resource(GVRMultiClusterHub).Namespace(MCHNamespace).Update(context.TODO(), mch, metav1.UpdateOptions{})
	Expect(err).To(BeNil())
}

// ValidateClusterImageSetsSubscriptionPause validates that the console-chart-sub created ClusterImageSets subscription is either subscription-pause true or false
func ValidateClusterImageSetsSubscriptionPause(expected string) error {
	appsub, err := DynamicKubeClient.Resource(GVRAppSub).Namespace(MCHNamespace).Get(context.TODO(), AppSubName, metav1.GetOptions{})
	Expect(err).To(BeNil())
	spec, ok := appsub.Object["spec"].(map[string]interface{})
	if !ok || spec == nil {
		return fmt.Errorf("MultiClusterHub: %s has no 'spec' map", appsub.GetName())
	}
	packageoverrides_outer, ok := spec["packageOverrides"].([]interface{})
	if !ok || packageoverrides_outer == nil {
		return fmt.Errorf("MultiClusterHub: %s has no 'packageoverrides' outer map", appsub.GetName())
	}
	packageoverrides_outer_first := packageoverrides_outer[0].(map[string]interface{})
	packageoverrides_inner, ok := packageoverrides_outer_first["packageOverrides"].([]interface{})
	if !ok || packageoverrides_inner == nil {
		return fmt.Errorf("MultiClusterHub: %s has no 'packageoverrides' inner map", appsub.GetName())
	}
	packageoverrides_inner_first := packageoverrides_inner[0].(map[string]interface{})
	value, ok := packageoverrides_inner_first["value"].(map[string]interface{})
	if !ok || value == nil {
		return fmt.Errorf("MultiClusterHub: %s has no 'value' map", appsub.GetName())
	}
	clusterimageset, ok := value["clusterImageSets"].(map[string]interface{})
	if !ok || clusterimageset == nil {
		return fmt.Errorf("MultiClusterHub: %s has no 'clusterimageset' map", appsub.GetName())
	}
	subscriptionPauseValue, ok := clusterimageset["subscriptionPause"]
	if !ok || subscriptionPauseValue == nil {
		return fmt.Errorf("MultiClusterHub: %s has no 'subscriptionPauseValue'", appsub.GetName())
	}
	if subscriptionPauseValue != expected {
		return fmt.Errorf("subscriptionPause attribute is not correct")
	}
	return nil
}

// listByGVR keeps polling to get the object for timeout seconds
func listByGVR(clientHubDynamic dynamic.Interface, gvr schema.GroupVersionResource, namespace string, timeout int, expectedTotal int) *unstructured.UnstructuredList {
	if timeout < 1 {
		timeout = 1
	}
	var obj *unstructured.UnstructuredList

	Eventually(func() error {
		var err error
		namespace := clientHubDynamic.Resource(gvr).Namespace(namespace)

		labelSelector := fmt.Sprintf("installer.name=%s, installer.namespace=%s", MCHName, MCHNamespace)
		listOptions := metav1.ListOptions{
			LabelSelector: labelSelector,
			Limit:         100,
		}

		obj, err = namespace.List(context.TODO(), listOptions)
		if err != nil {
			return err
		}
		if len(obj.Items) < expectedTotal {
			return fmt.Errorf("not all resources created in time. %d/%d appsubs found", len(obj.Items), expectedTotal)
		}
		return nil
	}, timeout, 1).Should(BeNil())
	return obj
}

// GetSubscriptionSpec Returns Install Plan Mode
func GetSubscriptionSpec() map[string]interface{} {
	if os.Getenv("TEST_MODE") == "update" {
		return map[string]interface{}{
			"sourceNamespace":     os.Getenv("sourceNamespace"),
			"source":              os.Getenv("source"),
			"channel":             os.Getenv("channel"),
			"installPlanApproval": "Manual",
			"name":                os.Getenv("name"),
			"startingCSV":         fmt.Sprintf("advanced-cluster-management.v%s", os.Getenv("startVersion")),
		}
	}
	return map[string]interface{}{
		"sourceNamespace":     os.Getenv("sourceNamespace"),
		"source":              os.Getenv("source"),
		"channel":             os.Getenv("channel"),
		"installPlanApproval": "Automatic",
		"name":                os.Getenv("name"),
		"config":              map[string]interface{}{"nodeSelector": map[string]string{"beta.kubernetes.io/os": "linux"}, "tolerations": []map[string]interface{}{map[string]interface{}{"operator": "Exists"}}, "env": []map[string]interface{}{map[string]interface{}{"name": "HTTPS_PROXY", "value": "test"}}},
	}
}

// GetInstallPlanNameFromSub ...
func GetInstallPlanNameFromSub(sub *unstructured.Unstructured) (string, error) {
	if _, ok := sub.Object["status"]; !ok {
		return "", fmt.Errorf("Sub: %s has no 'status' field", sub.GetName())
	}
	status, ok := sub.Object["status"].(map[string]interface{})
	if !ok || status == nil {
		return "", fmt.Errorf("Sub: %s has no 'status' map", sub.GetName())
	}
	installplan, ok := status["installplan"].(map[string]interface{})
	if !ok || status == nil {
		return "", fmt.Errorf("Sub: %s has no 'installplan' map", sub.GetName())
	}

	return installplan["name"].(string), nil
}

// MarkInstallPlanAsApproved ...
func MarkInstallPlanAsApproved(ip *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	spec, ok := ip.Object["spec"].(map[string]interface{})
	if !ok || spec == nil {
		return nil, fmt.Errorf("Installplan: %s has no 'spec' map", ip.GetName())
	}
	spec["approved"] = true
	return ip, nil
}

// ShouldSkipSubscription skips subscription operations if set as true
func ShouldSkipSubscription() bool {
	skipSubscription := os.Getenv("skipSubscription")
	if skipSubscription == "true" {
		return true
	}
	return false
}

// GetCurrentVersionFromMCH ...
func GetCurrentVersionFromMCH() (string, error) {
	mch, err := DynamicKubeClient.Resource(GVRMultiClusterHub).Namespace(MCHNamespace).Get(context.TODO(), MCHName, metav1.GetOptions{})
	Expect(err).To(BeNil())
	status, ok := mch.Object["status"].(map[string]interface{})
	if !ok || status == nil {
		return "", fmt.Errorf("MultiClusterHub: %s has no 'status' map", mch.GetName())
	}
	version, ok := status["currentVersion"]
	if !ok {
		return "", fmt.Errorf("MultiClusterHub: %s status has no 'currentVersion' field", mch.GetName())
	}
	return version.(string), nil
}

// CreateDiscoveryConfig ...
func CreateDiscoveryConfig() {
	By("- Creating DiscoveryConfig CR if it does not exist")

	_, err := DynamicKubeClient.Resource(GVRDiscoveryConfig).Namespace(MCHNamespace).Get(context.TODO(), "discoveryconfig", metav1.GetOptions{})
	if err == nil {
		return
	}

	discoveryConfigByte, err := ioutil.ReadFile("../resources/discoveryconfig.yaml")
	Expect(err).To(BeNil())

	discoveryConfig := &unstructured.Unstructured{Object: map[string]interface{}{}}
	err = yaml.Unmarshal(discoveryConfigByte, &discoveryConfig.Object)
	Expect(err).To(BeNil())

	_, err = DynamicKubeClient.Resource(GVRDiscoveryConfig).Namespace(MCHNamespace).Create(context.TODO(), discoveryConfig, metav1.CreateOptions{})
	Expect(err).To(BeNil())
}

// DeleteDiscoveryConfig ...
func DeleteDiscoveryConfig() {
	By("- Deleting DiscoveryConfig CR if it exists")

	discoveryConfigByte, err := ioutil.ReadFile("../resources/discoveryconfig.yaml")
	Expect(err).To(BeNil())

	discoveryConfig := &unstructured.Unstructured{Object: map[string]interface{}{}}
	err = yaml.Unmarshal(discoveryConfigByte, &discoveryConfig.Object)
	Expect(err).To(BeNil())

	err = DynamicKubeClient.Resource(GVRDiscoveryConfig).Namespace(MCHNamespace).Delete(context.TODO(), discoveryConfig.GetName(), metav1.DeleteOptions{})
	Expect(err).To(BeNil())
}

// CreateObservabilityCRD ...
func CreateObservabilityCRD() {
	By("- Creating Observability CRD if it does not exist")
	_, err := DynamicKubeClient.Resource(GVRCustomResourceDefinition).Get(context.TODO(), "multiclusterobservabilities.observability.open-cluster-management.io", metav1.GetOptions{})
	if err == nil {
		return
	}

	crd, err := ioutil.ReadFile("../resources/observability-crd.yaml")
	Expect(err).To(BeNil())

	unstructuredCRD := &unstructured.Unstructured{Object: map[string]interface{}{}}
	err = yaml.Unmarshal(crd, &unstructuredCRD.Object)
	Expect(err).To(BeNil())

	_, err = DynamicKubeClient.Resource(GVRCustomResourceDefinition).Create(context.TODO(), unstructuredCRD, metav1.CreateOptions{})
	Expect(err).To(BeNil())
}

// CreateMultiClusterEngineCRD ...
func CreateMultiClusterEngineCRD() {
	By("- Creating MultiClusterEngine CRD if it does not exist")
	_, err := DynamicKubeClient.Resource(GVRCustomResourceDefinition).Get(context.TODO(), "multiclusterengines.multicluster.openshift.io", metav1.GetOptions{})
	if err == nil {
		return
	}

	crd, err := ioutil.ReadFile("../resources/multiclusterengine-crd.yaml")
	Expect(err).To(BeNil())

	unstructuredCRD := &unstructured.Unstructured{Object: map[string]interface{}{}}
	err = yaml.Unmarshal(crd, &unstructuredCRD.Object)
	Expect(err).To(BeNil())

	_, err = DynamicKubeClient.Resource(GVRCustomResourceDefinition).Create(context.TODO(), unstructuredCRD, metav1.CreateOptions{})
	Expect(err).To(BeNil())
}

// CreateMultiClusterEngineCR ...
func CreateMultiClusterEngineCR() {
	By("- Creating MultiClusterEngine CR if it does not exist")

	_, err := DynamicKubeClient.Resource(GVRMultiClusterEngine).Get(context.TODO(), "multiclusterengine-sample", metav1.GetOptions{})
	if err == nil {
		return
	}

	crd, err := ioutil.ReadFile("../resources/multiclusterengine-cr.yaml")
	Expect(err).To(BeNil())

	unstructuredCRD := &unstructured.Unstructured{Object: map[string]interface{}{}}
	err = yaml.Unmarshal(crd, &unstructuredCRD.Object)
	Expect(err).To(BeNil())

	_, err = DynamicKubeClient.Resource(GVRMultiClusterEngine).Create(context.TODO(), unstructuredCRD, metav1.CreateOptions{})
	Expect(err).To(BeNil())
}

// DeleteMultiClusterEngineCR ...
func DeleteMultiClusterEngineCR() {
	By("- Deleting MultiClusterEngine CR if it exists")

	crd, err := ioutil.ReadFile("../resources/multiclusterengine-cr.yaml")
	Expect(err).To(BeNil())

	unstructuredCRD := &unstructured.Unstructured{Object: map[string]interface{}{}}
	err = yaml.Unmarshal(crd, &unstructuredCRD.Object)
	Expect(err).To(BeNil())

	err = DynamicKubeClient.Resource(GVRMultiClusterEngine).Delete(context.TODO(), "multiclusterengine-sample", metav1.DeleteOptions{})
	Expect(err).To(BeNil())
}

// DeleteMultiClusterEngineCRD ...
func DeleteMultiClusterEngineCRD() {
	By("- Deleting MultiClusterEngine CRD if it exists")

	crd, err := ioutil.ReadFile("../resources/multiclusterengine-crd.yaml")
	Expect(err).To(BeNil())

	unstructuredCRD := &unstructured.Unstructured{Object: map[string]interface{}{}}
	err = yaml.Unmarshal(crd, &unstructuredCRD.Object)
	Expect(err).To(BeNil())

	err = DynamicKubeClient.Resource(GVRCustomResourceDefinition).Delete(context.TODO(), "multiclusterengines.multicluster.openshift.io", metav1.DeleteOptions{})
	Expect(err).To(BeNil())
}

// CreateObservabilityCR ...
func CreateObservabilityCR() {
	By("- Creating Observability CR if it does not exist")

	_, err := DynamicKubeClient.Resource(GVRObservability).Get(context.TODO(), "observability", metav1.GetOptions{})
	if err == nil {
		return
	}

	crd, err := ioutil.ReadFile("../resources/observability-cr.yaml")
	Expect(err).To(BeNil())

	unstructuredCRD := &unstructured.Unstructured{Object: map[string]interface{}{}}
	err = yaml.Unmarshal(crd, &unstructuredCRD.Object)
	Expect(err).To(BeNil())

	_, err = DynamicKubeClient.Resource(GVRObservability).Create(context.TODO(), unstructuredCRD, metav1.CreateOptions{})
	Expect(err).To(BeNil())
}

// DeleteObservabilityCR ...
func DeleteObservabilityCR() {
	By("- Deleting Observability CR if it exists")

	crd, err := ioutil.ReadFile("../resources/observability-cr.yaml")
	Expect(err).To(BeNil())

	unstructuredCRD := &unstructured.Unstructured{Object: map[string]interface{}{}}
	err = yaml.Unmarshal(crd, &unstructuredCRD.Object)
	Expect(err).To(BeNil())

	err = DynamicKubeClient.Resource(GVRObservability).Delete(context.TODO(), "observability", metav1.DeleteOptions{})
	Expect(err).To(BeNil())
}

// DeleteObservabilityCRD ...
func DeleteObservabilityCRD() {
	By("- Deleting Observability CRD if it exists")

	crd, err := ioutil.ReadFile("../resources/observability-crd.yaml")
	Expect(err).To(BeNil())

	unstructuredCRD := &unstructured.Unstructured{Object: map[string]interface{}{}}
	err = yaml.Unmarshal(crd, &unstructuredCRD.Object)
	Expect(err).To(BeNil())

	err = DynamicKubeClient.Resource(GVRCustomResourceDefinition).Delete(context.TODO(), "multiclusterobservabilities.observability.open-cluster-management.io", metav1.DeleteOptions{})
	Expect(err).To(BeNil())
}

// CreateBareMetalAssetsCR ...
func CreateBareMetalAssetsCR() {
	By("- Creating BareMetalAsset CR if it does not exist")

	_, err := DynamicKubeClient.Resource(GVRBareMetalAsset).Namespace(MCHNamespace).Get(context.TODO(), "mch-test-bma", metav1.GetOptions{})
	if err == nil {
		return
	}

	crd, err := ioutil.ReadFile("../resources/baremetalasset-cr.yaml")
	Expect(err).To(BeNil())

	unstructuredCRD := &unstructured.Unstructured{Object: map[string]interface{}{}}
	err = yaml.Unmarshal(crd, &unstructuredCRD.Object)
	Expect(err).To(BeNil())

	_, err = DynamicKubeClient.Resource(GVRBareMetalAsset).Namespace(MCHNamespace).Create(context.TODO(), unstructuredCRD, metav1.CreateOptions{})
	Expect(err).To(BeNil())
}

// DeleteBareMetalAssetsCR ...
func DeleteBareMetalAssetsCR() {
	By("- Deleting BareMetal CR if it exists")

	_, err := ioutil.ReadFile("../resources/baremetalasset-cr.yaml")
	Expect(err).To(BeNil())

	err = DynamicKubeClient.Resource(GVRBareMetalAsset).Namespace(MCHNamespace).Delete(context.TODO(), "mch-test-bma", metav1.DeleteOptions{})
	Expect(err).To(BeNil())
}

func getCRDs() ([]string, error) {
	err := os.Setenv("CRDS_PATH", "../../../pkg/templates/crds")
	if err != nil {
		return nil, err
	}
	defer os.Unsetenv("CRDS_PATH")
	crdDir, found := os.LookupEnv("CRDS_PATH")
	if !found {
		return nil, fmt.Errorf("CRDS_PATH environment variable is required")
	}

	var crds []string
	files, err := ioutil.ReadDir(crdDir)
	Expect(err).To(BeNil())
	for _, file := range files {
		if filepath.Ext(file.Name()) != ".yaml" {
			continue
		}
		filePath := path.Join(crdDir, file.Name())
		src, err := ioutil.ReadFile(filepath.Clean(filePath)) // #nosec G304 (filepath cleaned)
		if err != nil {
			return nil, err
		}

		crd := &unstructured.Unstructured{}
		err = yaml.Unmarshal(src, crd)
		if err != nil {
			return nil, err
		}

		crdName, _, err := unstructured.NestedString(crd.Object, "metadata", "name")
		if err != nil {
			return nil, err
		}

		crds = append(crds, crdName)
	}
	return crds, nil
}

// CoffeeBreak ...
func CoffeeBreak(minutes int) {
	log.Println(fmt.Sprintf("Starting coffee break for %d minutes...\n", minutes))
	slept_minutes := 0
	for slept_minutes < minutes {
		time.Sleep(time.Duration(1) * time.Minute)
		slept_minutes += 1
		log.Println(fmt.Sprintf("... slept %d minutes...\n", slept_minutes))
	}
	log.Println(fmt.Sprintf("... ending coffee break after %d minutes!\n", slept_minutes))
}
