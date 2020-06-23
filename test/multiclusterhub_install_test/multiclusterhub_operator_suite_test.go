// Copyright (c) 2020 Red Hat, Inc.
package multiclusterhub_install_test

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	utils "github.com/open-cluster-management/multicloudhub-operator/test/utils"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog"
)

var ()

func init() {
	klog.SetOutput(GinkgoWriter)
	klog.InitFlags(nil)
}

var _ = BeforeSuite(func() {

	// Create Resources
	By("Creating Namespace if needed")
	namespaces := utils.KubeClient.CoreV1().Namespaces()
	if _, err := namespaces.Get(context.TODO(), utils.MCHNamespace, metav1.GetOptions{}); err != nil && errors.IsNotFound(err) {
		ns := utils.NewNamespace(utils.MCHNamespace)
		Expect(namespaces.Create(context.TODO(), ns, metav1.CreateOptions{})).NotTo(BeNil())
	}
	Expect(namespaces.Get(context.TODO(), utils.MCHNamespace, metav1.GetOptions{})).NotTo(BeNil())

	By("Creating OperatorGroup if needed")
	operatorGroups := utils.DynamicKubeClient.Resource(utils.GVROperatorGroup).Namespace(utils.MCHNamespace)
	ogList, err := operatorGroups.List(context.TODO(), metav1.ListOptions{})
	if err != nil || len(ogList.Items) < 1 {
		utils.CreateNewUnstructured(
			utils.DynamicKubeClient, utils.GVROperatorGroup, utils.NewOperatorGroup(utils.MCHNamespace), "default", utils.MCHNamespace,
		)
	}
	Expect(operatorGroups.Get(context.TODO(), "default", metav1.GetOptions{})).NotTo(BeNil())

	By("Creating 'multiclusterhub-operator-pull-secret' Secret if needed")
	secrets := utils.KubeClient.CoreV1().Secrets(utils.MCHNamespace)
	if _, err := secrets.Get(context.TODO(), utils.MCHPullSecretName, metav1.GetOptions{}); err != nil && errors.IsNotFound(err) {
		pullSecret := utils.NewPullSecret(utils.MCHPullSecretName, utils.MCHNamespace)
		Expect(secrets.Create(context.TODO(), pullSecret, metav1.CreateOptions{})).NotTo(BeNil())
	}
	Expect(secrets.Get(context.TODO(), utils.MCHPullSecretName, metav1.GetOptions{})).NotTo(BeNil())

	By("Creating ACM Operator Subscription")
	subscription := utils.DynamicKubeClient.Resource(utils.GVRSub).Namespace(utils.MCHNamespace)
	_, err = subscription.Get(context.TODO(), utils.ACMSubscriptionName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		acmSub := utils.NewACMSubscription(utils.MCHNamespace)
		utils.CreateNewUnstructured(utils.DynamicKubeClient, utils.GVRSub, acmSub, utils.ACMSubscriptionName, utils.MCHNamespace)
	}
	Expect(subscription.Get(context.TODO(), utils.ACMSubscriptionName, metav1.GetOptions{})).NotTo(BeNil())

	By("Wait for MCH Operator to be available")
	var deploy *appsv1.Deployment
	When("Subscription is created, wait for Operator to run", func() {
		Eventually(func() error {
			var err error
			klog.V(1).Info("Wait multiclusterhub-operator deployment...")
			deploy, err = utils.KubeClient.AppsV1().Deployments(utils.MCHNamespace).Get(context.TODO(), utils.MCHOperatorName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if deploy.Status.AvailableReplicas == 0 {
				return fmt.Errorf("MCH Operator not available")
			}
			return err
		}, 45, 1).Should(BeNil())
		klog.V(1).Info("MCH Operator deployment available")
	})

	By("Ensuring MCH CR is not yet installed")
	mchLink := utils.DynamicKubeClient.Resource(utils.GVRMultiClusterHub).Namespace(utils.MCHNamespace)
	mchList, err := mchLink.List(context.TODO(), metav1.ListOptions{})
	Expect(err).To(BeNil())
	Expect(len(mchList.Items)).Should(BeEquivalentTo(0))
})

func TestMultiClusterHubOperatorInstall(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MultiClusterHubOperator Install Suite")
}

// listAppSubs keeps polling to get the object for timeout seconds
func listByGVR(clientHubDynamic dynamic.Interface, gvr schema.GroupVersionResource, namespace string, timeout int, expectedTotal int) *unstructured.UnstructuredList {
	if timeout < 1 {
		timeout = 1
	}
	var obj *unstructured.UnstructuredList

	Eventually(func() error {
		var err error
		namespace := clientHubDynamic.Resource(gvr).Namespace(namespace)

		// labelSelector := fmt.Sprintf("installer.name=%s, installer.namespace=%s", mchName, mchNamespace)
		// listOptions := metav1.ListOptions{
		// 	LabelSelector: labelSelector,
		// 	Limit:         100,
		// }
		// obj, err = namespace.List(context.TODO(), listOptions)

		obj, err = namespace.List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return err
		}
		if len(obj.Items) < expectedTotal {
			return fmt.Errorf("Not all Appsubs created in time. %d/%d appsubs found.", len(obj.Items), expectedTotal)
		}
		return nil
	}, timeout, 1).Should(BeNil())
	return obj
}
