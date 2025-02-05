// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	operatorsapiv2 "github.com/operator-framework/api/pkg/operators/v2"
	olmapi "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengine"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	resources "github.com/stolostron/multiclusterhub-operator/test/unit-tests"
	searchv2v1alpha1 "github.com/stolostron/search-v2-operator/api/v1alpha1"
	ocmapi "open-cluster-management.io/api/addon/v1alpha1"

	configv1 "github.com/openshift/api/config/v1"
	ocopv1 "github.com/openshift/api/operator/v1"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	timeout  = time.Second * 90
	interval = time.Millisecond * 250

	mchName      = "multiclusterhub-operator"
	mchNamespace = "open-cluster-management"
)

var (
	ctrlCtx    context.Context
	ctrlCancel context.CancelFunc
	recon      = MultiClusterHubReconciler{
		Client: fake.NewClientBuilder().Build(),
		Scheme: scheme.Scheme,
	}
)

func ApplyPrereqs(k8sClient client.Client) {
	ctx := context.Background()

	By("Creating Ingress")
	Expect(k8sClient.Create(ctx, resources.OCPIngress())).Should(Succeed())

	// By("Creating Infrastructure")
	// Expect(k8sClient.Create(ctx, resources.OCPInfrastructure())).Should(Succeed())

	By("Applying Namespace")
	Expect(k8sClient.Create(ctx, resources.OCMNamespace())).Should(Succeed())
	Expect(k8sClient.Create(ctx, resources.MonitoringNamespace())).Should(Succeed())

	By("Applying MCE CatalogSource")
	Expect(k8sClient.Create(ctx, resources.MarketNamespace())).Should(Succeed())
	Expect(k8sClient.Create(ctx, resources.MCECatalogSource())).Should(Succeed())
}

func removeSubmarinerFinalizer(k8sClient client.Client, reconciler *MultiClusterHubReconciler) {
	ctx := context.Background()

	addonName, _ := operatorv1.GetClusterManagementAddonName(operatorv1.SubmarinerAddon)
	clusterMgmtAddon := &ocmapi.ClusterManagementAddOn{}

	if err := k8sClient.Get(ctx, types.NamespacedName{Name: addonName}, clusterMgmtAddon); err == nil {
		// If the ClusterManagementAddon resource is found, remove the finalizer and update it.
		clusterMgmtAddon.SetFinalizers([]string{})
		if err := k8sClient.Update(ctx, clusterMgmtAddon); err != nil {
			reconciler.Log.Error(err, "failed to update ClusterManagementAddon")
		}
	}
}

func RunningState(k8sClient client.Client, reconciler *MultiClusterHubReconciler, mchDeployment *appsv1.Deployment) {
	By("Applying prereqs")
	ctx := context.Background()
	ApplyPrereqs(k8sClient)

	By("By creating a new Multiclusterhub")
	mch := resources.EmptyMCH()
	// To skip ensureKlusterletAddonConfig
	mch.Spec.DisableHubSelfManagement = true
	Expect(k8sClient.Create(ctx, &mch)).Should(Succeed())
	Expect(k8sClient.Create(ctx, mchDeployment)).Should(Succeed())

	By("Ensuring MCH is created")
	createdMCH := &operatorv1.MultiClusterHub{}
	Eventually(
		k8sClient.Get(ctx, resources.MCHLookupKey, createdMCH),
		timeout, interval,
	).Should(Succeed())

	annotations := map[string]string{
		"mch-imageRepository": "quay.io/test",
	}
	createdMCH.SetAnnotations(annotations)
	Expect(k8sClient.Update(ctx, createdMCH)).Should(Succeed())

	By("Ensuring Defaults are set")
	Eventually(func() operatorv1.MultiClusterHub {
		err := k8sClient.Get(ctx, resources.MCHLookupKey, createdMCH)
		Expect(err).Should(BeNil())
		return *createdMCH
	}, timeout, interval).Should(HaveField("Spec.AvailabilityConfig", operatorv1.HAHigh))

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
	Eventually(func() error {
		ns := LocalClusterNamespace()
		_, err := reconciler.ensureNamespace(createdMCH, ns)
		return err
	}, timeout, interval).Should(Succeed(), "KlusterletAddon should be created")

	By("Waiting for MCH to be in the running state")
	Eventually(func(g Gomega) operatorv1.MultiClusterHub {
		mch := &operatorv1.MultiClusterHub{}
		g.Expect(k8sClient.Get(ctx, resources.MCHLookupKey, mch)).Should(Succeed())
		return *mch
	}, timeout, interval).Should(HaveField("Status.Phase", operatorv1.HubRunning))

	By("Ensuring the trusted-ca-bundle ConfigMap is created")
	Eventually(func() error {
		ctx := context.Background()
		namespacedName := types.NamespacedName{
			Name:      defaultTrustBundleName,
			Namespace: mchNamespace,
		}
		res := &corev1.ConfigMap{}
		return k8sClient.Get(ctx, namespacedName, res)
	}, timeout, interval).Should(Succeed(), "trusted-ca-bundle ConfigMap should be created")

	By("Ensuring the acm consoleplugin is enabled on the cluster")
	clusterConsole := &ocopv1.Console{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "cluster"}, clusterConsole)).To(Succeed())
	Expect(clusterConsole.Spec.Plugins).To(ContainElement("acm"))
}

