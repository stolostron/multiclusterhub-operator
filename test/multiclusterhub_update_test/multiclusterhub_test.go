// Copyright (c) 2020 Red Hat, Inc.
package multiclusterhub_update_test

import (
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

	By("Beginning Basic Update Test Suite ...")
	It("Install Default MCH CR", func() {
		By("Creating MultiClusterHub")
		utils.ValidateMCH(CreateDefaultMCH())
	})
})

func CreateDefaultMCH() *unstructured.Unstructured {
	mch := utils.NewMultiClusterHub(utils.MCHName, utils.MCHNamespace)
	utils.CreateNewUnstructured(utils.DynamicKubeClient, utils.GVRMultiClusterHub, mch, utils.MCHName, utils.MCHNamespace)
	return mch
}
