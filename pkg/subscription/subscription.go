// Copyright (c) 2020 Red Hat, Inc.

package subscription

import (
	"bytes"

	operatorsv1beta1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1beta1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/channel"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
func newSubscription(m *operatorsv1beta1.MultiClusterHub, s *Subscription) *unstructured.Unstructured {
	sub := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps.open-cluster-management.io/v1",
			"kind":       "Subscription",
			"metadata": map[string]interface{}{
				"name":      s.Name + "-sub",
				"namespace": s.Namespace,
			},
			"spec": map[string]interface{}{
				"channel": m.Namespace + "/" + channel.ChannelName,
				"name":    s.Name,
				"placement": map[string]interface{}{
					"local": true,
				},
				"packageOverrides": []map[string]interface{}{
					{
						"packageName": s.Name,
						"packageOverrides": []map[string]interface{}{
							{
								"path":  "spec",
								"value": s.Overrides,
							},
						},
					},
				},
			},
		},
	}
	utils.AddInstallerLabel(sub, m.Name, m.Namespace)
	sub.SetOwnerReferences([]metav1.OwnerReference{
		*metav1.NewControllerRef(m, m.GetObjectKind().GroupVersionKind()),
	})
	return sub
}

// Validate returns true if an update is needed to reconcile differences with the current spec. If an update
// is needed it returns the object with the new spec to update with.
func Validate(found *unstructured.Unstructured, want *unstructured.Unstructured) (*unstructured.Unstructured, bool) {
	var log = logf.Log.WithValues("Namespace", found.GetNamespace(), "Name", found.GetName(), "Kind", found.GetKind())

	desired, err := yaml.Marshal(want.Object["spec"])
	if err != nil {
		log.Error(err, "issue parsing desired subscription values")
	}
	current, err := yaml.Marshal(found.Object["spec"])
	if err != nil {
		log.Error(err, "issue parsing current subscription values")
	}

	if res := bytes.Compare(desired, current); res != 0 {
		// Return current object with adjusted spec, preserving metadata
		log.V(1).Info("Subscription doesn't match spec", "Want", want.Object["spec"], "Have", found.Object["spec"])
		found.Object["spec"] = want.Object["spec"]
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
