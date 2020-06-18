package multiclusterhub_uninstall_test

import (
	"context"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	utils "github.com/open-cluster-management/multicloudhub-operator/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

func init() {
	klog.SetOutput(GinkgoWriter)
	klog.InitFlags(nil)
}

var _ = AfterSuite(func() {
	By("Deleting ACM Subscription")
	// Delete Subscription
	err := utils.DynamicKubeClient.Resource(utils.GVRSub).Namespace(utils.MCHNamespace).Delete(context.TODO(), utils.ACMSubscriptionName, metav1.DeleteOptions{})
	Expect(err).Should(BeNil())

	// Delete CSVs
	csvLink := utils.DynamicKubeClient.Resource(utils.GVRCSV).Namespace(utils.MCHNamespace)
	csvList, err := csvLink.List(context.TODO(), metav1.ListOptions{})
	Expect(err).Should(BeNil())
	for _, csv := range csvList.Items {
		for _, csvName := range utils.CSVNameSlice {
			if strings.Contains(csv.GetName(), csvName) {
				err = csvLink.Delete(context.TODO(), csv.GetName(), metav1.DeleteOptions{})
			}
		}
	}
	// Delete Secret
	err = utils.KubeClient.CoreV1().Secrets(utils.MCHNamespace).Delete(context.TODO(), utils.MCHPullSecretName, metav1.DeleteOptions{})
	Expect(err).Should(BeNil())

	// Delete OperatorGroup
	err = utils.DynamicKubeClient.Resource(utils.GVROperatorGroup).Namespace(utils.MCHNamespace).Delete(context.TODO(), "default", metav1.DeleteOptions{})
	Expect(err).Should(BeNil())
})

func TestMultiClusterHubOperatorUninstall(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MultiClusterHubOperator Suite")
}
