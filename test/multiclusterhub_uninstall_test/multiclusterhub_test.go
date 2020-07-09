// Copyright (c) 2020 Red Hat, Inc.
package multiclusterhub_uninstall_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	utils "github.com/open-cluster-management/multicloudhub-operator/test/utils"
)

var _ = Describe("Multiclusterhub", func() {

	It("Delete MCH CR", func() {
		By("Deleting MultiClusterHub")
		utils.DeleteIfExists(utils.DynamicKubeClient, utils.GVRMultiClusterHub, utils.MCHName, utils.MCHNamespace)

		Expect(utils.EnsureHelmReleasesAreRemoved(utils.DynamicKubeClient)).Should(BeNil())
	})
})
