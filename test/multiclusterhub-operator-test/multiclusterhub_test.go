package multicloudhub_operator_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

var _ = Describe("Multiclusterhub", func() {

	AfterEach(func() {
		By("Delete MultiClusterHub if it exists")
		deleteIfExists(clientHubDynamic, gvrMultiClusterHub, testName, testNamespace)

	})

	It("MCH Should Create MultiClusterHub-Repo", func() {
		By("Creating MultiClusterHub")
		mch := newMultiClusterHub(testName, testNamespace)
		createNewUnstructured(clientHubDynamic, gvrMultiClusterHub, mch, testName, testNamespace)

		var deploy *appsv1.Deployment
		When("MultiClusterHub is created, wait for MultiClusterHub Repo to be available", func() {
			Eventually(func() error {
				var err error
				klog.V(1).Info("Wait MCH Repo deployment...")
				deploy, err = clientHub.AppsV1().Deployments(testNamespace).Get(context.TODO(), multiClusterHubRepo, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if deploy.Status.AvailableReplicas == 0 {
					return fmt.Errorf("MCH Repo not available")
				}
				return err
			}, 30, 1).Should(BeNil())
			klog.V(1).Info("MCH Repo deployment available")
		})
		By("Checking ownerRef", func() {
			Expect(isOwner(mch, &deploy.ObjectMeta)).To(Equal(true))
		})
	})

	It("MCH Should Create all AppSubs", func() {
		By("Creating MultiClusterHub")
		mch := newMultiClusterHub(testName, testNamespace)
		createNewUnstructured(clientHubDynamic, gvrMultiClusterHub, mch, testName, testNamespace)

		When("MultiClusterHub is created, wait for Application Subscriptions to be available", func() {
			Eventually(func() error {
				unstructuredAppSubs := listAppSubs(clientHubDynamic, gvrSubscription, testNamespace, true, 60, len(appsubs))

				// ready := false
				for _, appsub := range unstructuredAppSubs.Items {
					if _, ok := appsub.Object["status"]; !ok {
						return fmt.Errorf("Appsub: %s has no 'status' field", appsub.GetName())
					}
					status, ok := appsub.Object["status"].(map[string]interface{})
					if !ok || status == nil {
						return fmt.Errorf("Appsub: %s has no 'status' map", appsub.GetName())
					}
					if status["message"] != "Active" || status["phase"] != "Subscribed" {
						return fmt.Errorf("Appsub: %s is not active", appsub.GetName())
					}
				}
				return nil
			}, 180, 1).Should(BeNil())
		})
	})
})
