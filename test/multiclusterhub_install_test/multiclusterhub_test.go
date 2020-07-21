// Copyright (c) 2020 Red Hat, Inc.
package multiclusterhub_install_test

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	utils "github.com/open-cluster-management/multicloudhub-operator/test/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("Multiclusterhub", func() {

	BeforeEach(func() {
		By("Attempting to delete MultiClusterHub if it exists")
		utils.DeleteIfExists(utils.DynamicKubeClient, utils.GVRMultiClusterHub, utils.MCHName, utils.MCHNamespace)

		Expect(utils.EnsureHelmReleasesAreRemoved(utils.DynamicKubeClient)).Should(BeNil())
	})

	if os.Getenv("full_test_suite") == "true" {
		By("Beginning Full Install Test Suite ...")
		totalAttempts := 10
		for i := 1; i <= totalAttempts; i++ {
			ok := It(fmt.Sprintf("Installing MCH - Attempt %d of %d", i, totalAttempts), func() {
				By("Creating MultiClusterHub")
				err := utils.ValidateMCH(CreateDefaultMCH())
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
			utils.ValidateMCH(CreateDefaultMCH())
		})
	}
})

func CreateDefaultMCH() *unstructured.Unstructured {
	mch := utils.NewMultiClusterHub(utils.MCHName, utils.MCHNamespace)
	utils.CreateNewUnstructured(utils.DynamicKubeClient, utils.GVRMultiClusterHub, mch, utils.MCHName, utils.MCHNamespace)
	return mch
}
