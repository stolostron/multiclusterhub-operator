// Copyright (c) 2020 Red Hat, Inc.
package multiclusterhub_install_test

import (
	"context"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	utils "github.com/open-cluster-management/multicloudhub-operator/test/utils"
)

var _ = Describe("Multiclusterhub", func() {

	BeforeEach(func() {
		By("Attempting to delete MultiClusterHub if it exists")
		// utils.DeleteIfExists(utils.DynamicKubeClient, utils.GVRMultiClusterHub, utils.MCHName, utils.MCHNamespace, true)

		// Expect(utils.ValidateDelete(utils.DynamicKubeClient)).Should(BeNil())
	})

	if os.Getenv("full_test_suite") == "true" {
		By("Beginning Full Install Test Suite ...")
		FullInstallTestSuite()
	} else {
		By("Beginning Basic Install Test Suite ...")
		It("Install Default MCH CR", func() {
			By("Creating MultiClusterHub")
			utils.ValidateMCH(utils.CreateDefaultMCH())
		})
	}
})

func FullInstallTestSuite() {

	It("Testing Image Overrides Configmap Sad Cases", func() {
		// By("Creating MultiClusterHub")
		// err := utils.ValidateMCH(utils.CreateDefaultMCH())
		// Expect(err).To(BeNil())

		By("- Overrwite Image Overrides Configmap")
		currentVersion, err := utils.GetCurrentVersionFromMCH()
		Expect(err).To(BeNil())
		configmap, err := utils.KubeClient.CoreV1().ConfigMaps(utils.MCHNamespace).Get(context.TODO(), fmt.Sprintf("mch-image-manifest-%s", currentVersion), metav1.GetOptions{})
		Expect(err).To(BeNil())

		// Clear all data in configmap
		configmap.Data = make(map[string]string)
		configmap, err = utils.KubeClient.CoreV1().ConfigMaps(utils.MCHNamespace).Update(context.TODO(), configmap, metav1.UpdateOptions{})
		Expect(err).To(BeNil())

		Expect(len(configmap.Data)).Should(Equal(0))

		configmap, err = utils.KubeClient.CoreV1().ConfigMaps(utils.MCHNamespace).Get(context.TODO(), fmt.Sprintf("mch-image-manifest-%s", currentVersion), metav1.GetOptions{})
		Expect(len(configmap.Data)).ShouldNot(Equal(0))

		return
	})

	// It("Installing MCH with bad pull secret - should have Pending status", func() {
	// 	By("Creating MultiClusterHub")
	// 	err := utils.ValidateMCHUnsuccessful(utils.CreateMCHBadPullSecret())
	// 	if err != nil {
	// 		fmt.Println(fmt.Sprintf("Error: %s\n", err.Error()))
	// 		return
	// 	}
	// 	return
	// })

	// totalAttempts := 10
	// for i := 1; i <= totalAttempts; i++ {
	// 	It(fmt.Sprintf("Installing MCH - Attempt %d of %d", i, totalAttempts), func() {
	// 		By("Creating MultiClusterHub")
	// 		err := utils.ValidateMCH(utils.CreateDefaultMCH())
	// 		if err != nil {
	// 			fmt.Println(fmt.Sprintf("Error: %s\n", err.Error()))
	// 			return
	// 		}
	// 		return
	// 	})
	// }
}
