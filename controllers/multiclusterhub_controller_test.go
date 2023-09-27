// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	operatorsapiv2 "github.com/operator-framework/api/pkg/operators/v2"
	olmapi "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	mchov1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	v1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengine"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	resources "github.com/stolostron/multiclusterhub-operator/test/unit-tests"
	searchv2v1alpha1 "github.com/stolostron/search-v2-operator/api/v1alpha1"
	ocmapi "open-cluster-management.io/api/addon/v1alpha1"

	configv1 "github.com/openshift/api/config/v1"
	netv1 "github.com/openshift/api/config/v1"
	consolev1 "github.com/openshift/api/operator/v1"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	timeout  = time.Second * 30
	interval = time.Millisecond * 250

	mchName      = "multiclusterhub-operator"
	mchNamespace = "open-cluster-management"
)

var ()

func ApplyPrereqs(k8sClient client.Client) {
	By("Applying Namespace")
	ctx := context.Background()
	Expect(k8sClient.Create(ctx, resources.OCMNamespace())).Should(Succeed())
	Expect(k8sClient.Create(ctx, resources.MonitoringNamespace())).Should(Succeed())
	Expect(k8sClient.Create(ctx, resources.SampleClusterManagementAddOn(operatorv1.SubmarinerAddon)))
}

func RunningState(k8sClient client.Client, reconciler *MultiClusterHubReconciler, mchoDeployment *appsv1.Deployment) {
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
		return createdMCH.Spec.AvailabilityConfig == mchov1.HAHigh
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

	By("Ensuring pull secret is created in backup")
	Eventually(func() bool {
		psn := createdMCH.Spec.ImagePullSecret

		if createdMCH.Enabled(operatorv1.ClusterBackup) && psn != "" {
			ns := BackupNamespace().Name
			nn := types.NamespacedName{
				Name:      psn,
				Namespace: ns,
			}
			pullSecret := &corev1.Secret{}
			err := k8sClient.Get(ctx, nn, pullSecret)
			return err == nil

		} else {
			return true
		}
	})

	By("Ensuring Klusterlet Addon is created")
	Eventually(func() bool {
		ns := LocalClusterNamespace()
		_, err := reconciler.ensureNamespace(createdMCH, ns)
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

	By("Ensuring the trusted-ca-bundle ConfigMap is created")
	Eventually(func(g Gomega) {
		ctx := context.Background()
		namespacedName := types.NamespacedName{
			Name:      defaultTrustBundleName,
			Namespace: mchNamespace,
		}
		res := &corev1.ConfigMap{}
		g.Expect(k8sClient.Get(ctx, namespacedName, res)).To(Succeed())
	}, timeout, interval).Should(Succeed())

	By("ensuring the acm consoleplugin is enabled on the cluster")
	clusterConsole := &consolev1.Console{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "cluster"}, clusterConsole)).To(Succeed())
	Expect(clusterConsole.Spec.Plugins).To(ContainElement("acm"))
}

