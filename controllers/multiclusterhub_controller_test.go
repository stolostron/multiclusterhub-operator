// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"fmt"
	"reflect"
	"time"

	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"
	resources "github.com/stolostron/multiclusterhub-operator/test/unit-tests"

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

			Eventually(func() bool {
				return createdMCH.Status.Phase == operatorsv1.HubRunning
			}, timeout, interval).Should(BeTrue())

		})
	})

})

// func Test_ReconcileMultiClusterHub(t *testing.T) {

// 	secret := &corev1.Secret{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      "pull-secret",
// 			Namespace: mch_namespace,
// 		},
// 		StringData: map[string]string{
// 			"test": "data",
// 		},
// 	}

// 	os.Setenv("UNIT_TEST", "true")
// 	os.Setenv("TEMPLATES_PATH", "../../../templates")
// 	os.Setenv("MANIFESTS_PATH", "../../../image-manifests")
// 	os.Setenv("CRDS_PATH", "../../../crds")
// 	defer os.Unsetenv("TEMPLATES_PATH")
// 	defer os.Unsetenv("MANIFESTS_PATH")
// 	defer os.Unsetenv("UNIT_TEST")
// 	defer os.Unsetenv("CRDS_PATH")

// 	// Without Status Prefilled
// 	mch2 := full_mch.DeepCopy()
// 	mch2.Status = operatorsv1.MultiClusterHubStatus{}

// 	// AvailabilityConfig
// 	mch3 := full_mch.DeepCopy()
// 	mch3.Spec.AvailabilityConfig = operatorsv1.HABasic

// 	// SeparateCertificateManagement
// 	mch5 := full_mch.DeepCopy()
// 	mch5.Spec.SeparateCertificateManagement = true

// 	tests := []struct {
// 		Name     string
// 		MCH      *operatorsv1.MultiClusterHub
// 		Expected error
// 	}{
// 		{
// 			Name:     "Full Valid MCH",
// 			MCH:      full_mch,
// 			Expected: nil,
// 		},
// 		{
// 			Name:     "Without Status",
// 			MCH:      mch2,
// 			Expected: nil,
// 		},
// 		{
// 			Name:     "AvailabilityConfig",
// 			MCH:      mch3,
// 			Expected: nil,
// 		},
// 		{
// 			Name:     "CloudPakCompatibility",
// 			MCH:      mch5,
// 			Expected: nil,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.Name, func(t *testing.T) {

// 			r, err := getTestReconciler(tt.MCH)
// 			if err != nil {
// 				t.Fatalf("Failed to create test reconciler")
// 			}
// 			// Mock request to simulate Reconcile() being called on an event for a
// 			// watched resource .
// 			req := reconcile.Request{
// 				NamespacedName: mch_namespaced,
// 			}

// 			if tt.MCH.Spec.SeparateCertificateManagement {
// 				err = r.Client.Create(context.TODO(), secret)
// 				if err != nil {
// 					t.Fatal(err.Error())
// 				}
// 			}

// 			res, err := r.Reconcile(nil, req)
// 			if !errorEquals(err, tt.Expected) {
// 				t.Fatalf("reconcile: (%v)", err)
// 			}

// 			// Check the result of reconciliation to make sure it has the desired state.
// 			if res.Requeue {
// 				t.Error("reconcile did not requeue request as expected")
// 			}

// 			// Check if MCH has been created
// 			mch := &operatorsv1.MultiClusterHub{}
// 			err = r.Client.Get(context.TODO(), mch_namespaced, mch)
// 			if err != nil {
// 				t.Errorf("Could not find MultiClusterHub resource")
// 			}

// 		})
// 	}

// }

// func Test_setDefaults(t *testing.T) {
// 	os.Setenv("TEMPLATES_PATH", "../../../templates")

// 	// Without Status Prefilled
// 	mch1 := full_mch.DeepCopy()
// 	mch1.Status = operatorsv1.MultiClusterHubStatus{}

// 	tests := []struct {
// 		Name     string
// 		MCH      *operatorsv1.MultiClusterHub
// 		Expected error
// 	}{
// 		{
// 			Name:     "Full Valid MCH",
// 			MCH:      full_mch,
// 			Expected: nil,
// 		},
// 		{
// 			Name:     "Without Status",
// 			MCH:      mch1,
// 			Expected: nil,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.Name, func(t *testing.T) {
// 			// Objects to track in the fake client.
// 			r, err := getTestReconciler(tt.MCH)
// 			if err != nil {
// 				t.Fatalf("Failed to create test reconciler")
// 			}

// 			_, err = r.setDefaults(tt.MCH)
// 			if !errorEquals(err, tt.Expected) {
// 				t.Fatalf("reconcile: (%v)", err)
// 			}
// 		})
// 	}
// 	os.Unsetenv("TEMPLATES_PATH")
// }

// func errorEquals(err, expected error) bool {
// 	if err == nil && expected == nil {
// 		return true
// 	} else if (err == nil && expected != nil) || (err != nil && expected == nil) {
// 		return false
// 	}

// 	if err.Error() == expected.Error() {
// 		return true
// 	}
// 	return false
// }

// func getTestReconciler(m *operatorsv1.MultiClusterHub) (*MultiClusterHubReconciler, error) {

// 	cfg, err := testEnv.Start()
// 	Expect(err).NotTo(HaveOccurred())
// 	Expect(cfg).NotTo(BeNil())

// 	objs := []runtime.Object{m}

// 	// Register operator types with the runtime scheme.
// 	s := scheme.Scheme

// 	if err := netv1.AddToScheme(s); err != nil {
// 		return nil, fmt.Errorf("Could not add ingress to test scheme")
// 	}

// 	if err := apiregistrationv1.AddToScheme(s); err != nil {
// 		return nil, fmt.Errorf("Could not add rbac to test scheme")
// 	}

// 	if err := apixv1.AddToScheme(s); err != nil {
// 		return nil, fmt.Errorf("Could not add CRDs to test scheme")
// 	}

// 	if err := appsubv1.AddToScheme(s); err != nil {
// 		os.Exit(1)
// 	}
// 	s.AddKnownTypes(operatorsv1.GroupVersion, m)

// 	// Create a fake client to mock API calls.
// 	cl := fake.NewFakeClient(objs...)

// 	// Create a ReconcileMultiClusterHub object with the scheme and fake client.
// 	return &MultiClusterHubReconciler{Client: cl, Scheme: s}, nil
// }

// func getTestReconcilerWithObjs(objs []runtime.Object) (*MultiClusterHubReconciler, error) {
// 	// Register operator types with the runtime scheme.
// 	s := scheme.Scheme

// 	if err := netv1.AddToScheme(s); err != nil {
// 		return nil, fmt.Errorf("Could not add ingress to test scheme")
// 	}

// 	if err := apiregistrationv1.AddToScheme(s); err != nil {
// 		return nil, fmt.Errorf("Could not add rbac to test scheme")
// 	}

// 	if err := apixv1.AddToScheme(s); err != nil {
// 		return nil, fmt.Errorf("Could not add CRDs to test scheme")
// 	}

// 	if err := appsubv1.AddToScheme(s); err != nil {
// 		os.Exit(1)
// 	}
// 	s.AddKnownTypes(operatorsv1.GroupVersion, &operatorsv1.MultiClusterHub{})

// 	// Create a fake client to mock API calls.
// 	cl := fake.NewFakeClient(objs...)

// 	// Create a ReconcileMultiClusterHub object with the scheme and fake client.
// 	return &MultiClusterHubReconciler{Client: cl, Scheme: s}, nil
// }
