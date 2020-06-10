package multicloudhub_operator_test

import (
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

var (
	testName           string
	testNamespace      string
	clientHub          kubernetes.Interface
	clientHubDynamic   dynamic.Interface
	gvrMultiClusterHub schema.GroupVersionResource
	gvrSubscription    schema.GroupVersionResource
	// gvrClusterregistry   schema.GroupVersionResource
	// gvrKlusterletconfig  schema.GroupVersionResource
	// gvrClusterdeployment schema.GroupVersionResource
	// gvrSyncset           schema.GroupVersionResource
	// gvrSelectorsyncset   schema.GroupVersionResource
	// gvrSecret            schema.GroupVersionResource
	// gvrServiceaccount    schema.GroupVersionResource
	optionsFile         string
	baseDomain          string
	kubeadminUser       string
	kubeadminCredential string
	kubeconfig          string

	defaultImageRegistry       string
	defaultImagePullSecretName string

	multiClusterHubRepo string
	appsubs             [12]string
)

func newMultiClusterHub(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "operators.open-cluster-management.io/v1beta1",
			"kind":       "MultiClusterHub",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"imagePullSecret": "quay-secret",
			},
		},
	}
}

func init() {
	klog.SetOutput(GinkgoWriter)
	klog.InitFlags(nil)

	flag.StringVar(&kubeadminUser, "kubeadmin-user", "kubeadmin", "Provide the kubeadmin credential for the cluster under test (e.g. -kubeadmin-user=\"xxxxx\").")
	flag.StringVar(&kubeadminCredential, "kubeadmin-credential", "",
		"Provide the kubeadmin credential for the cluster under test (e.g. -kubeadmin-credential=\"xxxxx-xxxxx-xxxxx-xxxxx\").")
	flag.StringVar(&baseDomain, "base-domain", "", "Provide the base domain for the cluster under test (e.g. -base-domain=\"demo.red-chesterfield.com\").")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Location of the kubeconfig to use; defaults to KUBECONFIG if not set")

	flag.StringVar(&optionsFile, "options", "", "Location of an \"options.yaml\" file to provide input for various tests")

}

var _ = BeforeSuite(func() {
	By("Setup Hub client")
	// gvrClusterregistry = schema.GroupVersionResource{Group: "clusterregistry.k8s.io", Version: "v1alpha1", Resource: "clusters"}
	// gvrKlusterletconfig = schema.GroupVersionResource{Group: "agent.open-cluster-management.io", Version: "v1beta1", Resource: "klusterletconfigs"}
	// gvrClusterdeployment = schema.GroupVersionResource{Group: "hive.openshift.io", Version: "v1", Resource: "clusterdeployments"}
	// gvrSyncset = schema.GroupVersionResource{Group: "hive.openshift.io", Version: "v1", Resource: "syncsets"}
	// gvrSelectorsyncset = schema.GroupVersionResource{Group: "hive.openshift.io", Version: "v1", Resource: "selectorsyncsets"}
	// gvrSecret = schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
	// gvrServiceaccount = schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}
	gvrMultiClusterHub = schema.GroupVersionResource{Group: "operators.open-cluster-management.io", Version: "v1beta1", Resource: "multiclusterhubs"}
	gvrSubscription = schema.GroupVersionResource{Group: "apps.open-cluster-management.io", Version: "v1", Resource: "subscriptions"}
	clientHub = NewKubeClient("", "", "")
	clientHubDynamic = NewKubeClientDynamic("", "", "")
	defaultImageRegistry = "quay.io/open-cluster-management"
	defaultImagePullSecretName = "multiclusterhub-operator-pull-secret"
	testName = "multiclusterhub-test"
	testNamespace = "open-cluster-management"
	multiClusterHubRepo = "multiclusterhub-repo"
	appsubs = [...]string{"application-chart-sub", "cert-manager-sub", "cert-manager-webhook-sub", "configmap-watcher-sub", "console-chart-sub",
		"grc-sub", "kui-web-terminal-sub", "management-ingress-sub", "multicluster-mongodb-sub", "rcm-sub", "search-prod-sub", "topology-sub"}
	By("Create Namesapce if needed")
	namespaces := clientHub.CoreV1().Namespaces()
	if _, err := namespaces.Get(testNamespace, metav1.GetOptions{}); err != nil && errors.IsNotFound(err) {
		Expect(namespaces.Create(&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		})).NotTo(BeNil())
	}
	Expect(namespaces.Get(testNamespace, metav1.GetOptions{})).NotTo(BeNil())
})