func PreexistingMCE(k8sClient client.Client, reconciler *MultiClusterHubReconciler, mchDeployment *appsv1.Deployment) {
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
	// To skip ensureKlusterletAddonConfig
	mch.Spec.DisableHubSelfManagement = true
	mch.Spec.ImagePullSecret = "testsecret"
	Expect(k8sClient.Create(ctx, &mch)).Should(Succeed())
	Expect(k8sClient.Create(ctx, mchDeployment)).Should(Succeed())

	By("Ensuring MCH is created")
	createdMCH := &operatorv1.MultiClusterHub{}
	Eventually(func() bool {
		err := k8sClient.Get(ctx, resources.MCHLookupKey, createdMCH)
		return err == nil
	}, timeout, interval).Should(BeTrue())

	By("Waiting for MCH to be in the running state")
	Eventually(func() bool {
		mch := &operatorv1.MultiClusterHub{}
		err := k8sClient.Get(ctx, resources.MCHLookupKey, mch)
		if err == nil {
			return mch.Status.Phase == operatorv1.HubRunning
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
		mch := &operatorv1.MultiClusterHub{}
		err := k8sClient.Get(ctx, resources.MCHLookupKey, mch)
		if err != nil && errors.IsNotFound(err) {
			return true
		} else {
			mch := resources.EmptyMCH()
			k8sClient.Delete(ctx, &mch)
		}

		removeSubmarinerFinalizer(k8sClient, reconciler)
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
		mchDeployment    *appsv1.Deployment
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

		mchDeployment = &appsv1.Deployment{
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
		Expect(operatorv1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(apiregistrationv1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(apixv1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(olmv1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(subv1alpha1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(mcev1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(configv1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(ocopv1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(olmapi.AddToScheme(clientScheme)).Should(Succeed())
		Expect(ocmapi.AddToScheme(clientScheme)).Should(Succeed())
		Expect(networking.AddToScheme(clientScheme)).Should(Succeed())
		Expect(operatorsapiv2.AddToScheme(clientScheme)).Should(Succeed())
		k8sManager, err := ctrl.NewManager(clientConfig, ctrl.Options{
			Scheme: clientScheme,
			Metrics: metricsserver.Options{
				BindAddress: "0",
			},
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
		// Expect(reconciler.SetupWithManager(k8sManager)).Should(Succeed())
		success, err := reconciler.SetupWithManager(k8sManager)
		Expect(success).ToNot(BeNil())
		Expect(err).ToNot(HaveOccurred())

		go func() {
			// For explanation of GinkgoRecover in a go routine, see
			// https://onsi.github.io/ginkgo/#mental-model-how-ginkgo-handles-failure
			defer GinkgoRecover()

			ctrlCtx, ctrlCancel = context.WithCancel(context.TODO())

			Expect(k8sManager.Start(ctrlCtx)).Should(Succeed(), "MCH controller should start successfully")
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
		Expect(k8sClient.Create(context.Background(), &ocopv1.Console{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster",
			},
			Spec: ocopv1.ConsoleSpec{
				OperatorSpec: ocopv1.OperatorSpec{
					ManagementState: ocopv1.Managed,
				},
			},
		})).To(Succeed())

		Expect(k8sClient.Create(context.Background(), &configv1.Authentication{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster",
			},
			Spec: configv1.AuthenticationSpec{
				ServiceAccountIssuer: "",
			},
		})).To(Succeed())

		Expect(k8sClient.Create(context.Background(), &ocopv1.CloudCredential{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster",
			},
			Spec: ocopv1.CloudCredentialSpec{
				CredentialsMode: "",
				OperatorSpec: ocopv1.OperatorSpec{
					ManagementState: "Managed",
				},
			},
		})).To(Succeed())

		Expect(k8sClient.Create(context.Background(), &configv1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster",
			},
			Spec: configv1.InfrastructureSpec{
				PlatformSpec: configv1.PlatformSpec{
					Type: "AWS",
				},
			},
		})).To(Succeed())
	})

	os.Setenv("DIRECTORY_OVERRIDE", "../pkg/templates")
	defer os.Unsetenv("DIRECTORY_OVERRIDE")

	os.Setenv("ACM_HUB_OCP_VERSION", "4.12.0")
	defer os.Unsetenv("ACM_HUB_OCP_VERSION")

	Context("When updating Multiclusterhub status", func() {
		It("Should get to a running state", func() {
			os.Setenv("OPERATOR_PACKAGE", "advanced-cluster-management")
			defer os.Unsetenv("OPERATOR_PACKAGE")
			RunningState(k8sClient, reconciler, mchDeployment)
		})

		It("Should Manage Preexisting MCE", func() {
			os.Setenv("OPERATOR_PACKAGE", "advanced-cluster-management")
			defer os.Unsetenv("OPERATOR_PACKAGE")
			PreexistingMCE(k8sClient, reconciler, mchDeployment)
		})

		It("Should get to a running state in Community Mode", func() {
			os.Setenv("OPERATOR_PACKAGE", "stolostron")
			defer os.Unsetenv("OPERATOR_PACKAGE")
			RunningState(k8sClient, reconciler, mchDeployment)
		})

		It("Should Manage Preexisting MCE in Community Mode", func() {
			os.Setenv("OPERATOR_PACKAGE", "stolostron")
			defer os.Unsetenv("OPERATOR_PACKAGE")
			PreexistingMCE(k8sClient, reconciler, mchDeployment)
		})

		It("Should allow MCH components to be optional", func() {
			os.Setenv("OPERATOR_PACKAGE", "advanced-cluster-management")
			defer os.Unsetenv("OPERATOR_PACKAGE")

			By("Applying prereqs")
			ctx := context.Background()
			ApplyPrereqs(k8sClient)

			By("Creating a new Multiclusterhub with components disabled")
			mch := resources.NoComponentMCH()
			// To skip ensureKlusterletAddonConfig
			mch.Spec.DisableHubSelfManagement = true
			mch.Disable(operatorv1.Appsub)
			Expect(k8sClient.Create(ctx, &mch)).Should(Succeed())
			Expect(k8sClient.Create(ctx, mchDeployment)).Should(Succeed())

			By("Ensuring MCH is created")
			createdMCH := &operatorv1.MultiClusterHub{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, resources.MCHLookupKey, createdMCH)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Waiting for MCH to be in the running state")
			Eventually(func() bool {
				mch := &operatorv1.MultiClusterHub{}
				// To skip ensureKlusterletAddonConfig
				createdMCH.Spec.DisableHubSelfManagement = true

				err := k8sClient.Get(ctx, resources.MCHLookupKey, mch)
				if err == nil {
					return mch.Status.Phase == operatorv1.HubRunning
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

			createdMCH.Enable(operatorv1.Console)
			createdMCH.Enable(operatorv1.Insights)
			createdMCH.Enable(operatorv1.Search)
			createdMCH.Enable(operatorv1.GRC)
			createdMCH.Enable(operatorv1.ClusterLifecycle)
			createdMCH.Enable(operatorv1.MultiClusterObservability)
			createdMCH.Enable(operatorv1.Volsync)
			createdMCH.Enable(operatorv1.FlightControl)

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
				operatorv1.ComponentConfig{Name: operatorv1.Console, Enabled: false},
				operatorv1.ComponentConfig{Name: operatorv1.GRC, Enabled: false},
				operatorv1.ComponentConfig{Name: operatorv1.Insights, Enabled: false},
				operatorv1.ComponentConfig{Name: operatorv1.Search, Enabled: false},
				operatorv1.ComponentConfig{Name: operatorv1.ClusterLifecycle, Enabled: false},
				operatorv1.ComponentConfig{Name: operatorv1.MultiClusterObservability, Enabled: false},
				operatorv1.ComponentConfig{Name: operatorv1.Volsync, Enabled: false},
			)

			Expect(k8sClient.Update(ctx, createdMCH)).Should(Succeed())

			By("Pausing MCH to pause reconciliation")
			Eventually(func() bool {
				annotations := createdMCH.GetAnnotations()
				if annotations == nil {
					annotations = make(map[string]string)
				}

				annotations[utils.AnnotationMCHPause] = "true"
				createdMCH.Annotations = annotations
				_ = k8sClient.Update(ctx, createdMCH)

				return utils.IsPaused(createdMCH)
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When managing deployments", func() {
		It("Creates and removes a deployment", func() {
			By("Applying prereqs")
			ApplyPrereqs(k8sClient)
			ctx := context.Background()

			mch := resources.SpecMCH()
			testImages := map[string]string{}
			for _, v := range utils.GetTestImages() {
				testImages[v] = "quay.io/test/test:Test"
			}

			testCacheSpec := CacheSpec{
				ImageOverrides:    testImages,
				TemplateOverrides: map[string]string{},
			}

			By("Ensuring Insights")
			result, err := reconciler.ensureComponent(ctx, mch, operatorv1.Insights, testCacheSpec, false)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring No Insights")
			Eventually(func() bool {
				result, err = reconciler.ensureNoComponent(ctx, mch, operatorv1.Insights, testCacheSpec, false)
				return (err == nil && result == ctrl.Result{})
			}, timeout, interval).Should(BeTrue())

			By("Ensuring Cluster Backup")
			ns := BackupNamespace()
			_, err = reconciler.ensureNamespace(mch, ns)
			Expect(err).To(BeNil())

			result, err = reconciler.ensureComponent(ctx, mch, operatorv1.ClusterBackup, testCacheSpec, false)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring No Cluster Backup")
			Eventually(func() bool {
				result, err = reconciler.ensureNoComponent(ctx, mch, operatorv1.ClusterBackup, testCacheSpec, false)
				return (err == nil && result == ctrl.Result{})
			}, timeout, interval).Should(BeTrue())

			By("Ensuring Search-v2")
			result, err = reconciler.ensureComponent(ctx, mch, operatorv1.Search, testCacheSpec, false)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring No Search-v2")
			Eventually(func() bool {
				result, err := reconciler.ensureNoComponent(ctx, mch, operatorv1.Search, testCacheSpec, false)
				return (err == nil && result == ctrl.Result{})
			}, timeout, interval).Should(BeTrue())

			By("Ensuring CLC")
			result, err = reconciler.ensureComponent(ctx, mch, operatorv1.ClusterLifecycle, testCacheSpec, false)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring No CLC")
			Eventually(func() bool {
				result, err = reconciler.ensureNoComponent(ctx, mch, operatorv1.ClusterLifecycle, testCacheSpec, false)
				return (err == nil && result == ctrl.Result{})
			}, timeout, interval).Should(BeTrue())

			By("Ensuring App-Lifecycle")
			result, err = reconciler.ensureComponent(ctx, mch, operatorv1.Appsub, testCacheSpec, false)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring No App-Lifecycle")
			Eventually(func() bool {
				result, err = reconciler.ensureNoComponent(ctx, mch, operatorv1.Appsub, testCacheSpec, false)
				return (err == nil && result == ctrl.Result{})
			}, timeout, interval).Should(BeTrue())

			By("Ensuring GRC")
			result, err = reconciler.ensureComponent(ctx, mch, operatorv1.GRC, testCacheSpec, false)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring No GRC")
			Eventually(func() bool {
				result, err = reconciler.ensureNoComponent(ctx, mch, operatorv1.GRC, testCacheSpec, false)
				return (err == nil && result == ctrl.Result{})
			}, timeout, interval).Should(BeTrue())

			By("Ensuring Console")
			result, err = reconciler.ensureComponent(ctx, mch, operatorv1.Console, testCacheSpec, false)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring No Console")
			Eventually(func() bool {
				result, err = reconciler.ensureNoComponent(ctx, mch, operatorv1.Console, testCacheSpec, false)
				return (err == nil && result == ctrl.Result{})
			}, timeout, interval).Should(BeTrue())

			By("Ensuring Volsync")
			result, err = reconciler.ensureComponent(ctx, mch, operatorv1.Volsync, testCacheSpec, false)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring No Volsync")
			Eventually(func() bool {
				result, err = reconciler.ensureNoComponent(ctx, mch, operatorv1.Volsync, testCacheSpec, false)
				return (err == nil && result == ctrl.Result{})
			}, timeout, interval).Should(BeTrue())

			By("Ensuring MultiClusterObservability")
			result, err = reconciler.ensureComponent(ctx, mch, operatorv1.MultiClusterObservability, testCacheSpec, false)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring No MultiClusterObservability")
			Eventually(func() bool {
				result, err = reconciler.ensureNoComponent(ctx, mch, operatorv1.MultiClusterObservability, testCacheSpec, false)
				return (err == nil && result == ctrl.Result{})
			}, timeout, interval).Should(BeTrue())

			By("Ensuring ClusterPermission")
			result, err = reconciler.ensureComponent(ctx, mch, operatorv1.ClusterPermission, testCacheSpec, false)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring No ClusterPermission")
			Eventually(func() bool {
				result, err = reconciler.ensureNoComponent(ctx, mch, operatorv1.ClusterPermission, testCacheSpec, false)
				return (err == nil && result == ctrl.Result{})
			}, timeout, interval).Should(BeTrue())

			By("Ensuring SubmarinerAddon")
			result, err = reconciler.ensureComponent(ctx, mch, operatorv1.SubmarinerAddon, testCacheSpec, false)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring No SubmarinerAddon")
			Eventually(func() bool {
				result, err := reconciler.ensureNoComponent(ctx, mch, operatorv1.SubmarinerAddon, testCacheSpec, false)
				removeSubmarinerFinalizer(k8sClient, reconciler)

				return (err == nil && result == ctrl.Result{})
			}, timeout, interval).Should(BeTrue())

			By("Ensuring SiteConfig")
			result, err = reconciler.ensureComponent(ctx, mch, operatorv1.SiteConfig, testCacheSpec, false)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())

			By("Ensuring No SiteConfig")
			Eventually(func() bool {
				result, err = reconciler.ensureNoComponent(ctx, mch, operatorv1.SiteConfig, testCacheSpec, false)
				return (err == nil && result == ctrl.Result{})
			}, timeout, interval).Should(BeTrue())

			By("Ensuring No ClusterManagementAddon")
			result, err = reconciler.ensureNoClusterManagementAddOn(mch, "unknown")
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(Not(BeNil()))

			By("Ensuring No Unregistered Component")
			result, err = reconciler.ensureNoComponent(ctx, mch, "unknown", testCacheSpec, false)
			Expect(result).To(Equal(ctrl.Result{RequeueAfter: resyncPeriod}))
			Expect(err).To(BeNil())

			By("Ensuring No OpenShift Cluster Monitoring Labels")
			mch2 := &operatorv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{Name: "mch", Namespace: "test-ns-1"},
			}

			result, err = reconciler.ensureOpenShiftNamespaceLabel(ctx, mch2)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).NotTo(BeNil())
		})
	})

	Context("Legacy clean up tasks", func() {
		It("Removes the legacy GRC Prometheus configuration", func() {
			By("Applying prereqs")
			ApplyPrereqs(k8sClient)

			By("Creating the legacy GRC PrometheusRule and ServiceMonitor")
			pr := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"groups": []interface{}{
							map[string]interface{}{
								"name": "some-group",
								"rules": []interface{}{
									map[string]interface{}{
										"expr": "something else",
									},
								},
							},
						},
					},
				},
			}
			pr.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "monitoring.coreos.com",
				Kind:    "PrometheusRule",
				Version: "v1",
			})
			pr.SetName("ocm-grc-policy-propagator-metrics")
			pr.SetNamespace("openshift-monitoring")

			err := k8sClient.Create(context.TODO(), pr)
			Expect(err).To(BeNil())

			sm := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"endpoints": []interface{}{
							map[string]interface{}{
								"path": "/some/path",
							},
						},
						"selector": map[string]interface{}{
							"matchLabels": map[string]interface{}{
								"app": "grc",
							},
						},
					},
				},
			}
			sm.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "monitoring.coreos.com",
				Kind:    "ServiceMonitor",
				Version: "v1",
			})
			sm.SetName("ocm-grc-policy-propagator-metrics")
			sm.SetNamespace("openshift-monitoring")

			err = k8sClient.Create(context.TODO(), sm)
			Expect(err).To(BeNil())

			legacyResourceKind := operatorv1.GetLegacyConfigKind()
			ns := "openshift-monitoring"

			By("Running the cleanup of the legacy configuration kinds")
			for _, kind := range legacyResourceKind {
				err = reconciler.removeLegacyConfigurations(context.TODO(), ns, kind)
				Expect(err).To(BeNil())
			}

			By("Verifying that the legacy GRC PrometheusRule is deleted")
			err = k8sClient.Get(context.TODO(), client.ObjectKeyFromObject(pr), pr)
			Expect(errors.IsNotFound(err)).To(BeTrue())

			By("Verifying that the legacy GRC ServiceMonitor is deleted")
			err = k8sClient.Get(context.TODO(), client.ObjectKeyFromObject(sm), sm)
			Expect(errors.IsNotFound(err)).To(BeTrue())

			By("Running the cleanup of the legacy configuration again should do nothing")
			for _, kind := range legacyResourceKind {
				err = reconciler.removeLegacyConfigurations(context.TODO(), ns, kind)
				Expect(err).To(BeNil())
			}
		})
	})

	AfterEach(func() {
		ctx := context.Background()
		By("Ensuring the MCH CR is deleted")
		Eventually(func() bool {
			mch := &operatorv1.MultiClusterHub{}
			err := k8sClient.Get(ctx, resources.MCHLookupKey, mch)
			if err != nil && errors.IsNotFound(err) {
				return true
			} else {
				mch := resources.EmptyMCH()
				k8sClient.Delete(ctx, &mch)
			}

			removeSubmarinerFinalizer(k8sClient, reconciler)
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

		By("Stopping the controller")
		ctrlCancel()

		// Teardown the test environment once controller is finished.
		// Otherwise from Kubernetes 1.21+, teardon timeouts waiting on
		// kube-apiserver to return.
		By("Tearing down the test environment")
		Expect(testEnv.Stop()).Should(Succeed())
	})
})

func registerScheme() {
	configv1.AddToScheme(scheme.Scheme)
	ocopv1.AddToScheme(scheme.Scheme)
	operatorv1.AddToScheme(scheme.Scheme)
	mcev1.AddToScheme(scheme.Scheme)
	subv1alpha1.AddToScheme(scheme.Scheme)
	olmv1.AddToScheme(scheme.Scheme)
}

func Test_ensureAuthenticationIssuerNotEmpty(t *testing.T) {
	tests := []struct {
		name string
		auth *configv1.Authentication
		want bool
	}{
		{
			name: "should ensure authentication issuer is not empty",
			auth: &configv1.Authentication{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec:       configv1.AuthenticationSpec{ServiceAccountIssuer: "foo"},
			},
			want: true,
		},
		{
			name: "should ensure authentication issuer is empty",
			auth: &configv1.Authentication{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec:       configv1.AuthenticationSpec{ServiceAccountIssuer: ""},
			},
			want: false,
		},
	}

	registerScheme()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				STSEnabledStatus = true
				recon.Client.Delete(context.TODO(), tt.auth)
			}()

			if err := recon.Client.Create(context.TODO(), tt.auth); err != nil {
				t.Errorf("failed to create authentication resource: %v", err)
			}

			_, authOk, _ := recon.ensureAuthenticationIssuerNotEmpty(context.TODO())
			if authOk != tt.want {
				t.Errorf("ensureInfrastructureAWS(ctx) = %v, want %v", authOk, tt.want)
			}
		})
	}
}

