package v1_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	mchov1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	mchoctrl "github.com/stolostron/multiclusterhub-operator/controllers"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"

	mcev1 "github.com/stolostron/backplane-operator/api/v1"

	configv1 "github.com/openshift/api/config/v1"
	netv1 "github.com/openshift/api/config/v1"
	consolev1 "github.com/openshift/api/operator/v1"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/scale/scheme"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	timeout  = time.Second * 10
	interval = time.Millisecond * 250
)

var _ = Describe("V1 API Webhook", func() {
	Context("when the control plane is not needed", func() {
		var mch *mchov1.MultiClusterHub

		BeforeEach(func() {
			By("creating an empty mch cr")
			mch = makeMCH()
		})

		It("does nothing during Default method", func() {
			mch.Default()
		})

		It("does nothing during ValidateUpdate method", func() {
			Expect(mch.ValidateUpdate(mch)).To(Succeed())
		})

		It("does nothing during ValidateDelete method", func() {
			Expect(mch.ValidateDelete()).To(Succeed())
		})

	})

	Context("Creating a Multiclusterhub", func() {
		It("Should fail to update multiclusterhub", func() {
			mch := makeMCH()

			By("because of DeploymentMode", func() {
				oldmch := mch.DeepCopyObject()
				mch.SetAnnotations(map[string]string{"deploymentmode": "Hosted"})
				Expect(mch.ValidateUpdate(oldmch)).NotTo(BeNil(), "DeploymentMode should not change")
			})

		})

	})

	Context("when the control plane is needed", func() {
		var (
			mch *mchov1.MultiClusterHub

			clientConfig *rest.Config
			clientScheme = runtime.NewScheme()
			k8sClient    client.Client
			k8sManager   ctrl.Manager
			testEnv      *envtest.Environment
		)

		BeforeEach(func() {
			By("creating an empty mch cr")
			mch = makeMCH()

			By("configuring the test environment")
			testEnv = &envtest.Environment{
				CRDDirectoryPaths: []string{
					filepath.Join("..", "..", "config", "crd", "bases"),
					filepath.Join("..", "..", "test", "unit-tests", "crds"),
				},
				CRDInstallOptions: envtest.CRDInstallOptions{
					CleanUpAfterUse: true,
				},
				ErrorIfCRDPathMissing: true,
			}

			By("bootstrapping test environment")
			Eventually(func() error {
				var err error
				clientConfig, err = testEnv.Start()
				return err
			}, timeout, interval).Should(Succeed())
			Expect(clientConfig).NotTo(BeNil())

			By("configuring test images")
			for _, v := range utils.GetTestImages() {
				key := fmt.Sprintf("OPERAND_IMAGE_%s", strings.ToUpper(v))
				err := os.Setenv(key, "quay.io/test/test:test")
				Expect(err).NotTo(HaveOccurred())
			}
			By("configuring the client scheme")
			Expect(scheme.AddToScheme(clientScheme)).Should(Succeed())
			Expect(appsv1.AddToScheme(clientScheme)).Should(Succeed())
			Expect(corev1.AddToScheme(clientScheme)).Should(Succeed())
			Expect(mchov1.AddToScheme(clientScheme)).Should(Succeed())
			Expect(apiregistrationv1.AddToScheme(clientScheme)).Should(Succeed())
			Expect(apixv1.AddToScheme(clientScheme)).Should(Succeed())
			Expect(netv1.AddToScheme(clientScheme)).Should(Succeed())
			Expect(olmv1.AddToScheme(clientScheme)).Should(Succeed())
			Expect(subv1alpha1.AddToScheme(clientScheme)).Should(Succeed())
			Expect(mcev1.AddToScheme(clientScheme)).Should(Succeed())
			Expect(configv1.AddToScheme(clientScheme)).Should(Succeed())
			Expect(consolev1.AddToScheme(clientScheme)).Should(Succeed())

			By("creating the k8s manager")
			var err error
			k8sManager, err = ctrl.NewManager(clientConfig, ctrl.Options{
				Scheme:                 clientScheme,
				MetricsBindAddress:     "0",
				HealthProbeBindAddress: "0",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(k8sManager).ToNot(BeNil())

			By("getting the k8s client")
			k8sClient = k8sManager.GetClient()
			Expect(k8sClient).ToNot(BeNil())

			By("setting up the reconciler")
			reconciler := &mchoctrl.MultiClusterHubReconciler{
				Client: k8sClient,
				Scheme: k8sManager.GetScheme(),
				Log:    ctrl.Log.WithName("controllers").WithName("MultiClusterHub"),
			}
			Expect(reconciler.SetupWithManager(k8sManager)).Should(Succeed())

			// By("starting the k8s manager")
			// go func() {
			// 	// For explanation of GinkgoRecover in a go routine, see
			// 	// https://onsi.github.io/ginkgo/#mental-model-how-ginkgo-handles-failure
			// 	defer GinkgoRecover()

			// 	Expect(k8sManager.Start(signalHandlerContext)).Should(Succeed())
			// }()
		})

		It("validates the creation of a new mch", func() {
			By("setting up the webhook")
			Expect(mch.SetupWebhookWithManager(k8sManager)).To(Succeed())

			//By("validating the new mch")
			//Expect(mch.ValidateCreate()).To(Succeed())
		})

		It("Should successfully create multiclusterhub", func() {
			By("by creating a new hosted Multiclusterhub resource", func() {
				mch.SetAnnotations(map[string]string{"deploymentmode": "Hosted"})
				Expect(mch.SetupWebhookWithManager(k8sManager)).To(Succeed())
			})
		})

		AfterEach(func() {
			By("tearing down the test environment")
			Expect(testEnv.Stop()).To(Succeed())
		})
	})
})
