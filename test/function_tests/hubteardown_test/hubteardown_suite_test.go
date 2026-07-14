// Copyright Contributors to the Open Cluster Management project

package hubteardown_test

import (
	"flag"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	utils "github.com/stolostron/multiclusterhub-operator/test/function_tests/utils"
	"k8s.io/klog/v2"
)

var reportFile string

func init() {
	klog.SetOutput(GinkgoWriter)
	klog.InitFlags(nil)
	flag.StringVar(&reportFile, "report-file", "../results/hubteardown-results.xml",
		"Path for JUnit results output.")
}

func TestHubTeardown(t *testing.T) {
	RegisterFailHandler(Fail)
	_ = utils.DynamicKubeClient // ensure client is initialized
	RunSpecs(t, "HubTeardown Functional Suite")
}
