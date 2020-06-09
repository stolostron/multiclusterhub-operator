package multicloudhub_operator_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestMulticloudhubOperator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MulticloudhubOperator Suite")
}
