// Copyright (c) 2020 Red Hat, Inc.
package multiclusterhub_install_test

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	utils "github.com/open-cluster-management/multicloudhub-operator/test/utils"
)

var _ = Describe("Multiclusterhub", func() {

	BeforeEach(func() {
		By("Attempting to delete MultiClusterHub if it exists")
		utils.DeleteIfExists(utils.DynamicKubeClient, utils.GVRMultiClusterHub, utils.MCHName, utils.MCHNamespace, true)

		Expect(utils.ValidateDelete(utils.DynamicKubeClient)).Should(BeNil())
	})

	if os.Getenv("full_test_suite") == "true" {
		By("Beginning Full Install Test Suite ...")

		It(fmt.Sprintf("Installing MCH with bad pull secret - should have Pending status"), func() {
			By("Creating MultiClusterHub")
			utils.CreateMCHBadPullSecret()
			err := utils.ValidateMCHUnsuccessful()
			if err != nil {
				fmt.Println(fmt.Sprintf("Error: %s\n", err.Error()))
				return
			}
			return
		})

		totalAttempts := 2
		for i := 1; i <= totalAttempts; i++ {
			ok := It(fmt.Sprintf("Installing MCH - Attempt %d of %d", i, totalAttempts), func() {
				By("Creating MultiClusterHub")
				utils.CreateDefaultMCH()
				if err := utils.ValidateComponentStatusExist(); err != nil {
					fmt.Println(fmt.Sprintf("Error: %s\n", err.Error()))
					return
				}
				err := utils.ValidateMCH()
				if err != nil {
					fmt.Println(fmt.Sprintf("Error: %s\n", err.Error()))
					return
				}
				return
			})
			if !ok {
				break
			}
		}
	} else {
		By("Beginning Basic Install Test Suite ...")
		It("Install Default MCH CR", func() {
			By("Creating MultiClusterHub")
			utils.CreateDefaultMCH()
			if err := utils.ValidateComponentStatusExist(); err != nil {
				fmt.Println(fmt.Sprintf("Error: %s\n", err.Error()))
				return
			}
			err := utils.ValidateMCH()
			if err != nil {
				fmt.Println(fmt.Sprintf("Error: %s\n", err.Error()))
				return
			}
			return
		})
	}
})
