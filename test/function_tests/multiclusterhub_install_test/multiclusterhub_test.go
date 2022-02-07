// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	utils "github.com/stolostron/multiclusterhub-operator/test/function_tests/utils"
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
			return
		})
	}
})

func FullInstallTestSuite() {
	It("Applies tolerations to components when present on the MCH CR", func() {
		By("Creating an MCH CR with tolerations")
		utils.CreateMCHTolerations()
		err := utils.ValidateMCH()
		Expect(err).To(BeNil())

		By("Ensuring tolerations are on MCH components")
		err = utils.ValidateMCHTolerations()
		Expect(err).To(BeNil())
	})

	It("Test Hiveconfig", func() {
		By("- If HiveConfig is edited directly, ensure changes are persisted")

		utils.CreateMCHNotManaged()
		err := utils.ValidateMCH()
		Expect(err).To(BeNil())

		By("- Editing HiveConfig")
		hiveConfig, err := utils.DynamicKubeClient.Resource(utils.GVRHiveConfig).Get(context.TODO(), utils.HiveConfigName, metav1.GetOptions{})
		Expect(err).To(BeNil()) // If HiveConfig does not exist, err

		spec, ok := hiveConfig.Object["spec"].(map[string]interface{})
		Expect(ok).To(BeTrue())
		spec["targetNamespace"] = "test-hive"
		spec["logLevel"] = "info"
		hiveConfig, err = utils.DynamicKubeClient.Resource(utils.GVRHiveConfig).Update(context.TODO(), hiveConfig, metav1.UpdateOptions{})
		Expect(err).To(BeNil()) // If HiveConfig does not exist, err

		By("- Confirming edit was successful")
		hiveConfig, err = utils.DynamicKubeClient.Resource(utils.GVRHiveConfig).Get(context.TODO(), utils.HiveConfigName, metav1.GetOptions{})
		Expect(err).To(BeNil()) // If HiveConfig does not exist, err
		spec, ok = hiveConfig.Object["spec"].(map[string]interface{})
		Expect(ok).To(BeTrue())
		Expect(spec["targetNamespace"]).To(BeEquivalentTo("test-hive"))
		Expect(spec["logLevel"]).To(BeEquivalentTo("info"))

		By("- Restart MCH Operator to ensure HiveConfig is not updated on reconcile")
		// Delete MCH Operator pod to force reconcile
		labelSelector := fmt.Sprintf("name=%s", "multiclusterhub-operator")
		listOptions := metav1.ListOptions{
			LabelSelector: labelSelector,
			Limit:         1,
		}
		err = utils.KubeClient.CoreV1().Pods(utils.MCHNamespace).DeleteCollection(context.TODO(), metav1.DeleteOptions{}, listOptions)
		Expect(err).To(BeNil()) // Deletion should always be successful
		time.Sleep(60 * time.Second)

		hiveConfig, err = utils.DynamicKubeClient.Resource(utils.GVRHiveConfig).Get(context.TODO(), utils.HiveConfigName, metav1.GetOptions{})
		Expect(err).To(BeNil()) // If HiveConfig does not exist, err
		spec, ok = hiveConfig.Object["spec"].(map[string]interface{})
		Expect(ok).To(BeTrue())
		Expect(spec["targetNamespace"]).To(BeEquivalentTo("test-hive"))
		Expect(spec["logLevel"]).To(BeEquivalentTo("info"))

		By("- If HiveConfig is Deleted, ensure it is recreated")
		err = utils.DynamicKubeClient.Resource(utils.GVRHiveConfig).Delete(context.TODO(), utils.HiveConfigName, metav1.DeleteOptions{})
		Expect(err).To(BeNil()) // If HiveConfig does not exist, err
		Eventually(func() error {
			hiveConfig, err = utils.DynamicKubeClient.Resource(utils.GVRHiveConfig).Get(context.TODO(), utils.HiveConfigName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("HiveConfig has not been recreated")
			}
			return nil
		}, utils.GetWaitInMinutes()*2, 1).Should(BeNil())

		By("- If MCH.spec.hive is edited, ensure edit is blocked")
		mch, err := utils.DynamicKubeClient.Resource(utils.GVRMultiClusterHub).Namespace(utils.MCHNamespace).Get(context.TODO(), utils.MCHName, metav1.GetOptions{})
		Expect(err).To(BeNil())

		spec, ok = mch.Object["spec"].(map[string]interface{})
		Expect(ok).To(BeTrue())
		spec["hive"] = map[string]interface{}{
			"maintenanceMode": true,
			"failedProvisionConfig": map[string]interface{}{
				"skipGatherLogs": true,
			},
		}
		_, err = utils.DynamicKubeClient.Resource(utils.GVRMultiClusterHub).Namespace(utils.MCHNamespace).Update(context.TODO(), mch, metav1.UpdateOptions{})
		Expect(err.Error()).To(BeEquivalentTo("admission webhook \"multiclusterhub.validating-webhook.open-cluster-management.io\" denied the request: Hive updates are forbidden"))
		return
	})

	It("Test PullPolicy", func() {
		By("- When creating the MCH, ensure that all deployments use the IfNotPresent pullPolicy")

		utils.CreateMCHNotManaged()
		err := utils.ValidateMCH()
		Expect(err).To(BeNil())

		By("- listing each deployment in the MCH namespace and checking")

		err = utils.ValidateDeploymentPolicies()
		Expect(err).To(BeNil())

	})

	It("Test MCE propagation", func() {
		By("- When creating the MCH, ensure that fields are applied to the owned MCE")

		utils.CreateMCHNotManaged()
		err := utils.ValidateMCH()
		Expect(err).To(BeNil())

		By("- inspecting MCE spec")
		k8sClient := utils.DynamicKubeClient.Resource(utils.GVRMultiClusterEngine)
		mce := &unstructured.Unstructured{}
		mce, err = k8sClient.Get(context.TODO(), "multiclusterengine", metav1.GetOptions{})
		Expect(err).To(BeNil())

		pullSecret := mce.Object["spec"].(map[string]interface{})["imagePullSecret"]
		Expect(pullSecret).To(BeEquivalentTo("multiclusterhub-operator-pull-secret"))

		By("- checking imagepullsecret clone created")
		_, err = utils.KubeClient.CoreV1().Secrets("multicluster-engine").Get(context.TODO(), "multiclusterhub-operator-pull-secret", metav1.GetOptions{})
		Expect(err).To(BeNil())

	})

	It("Test Subscription Propagation", func() {
		By("- When creating the MCH, ensure that the MCE subscription is propagated")

		utils.CreateMCHNotManaged()
		err := utils.ValidateMCH()
		Expect(err).To(BeNil())

		By("- checking the spec values of the deployment which is created")

		err = utils.ValidateMCESub()
		Expect(err).To(BeNil())

	})

	It("Testing Image Overrides Configmap", func() {
		By("- If configmap is manually overwitten, ensure MCH Operator will overwrite")

		utils.CreateMCHNotManaged()
		err := utils.ValidateMCH()
		Expect(err).To(BeNil())

		By("- Overwrite Image Overrides Configmap")
		currentVersion, err := utils.GetCurrentVersionFromMCH()
		Expect(err).To(BeNil())
		v, err := semver.NewVersion(currentVersion)
		Expect(err).Should(BeNil())
		c, err := semver.NewConstraint(">= 2.5.0")
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

		utils.CreateMCHNotManaged()
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
					  "image-remote": "quay.io/stolostron",
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
		c, err := semver.NewConstraint(">= 2.5.0")
		Expect(err).Should(BeNil())
		if c.Check(v) {
			_, err = utils.KubeClient.CoreV1().ConfigMaps(utils.MCHNamespace).Create(context.TODO(), configmap, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			// Annotate MCH
			annotations := make(map[string]string)
			annotations["mch-imageOverridesCM"] = "my-config"
			utils.UpdateAnnotations(annotations)

			Eventually(func() error {
				configmap, err = utils.KubeClient.CoreV1().ConfigMaps(utils.MCHNamespace).Get(context.TODO(), fmt.Sprintf("mch-image-manifest-%s", currentVersion), metav1.GetOptions{})
				if len(configmap.Data) == 0 {
					return fmt.Errorf("Configmap has not been updated")
				}
				if configmap.Data["application_ui"] != "quay.io/stolostron/application-ui:not-a-real-tag" {
					return fmt.Errorf("Configmap has not been updated from overrides CM.")
				}
				return nil
			}, utils.GetWaitInMinutes()*60, 1).Should(BeNil())

			annotations = make(map[string]string)
			utils.UpdateAnnotations(annotations)

			err = utils.KubeClient.CoreV1().ConfigMaps(utils.MCHNamespace).Delete(context.TODO(), "my-config", metav1.DeleteOptions{})
			Expect(err).To(BeNil())
		}
		return
	})

	It("- If `spec.disableUpdateClusterImageSets` controls the automatic updates of clusterImageSets", func() {
		By("- Verfiying default ")
		utils.CreateMCHNotManaged()
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
				return fmt.Errorf("Console AppSub not updated")
			}
			return nil
		}, utils.GetWaitInMinutes()*60, 1).Should(BeNil())

		// Set the disableUpdateCluterImageSets: false
		By("- Setting `spec.disableUpdateClusterImageSets` to false to enable automatic updates of clusterImageSets")
		utils.ToggleDisableUpdateClusterImageSets(false)

		Eventually(func() error {
			if err := utils.ValidateClusterImageSetsSubscriptionPause("false"); err != nil {
				return fmt.Errorf("Console AppSub not updated")
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

	It(fmt.Sprintf("Installing MCH with old components on cluster"), func() {
		By("Installing old component")
		subName := "topology-sub"
		sub := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps.open-cluster-management.io/v1",
				"kind":       "Subscription",
				"metadata": map[string]interface{}{
					"name":      subName,
					"namespace": utils.MCHNamespace,
				},
				"spec": map[string]interface{}{
					"channel": fmt.Sprintf("%s/charts-v1", utils.MCHNamespace),
					"name":    "test",
					"placement": map[string]interface{}{
						"local": true,
					},
				},
			},
		}
		k8sClient := utils.DynamicKubeClient.Resource(utils.GVRAppSub).Namespace(utils.MCHNamespace)
		_, err := k8sClient.Create(context.TODO(), sub, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
		// Clean up resource manually in case of failure
		defer k8sClient.Delete(context.TODO(), subName, metav1.DeleteOptions{})

		By("Installing MCH")
		utils.CreateMCHNotManaged()
		Expect(utils.ValidateMCH()).To(Succeed())

		By("Verifying old component has been removed")
		_, err = k8sClient.Get(context.TODO(), subName, metav1.GetOptions{})
		Expect(errors.IsNotFound(err)).To(BeTrue(), "should have been deleted by the reconciler and return a NotFound error")

		By("Verifying status is not complete while a resource has not been successfully pruned")
		// Create appsub again, this time with a finalizer
		finalizer := []string{"test-finalizer"}
		sub.SetFinalizers(finalizer)
		_, err = k8sClient.Create(context.TODO(), sub, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
		// Remove finalizer manually in case of failure
		defer func() {
			dsub, err := k8sClient.Get(context.TODO(), subName, metav1.GetOptions{})
			if err != nil {
				return
			}
			dsub.SetFinalizers([]string{})
			k8sClient.Update(context.TODO(), dsub, metav1.UpdateOptions{})
		}()

		// Force reconcile
		Expect(utils.DeleteMCHRepo()).To(Succeed())

		timeout := 2 * time.Minute
		interval := time.Second * 2
		Eventually(func() error {
			status, err := utils.GetMCHStatus()
			if err != nil {
				return err
			}
			return utils.FindCondition(status, "Progressing", "False")
		}, timeout, interval).Should(Succeed(), "the blocked resource deletion should prevent progress")

		By("Verifying status recovers once the blocked resource is cleaned up")
		Eventually(func() error {
			unblockedSub, _ := k8sClient.Get(context.TODO(), subName, metav1.GetOptions{})
			unblockedSub.SetFinalizers([]string{})
			_, err = k8sClient.Update(context.TODO(), unblockedSub, metav1.UpdateOptions{})
			return err
		}, time.Minute, time.Second).Should(Succeed(), "the blocked resource deletion should prevent progress")

		Expect(utils.ValidateMCH()).To(Succeed())

		return
	})

	It(fmt.Sprintf("Installing MCH with old rcm component on cluster"), func() {
		By("Installing old component")
		subName := "rcm-sub"
		sub := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps.open-cluster-management.io/v1",
				"kind":       "Subscription",
				"metadata": map[string]interface{}{
					"name":      subName,
					"namespace": utils.MCHNamespace,
				},
				"spec": map[string]interface{}{
					"channel": fmt.Sprintf("%s/charts-v1", utils.MCHNamespace),
					"name":    "test",
					"placement": map[string]interface{}{
						"local": true,
					},
				},
			},
		}
		k8sClient := utils.DynamicKubeClient.Resource(utils.GVRAppSub).Namespace(utils.MCHNamespace)
		_, err := k8sClient.Create(context.TODO(), sub, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
		// Clean up resource manually in case of failure
		defer k8sClient.Delete(context.TODO(), subName, metav1.DeleteOptions{})

		By("Installing MCH")
		utils.CreateMCHNotManaged()
		Expect(utils.ValidateMCH()).To(Succeed())

		By("Verifying old component has been removed")
		_, err = k8sClient.Get(context.TODO(), subName, metav1.GetOptions{})
		Expect(errors.IsNotFound(err)).To(BeTrue(), "should have been deleted by the reconciler and return a NotFound error")

		By("Verifying status is not complete while a resource has not been successfully pruned")
		// Create appsub again, this time with a finalizer
		finalizer := []string{"test-finalizer"}
		sub.SetFinalizers(finalizer)
		_, err = k8sClient.Create(context.TODO(), sub, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
		// Remove finalizer manually in case of failure
		defer func() {
			dsub, err := k8sClient.Get(context.TODO(), subName, metav1.GetOptions{})
			if err != nil {
				return
			}
			dsub.SetFinalizers([]string{})
			k8sClient.Update(context.TODO(), dsub, metav1.UpdateOptions{})
		}()

		// Force reconcile
		Expect(utils.DeleteMCHRepo()).To(Succeed())

		timeout := 2 * time.Minute
		interval := time.Second * 2
		Eventually(func() error {
			status, err := utils.GetMCHStatus()
			if err != nil {
				return err
			}
			return utils.FindCondition(status, "Progressing", "False")
		}, timeout, interval).Should(Succeed(), "the blocked resource deletion should prevent progress")

		By("Verifying status recovers once the blocked resource is cleaned up")
		Eventually(func() error {
			unblockedSub, _ := k8sClient.Get(context.TODO(), subName, metav1.GetOptions{})
			unblockedSub.SetFinalizers([]string{})
			_, err = k8sClient.Update(context.TODO(), unblockedSub, metav1.UpdateOptions{})
			return err
		}, time.Minute, time.Second).Should(Succeed(), "the blocked resource deletion should prevent progress")

		Expect(utils.ValidateMCH()).To(Succeed())

		return
	})

	totalAttempts := 2
	for i := 1; i <= totalAttempts; i++ {
		ok := It(fmt.Sprintf("Installing MCH - Attempt %d of %d", i, totalAttempts), func() {
			By("Creating MultiClusterHub")
			utils.CreateMCHNotManaged()
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
			oldImage, err := utils.BrickCLC()
			if err != nil {
				fmt.Println(fmt.Sprintf("Error: %s\n", err.Error()))
				return
			}
			if err := utils.ValidateMCHDegraded(); err != nil {
				fmt.Println(fmt.Sprintf("Error: %s\n", err.Error()))
				return
			}
			if err := utils.FixCLC(oldImage); err != nil {
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

	It("- If `spec.disableHubSelfManagement` controls the existence of the related resources", func() {
		Skip("Skipping all tests related to local cluster and self management")

		By("- Verifying default install has local-cluster resources")
		utils.CreateDefaultMCH()
		err := utils.ValidateMCH()
		Expect(err).To(BeNil())

		By("- Setting `spec.disableHubSelfManagement` to true to remove local-cluster resources")
		utils.ToggleDisableHubSelfManagement(true)
		By("- Sleeping some compulsory 15 minutes because of some foundation bug")
		utils.CoffeeBreak(15)
		By("- Returning from compulsory coffee break")
		Eventually(func() error {
			if err := utils.ValidateImportHubResourcesExist(false); err != nil {
				return fmt.Errorf("resources still exist")
			}
			return nil
		}, utils.GetWaitInMinutes()*60, 1).Should(BeNil())

		By("- Setting `spec.disableHubSelfManagement` to false to create local-cluster resources")
		utils.ToggleDisableHubSelfManagement(false)
		By("- Sleeping some compulsory 15 minutes because of some foundation bug")
		utils.CoffeeBreak(15)
		By("- Returning from compulsory coffee break")
		Eventually(func() error {
			if err := utils.ValidateImportHubResourcesExist(true); err != nil {
				return fmt.Errorf("resources don't exist")
			}
			return nil
		}, utils.GetWaitInMinutes()*60, 1).Should(BeNil())

	})

	It("- Delete ManagedCluster before it is joined/available", func() {
		Skip("Skipping all tests related to local cluster and self management")

		By("- Verifying install has local-cluster resources")
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
				return fmt.Errorf("resources don't exist")
			}
			return nil
		}, utils.GetWaitInMinutes()*60, 1).Should(BeNil())
	})

}
