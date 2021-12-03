// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"fmt"
	"reflect"
	"time"

	mcev1alpha1 "github.com/open-cluster-management/backplane-operator/api/v1alpha1"
	appsubv1 "github.com/open-cluster-management/multicloud-operators-subscription/pkg/apis/apps/v1"
	operatorsv1 "github.com/open-cluster-management/multiclusterhub-operator/api/v1"
	"github.com/open-cluster-management/multiclusterhub-operator/pkg/multiclusterengine"
	utils "github.com/open-cluster-management/multiclusterhub-operator/pkg/utils"
	resources "github.com/open-cluster-management/multiclusterhub-operator/test/unit-tests"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	mch_name      = "multiclusterhub-operator"
	mch_namespace = "open-cluster-management"
	// A MultiClusterHub object with metadata and spec.
	full_mch = &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mch_name,
			Namespace: mch_namespace,
		},
		Spec: operatorsv1.MultiClusterHubSpec{
			ImagePullSecret: "pull-secret",
			Ingress: operatorsv1.IngressSpec{
				SSLCiphers: []string{"foo", "bar", "baz"},
			},
			AvailabilityConfig: operatorsv1.HAHigh,
		},
		Status: operatorsv1.MultiClusterHubStatus{
			CurrentVersion: "2.0.0",
			Phase:          "Running",
		},
	}
	// A MultiClusterHub object with metadata and spec.
	empty_mch = &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mch_name,
			Namespace: mch_namespace,
		},
		Spec: operatorsv1.MultiClusterHubSpec{
			ImagePullSecret: "pull-secret",
		},
	}
	mch_namespaced = types.NamespacedName{
		Name:      mch_name,
		Namespace: mch_namespace,
	}
)

const (
	timeout  = time.Second * 10
	duration = time.Second * 10
	interval = time.Millisecond * 250
)

func ApplyPrereqs() {
	By("Applying Namespace")
	ctx := context.Background()

	Expect(k8sClient.Create(ctx, resources.OCMNamespace)).Should(Succeed())
}

var _ = Describe("MultiClusterHub controller", func() {
	// Define utility constants for object names and testing timeouts/durations and intervals.

	Context("When updating Multiclusterhub status", func() {
		It("Should get to a running state", func() {
			By("By creating a new Multiclusterhub")
			ctx := context.Background()

			ApplyPrereqs()
			Expect(k8sClient.Create(ctx, resources.EmptyMCH)).Should(Succeed())

			// Ensures MCH is Created
			mchLookupKey := types.NamespacedName{Name: resources.MulticlusterhubName, Namespace: resources.MulticlusterhubNamespace}
			createdMCH := &operatorsv1.MultiClusterHub{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, mchLookupKey, createdMCH)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("- Ensuring Defaults are set")
			// Ensures defaults are set
			Eventually(func() bool {
				err := k8sClient.Get(ctx, mchLookupKey, createdMCH)
				Expect(err).Should(BeNil())
				return reflect.DeepEqual(createdMCH.Spec.Ingress.SSLCiphers, utils.DefaultSSLCiphers) && createdMCH.Spec.AvailabilityConfig == operatorsv1.HAHigh
			}, timeout, interval).Should(BeTrue())

			// Ensure Deployments
			Eventually(func() bool {
				deploymentReferences := utils.GetDeployments(createdMCH)
				result := true
				for _, deploymentReference := range deploymentReferences {
					deployment := &appsv1.Deployment{}
					err := k8sClient.Get(ctx, deploymentReference, deployment)
					if err != nil {
						fmt.Println(err.Error())
						result = false
					}
				}
				return result
			}, timeout, interval).Should(BeTrue())

			// Ensure MultiClusterEngine
			Eventually(func() bool {
				mce := &mcev1alpha1.MultiClusterEngine{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: multiclusterengine.MulticlusterengineName}, mce)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			// Ensure Appsubs
			Eventually(func() bool {
				subscriptionReferences := utils.GetAppsubs(createdMCH)
				result := true
				for _, subscriptionReference := range subscriptionReferences {
					subscription := &appsubv1.Subscription{}
					err := k8sClient.Get(ctx, subscriptionReference, subscription)
					if err != nil {
						fmt.Println(err.Error())
						result = false
					}
				}
				return result
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				return createdMCH.Status.Phase == operatorsv1.HubRunning
			}, timeout, interval).Should(BeTrue())

		})
	})

})
