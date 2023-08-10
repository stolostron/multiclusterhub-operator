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
	"fmt"
	"reflect"

	admissionregistration "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	blockDeletionResources = []struct {
		Name           string
		GVK            schema.GroupVersionKind
		ExceptionTotal int
		Exceptions     []string
	}{
		{
			Name: "ManagedCluster",
			GVK: schema.GroupVersionKind{
				Group:   "cluster.open-cluster-management.io",
				Version: "v1",
				Kind:    "ManagedClusterList",
			},
			ExceptionTotal: 1,
			Exceptions:     []string{"local-cluster"},
		},
		{
			Name: "MultiClusterObservability",
			GVK: schema.GroupVersionKind{
				Group:   "observability.open-cluster-management.io",
				Version: "v1beta2",
				Kind:    "MultiClusterObservabilityList",
			},
			ExceptionTotal: 0,
			Exceptions:     []string{},
		},
		{
			Name: "DiscoveryConfig",
			GVK: schema.GroupVersionKind{
				Group:   "discovery.open-cluster-management.io",
				Version: "v1",
				Kind:    "DiscoveryConfigList",
			},
			ExceptionTotal: 0,
			Exceptions:     []string{},
		},
		{
			Name: "AgentServiceConfig",
			GVK: schema.GroupVersionKind{
				Group:   "agent-install.openshift.io",
				Version: "v1beta1",
				Kind:    "AgentServiceConfigList",
			},
			ExceptionTotal: 0,
			Exceptions:     []string{},
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
func (r *MultiClusterHub) ValidateCreate() error {
	mchlog.Info("validate create", "name", r.Name)
	multiClusterHubList := &MultiClusterHubList{}
	if err := Client.List(context.Background(), multiClusterHubList); err != nil {
		return fmt.Errorf("unable to list MultiClusterHubs: %s", err)
	}

	// Standalone MCH must exist before a hosted MCH can be created
	if len(multiClusterHubList.Items) == 0 && r.IsInHostedMode() {
		return fmt.Errorf("a hosted Mode MCH can only be created once a non-hosted MCH is present")

	}

	// Prevent two standaline MCH's
	for _, existing := range multiClusterHubList.Items {
		existingMCH := existing
		if !r.IsInHostedMode() && !existingMCH.IsInHostedMode() {
			return fmt.Errorf("MultiClusterHub in Standalone mode already exists: `%s`. Only one resource may exist in Standalone mode", existingMCH.Name)
		}
	}

	// Validate components
	if r.Spec.Overrides != nil {
		for _, c := range r.Spec.Overrides.Components {
			if !ValidComponent(c) {
				return fmt.Errorf("invalid component config: %s is not a known component", c.Name)
			}
		}
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *MultiClusterHub) ValidateUpdate(old runtime.Object) error {
	mchlog.Info("validate update", "name", r.Name)

	oldMCH := old.(*MultiClusterHub)

	if oldMCH.Spec.SeparateCertificateManagement != r.Spec.SeparateCertificateManagement {
		return fmt.Errorf("updating SeparateCertificateManagement is forbidden")
	}

	if oldMCH.IsInHostedMode() != r.IsInHostedMode() {
		return fmt.Errorf("changes cannot be made to DeploymentMode")
	}

	if !reflect.DeepEqual(oldMCH.Spec.Hive, r.Spec.Hive) {
		return fmt.Errorf("hive updates are forbidden")
	}

	if (r.Spec.AvailabilityConfig != HABasic) && (r.Spec.AvailabilityConfig != HAHigh) && (r.Spec.AvailabilityConfig != "") {
		return fmt.Errorf("invalid AvailabilityConfig given")
	}

	// Validate components
	if r.Spec.Overrides != nil {
		for _, c := range r.Spec.Overrides.Components {
			if !ValidComponent(c) {
				return fmt.Errorf("invalid componentconfig: %s is not a known component", c.Name)
			}
		}
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *MultiClusterHub) ValidateDelete() error {
	mchlog.Info("validate delete", "name", r.Name)

	ctx := context.Background()

	// Do not block delete of hosted mode, which does not spawn the resources
	if r.IsInHostedMode() {
		return nil
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return err
	}

	c, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return err
	}

	for _, resource := range blockDeletionResources {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(resource.GVK)
		err := discovery.ServerSupportsVersion(c, list.GroupVersionKind().GroupVersion())
		if err == nil {
			// List all resources
			if err := Client.List(ctx, list); err != nil {
				return fmt.Errorf("unable to list %s: %s", resource.Name, err)
			}
			// If there are any unexpected resources, deny deletion
			if len(list.Items) > resource.ExceptionTotal {
				return fmt.Errorf("cannot delete MultiClusterHub resource because %s resource(s) exist", resource.Name)
			}
			// if exception resources are present, check if they are the same as the exception resources
			if resource.ExceptionTotal > 0 {
				for _, item := range list.Items {
					if !contains(resource.Exceptions, item.GetName()) {
						return fmt.Errorf("cannot delete MultiClusterHub resource because %s resource(s) exist", resource.Name)
					}
				}
			}
		}
	}
	return nil
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