func Test_ensureCloudCredentialModeManual(t *testing.T) {
	tests := []struct {
		name      string
		cloudCred *ocopv1.CloudCredential
		want      bool
	}{
		{
			name: "should ensure cloud credential is Manual",
			cloudCred: &ocopv1.CloudCredential{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec:       ocopv1.CloudCredentialSpec{CredentialsMode: "Manual"},
			},
			want: true,
		},
		{
			name: "should ensure cloud credential is not Manual",
			cloudCred: &ocopv1.CloudCredential{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec:       ocopv1.CloudCredentialSpec{CredentialsMode: ""},
			},
			want: false,
		},
	}

	registerScheme()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				STSEnabledStatus = true
				recon.Client.Delete(context.TODO(), tt.cloudCred)
			}()

			if err := recon.Client.Create(context.TODO(), tt.cloudCred); err != nil {
				t.Errorf("failed to create authentication resource: %v", err)
			}

			_, cloudCredOK, _ := recon.ensureCloudCredentialModeManual(context.TODO())
			if cloudCredOK != tt.want {
				t.Errorf("ensureInfrastructureAWS(ctx) = %v, want %v", cloudCredOK, tt.want)
			}
		})
	}
}

func Test_ensureInfrastructureAWS(t *testing.T) {
	tests := []struct {
		name  string
		infra *configv1.Infrastructure
		want  bool
	}{
		{
			name: "should ensure infrastructure is AWS",
			infra: &configv1.Infrastructure{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec:       configv1.InfrastructureSpec{PlatformSpec: configv1.PlatformSpec{Type: "AWS"}},
			},
			want: true,
		},
		{
			name: "should ensure infrastructure is not AWS",
			infra: &configv1.Infrastructure{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec:       configv1.InfrastructureSpec{PlatformSpec: configv1.PlatformSpec{Type: "Azure"}},
			},
			want: false,
		},
	}

	registerScheme()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				STSEnabledStatus = true
				recon.Client.Delete(context.TODO(), tt.infra)
			}()

			if err := recon.Client.Create(context.TODO(), tt.infra); err != nil {
				t.Errorf("failed to create authentication resource: %v", err)
			}

			_, infraOk, _ := recon.ensureInfrastructureAWS(context.TODO())
			if infraOk != tt.want {
				t.Errorf("ensureInfrastructureAWS(ctx) = %v, want %v", infraOk, tt.want)
			}
		})
	}
}

