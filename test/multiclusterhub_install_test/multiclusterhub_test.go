// Copyright (c) 2020 Red Hat, Inc.
package multiclusterhub_install_test

import (
	"context"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

	It("Testing Image Overrides Configmap", func() {
		By("- If configmap is manually overwitten, ensure MCH Operator will overwrite")
		err := utils.ValidateMCH(utils.CreateDefaultMCH())
		Expect(err).To(BeNil())

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

		Eventually(func() error {
			configmap, err = utils.KubeClient.CoreV1().ConfigMaps(utils.MCHNamespace).Get(context.TODO(), fmt.Sprintf("mch-image-manifest-%s", currentVersion), metav1.GetOptions{})
			if len(configmap.Data) == 0 {
				return fmt.Errorf("Configmap has not been updated")
			}
			return nil
		}, 30, 1).Should(BeNil())

		By("- If `mch-imageOverridesCM` annotation is given, ensure Image Overrides Configmap is updated ")

		return
	})

	It("- If `mch-imageOverridesCM` annotation is given, ensure Image Overrides Configmap is updated ", func() {
		By("- Creating Developer Image Overrides Configmap")

		err := utils.ValidateMCH(utils.CreateDefaultMCH())
		Expect(err).To(BeNil())

		configmap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-config",
				Namespace: utils.MCHNamespace,
			},
			Data: map[string]string{
				"overrides.json": `[
					{
					  "image-name": "application-ui",
					  "image-tag": "not-a-real-tag",
					  "image-remote": "quay.io/open-cluster-management",
					  "image-key": "application_ui"
					}
				  ]`,
			},
		}

		// Create configmap overrides
		currentVersion, err := utils.GetCurrentVersionFromMCH()
		Expect(err).To(BeNil())
		_, err = utils.KubeClient.CoreV1().ConfigMaps(utils.MCHNamespace).Create(context.TODO(), configmap, metav1.CreateOptions{})
		Expect(err).To(BeNil())

		// Annotate MCH
		annotations := make(map[string]string)
		annotations["mch-imageOverridesCM"] = "my-config"
		mch, err := utils.DynamicKubeClient.Resource(utils.GVRMultiClusterHub).Namespace(utils.MCHNamespace).Get(context.TODO(), utils.MCHName, metav1.GetOptions{})
		Expect(err).To(BeNil())
		mch.SetAnnotations(annotations)
		mch, err = utils.DynamicKubeClient.Resource(utils.GVRMultiClusterHub).Namespace(utils.MCHNamespace).Update(context.TODO(), mch, metav1.UpdateOptions{})
		Expect(err).To(BeNil())

		Eventually(func() error {
			configmap, err = utils.KubeClient.CoreV1().ConfigMaps(utils.MCHNamespace).Get(context.TODO(), fmt.Sprintf("mch-image-manifest-%s", currentVersion), metav1.GetOptions{})
			if len(configmap.Data) == 0 {
				return fmt.Errorf("Configmap has not been updated")
			}
			if configmap.Data["application_ui"] != "quay.io/open-cluster-management/application-ui:not-a-real-tag" {
				return fmt.Errorf("Configmap has not been updated from overrides CM.")
			}
			return nil
		}, 30, 1).Should(BeNil())

		annotations = make(map[string]string)
		mch.SetAnnotations(annotations)
		_, err = utils.DynamicKubeClient.Resource(utils.GVRMultiClusterHub).Namespace(utils.MCHNamespace).Update(context.TODO(), mch, metav1.UpdateOptions{})
		Expect(err).To(BeNil())

		err = utils.KubeClient.CoreV1().ConfigMaps(utils.MCHNamespace).Delete(context.TODO(), "my-config", metav1.DeleteOptions{})
		Expect(err).To(BeNil())

		return
	})

	It("Installing MCH with bad pull secret - should have Pending status", func() {
		By("Creating MultiClusterHub")
		err := utils.ValidateMCHUnsuccessful(utils.CreateMCHBadPullSecret())
		if err != nil {
			fmt.Println(fmt.Sprintf("Error: %s\n", err.Error()))
			return
		}
		return
	})

	totalAttempts := 10
	for i := 1; i <= totalAttempts; i++ {
		It(fmt.Sprintf("Installing MCH - Attempt %d of %d", i, totalAttempts), func() {
			By("Creating MultiClusterHub")
			err := utils.ValidateMCH(utils.CreateDefaultMCH())
			if err != nil {
				fmt.Println(fmt.Sprintf("Error: %s\n", err.Error()))
				return
			}
			return
		})
	}
}
