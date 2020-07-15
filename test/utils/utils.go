// Copyright (c) 2020 Red Hat, Inc.
package utils

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (

	// KubeClient ...
	KubeClient = NewKubeClient("", "", "")
	// DynamicKubeClient ...
	DynamicKubeClient = NewKubeClientDynamic("", "", "")

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

	// DefaultImageRegistry ...
	DefaultImageRegistry = "quay.io/open-cluster-management"
	// DefaultImagePullSecretName ...
	DefaultImagePullSecretName = "multiclusterhub-operator-pull-secret"

	// MCHName ...
	MCHName = "multiclusterhub-test"
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

	// SubList contains the list of subscriptions to delete
	SubList = [...]string{
		OCMSubscriptionName,
		"hive-operator-alpha-community-operators-openshift-marketplace",
		"multicluster-operators-subscription-alpha-community-operators-openshift-marketplace",
	}

	// AppSubSlice ...
	AppSubSlice = [...]string{"application-chart-sub", "cert-manager-sub",
		"cert-manager-webhook-sub", "configmap-watcher-sub", "console-chart-sub",
		"grc-sub", "kui-web-terminal-sub", "management-ingress-sub",
		"rcm-sub", "search-prod-sub", "topology-sub"}

	// CSVName ...
	CSVName = "advanced-cluster-management"
)

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

// DeleteIfExists deletes resources by using gvr, name, and namespace.
// Will wait for deletion to complete by using eventually
func DeleteIfExists(clientHubDynamic dynamic.Interface, gvr schema.GroupVersionResource, name, namespace string) {
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
			return fmt.Errorf("found object %s in namespace %s after deletion", name, namespace)
		}
		return nil
	}, 10, 1).Should(BeNil())
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

// EnsureHelmReleasesAreRemoved ...
func EnsureHelmReleasesAreRemoved(clientHubDynamic dynamic.Interface) error {
	By("Waiting For HelmReleases to be deleted")
	helmReleasesDetected := false
	When("When MultiClusterHub is deleted, wait for all helmreleases to deleted", func() {
		Eventually(func() error {
			helmReleaseLink := clientHubDynamic.Resource(GVRHelmRelease)
			helmReleases, err := helmReleaseLink.List(context.TODO(), metav1.ListOptions{})
			Expect(err).Should(BeNil())

			if len(helmReleases.Items) == 0 {
				return nil
			}
			helmReleasesDetected = true
			return fmt.Errorf("%d helmreleases left to be uninstalled", len(helmReleases.Items))
		}, 60, 1).Should(BeNil())
		klog.V(1).Info("All Helmreleases deleted")
	})
	if helmReleasesDetected {
		By("Waiting for 2 minutes for resources to be uninstalled.")
		time.Sleep(2 * time.Minute)
	}
	return nil
}