func Test_equivalentKlusterletAddonConfig(t *testing.T) {
	grcEnabled := true

	match := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "agent.open-cluster-management.io/v1",
			"kind":       "KlusterletAddonConfig",
			"metadata": map[string]interface{}{
				"name":      KlusterletAddonConfigName,
				"namespace": ManagedClusterName,
			},
			"spec": map[string]interface{}{
				"applicationManager": map[string]interface{}{
					"enabled": true,
				},
				"certPolicyController": map[string]interface{}{
					"enabled": grcEnabled,
				},
				"policyController": map[string]interface{}{
					"enabled": grcEnabled,
				},
				"searchCollector": map[string]interface{}{
					"enabled": false,
				},
			},
		},
	}

	notMatch := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "agent.open-cluster-management.io/v1",
			"kind":       "KlusterletAddonConfig",
			"metadata": map[string]interface{}{
				"name":      KlusterletAddonConfigName,
				"namespace": ManagedClusterName,
			},
			"spec": map[string]interface{}{
				"applicationManager": map[string]interface{}{
					"enabled": true,
				},
				"certPolicyController": map[string]interface{}{
					"enabled": true,
				},
				"policyController": map[string]interface{}{
					"enabled": true,
				},
				"searchCollector": map[string]interface{}{
					"enabled": true,
				},
			},
		},
	}

	mch := &operatorv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Name: "mch", Namespace: "test-ns-1"},
		Spec: operatorv1.MultiClusterHubSpec{
			Overrides: &operatorv1.Overrides{
				Components: []operatorv1.ComponentConfig{
					{
						Name:    operatorv1.GRC,
						Enabled: true,
					},
				},
			},
		},
	}

	t.Run("Should return isUpdate false when label does not exist", func(t *testing.T) {
		isEquivalent, _, err := equivalentKlusterletAddonConfig(getKlusterletAddonConfig(mch), match, mch)
		if err != nil {
			t.Errorf("equivalentKlusterletAddonConfig has error: %v", err)
		}

		if isEquivalent {
			t.Errorf("isEquivalent should be false")
		}
	})

	t.Run("Should return isUpdate true when label exists", func(t *testing.T) {
		utils.AddInstallerLabel(match, mch.GetName(), mch.GetNamespace())
		isEquivalent, _, err := equivalentKlusterletAddonConfig(getKlusterletAddonConfig(mch), match, mch)
		if err != nil {
			t.Errorf("equivalentKlusterletAddonConfig has error: %v", err)
		}

		if !isEquivalent {
			t.Errorf("isEquivalent should be true")
		}
	})

	t.Run("Should return isUpdate false when not match", func(t *testing.T) {
		utils.AddInstallerLabel(notMatch, mch.GetName(), mch.GetNamespace())
		isEquivalent, _, err := equivalentKlusterletAddonConfig(getKlusterletAddonConfig(mch), notMatch, mch)
		if err != nil {
			t.Errorf("equivalentKlusterletAddonConfig has error: %v", err)
		}

		if isEquivalent {
			t.Errorf("isEquivalent should be false")
		}
	})
}

