// Copyright Contributors to the Open Cluster Management project

package hubteardown_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	utils "github.com/stolostron/multiclusterhub-operator/test/function_tests/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	GVRHubTeardown = schema.GroupVersionResource{
		Group:    "operator.open-cluster-management.io",
		Version:  "v1",
		Resource: "hubteardowns",
	}

	GVRSubscription = schema.GroupVersionResource{
		Group:    "operators.coreos.com",
		Version:  "v1alpha1",
		Resource: "subscriptions",
	}

	teardownName = "teardown-ft"
)

var _ = Describe("HubTeardown", func() {

	AfterEach(func() {
		By("Cleaning up HubTeardown CR if it exists")
		tdClient := utils.DynamicKubeClient.Resource(GVRHubTeardown).Namespace(utils.MCHNamespace)
		_ = tdClient.Delete(context.TODO(), teardownName, metav1.DeleteOptions{})
	})

	It("should populate dryRunReport in dry-run mode", func() {
		By("Creating HubTeardown CR with dryRun=true")
		td := newHubTeardown(teardownName, utils.MCHNamespace, true)
		tdClient := utils.DynamicKubeClient.Resource(GVRHubTeardown).Namespace(utils.MCHNamespace)
		_, err := tdClient.Create(context.TODO(), td, metav1.CreateOptions{})
		Expect(err).Should(BeNil())

		By("Waiting for dryRunReport to be populated")
		Eventually(func() bool {
			obj, err := tdClient.Get(context.TODO(), teardownName, metav1.GetOptions{})
			if err != nil {
				return false
			}
			report, found, _ := unstructured.NestedMap(obj.Object, "status", "dryRunReport")
			return found && report != nil
		}, utils.GetWaitInMinutes()*60, 5).Should(BeTrue(), "dryRunReport should be populated")

		By("Validating status.phase is DryRun")
		obj, err := tdClient.Get(context.TODO(), teardownName, metav1.GetOptions{})
		Expect(err).Should(BeNil())
		phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
		Expect(phase).To(Equal("DryRun"))

		By("Validating DryRunComplete condition is True")
		conditions, _, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
		foundDryRunComplete := false
		for _, c := range conditions {
			cond, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			if cond["type"] == "DryRunComplete" && cond["status"] == "True" {
				foundDryRunComplete = true
			}
		}
		Expect(foundDryRunComplete).To(BeTrue(), "DryRunComplete condition should be True")
	})

	It("should add the teardown finalizer to the CR", func() {
		By("Creating HubTeardown CR")
		td := newHubTeardown(teardownName, utils.MCHNamespace, true)
		tdClient := utils.DynamicKubeClient.Resource(GVRHubTeardown).Namespace(utils.MCHNamespace)
		_, err := tdClient.Create(context.TODO(), td, metav1.CreateOptions{})
		Expect(err).Should(BeNil())

		By("Waiting for the teardown finalizer to appear")
		Eventually(func() bool {
			obj, err := tdClient.Get(context.TODO(), teardownName, metav1.GetOptions{})
			if err != nil {
				return false
			}
			for _, f := range obj.GetFinalizers() {
				if f == "operator.open-cluster-management.io/teardown-job" {
					return true
				}
			}
			return false
		}, utils.GetWaitInMinutes()*60, 5).Should(BeTrue(), "teardown finalizer should be present")
	})

	It("should progress phases and emit events when dryRun=false", func() {
		By("Creating HubTeardown CR with dryRun=false")
		td := newHubTeardown(teardownName, utils.MCHNamespace, false)
		tdClient := utils.DynamicKubeClient.Resource(GVRHubTeardown).Namespace(utils.MCHNamespace)
		_, err := tdClient.Create(context.TODO(), td, metav1.CreateOptions{})
		Expect(err).Should(BeNil())

		By(fmt.Sprintf("Waiting for teardown to reach Complete phase (up to %d minutes)", utils.GetWaitInMinutes()))
		Eventually(func() string {
			obj, err := tdClient.Get(context.TODO(), teardownName, metav1.GetOptions{})
			if err != nil {
				return ""
			}
			phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
			return phase
		}, utils.GetWaitInMinutes()*60, 5).Should(Equal("Complete"), "teardown should reach Complete")

		By("Validating Complete condition is True")
		obj, err := tdClient.Get(context.TODO(), teardownName, metav1.GetOptions{})
		Expect(err).Should(BeNil())
		conditions, _, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
		foundComplete := false
		for _, c := range conditions {
			cond, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			if cond["type"] == "Complete" && cond["status"] == "True" {
				foundComplete = true
			}
		}
		Expect(foundComplete).To(BeTrue(), "Complete condition should be True")

		By("Validating phase statuses are populated")
		phases, _, _ := unstructured.NestedSlice(obj.Object, "status", "phases")
		Expect(len(phases)).To(BeNumerically(">", 0), "should have phase status entries")

		By("Checking events for phase transitions")
		events, err := utils.DynamicKubeClient.Resource(schema.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "events",
		}).Namespace(utils.MCHNamespace).List(context.TODO(), metav1.ListOptions{
			FieldSelector: "involvedObject.name=" + teardownName,
		})
		Expect(err).Should(BeNil())
		Expect(len(events.Items)).To(BeNumerically(">", 0), "should have events for teardown phases")
	})

	It("should handle OLM Subscription gate lifecycle", func() {
		By("Creating HubTeardown CR with dryRun=false")
		td := newHubTeardown(teardownName, utils.MCHNamespace, false)
		tdClient := utils.DynamicKubeClient.Resource(GVRHubTeardown).Namespace(utils.MCHNamespace)
		_, err := tdClient.Create(context.TODO(), td, metav1.CreateOptions{})
		Expect(err).Should(BeNil())

		By("Waiting for teardown to complete")
		Eventually(func() string {
			obj, err := tdClient.Get(context.TODO(), teardownName, metav1.GetOptions{})
			if err != nil {
				return ""
			}
			phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
			return phase
		}, utils.GetWaitInMinutes()*60, 5).Should(Equal("Complete"))

		By("Verifying OLM gate finalizer was released (not present on Subscription)")
		subClient := utils.DynamicKubeClient.Resource(GVRSubscription).Namespace(utils.MCHNamespace)
		subs, err := subClient.List(context.TODO(), metav1.ListOptions{})
		if err == nil {
			for _, sub := range subs.Items {
				for _, f := range sub.GetFinalizers() {
					Expect(f).NotTo(Equal("operator.open-cluster-management.io/teardown-gate"),
						"teardown gate finalizer should have been removed from Subscription "+sub.GetName())
				}
			}
		}
	})
})

func newHubTeardown(name, namespace string, dryRun bool) *unstructured.Unstructured {
	td := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "operator.open-cluster-management.io/v1",
			"kind":       "HubTeardown",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"dryRun": dryRun,
			},
		},
	}
	return td
}
