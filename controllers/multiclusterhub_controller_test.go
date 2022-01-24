// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"time"

	appsubv1 "github.com/open-cluster-management/multicloud-operators-subscription/pkg/apis/apps/v1"
	mcev1alpha1 "github.com/stolostron/backplane-operator/api/v1alpha1"
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengine"
	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"
	resources "github.com/stolostron/multiclusterhub-operator/test/unit-tests"
	corev1 "k8s.io/api/core/v1"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
	envs = []corev1.EnvVar{
		{
			Name:  "test",
			Value: "test",
		},
	}
	containers = []corev1.Container{
		{
			Name:            "test",
			Image:           "test",
			ImagePullPolicy: "Always",
			Env:             envs,
			Command:         []string{"/iks.sh"},
		},
	}
	specLabels              = &metav1.LabelSelector{MatchLabels: map[string]string{"test": "test"}}
	templateMetadata        = metav1.ObjectMeta{Labels: map[string]string{"test": "test"}}
	basicOperatorDeployment = &appsv1.Deployment{
		TypeMeta:   metav1.TypeMeta{Kind: "Deployment", APIVersion: "apps/v1"},
		ObjectMeta: metav1.ObjectMeta{Name: mch_name, Namespace: mch_namespace},
		Spec: appsv1.DeploymentSpec{
			Selector: specLabels,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: templateMetadata,
				Spec: corev1.PodSpec{
					Containers: containers,
				},
			},
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
	os.Setenv("POD_NAMESPACE", "open-cluster-management")
	Expect(k8sClient.Create(ctx, resources.OCMNamespace)).Should(Succeed())
}

var _ = Describe("MultiClusterHub controller", func() {
	Context("When updating Multiclusterhub status", func() {
		// Define utility constants for object names and testing timeouts/durations and intervals.
		AfterEach(func() {
			Eventually(func() bool {
				ctx := context.Background()
				mch := &operatorsv1.MultiClusterHub{}
				err := k8sClient.Get(ctx, resources.MCHLookupKey, mch)
				if err != nil && errors.IsNotFound(err) {
					return true
				} else {
					mch := resources.EmptyMCH
					k8sClient.Delete(ctx, &mch)
				}
				return false
			}, timeout, interval).Should(BeTrue())
		})
		It("Should get to a running state", func() {
			By("By creating a new Multiclusterhub")
			ctx := context.Background()

			ApplyPrereqs()
			mch := resources.EmptyMCH
			Expect(k8sClient.Create(ctx, &mch)).Should(Succeed())

			Expect(k8sClient.Create(ctx, basicOperatorDeployment)).Should(Succeed())

			// Ensures MCH is Created
			createdMCH := &operatorsv1.MultiClusterHub{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, resources.MCHLookupKey, createdMCH)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("- Ensuring Defaults are set")
			// Ensures defaults are set
			Eventually(func() bool {
				err := k8sClient.Get(ctx, resources.MCHLookupKey, createdMCH)
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
				result := true
				err := k8sClient.Get(ctx, types.NamespacedName{Name: multiclusterengine.MulticlusterengineName}, mce)
				if err != nil {
					fmt.Println(err.Error())
					result = false
				}
				return result
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

		It("Should Manage Preexisting MCE", func() {
			ctx := context.Background()
			mce := resources.EmptyMCE

			// Create MCE
			Expect(k8sClient.Create(ctx, &mce)).Should(Succeed())

			mch := resources.EmptyMCH
			// Create MCH
			Expect(k8sClient.Create(ctx, &mch)).Should(Succeed())

			// Wait for MCH to get to Running state
			Eventually(func() bool {
				mch := &operatorsv1.MultiClusterHub{}
				err := k8sClient.Get(ctx, resources.MCHLookupKey, mch)
				if err == nil {
					return mch.Status.Phase == operatorsv1.HubRunning
				}
				return false
			}, timeout, interval).Should(BeTrue())

			// Validates Managedby label is added to MCE
			Eventually(func() bool {
				existingMCE := &mcev1alpha1.MultiClusterEngine{}
				Expect(k8sClient.Get(ctx, resources.MCELookupKey, existingMCE)).Should(Succeed())
				labels := existingMCE.GetLabels()
				if labels == nil {
					return false
				}
				if val, ok := labels[utils.MCEManagedByLabel]; ok && val == "true" {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

			// Delete and wait for MCH to terminate
			Eventually(func() bool {
				ctx := context.Background()
				mch := &operatorsv1.MultiClusterHub{}
				err := k8sClient.Get(ctx, resources.MCHLookupKey, mch)
				if err != nil && errors.IsNotFound(err) {
					return true
				} else {
					mch := resources.EmptyMCH
					k8sClient.Delete(ctx, &mch)
				}
				return false
			}, timeout, interval).Should(BeTrue())

			// Ensure MCE remains
			Expect(k8sClient.Get(ctx, resources.MCELookupKey, &mcev1alpha1.MultiClusterEngine{})).Should(Succeed())

		})
	})

})
