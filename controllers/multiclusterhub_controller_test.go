// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"time"

	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	mchov1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	v1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengine"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	resources "github.com/stolostron/multiclusterhub-operator/test/unit-tests"
	appsub "open-cluster-management.io/multicloud-operators-subscription/pkg/apis"
	appsubv1 "open-cluster-management.io/multicloud-operators-subscription/pkg/apis/apps/v1"

	configv1 "github.com/openshift/api/config/v1"
	netv1 "github.com/openshift/api/config/v1"
	consolev1 "github.com/openshift/api/operator/v1"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	timeout  = time.Second * 10
	interval = time.Millisecond * 250

	mchName      = "multiclusterhub-operator"
	mchNamespace = "open-cluster-management"
)

var ()

func ApplyPrereqs(k8sClient client.Client) {
	By("Applying Namespace")
	ctx := context.Background()
	Expect(k8sClient.Create(ctx, resources.OCMNamespace())).Should(Succeed())
}

var _ = Describe("MultiClusterHub controller", func() {
	var (
		testEnv      *envtest.Environment
		clientConfig *rest.Config
		clientScheme = runtime.NewScheme()
		k8sClient    client.Client

		specLabels       *metav1.LabelSelector
		templateMetadata metav1.ObjectMeta
		envs             []corev1.EnvVar
		containers       []corev1.Container
		mchoDeployment   *appsv1.Deployment
	)

	BeforeEach(func() {
		specLabels = &metav1.LabelSelector{MatchLabels: map[string]string{"test": "test"}}
		templateMetadata = metav1.ObjectMeta{Labels: map[string]string{"test": "test"}}
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
		testEnv = &envtest.Environment{
			CRDDirectoryPaths: []string{
				filepath.Join("..", "config", "crd", "bases"),
				filepath.Join("..", "test", "unit-tests", "crds"),
			},
			CRDInstallOptions: envtest.CRDInstallOptions{
				CleanUpAfterUse: true,
			},
			ErrorIfCRDPathMissing: true,
		}
		mchoDeployment = &appsv1.Deployment{
			TypeMeta:   metav1.TypeMeta{Kind: "Deployment", APIVersion: "apps/v1"},
			ObjectMeta: metav1.ObjectMeta{Name: mchName, Namespace: mchNamespace},
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

		By("bootstrapping test environment")
		Eventually(func() error {
			var err error
			clientConfig, err = testEnv.Start()
			return err
		}, timeout, interval).Should(Succeed())
		Expect(clientConfig).NotTo(BeNil())

		Expect(scheme.AddToScheme(clientScheme)).Should(Succeed())
		Expect(mchov1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(appsub.AddToScheme(clientScheme)).Should(Succeed())
		Expect(apiregistrationv1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(apixv1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(netv1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(olmv1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(subv1alpha1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(mcev1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(configv1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(consolev1.AddToScheme(clientScheme)).Should(Succeed())

		k8sManager, err := ctrl.NewManager(clientConfig, ctrl.Options{
			Scheme:                 clientScheme,
			MetricsBindAddress:     "0",
			HealthProbeBindAddress: "0",
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(k8sManager).ToNot(BeNil())

		k8sClient = k8sManager.GetClient()
		Expect(k8sClient).ToNot(BeNil())

		reconciler := &MultiClusterHubReconciler{
			Client: k8sClient,
			Scheme: k8sManager.GetScheme(),
			Log:    ctrl.Log.WithName("controllers").WithName("MultiClusterHub"),
		}
		Expect(reconciler.SetupWithManager(k8sManager)).Should(Succeed())

		go func() {
			// For explanation of GinkgoRecover in a go routine, see
			// https://onsi.github.io/ginkgo/#mental-model-how-ginkgo-handles-failure
			defer GinkgoRecover()

			Expect(k8sManager.Start(signalHandlerContext)).Should(Succeed())
		}()
	})

	JustBeforeEach(func() {
		// Create ClusterVersion
		// Attempted to Store Version in status. Unable to get it to stick.
		Expect(k8sClient.Create(context.Background(), &configv1.ClusterVersion{
			ObjectMeta: metav1.ObjectMeta{
				Name: "version",
			},
			Spec: configv1.ClusterVersionSpec{
				Channel:   "stable-4.9",
				ClusterID: "12345678910",
			},
		})).To(Succeed())
	})

	Context("When updating Multiclusterhub status", func() {
		It("Should get to a running state", func() {
			By("Applying prereqs")
			ctx := context.Background()
			ApplyPrereqs(k8sClient)

			By("By creating a new Multiclusterhub")
			mch := resources.EmptyMCH()
			Expect(k8sClient.Create(ctx, &mch)).Should(Succeed())
			Expect(k8sClient.Create(ctx, mchoDeployment)).Should(Succeed())

			By("Ensuring MCH is created")
			createdMCH := &mchov1.MultiClusterHub{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, resources.MCHLookupKey, createdMCH)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			annotations := map[string]string{
				"mch-imageRepository": "quay.io/test",
			}
			createdMCH.SetAnnotations(annotations)
			Expect(k8sClient.Update(ctx, createdMCH)).Should(Succeed())

			By("Ensuring Defaults are set")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, resources.MCHLookupKey, createdMCH)
				Expect(err).Should(BeNil())
				return reflect.DeepEqual(createdMCH.Spec.Ingress.SSLCiphers, utils.DefaultSSLCiphers) && createdMCH.Spec.AvailabilityConfig == mchov1.HAHigh
			}, timeout, interval).Should(BeTrue())

			By("Ensuring Deployments")
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

			By("Ensuring MultiClusterEngine is running")
			Eventually(func() bool {
				mce := &mcev1.MultiClusterEngine{}
				result := true
				err := k8sClient.Get(ctx, types.NamespacedName{Name: multiclusterengine.MulticlusterengineName}, mce)
				if err != nil {
					fmt.Println(err.Error())
					result = false
				}
				mceAnnotations := mce.GetAnnotations()
				if val, ok := mceAnnotations["imageRepository"]; !ok || val != "quay.io/test" {
					result = false
				}
				return result
			}, timeout, interval).Should(BeTrue())

			By("Ensuring appsubs")
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

			By("Waiting for MCH to be in the running state")
			Eventually(func() bool {
				mch := &mchov1.MultiClusterHub{}
				err := k8sClient.Get(ctx, resources.MCHLookupKey, mch)
				if err == nil {
					return mch.Status.Phase == mchov1.HubRunning
				}
				return false
			}, timeout, interval).Should(BeTrue())
		})

		It("Should Manage Preexisting MCE", func() {
			By("Applying prereqs")
			ctx := context.Background()
			ApplyPrereqs(k8sClient)

			By("Creating MCE")
			mce := resources.EmptyMCE()
			Expect(k8sClient.Create(ctx, &mce)).Should(Succeed())

			By("Creating MCH")
			mch := resources.EmptyMCH()
			Expect(k8sClient.Create(ctx, &mch)).Should(Succeed())
			Expect(k8sClient.Create(ctx, mchoDeployment)).Should(Succeed())

			By("Ensuring MCH is created")
			createdMCH := &mchov1.MultiClusterHub{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, resources.MCHLookupKey, createdMCH)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Waiting for MCH to be in the running state")
			Eventually(func() bool {
				mch := &mchov1.MultiClusterHub{}
				err := k8sClient.Get(ctx, resources.MCHLookupKey, mch)
				if err == nil {
					return mch.Status.Phase == mchov1.HubRunning
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Validating the managedby label is added to MCE")
			Eventually(func() bool {
				existingMCE := &mcev1.MultiClusterEngine{}
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

			By("Deleting MCH and waiting for it to terminate")
			Eventually(func() bool {
				mch := &mchov1.MultiClusterHub{}
				err := k8sClient.Get(ctx, resources.MCHLookupKey, mch)
				if err != nil && errors.IsNotFound(err) {
					return true
				} else {
					mch := resources.EmptyMCH()
					k8sClient.Delete(ctx, &mch)
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Ensuring MCE remains")
			Expect(k8sClient.Get(ctx, resources.MCELookupKey, &mcev1.MultiClusterEngine{})).Should(Succeed())

		})

		It("Should allow Search to be optional", func() {
			By("Applying prereqs")
			ctx := context.Background()
			ApplyPrereqs(k8sClient)

			By("By creating a new Multiclusterhub with search disabled")
			mch := resources.NoSearchMCH()
			Expect(k8sClient.Create(ctx, &mch)).Should(Succeed())
			Expect(k8sClient.Create(ctx, mchoDeployment)).Should(Succeed())

			By("Ensuring MCH is created")
			createdMCH := &mchov1.MultiClusterHub{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, resources.MCHLookupKey, createdMCH)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Waiting for MCH to be in the running state")
			Eventually(func() bool {
				mch := &mchov1.MultiClusterHub{}
				err := k8sClient.Get(ctx, resources.MCHLookupKey, mch)
				if err == nil {
					return mch.Status.Phase == mchov1.HubRunning
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Ensuring search is not subscribed")
			Eventually(func() bool {
				searchSub := types.NamespacedName{
					Name:      "search-prod-sub",
					Namespace: mchNamespace,
				}
				subscription := appsubv1.Subscription{}
				err := k8sClient.Get(ctx, searchSub, &subscription)
				if err != nil && errors.IsNotFound(err) {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Updating MCH to enable search")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, resources.MCHLookupKey, createdMCH)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			createdMCH.Enable(v1.Search)
			Expect(k8sClient.Update(ctx, createdMCH)).Should(Succeed())

			By("Ensuring search is subscribed")
			Eventually(func() error {
				searchSub := types.NamespacedName{
					Name:      "search-prod-sub",
					Namespace: mchNamespace,
				}
				return k8sClient.Get(ctx, searchSub, &appsubv1.Subscription{})
			}, timeout, interval).Should(Succeed())

			By("Updating MCH to disable search")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, resources.MCHLookupKey, createdMCH)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			createdMCH.Disable(v1.Search)
			Expect(k8sClient.Update(ctx, createdMCH)).Should(Succeed())

			By("Ensuring search is not subscribed")
			Eventually(func() bool {
				searchSub := types.NamespacedName{
					Name:      "search-prod-sub",
					Namespace: mchNamespace,
				}
				subscription := appsubv1.Subscription{}
				err := k8sClient.Get(ctx, searchSub, &subscription)
				if err != nil && errors.IsNotFound(err) {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())
		})

	})

	AfterEach(func() {
		ctx := context.Background()
		By("Ensuring the MCH CR is deleted")
		Eventually(func() bool {
			mch := &mchov1.MultiClusterHub{}
			err := k8sClient.Get(ctx, resources.MCHLookupKey, mch)
			if err != nil && errors.IsNotFound(err) {
				return true
			} else {
				mch := resources.EmptyMCH()
				k8sClient.Delete(ctx, &mch)
			}
			return false
		}, timeout, interval).Should(BeTrue())

		By("Ensuring the MCE CR is deleted")
		Eventually(func() bool {
			mce := resources.EmptyMCE()
			err := k8sClient.Get(ctx, resources.MCELookupKey, &mce)
			if err != nil && errors.IsNotFound(err) {
				return true
			} else {
				mce = resources.EmptyMCE()
				k8sClient.Delete(ctx, &mce)
			}
			return false
		}, timeout, interval).Should(BeTrue())

		By("Tearing down the test environment")
		Expect(testEnv.Stop()).Should(Succeed())
	})
})
