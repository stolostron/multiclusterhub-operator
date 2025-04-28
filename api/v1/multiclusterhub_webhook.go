// Copyright Contributors to the Open Cluster Management project

/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"

	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	admissionregistration "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type BlockDeletionResource struct {
	Name            string
	GVK             schema.GroupVersionKind
	ExceptionTotal  int
	NameExceptions  []string
	LabelExceptions map[string]string
}

var (
	blockDeletionResources = []BlockDeletionResource{
		{
			Name: "MultiClusterObservability",
			GVK: schema.GroupVersionKind{
				Group:   "observability.open-cluster-management.io",
				Version: "v1beta2",
				Kind:    "MultiClusterObservabilityList",
			},
			ExceptionTotal: 0,
			NameExceptions: []string{},
		},
		{
			Name: "DiscoveryConfig",
			GVK: schema.GroupVersionKind{
				Group:   "discovery.open-cluster-management.io",
				Version: "v1",
				Kind:    "DiscoveryConfigList",
			},
			ExceptionTotal: 0,
			NameExceptions: []string{},
		},
		{
			Name: "AgentServiceConfig",
			GVK: schema.GroupVersionKind{
				Group:   "agent-install.openshift.io",
				Version: "v1beta1",
				Kind:    "AgentServiceConfigList",
			},
			ExceptionTotal: 0,
			NameExceptions: []string{},
		},
	}
)

var (
	mchlog = log.Log.WithName("multiclusterhub-resource")
	Client client.Client
)

func (r *MultiClusterHub) SetupWebhookWithManager(mgr ctrl.Manager) error {
	Client = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).For(r).Complete()
}

var _ webhook.Defaulter = &MultiClusterHub{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *MultiClusterHub) Default() {
	mchlog.Info("default", "name", r.Name)
}

//+kubebuilder:webhook:name=multiclusterhub-operator-validating-webhook,path=/validate-operator-open-cluster-management-io-v1-multiclusterhub,mutating=false,failurePolicy=fail,sideEffects=None,groups=operator.open-cluster-management.io,resources=multiclusterhubs,verbs=create;update;delete,versions=v1,name=multiclusterhub.validating-webhook.open-cluster-management.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &MultiClusterHub{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *MultiClusterHub) ValidateCreate() (admission.Warnings, error) {
	mchlog.Info("validate create", "Name", r.Name, "Namespace", r.Namespace)

	multiClusterHubList := &MultiClusterHubList{}
	if err := Client.List(context.Background(), multiClusterHubList); err != nil {
		return nil, fmt.Errorf("unable to list MultiClusterHubs: %s", err)
	}

	// Prevent two standalone MCH's
	if len(multiClusterHubList.Items) > 0 {
		existingMCH := multiClusterHubList.Items[0]
		return nil, fmt.Errorf("MultiClusterHub in Standalone mode already exists: `%s`", existingMCH.GetName())
	}

	if (r.Spec.AvailabilityConfig != HABasic) && (r.Spec.AvailabilityConfig != HAHigh) && (r.Spec.AvailabilityConfig != "") {
		return nil, fmt.Errorf("invalid AvailabilityConfig given")
	}

	// Validate components
	if r.Spec.Overrides != nil {
		for _, c := range r.Spec.Overrides.Components {
			if !ValidComponent(c, MCHComponents) {
				return nil, fmt.Errorf("invalid component config: %s is not a known component", c.Name)
			}
		}
	}

	// If MCE CR exists, then spec.localClusterName must match
	mceList := &mcev1.MultiClusterEngineList{}
	// If installing ACM standalone, then MCE will fail to list. This is expected
	if err := Client.List(context.Background(), mceList); errors.Is(err, errors.New("no matches for kind \"MultiClusterEngine\" in version \"multicluster.openshift.io/v1\"")) {
		return nil, err
	}
	if len(mceList.Items) == 1 {
		mce := mceList.Items[0]
		if mce.Spec.LocalClusterName != r.Spec.LocalClusterName {
			return nil, fmt.Errorf("Spec.LocalClusterName does not match MCE Spec.LocalClusterName: %s", mce.Spec.LocalClusterName)
		}
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *MultiClusterHub) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	mchlog.Info("validate update", "Name", r.Name, "Namespace", r.Namespace)

	oldMCH := old.(*MultiClusterHub)

	if oldMCH.Spec.SeparateCertificateManagement != r.Spec.SeparateCertificateManagement {
		return nil, fmt.Errorf("updating SeparateCertificateManagement is forbidden")
	}

	if !reflect.DeepEqual(oldMCH.Spec.Hive, r.Spec.Hive) {
		return nil, fmt.Errorf("hive updates are forbidden")
	}

	if (r.Spec.AvailabilityConfig != HABasic) && (r.Spec.AvailabilityConfig != HAHigh) && (r.Spec.AvailabilityConfig != "") {
		return nil, fmt.Errorf("invalid AvailabilityConfig given")
	}

	// Validate components
	if r.Spec.Overrides != nil {
		for _, c := range r.Spec.Overrides.Components {
			if !ValidComponent(c, MCHComponents) {
				return nil, fmt.Errorf("invalid componentconfig: %s is not a known component", c.Name)
			}
		}
	}

	// Block changing localClusterName if ManagdCluster with label `local-cluster = true` exists
	// if the Spec.LocalClusterName field has changed
	if oldMCH.Spec.LocalClusterName != r.Spec.LocalClusterName {
		ctx := context.Background()
		managedClusterGVK := schema.GroupVersionKind{
			Group:   "cluster.open-cluster-management.io",
			Version: "v1",
			Kind:    "ManagedClusterList",
		}
		mcName := oldMCH.Spec.LocalClusterName

		// list ManagedClusters
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(managedClusterGVK)
		if err := Client.List(ctx, list); err != nil {
			return nil, fmt.Errorf("unable to list ManagedCluster: %v", err)
		}

		// Error if any of the ManagedClusters is the `local-cluster`
		for _, managedCluster := range list.Items {
			if managedCluster.GetName() == mcName || managedCluster.GetLabels()["local-cluster"] == "true" {
				return nil, fmt.Errorf("cannot update Spec.LocalClusterName while local-cluster is enabled")
			}
		}
	}

	return nil, nil
}

