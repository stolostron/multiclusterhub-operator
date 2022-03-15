// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package multiclusterhub_install_test

import (
	"context"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	utils "github.com/stolostron/multiclusterhub-operator/test/function_tests/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

		By("Attempting to delete Image Overrides ConfigMap with bad image reference if it exists")
		err := utils.DeleteConfigMapIfExists(utils.ImageOverridesCMBadImageName, utils.MCHNamespace)
		Expect(err).Should(BeNil())
	})

	if os.Getenv("full_test_suite") == "true" {
		By("Beginning Full Install Test Suite ...")
		FullInstallTestSuite()
	} else {
		By("Beginning Basic Install Test Suite ...")
		It("Install Default MCH CR", func() {
			By("Creating MultiClusterHub")
			start := time.Now()

			utils.CreateMCHNotManaged()
			if err := utils.ValidateStatusesExist(); err != nil {
				fmt.Println(fmt.Sprintf("Error: %s\n", err.Error()))
				return
			}
			err := utils.ValidateMCH()
			if err != nil {
				fmt.Println(fmt.Sprintf("Error: %s\n", err.Error()))
				return
			}
			fmt.Printf("Installation Time: %s\n", time.Since(start))
		})
	}
})

func FullInstallTestSuite() {
	Describe("Test basic MCH Features (Tolerations, pull policy, image overrides)", testMCHFeatures())
	Describe("Test Hiveconfig", testHiveConfig())
	Describe("Test MCE", testMCE())
	Describe("Test MCH API fields", testMCHAPI())
	Describe("Test MCH sanity checks", testSanityChecks())
	Describe("Test managedcluster", testManagedCluster())
	It("- If the cluster backup namespace is created", func() {
		By("- Verfiying default ")
		utils.CreateMCHNotManaged()
		err := utils.ValidateMCH()
		Expect(err).To(BeNil())

		By("- enabling cluster backup and checking for namespace")

		mch, err := utils.DynamicKubeClient.Resource(utils.GVRMultiClusterHub).Namespace(utils.MCHNamespace).Get(context.TODO(), utils.MCHName, metav1.GetOptions{})
		Expect(err).To(BeNil())
		mch.Object["spec"].(map[string]interface{})["enableClusterBackup"] = true
		mch, err = utils.DynamicKubeClient.Resource(utils.GVRMultiClusterHub).Namespace(utils.MCHNamespace).Update(context.TODO(), mch, metav1.UpdateOptions{})
		Expect(err).To(BeNil())

		Eventually(func() error {
			_, err = utils.DynamicKubeClient.Resource(utils.GVRNamespace).Get(context.TODO(), "open-cluster-management-backup", metav1.GetOptions{})
			return err
		}, utils.GetWaitInMinutes()*2, 1).Should(BeNil())

	})

}
