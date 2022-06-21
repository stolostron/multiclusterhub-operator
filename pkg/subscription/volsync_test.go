package subscription

import (
	resources "github.com/stolostron/multiclusterhub-operator/test/unit-tests"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Volsync", func() {
	Context("create a volsync subscription", func() {
		It("creates a volsync subscription", func() {
			By("creating an mch cr")
			mch := resources.EmptyMCH()

			By("creating a subscription to volsync")
			u := Volsync(&mch, nil)
			Expect(u).ToNot(BeNil())
		})
	})
})