var cfg *rest.Config

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *MultiClusterHub) ValidateDelete() (admission.Warnings, error) {
	mchlog.Info("validate delete", "Name", r.Name, "Namespace", r.Namespace)

	if val, ok := os.LookupEnv("ENV_TEST"); !ok || val == "false" {
		var err error
		cfg, err = config.GetConfig()
		if err != nil {
			return nil, err
		}
	}

	c, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, err
	}

	tmpBlockDeletionResources := append(blockDeletionResources, BlockDeletionResource{
		Name: "ManagedCluster",
		GVK: schema.GroupVersionKind{
			Group:   "cluster.open-cluster-management.io",
			Version: "v1",
			Kind:    "ManagedClusterList",
		},
		ExceptionTotal:  1,
		NameExceptions:  []string{r.Spec.LocalClusterName},
		LabelExceptions: map[string]string{"local-cluster": "true"},
	})
	for _, resource := range tmpBlockDeletionResources {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(resource.GVK)
		err := discovery.ServerSupportsVersion(c, list.GroupVersionKind().GroupVersion())
		if err != nil {
			continue
		}
		// List all resources
		if err := Client.List(context.Background(), list); err != nil {
			return nil, fmt.Errorf("unable to list %s: %s", resource.Name, err)
		}
		// If there are any unexpected resources, deny deletion
		if len(list.Items) > resource.ExceptionTotal {
			return nil, fmt.Errorf("cannot delete MultiClusterHub resource because %s resource(s) exist", resource.Name)
		}
		// if exception resources are present, check if they are the same as the exception resources
		if resource.ExceptionTotal > 0 {
			for _, item := range list.Items {
				if !contains(resource.NameExceptions, item.GetName()) {
					return nil, fmt.Errorf("cannot delete MultiClusterHub resource because %s resource(s) exist", resource.Name)
				}
				if !hasIntersection(resource.LabelExceptions, item.GetLabels()) {
					return nil, fmt.Errorf("cannot delete MultiClusterHub resource because %s resource(s) are missing %v labels", resource.Name, resource.LabelExceptions)
				}
			}
		}
	}
	return nil, nil
}

func hasIntersection(smallerMap map[string]string, largerMap map[string]string) bool {
	// iterate through the keys of the smaller map to save time
	for k, sVal := range smallerMap {
		if lVal := largerMap[k]; lVal == sVal {
			return true // return true if A and B share any complete key-value pair
		}
	}
	return false
}

// ValidatingWebhook returns the ValidatingWebhookConfiguration used for the multiclusterhub
// linked to a service in the provided namespace
func ValidatingWebhook(namespace string) *admissionregistration.ValidatingWebhookConfiguration {
	fail := admissionregistration.Fail
	none := admissionregistration.SideEffectClassNone
	path := "/validate-operator-open-cluster-management-io-v1-multiclusterhub"
	return &admissionregistration.ValidatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admissionregistration.k8s.io/v1",
			Kind:       "ValidatingWebhookConfiguration",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "multiclusterhub-operator-validating-webhook",
			Annotations: map[string]string{"service.beta.openshift.io/inject-cabundle": "true"},
		},
		Webhooks: []admissionregistration.ValidatingWebhook{
			{
				AdmissionReviewVersions: []string{
					"v1",
					"v1beta1",
				},
				Name: "multiclusterhub.validating-webhook.open-cluster-management.io",
				ClientConfig: admissionregistration.WebhookClientConfig{
					Service: &admissionregistration.ServiceReference{
						Name:      "multiclusterhub-operator-webhook",
						Namespace: namespace,
						Path:      &path,
					},
				},
				FailurePolicy: &fail,
				Rules: []admissionregistration.RuleWithOperations{
					{
						Rule: admissionregistration.Rule{
							APIGroups:   []string{GroupVersion.Group},
							APIVersions: []string{GroupVersion.Version},
							Resources:   []string{"multiclusterhubs"},
						},
						Operations: []admissionregistration.OperationType{
							admissionregistration.Create,
							admissionregistration.Update,
							admissionregistration.Delete,
						},
					},
				},
				SideEffects: &none,
			},
		},
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