func TestMulticloudhubOperator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MulticloudhubOperator Suite")
}

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

func NewKubeClientDynamic(url, kubeconfig, context string) dynamic.Interface {
	klog.V(5).Infof("Create kubeclient dynamic for url %s using kubeconfig path %s\n", url, kubeconfig)
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

// deleteIfExists deletes resources by using gvr & name & namespace, will wait for deletion to complete by using eventually
func deleteIfExists(clientHubDynamic dynamic.Interface, gvr schema.GroupVersionResource, name, namespace string) {
	ns := clientHubDynamic.Resource(gvr).Namespace(namespace)
	if _, err := ns.Get(name, metav1.GetOptions{}); err != nil {
		Expect(errors.IsNotFound(err)).To(Equal(true))
		return
	}
	Expect(func() error {
		// possibly already got deleted
		err := ns.Delete(name, nil)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		return nil
	}()).To(BeNil())

	By("Wait for deletion")
	Eventually(func() error {
		var err error
		_, err = ns.Get(name, metav1.GetOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		if err == nil {
			return fmt.Errorf("found object %s in namespace %s after deletion", name, namespace)
		}
		return nil
	}, 10, 1).Should(BeNil())
}

// createNewUnstructured creates resources by using gvr & obj, will get object after create.
func createNewUnstructured(
	clientHubDynamic dynamic.Interface,
	gvr schema.GroupVersionResource,
	obj *unstructured.Unstructured,
	name, namespace string,
) {
	ns := clientHubDynamic.Resource(gvr).Namespace(namespace)
	Expect(ns.Create(obj, metav1.CreateOptions{})).NotTo(BeNil())
	Expect(ns.Get(name, metav1.GetOptions{})).NotTo(BeNil())
}

// isOwner checks if obj is owned by owner, obj can either be unstructured or ObjectMeta
func isOwner(owner *unstructured.Unstructured, obj interface{}) bool {
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

// listAppSubs keeps polling to get the object for timeout seconds until wantFound is met (true for found, false for not found)
func listAppSubs(
	clientHubDynamic dynamic.Interface,
	gvr schema.GroupVersionResource,
	namespace string,
	wantFound bool,
	timeout int,
	expectedTotal int,
) *unstructured.UnstructuredList {
	if timeout < 1 {
		timeout = 1
	}
	var obj *unstructured.UnstructuredList

	Eventually(func() error {
		var err error
		namespace := clientHubDynamic.Resource(gvr).Namespace(namespace)
		labelSelector := fmt.Sprintf("installer.name=%s, installer.namespace=%s", testName, testNamespace)
		listOptions := metav1.ListOptions{
			LabelSelector: labelSelector,
			Limit:         100,
		}
		obj, err = namespace.List(listOptions)
		// obj, err = namespace.Get(name, metav1.GetOptions{})
		if wantFound && err != nil {
			return err
		}
		if !wantFound && err == nil {
			return fmt.Errorf("expected to return IsNotFound error")
		}
		if !wantFound && err != nil && !errors.IsNotFound(err) {
			return err
		}
		if len(obj.Items) < expectedTotal {
			return fmt.Errorf("Not all Appsubs created in time. %d/%d appsubs found.", len(obj.Items), expectedTotal)
		}
		return nil
	}, timeout, 1).Should(BeNil())
	if wantFound {
		return obj
	}
	return nil
}
