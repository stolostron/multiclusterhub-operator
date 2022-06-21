package subscription

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSubscription(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Subscription Suite")
}
