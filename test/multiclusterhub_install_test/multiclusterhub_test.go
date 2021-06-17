// Copyright (c) 2020 Red Hat, Inc.
package multiclusterhub_install_test

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Masterminds/semver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	utils "github.com/open-cluster-management/multiclusterhub-operator/test/utils"
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

			utils.CreateDefaultMCH()
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
			return
		})
	}
})

func FullInstallTestSuite() {

	It("Testing Image Overrides Configmap", func() {
		By("- If configmap is manually overwitten, ensure MCH Operator will overwrite")

		utils.CreateDefaultMCH()
		err := utils.ValidateMCH()
		Expect(err).To(BeNil())

		By("- Overwrite Image Overrides Configmap")
		currentVersion, err := utils.GetCurrentVersionFromMCH()
		Expect(err).To(BeNil())
		v, err := semver.NewVersion(currentVersion)
		Expect(err).Should(BeNil())
		c, err := semver.NewConstraint(">= 2.2.5")
		Expect(err).Should(BeNil())

		if c.Check(v) {
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
			}, utils.GetWaitInMinutes()*60, 1).Should(BeNil())

		}
		return
	})

	It("- If `mch-imageOverridesCM` annotation is given, ensure Image Overrides Configmap is updated ", func() {
		By("- Creating Developer Image Overrides Configmap")

		utils.CreateDefaultMCH()
		err := utils.ValidateMCH()
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

		v, err := semver.NewVersion(currentVersion)
		Expect(err).Should(BeNil())
		c, err := semver.NewConstraint(">= 2.2.5")
		Expect(err).Should(BeNil())
		if c.Check(v) {
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
			}, utils.GetWaitInMinutes()*60, 1).Should(BeNil())

			annotations = make(map[string]string)
			mch.SetAnnotations(annotations)
			_, err = utils.DynamicKubeClient.Resource(utils.GVRMultiClusterHub).Namespace(utils.MCHNamespace).Update(context.TODO(), mch, metav1.UpdateOptions{})
			Expect(err).To(BeNil())

			err = utils.KubeClient.CoreV1().ConfigMaps(utils.MCHNamespace).Delete(context.TODO(), "my-config", metav1.DeleteOptions{})
			Expect(err).To(BeNil())
		}
		return
	})

	It("- If `spec.disableHubSelfManagement` controls the existence of the related resources", func() {
		By("- Verfiying default install has local-cluster resources")
		utils.CreateDefaultMCH()
		err := utils.ValidateMCH()
		Expect(err).To(BeNil())

		By("- Setting `spec.disableHubSelfManagement` to true to remove local-cluster resources")
		utils.ToggleDisableHubSelfManagement(true)

		Eventually(func() error {
			if err := utils.ValidateImportHubResourcesExist(false); err != nil {
				return fmt.Errorf("resources still exist")
			}
			return nil
		}, utils.GetWaitInMinutes()*60, 1).Should(BeNil())

		By("- Setting `spec.disableHubSelfManagement` to false to create local-cluster resources")
		utils.ToggleDisableHubSelfManagement(false)
		Eventually(func() error {
			if err := utils.ValidateImportHubResourcesExist(true); err != nil {
				return fmt.Errorf("resources still exist")
			}
			return nil
		}, utils.GetWaitInMinutes()*60, 1).Should(BeNil())

	})

	It("- If `spec.disableUpdateClusterImageSets` controls the automatic updates of clusterImageSets", func() {
		By("- Verfiying default ")
		utils.CreateDefaultMCH()
		err := utils.ValidateMCH()
		Expect(err).To(BeNil())

		// Test initial case with no setting, is equivalent to disableUpdateClusterImageSets: false
		err = utils.ValidateClusterImageSetsSubscriptionPause("false")
		Expect(err).To(BeNil())

		// Set the disableUpdateCluterImageSets: true
		By("- Setting `spec.disableUpdateClusterImageSets` to true to disable automatic updates of clusterImageSets")
		utils.ToggleDisableUpdateClusterImageSets(true)

		Eventually(func() error {
			if err := utils.ValidateClusterImageSetsSubscriptionPause("true"); err != nil {
				return fmt.Errorf("resources still exist")
			}
			return nil
		}, utils.GetWaitInMinutes()*60, 1).Should(BeNil())

		// Set the disableUpdateCluterImageSets: false
		By("- Setting `spec.disableUpdateClusterImageSets` to false to enable automatic updates of clusterImageSets")
		utils.ToggleDisableUpdateClusterImageSets(false)

		Eventually(func() error {
			if err := utils.ValidateClusterImageSetsSubscriptionPause("false"); err != nil {
				return fmt.Errorf("resources still exist")
			}
			return nil
		}, utils.GetWaitInMinutes()*60, 1).Should(BeNil())
	})

	It("- Delete ManagedCluster before it is joined/available", func() {
		By("- Verifying default install has local-cluster resources")
		utils.CreateDefaultMCH()
		Eventually(func() error {
			if err := utils.ValidateImportHubResourcesExist(true); err != nil {
				return fmt.Errorf("resources still exist")
			}
			return nil
		}, utils.GetWaitInMinutes()*60, 1).Should(BeNil())

		By("- Setting `spec.disableHubSelfManagement` to true to remove local-cluster resources")
		utils.ToggleDisableHubSelfManagement(true)
		Eventually(func() error {
			if err := utils.ValidateImportHubResourcesExist(false); err != nil {
				return fmt.Errorf("resources still exist")
			}
			return nil
		}, utils.GetWaitInMinutes()*60, 1).Should(BeNil())

		By("- Setting `spec.disableHubSelfManagement` to false to create local-cluster resources")
		utils.ToggleDisableHubSelfManagement(false)
		Eventually(func() error {
			if err := utils.ValidateImportHubResourcesExist(true); err != nil {
				return fmt.Errorf("resources still exist")
			}
			return nil
		}, utils.GetWaitInMinutes()*60, 1).Should(BeNil())
	})

	It(fmt.Sprintf("Installing MCH with bad image reference - should have Installing status"), func() {
		By("Creating Bad Image Overrides Configmap")
		imageOverridesCM := utils.NewImageOverridesConfigmapBadImageRef(utils.ImageOverridesCMBadImageName, utils.MCHNamespace)
		err := utils.CreateNewConfigMap(imageOverridesCM, utils.MCHNamespace)
		Expect(err).To(BeNil())

		By("Creating MultiClusterHub with image overrides annotation")
		utils.CreateMCHImageOverridesAnnotation(utils.ImageOverridesCMBadImageName)
		err = utils.ValidateMCHUnsuccessful()
	})

	totalAttempts := 2
	for i := 1; i <= totalAttempts; i++ {
		ok := It(fmt.Sprintf("Installing MCH - Attempt %d of %d", i, totalAttempts), func() {
			By("Creating MultiClusterHub")
			utils.CreateDefaultMCH()
			if err := utils.ValidateStatusesExist(); err != nil {
				fmt.Println(fmt.Sprintf("Error: %s\n", err.Error()))
				Expect(err).To(BeNil())
				return
			}
			err := utils.ValidateMCH()
			if err != nil {
				fmt.Println(fmt.Sprintf("Error: %s\n", err.Error()))
				Expect(err).To(BeNil())
				return
			}

			By("Degrading the installation")
			oldImage, err := utils.BrickKUI()
			if err != nil {
				fmt.Println(fmt.Sprintf("Error: %s\n", err.Error()))
				return
			}
			if err := utils.ValidateMCHDegraded(); err != nil {
				fmt.Println(fmt.Sprintf("Error: %s\n", err.Error()))
				return
			}
			if err := utils.FixKUI(oldImage); err != nil {
				fmt.Println(fmt.Sprintf("Error: %s\n", err.Error()))
				return
			}

			if err := utils.ValidateMCH(); err != nil {
				fmt.Println(fmt.Sprintf("Error: %s\n", err.Error()))
				return
			}

			if err := utils.BrickMCHRepo(); err != nil {
				fmt.Println(fmt.Sprintf("Error: %s\n", err.Error()))
				Expect(err).To(BeNil())
				return
			}
			if err := utils.ValidateMCHDegraded(); err != nil {
				fmt.Println(fmt.Sprintf("Error: %s\n", err.Error()))
				Expect(err).To(BeNil())
				return
			}
			if err := utils.FixMCHRepo(); err != nil {
				fmt.Println(fmt.Sprintf("Error: %s\n", err.Error()))
				Expect(err).To(BeNil())
				return
			}
			return
		})
		if !ok {
			break
		}
	}
}
