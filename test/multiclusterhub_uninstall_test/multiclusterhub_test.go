// Copyright (c) 2020 Red Hat, Inc.
package multiclusterhub_uninstall_test

import (
	"context"
	"fmt"
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	utils "github.com/open-cluster-management/multicloudhub-operator/test/utils"
	"k8s.io/client-go/dynamic"
)

var _ = Describe("Multiclusterhub", func() {

	It("Deleting and Validating MCH CR", func() {
		By("Deleting MultiClusterHub")
		utils.DeleteIfExists(utils.DynamicKubeClient, utils.GVRMultiClusterHub, utils.MCHName, utils.MCHNamespace, true)

		Expect(utils.ValidateDelete(utils.DynamicKubeClient)).Should(BeNil())
	})

	if os.Getenv("full_test_suite") == "true" {
		It("SAD CASE: Fail to remove a helmrelease (Left behind finalizer)", func() {
			By("Creating MultiClusterHub")
			utils.CreateDefaultMCH()
			utils.ValidateMCH()
			AddFinalizerToHelmRelease(utils.DynamicKubeClient)
			utils.DeleteIfExists(utils.DynamicKubeClient, utils.GVRMultiClusterHub, utils.MCHName, utils.MCHNamespace, false)
			Expect(utils.ValidateDelete(utils.DynamicKubeClient)).ShouldNot(BeNil())
			Eventually(func() error {
				err := RemoveFinalizerFromHelmRelease(utils.DynamicKubeClient)
				if err != nil {
					return err
				}
				return nil
			}, 20, 1).Should(BeNil())
			utils.DeleteIfExists(utils.DynamicKubeClient, utils.GVRMultiClusterHub, utils.MCHName, utils.MCHNamespace, true)
			Expect(utils.ValidateDelete(utils.DynamicKubeClient)).Should(BeNil())
		})
	}
})

// AddFinalizerToHelmRelease ...
func AddFinalizerToHelmRelease(clientHubDynamic dynamic.Interface) error {
	By("Adding a test finalizer to a helmrelease")

	appSubLink := clientHubDynamic.Resource(utils.GVRAppSub).Namespace(utils.MCHNamespace)
	appSub, err := appSubLink.Get(context.TODO(), "application-chart-sub", metav1.GetOptions{})
	Expect(err).Should(BeNil())

	helmReleaseName := fmt.Sprintf("%s-%s", strings.Replace(appSub.GetName(), "-sub", "", 1), appSub.GetUID()[0:5])

	helmReleaseLink := clientHubDynamic.Resource(utils.GVRHelmRelease).Namespace(utils.MCHNamespace)
	helmRelease, err := helmReleaseLink.Get(context.TODO(), helmReleaseName, metav1.GetOptions{})
	Expect(err).Should(BeNil())

	finalizers := []string{"test-finalizer"}

	helmRelease.SetFinalizers(finalizers)
	_, err = helmReleaseLink.Update(context.TODO(), helmRelease, metav1.UpdateOptions{})
	Expect(err).Should(BeNil())

	return nil
}

// RemoveFinalizerFromHelmRelease ...
func RemoveFinalizerFromHelmRelease(clientHubDynamic dynamic.Interface) error {
	By("Removing test finalizer from helmrelease")

	labelSelector := fmt.Sprintf("installer.name=%s, installer.namespace=%s", utils.MCHName, utils.MCHNamespace)
	listOptions := metav1.ListOptions{
		LabelSelector: labelSelector,
		Limit:         100,
	}

	helmReleaseLink := clientHubDynamic.Resource(utils.GVRHelmRelease).Namespace(utils.MCHNamespace)
	helmReleases, err := helmReleaseLink.List(context.TODO(), listOptions)
	if err != nil {
		return err
	}

	helmRelease := helmReleases.Items[0]
	helmRelease.SetFinalizers([]string{})

	_, err = helmReleaseLink.Update(context.TODO(), &helmRelease, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}
