// Copyright (c) 2020 Red Hat, Inc.
package multiclusterhub_install_test

import (
	"context"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	utils "github.com/open-cluster-management/multicloudhub-operator/test/utils"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog"
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
		for i := 1; i < totalAttempts; i++ {
			ok := It(fmt.Sprintf("Installing MCH - Attempt %d of %d", i, totalAttempts), func() {
				By("Creating MultiClusterHub")
				err := ValidateMCH(CreateDefaultMCH())
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
			ValidateMCH(CreateDefaultMCH())
		})
	}
})

func CreateDefaultMCH() *unstructured.Unstructured {
	mch := utils.NewMultiClusterHub(utils.MCHName, utils.MCHNamespace)
	utils.CreateNewUnstructured(utils.DynamicKubeClient, utils.GVRMultiClusterHub, mch, utils.MCHName, utils.MCHNamespace)
	return mch
}

func ValidateMCH(mch *unstructured.Unstructured) error {
	var deploy *appsv1.Deployment
	When("Wait for MultiClusterHub Repo to be available", func() {
		Eventually(func() error {
			var err error
			klog.V(1).Info("Wait MCH Repo deployment...")
			deploy, err = utils.KubeClient.AppsV1().Deployments(utils.MCHNamespace).Get(context.TODO(), utils.MCHRepoName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if deploy.Status.AvailableReplicas == 0 {
				return fmt.Errorf("MCH Repo not available")
			}
			return err
		}, 60, 1).Should(BeNil())
		klog.V(1).Info("MCH Repo deployment available")
	})
	By("Checking ownerRef", func() {
		Expect(utils.IsOwner(mch, &deploy.ObjectMeta)).To(Equal(true))
	})

	By("Checking Appsubs")
	ok := When("Wait for Application Subscriptions to be Active", func() {
		Eventually(func() error {
			unstructuredAppSubs := listByGVR(utils.DynamicKubeClient, utils.GVRAppSub, utils.MCHNamespace, 60, len(utils.AppSubSlice))

			for _, appsub := range unstructuredAppSubs.Items {
				if _, ok := appsub.Object["status"]; !ok {
					return fmt.Errorf("Appsub: %s has no 'status' field", appsub.GetName())
				}
				status, ok := appsub.Object["status"].(map[string]interface{})
				if !ok || status == nil {
					return fmt.Errorf("Appsub: %s has no 'status' map", appsub.GetName())
				}
				klog.V(5).Infof("Checking Appsub - %s", appsub.GetName())
				Expect(status["message"]).To(Equal("Active"))
				Expect(status["phase"]).To(Equal("Subscribed"))
			}
			return nil
		}, 180, 1).Should(BeNil())
	})
	if !ok {
		return fmt.Errorf("Unable to create all Application Subscriptions")
	}

	By("Checking HelmReleases")
	ok = When("Wait for HelmReleases to be successfully installed", func() {
		Eventually(func() error {
			unstructuredHelmReleases := listByGVR(utils.DynamicKubeClient, utils.GVRHelmRelease, utils.MCHNamespace, 60, len(utils.AppSubSlice))
			// ready := false
			for _, helmRelease := range unstructuredHelmReleases.Items {
				klog.V(5).Infof("Checking HelmRelease - %s", helmRelease.GetName())

				status, ok := helmRelease.Object["status"].(map[string]interface{})
				if !ok || status == nil {
					return fmt.Errorf("HelmRelease: %s has no 'status' map", helmRelease.GetName())
				}

				conditions, ok := status["conditions"].([]interface{})
				if !ok || conditions == nil {
					return fmt.Errorf("HelmRelease: %s has no 'conditions' interface", helmRelease.GetName())
				}

				finalCondition, ok := conditions[len(conditions)-1].(map[string]interface{})
				if finalCondition["reason"] != "InstallSuccessful" || finalCondition["type"] != "Deployed" {
					return fmt.Errorf("HelmRelease: %s not ready", helmRelease.GetName())
				}

				Expect(finalCondition["reason"]).To(Equal("InstallSuccessful"))
				Expect(finalCondition["type"]).To(Equal("Deployed"))
			}
			return nil
		}, 500, 1).Should(BeNil())
	})
	if !ok {
		return fmt.Errorf("Unable to create all Helm Releases successfully")
	}
	return nil
}