func Test_ensureNamespaceAndPullSecret(t *testing.T) {
	tests := []struct {
		name      string
		mch       *operatorv1.MultiClusterHub
		ns        *corev1.Namespace
		mceNS     *corev1.Namespace
		mceSecret *corev1.Secret
		mchSecret *corev1.Secret
		want      error
	}{
		{
			name:  "should ensure namespace and pull secret are created",
			mch:   &operatorv1.MultiClusterHub{ObjectMeta: metav1.ObjectMeta{Name: "mch", Namespace: "ocm"}},
			ns:    &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ocm"}},
			mceNS: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "mce"}},
			mceSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mce-image-pull-secret",
					Namespace: "mce",
					Labels: map[string]string{
						"installer.name":      "mch",
						"installer.namespace": "ocm",
					},
				},
			},
			mchSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mch-image-pull-secret",
					Namespace: "ocm",
					Labels: map[string]string{
						"installer.name":      "mch",
						"installer.namespace": "ocm",
					},
				},
			},
			want: nil,
		},
	}

	registerScheme()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure that the namespace for MCE is created
			if _, err := recon.ensureNamespaceAndPullSecret(tt.mch, tt.mceNS); err != nil {
				t.Errorf("ensureNamespaceAndPullSecret(tt.mch, tt.ns) = %v, want = %v", err, tt.want)
			}

			// Verify that the namespace was created and available
			ns := &corev1.Namespace{}
			if err := recon.Client.Get(context.TODO(), types.NamespacedName{Name: tt.mceNS.GetName()}, ns); err != nil {
				t.Errorf("failed to get namespace: %v", err)
			}

			// Update the namespace status phase to an active state
			ns.Status.Phase = corev1.NamespaceActive
			if err := recon.Client.Status().Update(context.TODO(), ns); err != nil {
				t.Errorf("failed to update namespace status: %v", err)
			}

			// Create MCE secret
			if err := recon.Client.Create(context.TODO(), tt.mceSecret); err != nil {
				t.Errorf("failed to create MCE secret: %v", err)
			}

			secretList := &corev1.SecretList{}
			if err := recon.Client.List(context.TODO(), secretList); err != nil {
				t.Errorf("failed to list secrets: %v", err)
			}

			// MCE imagepullsecret should be deleted since MCH secret was not initialized
			if _, err := recon.ensureNamespaceAndPullSecret(tt.mch, tt.mceNS); err != nil {
				t.Errorf("ensureNamespaceAndPullSecret(tt.mch, tt.ns) = %v, want = %v", err, tt.want)
			}

			secretList = &corev1.SecretList{}
			if err := recon.Client.List(context.TODO(), secretList); err != nil {
				t.Errorf("failed to list secrets: %v", err)
			}

			if len(secretList.Items) != 0 {
				t.Errorf("expected 0 secrets, got %d", len(secretList.Items))
			}

			// An error should occur because the MCH image pull secret has not been created yet
			tt.mch.Spec.ImagePullSecret = tt.mchSecret.GetName()
			if _, err := recon.ensureNamespaceAndPullSecret(tt.mch, tt.mceNS); err == nil {
				t.Errorf("ensureNamespaceAndPullSecret(tt.mch, tt.ns) = %v, want = %v", err, tt.want)
			}

			if err := recon.Client.Create(context.TODO(), tt.mchSecret); err != nil {
				t.Errorf("failed to create mch secret: %v", err)
			}

			// Fake clients does not allow for apply patching; therefore we will accept the error.
			if _, err := recon.ensureNamespaceAndPullSecret(tt.mch, tt.mceNS); err == nil {
				t.Errorf("ensureNamespaceAndPullSecret(tt.mch, tt.ns) = %v, want = %v", err, tt.want)
			}
		})
	}
}

