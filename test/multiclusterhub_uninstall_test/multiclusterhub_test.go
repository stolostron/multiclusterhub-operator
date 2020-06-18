// Copyright (c) 2020 Red Hat, Inc.
package multiclusterhub_uninstall_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	utils "github.com/open-cluster-management/multicloudhub-operator/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

var _ = Describe("Multiclusterhub", func() {

	It("Delete MCH CR", func() {
		By("Deleting MultiClusterHub")
		utils.DeleteIfExists(utils.DynamicKubeClient, utils.GVRMultiClusterHub, utils.MCHName, utils.MCHNamespace)

		By("Waiting For HelmReleases to be Deleted")
		When("When MultiClusterHub is deleted, wait for all helmreleases to deleted", func() {
			Eventually(func() error {
				helmReleaseLink := utils.DynamicKubeClient.Resource(utils.GVRHelmRelease)
				helmReleases, err := helmReleaseLink.List(context.TODO(), metav1.ListOptions{})
				Expect(err).Should(BeNil())

				if len(helmReleases.Items) == 0 {
					return nil
				}
				return fmt.Errorf("%d helmreleases left to be uninstalled.", len(helmReleases.Items))
			}, 60, 1).Should(BeNil())
			klog.V(1).Info("All Helmreleases deleted")
		})
	})
})
