package manifest

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestManifest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Manifest Suite")
}

var _ = BeforeSuite(func() {
	os.Setenv("MANIFESTS_PATH", "../../test/unit-tests/manifest")
})
