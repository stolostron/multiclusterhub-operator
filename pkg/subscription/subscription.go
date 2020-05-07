// Copyright (c) 2020 Red Hat, Inc.

package subscription

import (
	"bytes"
	"encoding/json"
	"fmt"

	plrv1alpha1 "github.com/open-cluster-management/multicloud-operators-placementrule/pkg/apis/apps/v1"
	subalpha1 "github.com/open-cluster-management/multicloud-operators-subscription/pkg/apis/apps/v1"
	operatorsv1beta1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1beta1"
	"github.com/prometheus/common/log"

	"github.com/open-cluster-management/multicloudhub-operator/pkg/channel"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/apimachinery/pkg/runtime/schema"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

// Schema is the GVK for an application subscription
var Schema = schema.GroupVersionResource{Group: "apps.open-cluster-management.io", Version: "v1", Resource: "subscriptions"}

// Subscription represents the unique elements of a Multicluster subscription object
type Subscription struct {
	Name      string
	Namespace string
	Overrides map[string]interface{}
}

// newSubscription creates a new instance of an unstructured open-cluster-management.io Subscription object
func newSubscription(m *operatorsv1beta1.MultiClusterHub, s *Subscription) *subalpha1.Subscription {
	packageOverrides := []map[string]interface{}{
		{
			"path":  "spec",
			"value": s.Overrides,
		},
	}
	byteArr, err := json.Marshal(packageOverrides)
	if err != nil {
		log.Error(err, "unable to marshal packageOverrides")
	}

	override := subalpha1.Overrides{
		PackageName: s.Name,
		PackageOverrides: []subalpha1.PackageOverride{
			subalpha1.PackageOverride{
				runtime.RawExtension{
					Raw: byteArr,
				},
			},
		},
	}
	placement := true
	sub := &subalpha1.Subscription{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps.open-cluster-management.io/v1",
			Kind:       "Subscription",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-sub", s.Name),
			Namespace: s.Namespace,
		},
		Spec: subalpha1.SubscriptionSpec{
			Channel: fmt.Sprintf("%s/%s", m.Namespace, channel.ChannelName),
			Placement: &plrv1alpha1.Placement{
				Local: &placement,
			},
			PackageOverrides: []*subalpha1.Overrides{
				&override,
			},
		},
	}

	if m.UID != "" {
		sub.SetOwnerReferences([]metav1.OwnerReference{
			*metav1.NewControllerRef(m, m.GetObjectKind().GroupVersionKind()),
		})
	}

	return sub
}

// Validate returns true if an update is needed to reconcile differences with the current spec. If an update
// is needed it returns the object with the new spec to update with.
func Validate(found *subalpha1.Subscription, want *subalpha1.Subscription) (*subalpha1.Subscription, bool) {
	var log = logf.Log.WithValues("Namespace", found.GetNamespace(), "Name", found.GetName(), "Kind", found.Kind)

	desired, err := yaml.Marshal(want.Spec)
	if err != nil {
		log.Error(err, "issue parsing desired subscription values")
	}
	current, err := yaml.Marshal(found.Spec)
	if err != nil {
		log.Error(err, "issue parsing current subscription values")
	}

	if res := bytes.Compare(desired, current); res != 0 {
		// Return current object with adjusted spec, preserving metadata
		log.V(1).Info("Subscription doesn't match spec", "Want", want.Spec, "Have", found.Spec)
		found.Spec = want.Spec
		return found, true
	}

	return nil, false
}

func imageSuffix(m *operatorsv1beta1.MultiClusterHub) (s string) {
	s = m.Spec.Overrides.ImageTagSuffix
	if s != "" {
		s = "-" + s
	}
	return
}

func networkVersion(m *operatorsv1beta1.MultiClusterHub) (ipv string) {
	if m.Spec.IPv6 {
		return "ipv6"
	}
	return "ipv4"
}