func Test_ensureInternalHubComponent(t *testing.T) {
	tests := []struct {
		name string
		mch  *operatorv1.MultiClusterHub
		ns   *corev1.Namespace
		want bool
	}{
		{
			name: "should ensure InternalHubComponent created",
			mch: &operatorv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{Name: "mch", Namespace: "test-ns"},
				Spec: operatorv1.MultiClusterHubSpec{
					Overrides: &operatorv1.Overrides{
						Components: []operatorv1.ComponentConfig{
							{
								Enabled: true,
								Name:    "app-lifecycle",
							},
							{
								Enabled: true,
								Name:    "cluster-lifecycle",
							},
						},
					},
				},
			},
			ns: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns"},
			},
			want: false,
		},
	}

	registerScheme()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ihc := &operatorv1.InternalHubComponent{}

			defer func() {
				if err := recon.Client.Delete(context.TODO(), tt.ns); err != nil {
					t.Errorf("failed to delete namespace: %v", err)
				}
			}()

			if err := recon.Client.Create(context.TODO(), tt.ns); err != nil {
				t.Errorf("failed to create namespace %v: %v", tt.name, err)
			}

			for _, c := range tt.mch.Spec.Overrides.Components {
				if _, err := recon.ensureInternalHubComponent(context.TODO(), tt.mch, c.Name); err != nil {
					t.Errorf("ensureInternalHubComponent(context.TODO(), tt.mch, c.Name) = %v", err)
				}

				if err := recon.Client.Get(context.TODO(), types.NamespacedName{Name: c.Name,
					Namespace: tt.mch.GetNamespace()}, ihc); err != nil {
					t.Errorf("failed to get InternalHubComponent: %v", err)
				}
			}
		})
	}
}

