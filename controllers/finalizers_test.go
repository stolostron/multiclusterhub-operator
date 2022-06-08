package controllers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	mchov1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"
	resources "github.com/stolostron/multiclusterhub-operator/test/unit-tests"

	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	appsub "open-cluster-management.io/multicloud-operators-subscription/pkg/apis"

	configv1 "github.com/openshift/api/config/v1"
	netv1 "github.com/openshift/api/config/v1"
	consolev1 "github.com/openshift/api/operator/v1"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("controller finalizer functions", func() {
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

		reconciler *MultiClusterHubReconciler
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

		for _, v := range utils.GetTestImages() {
			key := fmt.Sprintf("OPERAND_IMAGE_%s", strings.ToUpper(v))
			err := os.Setenv(key, "quay.io/test/test:test")
			Expect(err).ToNot(HaveOccurred())
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

		Expect(scheme.AddToScheme(clientScheme)).To(Succeed())
		Expect(mchov1.AddToScheme(clientScheme)).To(Succeed())
		Expect(appsub.AddToScheme(clientScheme)).To(Succeed())
		Expect(apiregistrationv1.AddToScheme(clientScheme)).To(Succeed())
		Expect(apixv1.AddToScheme(clientScheme)).To(Succeed())
		Expect(netv1.AddToScheme(clientScheme)).To(Succeed())
		Expect(olmv1.AddToScheme(clientScheme)).To(Succeed())
		Expect(subv1alpha1.AddToScheme(clientScheme)).To(Succeed())
		Expect(mcev1.AddToScheme(clientScheme)).To(Succeed())
		Expect(configv1.AddToScheme(clientScheme)).To(Succeed())
		Expect(consolev1.AddToScheme(clientScheme)).To(Succeed())
		Expect(promv1.AddToScheme(clientScheme)).To(Succeed())

		k8sManager, err := ctrl.NewManager(clientConfig, ctrl.Options{
			Scheme:                 clientScheme,
			MetricsBindAddress:     "0",
			HealthProbeBindAddress: "0",
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(k8sManager).ToNot(BeNil())

		k8sClient = k8sManager.GetClient()
		Expect(k8sClient).ToNot(BeNil())

		reconciler = &MultiClusterHubReconciler{
			Client: k8sClient,
			Scheme: k8sManager.GetScheme(),
			Log:    ctrl.Log.WithName("controllers").WithName("MultiClusterHub"),
		}
		Expect(reconciler.SetupWithManager(k8sManager)).To(Succeed())

		//ctx = context.Background()

		go func() {
			// For explanation of GinkgoRecover in a go routine, see
			// https://onsi.github.io/ginkgo/#mental-model-how-ginkgo-handles-failure
			defer GinkgoRecover()

			Expect(k8sManager.Start(signalHandlerContext)).To(Succeed())
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

	It("Should allow Finalizers to work properly", func() {
		os.Setenv("DIRECTORY_OVERRIDE", "../pkg/templates")
		defer os.Unsetenv("DIRECTORY_OVERRIDE")
		ctx := context.Background()
		By("Applying Namespace")
		Expect(k8sClient.Create(ctx, resources.OCMNamespace())).Should(Succeed())
		Expect(k8sClient.Create(ctx, resources.MonitoringNamespace())).Should(Succeed())

		By("By creating a new Multiclusterhub with Insights enabled")
		mch := resources.InsightsMCH()
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

		By("Executing Finalizers")
		err := reconciler.cleanupDeployments(reconciler.Log, &mch)
		Expect(err).To(BeNil())

		err = reconciler.cleanupServices(reconciler.Log, &mch)
		Expect(err).To(BeNil())

		err = reconciler.cleanupServiceAccounts(reconciler.Log, &mch)
		Expect(err).To(BeNil())

		err = reconciler.cleanupServiceMonitors(reconciler.Log, &mch)
		Expect(err).To(BeNil())

	})
})
