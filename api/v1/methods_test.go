package v1_test

import (
	// "os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	api "github.com/stolostron/multiclusterhub-operator/api/v1"
	// "github.com/stolostron/multiclusterhub-operator/pkg/utils"
)

func config(name string, enabled bool) api.ComponentConfig {
	return api.ComponentConfig{
		Name:    name,
		Enabled: enabled,
	}
}

func makeMCH(configs ...api.ComponentConfig) *api.MultiClusterHub {
	mch := &api.MultiClusterHub{
		Spec: api.MultiClusterHubSpec{},
	}
	if len(configs) == 0 {
		return mch
	}
	mch.Spec.Overrides = &api.Overrides{
		Components: make([]api.ComponentConfig, len(configs)),
	}
	for i := range configs {
		mch.Spec.Overrides.Components[i] = configs[i]
	}
	return mch
}

var _ = Describe("V1 API Methods", func() {
	Context("when the spec is empty", func() {
		var mch *api.MultiClusterHub

		BeforeEach(func() {
			mch = makeMCH()
		})

		It("correctly indicates if a component is present", func() {
			Expect(mch.ComponentPresent(api.Search)).To(BeFalse())
			Expect(mch.ComponentPresent(api.GRC)).To(BeFalse())
		})

		It("correctly indicates if a component is enabled", func() {
			Expect(mch.Enabled(api.Search)).To(BeFalse())
		})

		It("enables a component", func() {
			Expect(mch.ComponentPresent(api.Search)).To(BeFalse())
			Expect(mch.Enabled(api.Search)).To(BeFalse())
			mch.Enable(api.Search)
			Expect(mch.ComponentPresent(api.Search)).To(BeTrue())
			Expect(mch.Enabled(api.Search)).To(BeTrue())
		})

		It("disables a component", func() {
			Expect(mch.ComponentPresent(api.Search)).To(BeFalse())
			Expect(mch.Enabled(api.Search)).To(BeFalse())
			mch.Disable(api.Search)
			Expect(mch.ComponentPresent(api.Search)).To(BeTrue())
			Expect(mch.Enabled(api.Search)).To(BeFalse())
		})
	})

	Context("when the spec is not empty, but the component is not present", func() {
		var mch *api.MultiClusterHub

		BeforeEach(func() {
			mch = makeMCH(config(api.GRC, false))
		})

		It("correctly indicates if a component is present", func() {
			Expect(mch.ComponentPresent(api.GRC)).To(BeTrue())
			Expect(mch.ComponentPresent(api.Search)).To(BeFalse())
		})

		It("correctly indicates if a component is enabled", func() {
			Expect(mch.Enabled(api.Search)).To(BeFalse())
		})

		It("enables a component", func() {
			Expect(mch.ComponentPresent(api.Search)).To(BeFalse())
			Expect(mch.Enabled(api.Search)).To(BeFalse())
			mch.Enable(api.Search)
			Expect(mch.ComponentPresent(api.Search)).To(BeTrue())
			Expect(mch.Enabled(api.Search)).To(BeTrue())
		})

		It("disables a component", func() {
			Expect(mch.ComponentPresent(api.Search)).To(BeFalse())
			Expect(mch.Enabled(api.Search)).To(BeFalse())
			mch.Disable(api.Search)
			Expect(mch.ComponentPresent(api.Search)).To(BeTrue())
			Expect(mch.Enabled(api.Search)).To(BeFalse())
		})
	})

	Context("when the spec is not empty, and the component is present", func() {
		var mch *api.MultiClusterHub

		BeforeEach(func() {
			mch = makeMCH(config(api.GRC, false), config(api.Search, false))
		})

		It("correctly indicates if a component is present", func() {
			Expect(mch.ComponentPresent(api.GRC)).To(BeTrue())
			Expect(mch.ComponentPresent(api.Search)).To(BeTrue())
		})

		It("correctly indicates if a component is enabled", func() {
			Expect(mch.Enabled(api.Search)).To(BeFalse())
		})

		It("enables a component", func() {
			Expect(mch.ComponentPresent(api.Search)).To(BeTrue())
			Expect(mch.Enabled(api.Search)).To(BeFalse())
			mch.Enable(api.Search)
			Expect(mch.ComponentPresent(api.Search)).To(BeTrue())
			Expect(mch.Enabled(api.Search)).To(BeTrue())
		})

		It("disables a component", func() {
			Expect(mch.ComponentPresent(api.Search)).To(BeTrue())
			Expect(mch.Enabled(api.Search)).To(BeFalse())
			mch.Disable(api.Search)
			Expect(mch.ComponentPresent(api.Search)).To(BeTrue())
			Expect(mch.Enabled(api.Search)).To(BeFalse())
		})
	})

	It("correctly validates a component name", func() {
		Expect(api.ValidComponent(config(api.Search, true))).To(BeTrue())
		Expect(api.ValidComponent(config("invalid", true))).To(BeFalse())
	})

	It("gets the correct number of  default enabled components", func() {
		components, err := api.GetDefaultEnabledComponents()

		Expect(len(components)).To(Equal(9))
		Expect(err).To(BeNil())
	})
})
