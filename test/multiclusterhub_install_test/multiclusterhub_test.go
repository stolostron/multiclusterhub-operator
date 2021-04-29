// Copyright (c) 2020 Red Hat, Inc.
package multiclusterhub_install_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	utils "github.com/open-cluster-management/multiclusterhub-operator/test/utils"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

var _ = Describe("Multiclusterhub", func() {

	BeforeEach(func() {
		By("Delete MultiClusterHub if it exists")
		utils.DeleteIfExists(utils.DynamicKubeClient, utils.GVRMultiClusterHub, utils.MCHName, utils.MCHNamespace)

		Expect(utils.EnsureHelmReleasesAreRemoved(utils.DynamicKubeClient)).Should(BeNil())
	})

	It("Install Default MCH CR", func() {
		By("Creating MultiClusterHub")
		mch := utils.NewMultiClusterHub(utils.MCHName, utils.MCHNamespace)
		utils.CreateNewUnstructured(utils.DynamicKubeClient, utils.GVRMultiClusterHub, mch, utils.MCHName, utils.MCHNamespace)

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
		When("Wait for Application Subscriptions to be Active", func() {
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

		By("Checking HelmReleases")
		When("Wait for HelmReleases to be successfully installed", func() {
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
	})
})
