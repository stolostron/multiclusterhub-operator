// Copyright (c) 2020 Red Hat, Inc.
package multiclusterhub_update_test

import (
	"context"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	utils "github.com/open-cluster-management/multicloudhub-operator/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog"
)

var _ = Describe("Multiclusterhub", func() {

	BeforeEach(func() {
		By("Attempting to delete MultiClusterHub if it exists")
		utils.DeleteIfExists(utils.DynamicKubeClient, utils.GVRMultiClusterHub, utils.MCHName, utils.MCHNamespace)

		Expect(utils.ValidateDelete(utils.DynamicKubeClient)).Should(BeNil())
	})

	By("Beginning Basic Update Test Suite ...")
	It("Install Default MCH CR", func() {
		By("Creating MultiClusterHub")
		defaultMCH := CreateDefaultMCH()
		utils.ValidateMCH(defaultMCH)

		By("Approving Update InstallPlan")
		subscription := utils.DynamicKubeClient.Resource(utils.GVRSub).Namespace(utils.MCHNamespace)
		acmSub, err := subscription.Get(context.TODO(), utils.OCMSubscriptionName, metav1.GetOptions{})
		Expect(err).To(BeNil())

		installPlanName, err := utils.GetInstallPlanNameFromSub(acmSub)
		Expect(err).To(BeNil())

		installPlanLink := utils.DynamicKubeClient.Resource(utils.GVRInstallPlan).Namespace(utils.MCHNamespace)
		installPlan, err := installPlanLink.Get(context.TODO(), installPlanName, metav1.GetOptions{})
		Expect(err).To(BeNil())
		approvedInstallPlan, err := utils.MarkInstallPlanAsApproved(installPlan)
		Expect(err).To(BeNil())
		installPlan, err = installPlanLink.Update(context.TODO(), approvedInstallPlan, metav1.UpdateOptions{})
		Expect(err).To(BeNil())

		When("Operator Is Upgraded, wait for MCH Version to Update", func() {
			Eventually(func() error {
				var err error
				mch, err := utils.DynamicKubeClient.Resource(utils.GVRMultiClusterHub).Namespace(utils.MCHNamespace).Get(context.TODO(), utils.MCHName, metav1.GetOptions{})
				Expect(err).To(BeNil())
				status, ok := mch.Object["status"].(map[string]interface{})
				if !ok || status == nil {
					return fmt.Errorf("MultiClusterHub: %s has no 'status' map", mch.GetName())
				}
				version, ok := status["currentVersion"]
				if !ok {
					return fmt.Errorf("MultiClusterHub: %s status has no 'currentVersion' field", mch.GetName())
				}
				if version != os.Getenv("updateVersion") {
					return fmt.Errorf("MCH: %s current version mismatch '%s' != %s", mch.GetName(), version, os.Getenv("updateVersion"))
				}
				Expect(status["currentVersion"]).To(Equal(os.Getenv("updateVersion")))
				Expect(status["desiredVersion"]).To(Equal(os.Getenv("updateVersion")))
				return nil
			}, 800, 1).Should(BeNil())
			klog.V(1).Info("MCH Operator upgraded successfully")
		})

		utils.ValidateMCH(defaultMCH)
	})
})

func CreateDefaultMCH() *unstructured.Unstructured {
	mch := utils.NewMultiClusterHub(utils.MCHName, utils.MCHNamespace)
	utils.CreateNewUnstructured(utils.DynamicKubeClient, utils.GVRMultiClusterHub, mch, utils.MCHName, utils.MCHNamespace)
	return mch
}
