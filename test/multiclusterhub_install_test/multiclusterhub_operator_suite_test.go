// Copyright (c) 2020 Red Hat, Inc.
package multiclusterhub_install_test

import (
	"context"
	"flag"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	utils "github.com/open-cluster-management/multiclusterhub-operator/test/utils"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog"
)

var (
	optionsFile         string
	baseDomain          string
	kubeadminUser       string
	kubeadminCredential string
	kubeconfig          string
	reportFile          string
)

func init() {
	klog.SetOutput(GinkgoWriter)
	klog.InitFlags(nil)

	flag.StringVar(&kubeadminUser, "kubeadmin-user", "kubeadmin", "Provide the kubeadmin credential for the cluster under test (e.g. -kubeadmin-user=\"xxxxx\").")
	flag.StringVar(&kubeadminCredential, "kubeadmin-credential", "",
		"Provide the kubeadmin credential for the cluster under test (e.g. -kubeadmin-credential=\"xxxxx-xxxxx-xxxxx-xxxxx\").")
	flag.StringVar(&baseDomain, "base-domain", "", "Provide the base domain for the cluster under test (e.g. -base-domain=\"demo.red-chesterfield.com\").")

	flag.StringVar(&optionsFile, "options", "", "Location of an \"options.yaml\" file to provide input for various tests")
	flag.StringVar(&reportFile, "report-file", "results.xml", "Provide the path to where the junit results will be printed.")

}

var _ = BeforeSuite(func() {
	By("Creating OCM Operator Subscription")
	subscription := utils.DynamicKubeClient.Resource(utils.GVRSub).Namespace(utils.MCHNamespace)
	_, err := subscription.Get(context.TODO(), utils.OCMSubscriptionName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		ocmSub := utils.NewOCMSubscription(utils.MCHNamespace)
		utils.CreateNewUnstructured(utils.DynamicKubeClient, utils.GVRSub, ocmSub, utils.OCMSubscriptionName, utils.MCHNamespace)
	}
	Expect(subscription.Get(context.TODO(), utils.OCMSubscriptionName, metav1.GetOptions{})).NotTo(BeNil())

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
	junitReporter := reporters.NewJUnitReporter(reportFile)
	RunSpecsWithDefaultAndCustomReporters(t, "MultiClusterHubOperator Install Suite", []Reporter{junitReporter})
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
