package multiclusterhub_operator_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestMulticlusterhubOperator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MulticlusterhubOperator Suite")
}
