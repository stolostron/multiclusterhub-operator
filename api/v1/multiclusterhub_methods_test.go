// Copyright Contributors to the Open Cluster Management project
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func Test_ComponentEnabled(t *testing.T) {
	// tracker := StatusTracker{Client: fake.NewClientBuilder().Build()}

	t.Run("No components specified", func(t *testing.T) {
		mch := MultiClusterHub{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "multicluster.openshift.io/v1",
				Kind:       "MultiClusterHub",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: MultiClusterHubSpec{},
		}

		searchEnabled := mch.ComponentEnabled(Search)
		if !searchEnabled {
			t.Fatal("Expected no component enabled, but Search disabled")
		}

		msaEnabled := mch.ComponentEnabled(ManagedServiceAccount)
		if msaEnabled {
			t.Fatal("Expected no component specified, but ManagedServiceAccount enabled")
		}
		// FUTURE: INCLUDE ALL OTHER COMPONENT ENABLED OPTIONS HERE, ONCE THEY EXIST
	})

	t.Run("Seach enabled", func(t *testing.T) {
		mch := MultiClusterHub{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "multicluster.openshift.io/v1",
				Kind:       "MultiClusterHub",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: MultiClusterHubSpec{},
		}

		searchEnabled := mch.ComponentEnabled(Search)
		if !searchEnabled {
			t.Fatal("Expected search to be enabled (no ComponentConfig)")
		}

		mch = MultiClusterHub{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "multicluster.openshift.io/v1",
				Kind:       "MultiClusterHub",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: MultiClusterHubSpec{
				ComponentConfig: &ComponentConfig{},
			},
		}

		searchEnabled = mch.ComponentEnabled(Search)
		if !searchEnabled {
			t.Fatal("Expected search to be enabled (empty ComponentConfig)")
		}

		mch = MultiClusterHub{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "multicluster.openshift.io/v1",
				Kind:       "MultiClusterHub",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: MultiClusterHubSpec{
				ComponentConfig: &ComponentConfig{
					Search: &SearchConfig{},
				},
			},
		}

		searchEnabled = mch.ComponentEnabled(Search)
		if !searchEnabled {
			t.Fatal("Expected search to be enabled (empty SearchConfig)")
		}

		mch = MultiClusterHub{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "multicluster.openshift.io/v1",
				Kind:       "MultiClusterHub",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: MultiClusterHubSpec{
				ComponentConfig: &ComponentConfig{
					Search: &SearchConfig{
						Disable: false,
					},
				},
			},
		}

		searchEnabled = mch.ComponentEnabled(Search)
		if !searchEnabled {
			t.Fatal("Expected search to be enabled (disable set to false)")
		}
	})

	t.Run("Search disable", func(t *testing.T) {
		mch := MultiClusterHub{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "multicluster.openshift.io/v1",
				Kind:       "MultiClusterHub",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: MultiClusterHubSpec{
				ComponentConfig: &ComponentConfig{
					Search: &SearchConfig{
						Disable: true,
					},
				},
			},
		}

		searchEnabled := mch.ComponentEnabled(Search)
		if searchEnabled {
			t.Fatal("Expected search to be disabled")
		}
	})

	t.Run("ManagedServiceAccount not enabled", func(t *testing.T) {
		mch := MultiClusterHub{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "multicluster.openshift.io/v1",
				Kind:       "MultiClusterHub",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: MultiClusterHubSpec{},
		}

		msaEnabled := mch.ComponentEnabled(ManagedServiceAccount)
		if msaEnabled {
			t.Fatal("Expected ManagedServiceAccount to not be enabled (no ComponentConfig)")
		}

		mch = MultiClusterHub{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "multicluster.openshift.io/v1",
				Kind:       "MultiClusterHub",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: MultiClusterHubSpec{
				ComponentConfig: &ComponentConfig{},
			},
		}

		msaEnabled = mch.ComponentEnabled(ManagedServiceAccount)
		if msaEnabled {
			t.Fatal("Expected ManagedServiceAccount to not be enabled (empty ComponentConfig)")
		}

		mch = MultiClusterHub{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "multicluster.openshift.io/v1",
				Kind:       "MultiClusterHub",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: MultiClusterHubSpec{
				ComponentConfig: &ComponentConfig{
					ManagedServiceAccount: &ManagedServiceAccountConfig{},
				},
			},
		}

		msaEnabled = mch.ComponentEnabled(ManagedServiceAccount)
		if msaEnabled {
			t.Fatal("Expected ManagedServiceAccount to not be enabled (empty ManagedServiceAccountConfig)")
		}

		mch = MultiClusterHub{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "multicluster.openshift.io/v1",
				Kind:       "MultiClusterHub",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: MultiClusterHubSpec{
				ComponentConfig: &ComponentConfig{
					ManagedServiceAccount: &ManagedServiceAccountConfig{
						Enable: false,
					},
				},
			},
		}

		msaEnabled = mch.ComponentEnabled(ManagedServiceAccount)
		if msaEnabled {
			t.Fatal("Expected ManagedServiceAccount to not be enabled (Enable: false)")
		}
	})

	t.Run("ManagedServiceAccount enabled", func(t *testing.T) {
		mch := MultiClusterHub{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "multicluster.openshift.io/v1",
				Kind:       "MultiClusterHub",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: MultiClusterHubSpec{
				ComponentConfig: &ComponentConfig{
					ManagedServiceAccount: &ManagedServiceAccountConfig{
						Enable: true,
					},
				},
			},
		}

		msaEnabled := mch.ComponentEnabled(ManagedServiceAccount)
		if !msaEnabled {
			t.Fatal("Expected ManagedServiceAccount to be enabled")
		}
		// FUTURE: INCLUDE ALL OTHER COMPONENT ENABLED OPTIONS HERE, ONCE THEY EXIST
	})
}
