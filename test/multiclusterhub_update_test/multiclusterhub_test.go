// Copyright (c) 2020 Red Hat, Inc.
package multiclusterhub_update_test

import (
	"context"
	"fmt"
	"os"

	"github.com/Masterminds/semver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	utils "github.com/open-cluster-management/multiclusterhub-operator/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

var _ = Describe("Multiclusterhub", func() {

	BeforeEach(func() {
		By("Attempting to delete MultiClusterHub if it exists")
		utils.DeleteIfExists(utils.DynamicKubeClient, utils.GVRMultiClusterHub, utils.MCHName, utils.MCHNamespace, true)

		Eventually(func() error {
			err := utils.ValidateDelete(utils.DynamicKubeClient)
			if err != nil {
				return err
			}
			return nil
		}, utils.GetWaitInMinutes()*60, 1).Should(BeNil())
	})

	By("Beginning Basic Update Test Suite ...")
	It("Install Default MCH CR", func() {
		By("Creating MultiClusterHub")
		utils.CreateDefaultMCH()
		utils.ValidateMCH()

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

		var phaseError error
		When("Operator Is Upgraded, wait for MCH Version to Update", func() {
			Eventually(func() error {
				version, err := utils.GetCurrentVersionFromMCH()
				if err != nil {
					return fmt.Errorf("MultiClusterHub: %s status has no 'currentVersion' field", utils.MCHName)
				}
				if version != os.Getenv("updateVersion") {
					if phaseError == nil {
						phaseError = utils.ValidatePhase("Updating")
					}
					return fmt.Errorf("MCH: %s current version mismatch '%s' != %s", utils.MCHName, version, os.Getenv("updateVersion"))
				}
				Expect(version).To(Equal(os.Getenv("updateVersion")))
				Expect(version).To(Equal(os.Getenv("updateVersion")))
				return nil
			}, utils.GetWaitInMinutes()*60, 1).Should(BeNil())
			klog.V(1).Info("MCH Operator upgraded successfully")
		})

		Expect(phaseError).To(BeNil())
		utils.ValidateMCH()

		startVersion, err := semver.NewVersion(os.Getenv(("startVersion")))
		Expect(err).Should(BeNil())
		updateVersion, err := semver.NewVersion(os.Getenv(("updateVersion")))
		Expect(err).Should(BeNil())

		c, err := semver.NewConstraint(">= 2.2.5")
		Expect(err).Should(BeNil())
		configmapCount := 0
		if c.Check(startVersion) {
			configmapCount = 2
		} else if c.Check(updateVersion) {
			configmapCount = 1
		}

		if configmapCount > 0 {
			By("Validating Image Manifest Configmaps Exist")
			labelSelector := fmt.Sprintf("ocm-configmap-type=%s", "image-manifest")
			listOptions := metav1.ListOptions{
				LabelSelector: labelSelector,
				Limit:         100,
			}
			configmaps, err := utils.KubeClient.CoreV1().ConfigMaps(utils.MCHNamespace).List(context.TODO(), listOptions)
			Expect(err).To(BeNil())
			Expect(len(configmaps.Items)).Should(Equal(configmapCount))
		}
	})
})
