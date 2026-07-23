// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"os"
	"path/filepath"
	"time"

	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	batchv1 "k8s.io/api/batch/v1"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HubTeardown controller", func() {
	const (
		tdTimeout  = time.Second * 30
		tdInterval = time.Millisecond * 250
		tdName     = "teardown"
		tdNs       = "open-cluster-management"
	)

	var (
		testEnv      *envtest.Environment
		k8sClient    client.Client
		clientScheme = runtime.NewScheme()
		ctxTD        context.Context
		cancelTD     context.CancelFunc
	)

	BeforeEach(func() {
		testEnv = &envtest.Environment{
			CRDDirectoryPaths: []string{
				filepath.Join("..", "config", "crd", "bases"),
			},
			CRDInstallOptions: envtest.CRDInstallOptions{
				CleanUpAfterUse: true,
			},
			ErrorIfCRDPathMissing: true,
		}

		By("bootstrapping HubTeardown test environment")
		var err error
		cfg, err := testEnv.Start()
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg).NotTo(BeNil())

		Expect(operatorv1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(batchv1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(apixv1.AddToScheme(clientScheme)).Should(Succeed())
		Expect(subv1alpha1.AddToScheme(clientScheme)).Should(Succeed())

		k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme: clientScheme,
			Metrics: metricsserver.Options{
				BindAddress: "0",
			},
			HealthProbeBindAddress: "0",
		})
		Expect(err).ToNot(HaveOccurred())

		k8sClient = k8sManager.GetClient()
		Expect(k8sClient).NotTo(BeNil())

		uncachedClient, err := client.New(cfg, client.Options{Scheme: clientScheme})
		Expect(err).NotTo(HaveOccurred())

		tdReconciler := &HubTeardownReconciler{
			Client:         k8sClient,
			UncachedClient: uncachedClient,
			Scheme:         k8sManager.GetScheme(),
			Log:            ctrl.Log.WithName("test").WithName("HubTeardown"),
		}
		Expect(tdReconciler.SetupWithManager(k8sManager)).Should(Succeed())

		ctxTD, cancelTD = context.WithCancel(context.Background())
		go func() {
			defer GinkgoRecover()
			Expect(k8sManager.Start(ctxTD)).Should(Succeed())
		}()
	})

	AfterEach(func() {
		cancelTD()
		Expect(testEnv.Stop()).Should(Succeed())
	})

	It("should set DryRunComplete condition in dry-run mode", func() {
		td := &operatorv1.HubTeardown{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tdName,
				Namespace: tdNs,
			},
			Spec: operatorv1.HubTeardownSpec{
				DryRun: true,
			},
		}

		Expect(k8sClient.Create(ctxTD, td)).Should(Succeed())

		Eventually(func() bool {
			fetched := &operatorv1.HubTeardown{}
			if err := k8sClient.Get(ctxTD, types.NamespacedName{Name: tdName, Namespace: tdNs}, fetched); err != nil {
				return false
			}
			for _, c := range fetched.Status.Conditions {
				if c.Type == conditionTypeDryRunComplete && c.Status == metav1.ConditionTrue {
					return true
				}
			}
			return false
		}, tdTimeout, tdInterval).Should(BeTrue(), "DryRunComplete condition should be True")

		fetched := &operatorv1.HubTeardown{}
		Expect(k8sClient.Get(ctxTD, types.NamespacedName{Name: tdName, Namespace: tdNs}, fetched)).Should(Succeed())
		Expect(fetched.Status.Phase).To(Equal(operatorv1.TeardownPhaseDryRun))
		Expect(fetched.Status.DryRunReport).NotTo(BeNil())
	})

	It("should add the teardown finalizer on the CR", func() {
		td := &operatorv1.HubTeardown{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tdName + "-fin",
				Namespace: tdNs,
			},
			Spec: operatorv1.HubTeardownSpec{
				DryRun: true,
			},
		}

		Expect(k8sClient.Create(ctxTD, td)).Should(Succeed())

		Eventually(func() bool {
			fetched := &operatorv1.HubTeardown{}
			if err := k8sClient.Get(ctxTD, types.NamespacedName{Name: td.Name, Namespace: tdNs}, fetched); err != nil {
				return false
			}
			for _, f := range fetched.GetFinalizers() {
				if f == teardownJobFinalizer {
					return true
				}
			}
			return false
		}, tdTimeout, tdInterval).Should(BeTrue(), "teardown finalizer should be added to the CR")
	})

	It("should progress through phases when dryRun is disabled", func() {
		os.Setenv("OPERATOR_IMAGE", "quay.io/test/operator:latest")
		defer os.Unsetenv("OPERATOR_IMAGE")

		td := &operatorv1.HubTeardown{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tdName + "-active",
				Namespace: tdNs,
			},
			Spec: operatorv1.HubTeardownSpec{
				DryRun: false,
			},
		}

		Expect(k8sClient.Create(ctxTD, td)).Should(Succeed())

		// With no blocking resources on a clean cluster, all phases should complete quickly
		Eventually(func() operatorv1.TeardownPhase {
			fetched := &operatorv1.HubTeardown{}
			if err := k8sClient.Get(ctxTD, types.NamespacedName{Name: td.Name, Namespace: tdNs}, fetched); err != nil {
				return ""
			}
			return fetched.Status.Phase
		}, tdTimeout, tdInterval).Should(Equal(operatorv1.TeardownPhaseComplete), "teardown should reach Complete phase")

		fetched := &operatorv1.HubTeardown{}
		Expect(k8sClient.Get(ctxTD, types.NamespacedName{Name: td.Name, Namespace: tdNs}, fetched)).Should(Succeed())

		// Verify the Complete condition is set
		foundComplete := false
		for _, c := range fetched.Status.Conditions {
			if c.Type == conditionTypeTeardownComplete && c.Status == metav1.ConditionTrue {
				foundComplete = true
			}
		}
		Expect(foundComplete).To(BeTrue(), "Complete condition should be True")
	})

	It("should pause execution when dryRun is toggled back to true", func() {
		td := &operatorv1.HubTeardown{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tdName + "-pause",
				Namespace: tdNs,
			},
			Spec: operatorv1.HubTeardownSpec{
				DryRun: true,
			},
		}

		Expect(k8sClient.Create(ctxTD, td)).Should(Succeed())

		// Wait for dry-run to complete
		Eventually(func() bool {
			fetched := &operatorv1.HubTeardown{}
			if err := k8sClient.Get(ctxTD, types.NamespacedName{Name: td.Name, Namespace: tdNs}, fetched); err != nil {
				return false
			}
			return fetched.Status.Phase == operatorv1.TeardownPhaseDryRun
		}, tdTimeout, tdInterval).Should(BeTrue())

		// Switch to active
		fetched := &operatorv1.HubTeardown{}
		Expect(k8sClient.Get(ctxTD, types.NamespacedName{Name: td.Name, Namespace: tdNs}, fetched)).Should(Succeed())
		fetched.Spec.DryRun = false
		Expect(k8sClient.Update(ctxTD, fetched)).Should(Succeed())

		// Wait for it to be in progress or complete, then toggle back
		time.Sleep(500 * time.Millisecond)
		Expect(k8sClient.Get(ctxTD, types.NamespacedName{Name: td.Name, Namespace: tdNs}, fetched)).Should(Succeed())
		fetched.Spec.DryRun = true
		Expect(k8sClient.Update(ctxTD, fetched)).Should(Succeed())

		// Verify it goes back to dry-run
		Eventually(func() operatorv1.TeardownPhase {
			latest := &operatorv1.HubTeardown{}
			if err := k8sClient.Get(ctxTD, types.NamespacedName{Name: td.Name, Namespace: tdNs}, latest); err != nil {
				return ""
			}
			return latest.Status.Phase
		}, tdTimeout, tdInterval).Should(Equal(operatorv1.TeardownPhaseDryRun))
	})
})