func Test_ensureNoInternalHubComponent(t *testing.T) {
	tests := []struct {
		name string
		mch  *operatorv1.MultiClusterHub
		ns   *corev1.Namespace
		want bool
	}{
		{
			name: "should ensure no InternalHubComponent exist",
			mch: &operatorv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{Name: "mch", Namespace: "test-ns"},
				Spec: operatorv1.MultiClusterHubSpec{
					Overrides: &operatorv1.Overrides{
						Components: []operatorv1.ComponentConfig{
							{
								Enabled: true,
								Name:    "app-lifecycle",
							},
							{
								Enabled: true,
								Name:    "cluster-lifecycle",
							},
						},
					},
				},
			},
			ns: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns"},
			},
			want: false,
		},
	}

	registerScheme()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if err := recon.Client.Delete(context.TODO(), tt.ns); err != nil {
					t.Errorf("failed to delete namespace: %v", err)
				}
			}()

			if err := recon.Client.Create(context.TODO(), tt.ns); err != nil {
				t.Errorf("failed to create namespace %v: %v", tt.name, err)
			}

			for _, c := range tt.mch.Spec.Overrides.Components {
				// Should return nil since we haven't created any InternalHubComponents yet
				if _, err := recon.ensureNoInternalHubComponent(context.TODO(), tt.mch, c.Name); err != nil {
					t.Errorf("ensureNoInternalHubComponent(context.TODO(), tt.mch, c.Name) = %v", err)
				}

				// Create instances of the InternalHubComponent
				if _, err := recon.ensureInternalHubComponent(context.TODO(), tt.mch, c.Name); err != nil {
					t.Errorf("ensureInternalHubComponent(context.TODO(), tt.mch, c.Name) = %v", err)
				}

				ihc := &operatorv1.InternalHubComponent{}
				if err := recon.Client.Get(context.TODO(), types.NamespacedName{Name: c.Name,
					Namespace: tt.mch.GetNamespace()}, ihc); err != nil {
					t.Errorf("failed to get InternalHubComponent: %v", err)
				}

				// Add finalizer to the InternalHubComponent
				ihc.SetFinalizers([]string{"foo/bar"})
				if err := recon.Client.Update(context.TODO(), ihc); err != nil {
					t.Errorf("failed to update InternalHubComponent: %v", err)
				}

				// Should delete the InternalHubComponent but leave it existing due to the finalizer
				if _, err := recon.ensureNoInternalHubComponent(context.TODO(), tt.mch, c.Name); err != nil {
					t.Errorf("ensureInternalHubComponent(context.TODO(), tt.mch, c.Name) = %v", err)
				}

				// Check for the DeletionTimestamp on the InternalHubComponent
				if err := recon.Client.Get(context.TODO(), types.NamespacedName{Name: ihc.GetName(),
					Namespace: ihc.GetNamespace()}, ihc); err != nil {
					t.Errorf("failed to get InternalHubComponent: %v", err)
				}
				if ihc.GetDeletionTimestamp() == nil {
					t.Errorf("InternalHubComponent should have DeletionTimestamp")
				}

				// Reset finalizers on the InternalHubComponent
				ihc.SetFinalizers([]string{})

				// Resource should be deleted
				if _, err := recon.ensureNoInternalHubComponent(context.TODO(), tt.mch, c.Name); err != nil {
					t.Errorf("ensureInternalHubComponent(context.TODO(), tt.mch, c.Name) = %v", err)
				}
			}
		})
	}
}

func Test_getComponentConfig(t *testing.T) {
	tests := []struct {
		name      string
		component string
		mch       operatorv1.MultiClusterHub
		want      operatorv1.ComponentConfig
	}{
		{
			name:      "should get search ComponentConfig",
			component: operatorv1.Search,
			mch: operatorv1.MultiClusterHub{
				Spec: operatorv1.MultiClusterHubSpec{
					Overrides: &operatorv1.Overrides{
						Components: []operatorv1.ComponentConfig{
							{
								Name:            operatorv1.ClusterLifecycle,
								Enabled:         false,
								ConfigOverrides: operatorv1.ConfigOverride{},
							},
							{
								Name:            operatorv1.Search,
								Enabled:         true,
								ConfigOverrides: operatorv1.ConfigOverride{},
							},
						},
					},
				},
			},
			want: operatorv1.ComponentConfig{
				Name:            operatorv1.Search,
				Enabled:         true,
				ConfigOverrides: operatorv1.ConfigOverride{},
			},
		},
		{
			name:      "should get no ComponentConfig",
			component: "foobar",
			mch: operatorv1.MultiClusterHub{
				Spec: operatorv1.MultiClusterHubSpec{
					Overrides: &operatorv1.Overrides{
						Components: []operatorv1.ComponentConfig{
							{
								Name:            operatorv1.ClusterLifecycle,
								Enabled:         false,
								ConfigOverrides: operatorv1.ConfigOverride{},
							},
							{
								Name:            operatorv1.Search,
								Enabled:         true,
								ConfigOverrides: operatorv1.ConfigOverride{},
							},
						},
					},
				},
			},
			want: operatorv1.ComponentConfig{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := recon.getComponentConfig(tt.mch.Spec.Overrides.Components, tt.component)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getComponentConfig(tt.mch.Spec.Overrides.Components) = %v, want = %v", got, tt.want)
			}
		})
	}
}

