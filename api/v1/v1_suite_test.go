package v1_test

import (
	"context"
	"os"
	"testing"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestV1(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "API V1 Suite")
}

var signalHandlerContext context.Context

var _ = BeforeSuite(func() {
	log.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	// SetupSignalHandler can only be called once, so we'll save the
	// context it returns and reuse it each time we start a new
	// manager.
	signalHandlerContext = ctrl.SetupSignalHandler()

	os.Setenv("POD_NAMESPACE", "open-cluster-management")
	os.Setenv("CRDS_PATH", "../../bin/crds")
	os.Setenv("TEMPLATES_PATH", "../../pkg/templates")
	os.Setenv("UNIT_TEST", "true")
})
