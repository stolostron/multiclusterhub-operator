// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package multiclusterhub_uninstall_test

import (
	"context"
	"flag"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	utils "github.com/open-cluster-management/multiclusterhub-operator/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

var (
	optionsFile         string
	baseDomain          string
	kubeadminUser       string
	kubeadminCredential string
	kubeconfig          string
	reportFile          string
)

func init() {
	klog.SetOutput(GinkgoWriter)
	klog.InitFlags(nil)
	flag.StringVar(&kubeadminUser, "kubeadmin-user", "kubeadmin", "Provide the kubeadmin credential for the cluster under test (e.g. -kubeadmin-user=\"xxxxx\").")
	flag.StringVar(&kubeadminCredential, "kubeadmin-credential", "",
		"Provide the kubeadmin credential for the cluster under test (e.g. -kubeadmin-credential=\"xxxxx-xxxxx-xxxxx-xxxxx\").")
	flag.StringVar(&baseDomain, "base-domain", "", "Provide the base domain for the cluster under test (e.g. -base-domain=\"demo.red-chesterfield.com\").")

	flag.StringVar(&optionsFile, "options", "", "Location of an \"options.yaml\" file to provide input for various tests")
	flag.StringVar(&reportFile, "report-file", "../results/uninstall-results.xml", "Provide the path to where the junit results will be printed.")

}

var _ = AfterSuite(func() {
	if utils.ShouldSkipSubscription() {
		return
	}
	By("Deleting OCM Subscriptions")
	// Delete Subscription
	subLink := utils.DynamicKubeClient.Resource(utils.GVRSub).Namespace(utils.MCHNamespace)

	subList, err := subLink.List(context.TODO(), metav1.ListOptions{})
	Expect(err).Should(BeNil())
	for _, sub := range subList.Items {
		for _, subName := range utils.SubList {
			if strings.Contains(sub.GetName(), subName) {
				err = subLink.Delete(context.TODO(), sub.GetName(), metav1.DeleteOptions{})
				Expect(err).Should(BeNil())
			}
		}
	}
	Expect(err).Should(BeNil())

	By("Deleting CSVs")
	// Delete CSVs
	csvLink := utils.DynamicKubeClient.Resource(utils.GVRCSV).Namespace(utils.MCHNamespace)
	csvList, err := csvLink.List(context.TODO(), metav1.ListOptions{})
	Expect(err).Should(BeNil())
	for _, csv := range csvList.Items {
		if strings.Contains(csv.GetName(), utils.CSVName) {
			err = csvLink.Delete(context.TODO(), csv.GetName(), metav1.DeleteOptions{})
			Expect(err).Should(BeNil())
		}
	}
})

func TestMultiClusterHubOperatorUninstall(t *testing.T) {
	RegisterFailHandler(Fail)
	junitReporter := reporters.NewJUnitReporter(reportFile)
	RunSpecsWithDefaultAndCustomReporters(t, "MultiClusterHubOperator Install Suite", []Reporter{junitReporter})
}