func PreexistingMCE(k8sClient client.Client, reconciler *MultiClusterHubReconciler, mchoDeployment *appsv1.Deployment) {
	By("Applying prereqs")
	ctx := context.Background()
	ApplyPrereqs(k8sClient)

	By("Creating MCE")
	Expect(k8sClient.Create(ctx, resources.MCENamespace())).Should(Succeed())
	mcesub := &subv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multicluster-engine",
			Namespace: "multicluster-engine",
			Labels:    map[string]string{utils.MCEManagedByLabel: "true"},
		},
		Spec: &subv1alpha1.SubscriptionSpec{
			Package: multiclusterengine.DesiredPackage(),
		},
	}
	Expect(k8sClient.Create(ctx, mcesub)).Should(Succeed())
	mce := resources.EmptyMCE()
	Expect(k8sClient.Create(ctx, &mce)).Should(Succeed())

	By("Creating MCH")
	testsecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testsecret",
			Namespace: "open-cluster-management",
		},
		Data: map[string][]byte{".dockerconfigjson": []byte("{\"auth\": \"test\"}")},
		Type: corev1.SecretTypeDockerConfigJson,
	}
	Expect(k8sClient.Create(context.TODO(), testsecret)).Should(Succeed())
	mch := resources.EmptyMCH()
	mch.Spec.ImagePullSecret = "testsecret"
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

	By("Ensuring pull secret is created in MCE namespace")
	nn := types.NamespacedName{
		Name:      "testsecret",
		Namespace: "multicluster-engine",
	}
	pullSecret := &corev1.Secret{}
	Expect(k8sClient.Get(ctx, nn, pullSecret)).Should(Succeed(), "Didn't find imagePullSecret copied to MCE's namespace")

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
		reconciler       *MultiClusterHubReconciler
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
			Expect(err).NotTo(HaveOccurred())
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
		Expect(searchv2v1alpha1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(promv1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(mchov1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(apiregistrationv1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(apixv1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(netv1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(olmv1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(subv1alpha1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(mcev1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(configv1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(consolev1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(olmapi.AddToScheme(clientScheme)).Should(Succeed())
		Expect(ocmapi.AddToScheme(clientScheme)).Should(Succeed())
		Expect(networking.AddToScheme(clientScheme)).Should(Succeed())
		Expect(operatorsapiv2.AddToScheme(clientScheme)).Should(Succeed())
		k8sManager, err := ctrl.NewManager(clientConfig, ctrl.Options{
			Scheme:                 clientScheme,
			MetricsBindAddress:     "0",
			HealthProbeBindAddress: "0",
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(k8sManager).ToNot(BeNil())

		k8sClient = k8sManager.GetClient()
		Expect(k8sClient).ToNot(BeNil())
		upgradeableCondition, _ := utils.NewOperatorCondition(k8sClient, operatorsapiv2.Upgradeable)
		reconciler = &MultiClusterHubReconciler{
			Client:          k8sClient,
			Scheme:          k8sManager.GetScheme(),
			Log:             ctrl.Log.WithName("controllers").WithName("MultiClusterHub"),
			UpgradeableCond: upgradeableCondition,
			// CacheSpec: CacheSpec{
			// 	ImageOverrides: map[string]string{},
			// },
		}
		//Expect(reconciler.SetupWithManager(k8sManager)).Should(Succeed())
		success, err := reconciler.SetupWithManager(k8sManager)
		Expect(success).ToNot(BeNil())
		Expect(err).ToNot(HaveOccurred())

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

		// Create a console (for configuring consoleplugin)
		Expect(k8sClient.Create(context.Background(), &consolev1.Console{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster",
			},
			Spec: consolev1.ConsoleSpec{
				OperatorSpec: consolev1.OperatorSpec{
					ManagementState: consolev1.Managed,
				},
			},
		})).To(Succeed())

	})

	Context("When updating Multiclusterhub status", func() {
		It("Should get to a running state", func() {
			os.Setenv("DIRECTORY_OVERRIDE", "../pkg/templates")
			defer os.Unsetenv("DIRECTORY_OVERRIDE")
			os.Setenv("OPERATOR_PACKAGE", "advanced-cluster-management")
			defer os.Unsetenv("OPERATOR_PACKAGE")
			RunningState(k8sClient, reconciler, mchoDeployment)
		})

		It("Should Manage Preexisting MCE", func() {
			os.Setenv("DIRECTORY_OVERRIDE", "../pkg/templates")
			defer os.Unsetenv("DIRECTORY_OVERRIDE")
			os.Setenv("OPERATOR_PACKAGE", "advanced-cluster-management")
			defer os.Unsetenv("OPERATOR_PACKAGE")
			PreexistingMCE(k8sClient, reconciler, mchoDeployment)

		})

		It("Should get to a running state in Community Mode", func() {
			os.Setenv("DIRECTORY_OVERRIDE", "../pkg/templates")
			defer os.Unsetenv("DIRECTORY_OVERRIDE")
			os.Setenv("OPERATOR_PACKAGE", "stolostron")
			defer os.Unsetenv("OPERATOR_PACKAGE")
			os.Setenv("ACM_HUB_OCP_VERSION", "4.12.0")
			defer os.Unsetenv("ACM_HUB_OCP_VERSION")
			RunningState(k8sClient, reconciler, mchoDeployment)
		})

		It("Should Manage Preexisting MCE in Community Mode", func() {
			os.Setenv("DIRECTORY_OVERRIDE", "../pkg/templates")
			defer os.Unsetenv("DIRECTORY_OVERRIDE")
			os.Setenv("OPERATOR_PACKAGE", "stolostron")
			defer os.Unsetenv("OPERATOR_PACKAGE")
			PreexistingMCE(k8sClient, reconciler, mchoDeployment)

		})

		It("Should allow MCH components to be optional", func() {
			os.Setenv("DIRECTORY_OVERRIDE", "../pkg/templates")
			defer os.Unsetenv("DIRECTORY_OVERRIDE")
			os.Setenv("OPERATOR_PACKAGE", "advanced-cluster-management")
			defer os.Unsetenv("OPERATOR_PACKAGE")
			By("Applying prereqs")
			ctx := context.Background()
			ApplyPrereqs(k8sClient)

			By("Creating a new Multiclusterhub with components disabled")
			mch := resources.NoComponentMCH()
			mch.Disable(operatorv1.Appsub)
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

			By("Ensuring console is not subscribed")
			Eventually(func() bool {
				consoleDep := types.NamespacedName{
					Name:      "console-chart-console-v2",
					Namespace: mchNamespace,
				}
				deployment := appsv1.Deployment{}
				err := k8sClient.Get(ctx, consoleDep, &deployment)
				if err != nil && errors.IsNotFound(err) {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Ensuring insights is not subscribed")
			Eventually(func() bool {
				insightsDep := types.NamespacedName{
					Name:      "insights-client",
					Namespace: mchNamespace,
				}
				deployment := appsv1.Deployment{}
				err := k8sClient.Get(ctx, insightsDep, &deployment)
				if err != nil && errors.IsNotFound(err) {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Ensuring search is not subscribed")
			Eventually(func() bool {
				searchDep := types.NamespacedName{
					Name:      "search-v2-operator-controller-manager",
					Namespace: mchNamespace,
				}
				deployment := appsv1.Deployment{}
				err := k8sClient.Get(ctx, searchDep, &deployment)
				if err != nil && errors.IsNotFound(err) {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Ensuring grc is not subscribed")
			Eventually(func() bool {
				grcDep := types.NamespacedName{
					Name:      "grc-policy-addon-controller",
					Namespace: mchNamespace,
				}
				deployment := appsv1.Deployment{}
				err := k8sClient.Get(ctx, grcDep, &deployment)
				if err != nil && errors.IsNotFound(err) {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Ensuring clusterlifecycle is not subscribed")
			Eventually(func() bool {
				clcDep := types.NamespacedName{
					Name:      "klusterlet-addon-controller-v2",
					Namespace: mchNamespace,
				}
				deployment := appsv1.Deployment{}
				err := k8sClient.Get(ctx, clcDep, &deployment)
				if err != nil && errors.IsNotFound(err) {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Ensuring observability is not subscribed")
			Eventually(func() bool {
				obsDep := types.NamespacedName{
					Name:      "multicluster-observability-operator",
					Namespace: mchNamespace,
				}
				deployment := appsv1.Deployment{}
				err := k8sClient.Get(ctx, obsDep, &deployment)
				if err != nil && errors.IsNotFound(err) {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Ensuring volsync is not subscribed")
			Eventually(func() bool {
				volDep := types.NamespacedName{
					Name:      "volsync-addon-controller",
					Namespace: mchNamespace,
				}
				deployment := appsv1.Deployment{}
				err := k8sClient.Get(ctx, volDep, &deployment)
				if err != nil && errors.IsNotFound(err) {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Updating MCH to enable components")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, resources.MCHLookupKey, createdMCH)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			createdMCH.Enable(v1.Console)
			createdMCH.Enable(v1.Insights)
			createdMCH.Enable(v1.Search)
			createdMCH.Enable(v1.GRC)
			createdMCH.Enable(v1.ClusterLifecycle)
			createdMCH.Enable(v1.MultiClusterObservability)
			createdMCH.Enable(v1.Volsync)

			Expect(k8sClient.Update(ctx, createdMCH)).Should(Succeed())

			By("Ensuring console is subscribed")
			Eventually(func() error {
				consoleDep := types.NamespacedName{
					Name:      "console-chart-console-v2",
					Namespace: mchNamespace,
				}
				return k8sClient.Get(ctx, consoleDep, &appsv1.Deployment{})
			}, timeout, interval).Should(Succeed())

			By("Ensuring insights is subscribed")
			Eventually(func() error {
				insightsDep := types.NamespacedName{
					Name:      "insights-client",
					Namespace: mchNamespace,
				}
				return k8sClient.Get(ctx, insightsDep, &appsv1.Deployment{})
			}, timeout, interval).Should(Succeed())

			By("Ensuring search is subscribed")
			Eventually(func() error {
				searchDep := types.NamespacedName{
					Name:      "search-v2-operator-controller-manager",
					Namespace: mchNamespace,
				}
				return k8sClient.Get(ctx, searchDep, &appsv1.Deployment{})
			}, timeout, interval).Should(Succeed())

			By("Ensuring grc is subscribed")
			Eventually(func() error {
				grcDep := types.NamespacedName{
					Name:      "grc-policy-addon-controller",
					Namespace: mchNamespace,
				}
				return k8sClient.Get(ctx, grcDep, &appsv1.Deployment{})
			}, timeout, interval).Should(Succeed())

			By("Ensuring clusterlifecycle is subscribed")
			Eventually(func() error {
				clcDep := types.NamespacedName{
					Name:      "klusterlet-addon-controller-v2",
					Namespace: mchNamespace,
				}
				return k8sClient.Get(ctx, clcDep, &appsv1.Deployment{})
			}, timeout, interval).Should(Succeed())

			By("Ensuring observability is subscribed")
			Eventually(func() error {
				obsDep := types.NamespacedName{
					Name:      "multicluster-observability-operator",
					Namespace: mchNamespace,
				}
				return k8sClient.Get(ctx, obsDep, &appsv1.Deployment{})
			}, timeout, interval).Should(Succeed())

			By("Ensuring volsync is subscribed")
			Eventually(func() error {
				volDep := types.NamespacedName{
					Name:      "volsync-addon-controller",
					Namespace: mchNamespace,
				}
				return k8sClient.Get(ctx, volDep, &appsv1.Deployment{})
			}, timeout, interval).Should(Succeed())

			By("Updating MCH to disable search")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, resources.MCHLookupKey, createdMCH)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			// Appending to components rather than replacing with `Disable()`
			createdMCH.Spec.Overrides.Components = append(
				createdMCH.Spec.Overrides.Components,
				v1.ComponentConfig{Name: v1.Console, Enabled: false},
				v1.ComponentConfig{Name: v1.GRC, Enabled: false},
				v1.ComponentConfig{Name: v1.Insights, Enabled: false},
				v1.ComponentConfig{Name: v1.Search, Enabled: false},
				v1.ComponentConfig{Name: v1.ClusterLifecycle, Enabled: false},
				v1.ComponentConfig{Name: v1.MultiClusterObservability, Enabled: false},
				v1.ComponentConfig{Name: v1.Volsync, Enabled: false},
			)

			Expect(k8sClient.Update(ctx, createdMCH)).Should(Succeed())
		})
	})

	Context("When managing deployments", func() {
		It("Creates and removes a deployment", func() {
			os.Setenv("DIRECTORY_OVERRIDE", "../pkg/templates")
			defer os.Unsetenv("DIRECTORY_OVERRIDE")
			By("Applying prereqs")
			ApplyPrereqs(k8sClient)
			ctx := context.Background()

			By("Ensuring Insights")
			mch := resources.SpecMCH()
			testImages := map[string]string{}
			for _, v := range utils.GetTestImages() {
				testImages[v] = "quay.io/test/test:Test"
			}

			result, err := reconciler.ensureComponent(ctx, mch, operatorv1.Insights, testImages)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring No Insights")
			result, err = reconciler.ensureNoComponent(ctx, mch, operatorv1.Insights, testImages)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring Cluster Backup")
			ns := BackupNamespace()
			result, err = reconciler.ensureNamespace(mch, ns)
			Expect(err).To(BeNil())

			result, err = reconciler.ensureComponent(ctx, mch, operatorv1.ClusterBackup, testImages)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring No Cluster Backup")
			result, err = reconciler.ensureNoComponent(ctx, mch, operatorv1.ClusterBackup, testImages)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring Search-v2")
			result, err = reconciler.ensureComponent(ctx, mch, operatorv1.Search, testImages)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring No Search-v2")
			Eventually(func() bool {
				result, err := reconciler.ensureNoComponent(ctx, mch, operatorv1.Search, testImages)
				return (err == nil && result == ctrl.Result{})
			}, timeout, interval).Should(BeTrue())

			By("Ensuring CLC")
			result, err = reconciler.ensureComponent(ctx, mch, operatorv1.ClusterLifecycle, testImages)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring No CLC")
			result, err = reconciler.ensureNoComponent(ctx, mch, operatorv1.ClusterLifecycle, testImages)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring App-Lifecycle")
			result, err = reconciler.ensureComponent(ctx, mch, operatorv1.Appsub, testImages)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring No App-Lifecycle")
			result, err = reconciler.ensureNoComponent(ctx, mch, operatorv1.Appsub, testImages)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring GRC")
			result, err = reconciler.ensureComponent(ctx, mch, operatorv1.GRC, testImages)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring No GRC")
			result, err = reconciler.ensureNoComponent(ctx, mch, operatorv1.GRC, testImages)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring Console")
			result, err = reconciler.ensureComponent(ctx, mch, operatorv1.Console, testImages)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring No Console")
			result, err = reconciler.ensureNoComponent(ctx, mch, operatorv1.Console, testImages)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring Volsync")
			result, err = reconciler.ensureComponent(ctx, mch, operatorv1.Volsync, testImages)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring No Volsync")
			result, err = reconciler.ensureNoComponent(ctx, mch, operatorv1.Volsync, testImages)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring MultiClusterObservability")
			result, err = reconciler.ensureComponent(ctx, mch, operatorv1.MultiClusterObservability, testImages)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring No MultiClusterObservability")
			result, err = reconciler.ensureNoComponent(ctx, mch, operatorv1.MultiClusterObservability, testImages)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring ClusterPermission")
			result, err = reconciler.ensureComponent(ctx, mch, operatorv1.ClusterPermission, testImages)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring No ClusterPermission")
			result, err = reconciler.ensureNoComponent(ctx, mch, operatorv1.ClusterPermission, testImages)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring SubmarinerAddon")
			result, err = reconciler.ensureComponent(ctx, mch, operatorv1.SubmarinerAddon, testImages)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring No SubmarinerAddon")
			Eventually(func() bool {
				result, err := reconciler.ensureNoComponent(ctx, mch, operatorv1.SubmarinerAddon, testImages)
				return (err == nil && result == ctrl.Result{})
			}, timeout, interval).Should(BeTrue())

			By("Ensuring No ClusterManagementAddon")
			result, err = reconciler.ensureNoClusterManagementAddOn(mch, "unknown")
			Expect(result).To(Equal(ctrl.Result{Requeue: true}))
			Expect(err).To(Not(BeNil()))

			By("Ensuring No Unregistered Component")
			result, err = reconciler.ensureNoComponent(ctx, mch, "unknown", testImages)
			Expect(result).To(Equal(ctrl.Result{RequeueAfter: resyncPeriod}))
			Expect(err).To(BeNil())

			By("Ensuring No OpenShift Cluster Monitoring Labels")
			mch2 := &mchov1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{Name: "mch", Namespace: "test-ns-1"},
			}

			result, err = reconciler.ensureOpenShiftNamespaceLabel(ctx, mch2)
			Expect(result).To(Equal(ctrl.Result{Requeue: true}))
		})
	})

	Context("When managing deployments", func() {
		It("Creates and removes a deployment", func() {
			os.Setenv("DIRECTORY_OVERRIDE", "../../pkg/templates")
			defer os.Unsetenv("DIRECTORY_OVERRIDE")
			By("Applying prereqs")
			ApplyPrereqs(k8sClient)
			ctx := context.Background()

			By("Ensuring Insights")
			mch := resources.SpecMCH()
			testImages := map[string]string{}
			for _, v := range utils.GetTestImages() {
				testImages[v] = "quay.io/test/test:Test"
			}

			result, err := reconciler.ensureComponent(ctx, mch, operatorv1.Appsub, testImages)
			Expect(result).To(Equal(ctrl.Result{RequeueAfter: 20000000000}))
			Expect(err).To(BeNil())

			result, err = reconciler.ensureNoComponent(ctx, mch, operatorv1.Appsub, testImages)
			Expect(result).To(Equal(ctrl.Result{RequeueAfter: 20000000000}))
			Expect(err).To(BeNil())
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