func Test_getDeploymentConfig(t *testing.T) {
	tests := []struct {
		name           string
		componentName  string
		deploymentName string
		mch            operatorv1.MultiClusterHub
		want           *operatorv1.DeploymentConfig
	}{
		{
			name:           "should get search DeploymentConfig",
			componentName:  "search",
			deploymentName: "search-v2-operator-controller-manager",
			mch: operatorv1.MultiClusterHub{
				Spec: operatorv1.MultiClusterHubSpec{
					Overrides: &operatorv1.Overrides{
						Components: []operatorv1.ComponentConfig{
							{
								Name:            operatorv1.ClusterLifecycle,
								Enabled:         false,
								ConfigOverrides: operatorv1.ConfigOverride{},
							},
							{
								Name:    operatorv1.Search,
								Enabled: true,
								ConfigOverrides: operatorv1.ConfigOverride{
									Deployments: []operatorv1.DeploymentConfig{
										{
											Name: "search-v2-operator-controller-manager",
											Containers: []operatorv1.ContainerConfig{
												{
													Name: "kube-rbac-proxy",
													Env: []operatorv1.EnvConfig{
														{
															Name:  "foo",
															Value: "bar",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: &operatorv1.DeploymentConfig{
				Name: "search-v2-operator-controller-manager",
				Containers: []operatorv1.ContainerConfig{
					{
						Name: "kube-rbac-proxy",
						Env: []operatorv1.EnvConfig{
							{
								Name:  "foo",
								Value: "bar",
							},
						},
					},
				},
			},
		},
		{
			name:           "should get no DeploymentConfig",
			componentName:  operatorv1.ClusterLifecycle,
			deploymentName: "klusterlet-addon-controller",
			mch: operatorv1.MultiClusterHub{
				Spec: operatorv1.MultiClusterHubSpec{
					Overrides: &operatorv1.Overrides{
						Components: []operatorv1.ComponentConfig{
							{
								Name:            operatorv1.ClusterLifecycle,
								Enabled:         false,
								ConfigOverrides: operatorv1.ConfigOverride{},
							},
							{
								Name:            operatorv1.Search,
								Enabled:         true,
								ConfigOverrides: operatorv1.ConfigOverride{},
							},
						},
					},
				},
			},
			want: &operatorv1.DeploymentConfig{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if componentConfig, found := recon.getComponentConfig(tt.mch.Spec.Overrides.Components, tt.componentName); found {
				if got, _ := recon.getDeploymentConfig(componentConfig.ConfigOverrides.Deployments,
					tt.deploymentName); !reflect.DeepEqual(got, tt.want) {
					t.Errorf("getDeploymentConfig(componentConfig.ConfigOverrides.Deployments, tt.deploymentName) = %v, want = %v", got, tt.want)
				}
			}
		})
	}
}

func Test_applyEnvConfig(t *testing.T) {
	tests := []struct {
		name          string
		containerName string
		template      *unstructured.Unstructured
		envConfig     []operatorv1.EnvConfig
		want          error
	}{
		{
			name:          "should apply env config",
			containerName: "kube-rbac-proxy",
			template: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "search-v2-operator-controller-manager",
						"namespace": "test-ns",
					},
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"name": "kube-rbac-proxy",
										"env":  []interface{}{},
									},
									map[string]interface{}{
										"name": "manager",
										"env":  []interface{}{},
									},
								},
							},
						},
					},
				},
			},
			envConfig: []operatorv1.EnvConfig{
				{Name: "foo", Value: "bar"},
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := recon.applyEnvConfig(tt.template, tt.containerName, tt.envConfig); err != nil {
				t.Errorf("applyEnvConfig(tt.template, tt.containerName, tt.envConfig) = %v, want %v", err, tt.want)
			}

			deployment := &appsv1.Deployment{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(tt.template.Object, deployment); err != nil {
				t.Errorf("failed to convert unstructured object to deployment")
			}

			for _, c := range deployment.Spec.Template.Spec.Containers {
				if c.Name == tt.containerName {
					// Ensure envConfig is correctly applied to container Env
					for _, envVar := range tt.envConfig {
						found := false
						for _, containerEnvVar := range c.Env {
							if containerEnvVar.Name == envVar.Name && containerEnvVar.Value == envVar.Value {
								found = true
								break
							}
						}
						if !found {
							t.Errorf("env variable %v=%v not found in container %v", envVar.Name, envVar.Value, c.Name)
						}
					}
					break
				}
			}
		})
	}
}

func Test_logAndSetCondition(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		mch      *operatorv1.MultiClusterHub
		message  string
		template *unstructured.Unstructured
	}{
		{
			name: "should log and set condition of MCH",
			err:  fmt.Errorf("Test error"),
			mch: &operatorv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multiclusterhub",
					Namespace: "test-ns",
				},
			},
			message: "This is a test error",
			template: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "test-ns",
					},
					"spec": map[string]interface{}{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if _, err := recon.logAndSetCondition(tt.err, tt.message, tt.template, tt.mch); err == nil {
				t.Errorf("logAndSetCondition() = %v, expected an error", err)
			}
		})
	}
}

func Test_ensureResourceVersionAlignment(t *testing.T) {
	tests := []struct {
		name     string
		mch      *operatorv1.MultiClusterHub
		template *unstructured.Unstructured
		want     bool
	}{
		{
			name: "should ensure resource version alignment is false",
			mch: &operatorv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multiclusterhub",
					Namespace: "test-ns",
				},
			},
			template: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							utils.AnnotationReleaseVersion: "2.12.0",
						},
						"name":      "test-deployment",
						"namespace": "test-ns",
					},
					"spec": map[string]interface{}{},
				},
			},
			want: false,
		},
		{
			name: "should ensure resource version alignment is true",
			mch: &operatorv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multiclusterhub",
					Namespace: "test-ns",
				},
			},
			template: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							utils.AnnotationReleaseVersion: "2.13.0",
						},
						"name":      "test-deployment",
						"namespace": "test-ns",
					},
					"spec": map[string]interface{}{},
				},
			},
			want: true,
		},
	}

	os.Setenv("OPERATOR_VERSION", "2.13.0")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := recon.ensureResourceVersionAlignment(tt.template, os.Getenv("OPERATOR_VERSION"))
			if got != tt.want {
				t.Errorf("ensureResourceVersionAlignment() = %v, want: %v", got, tt.want)
			}
		})
	}
}
