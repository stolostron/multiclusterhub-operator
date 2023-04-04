// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package webhook

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
)

type multiClusterHubValidator struct {
	client  client.Client
	decoder *admission.Decoder
}

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

// Handle set the default values to every incoming MultiClusterHub cr.
// Currently only handles create/update
func (m *multiClusterHubValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	multiClusterHubs := &operatorsv1.MultiClusterHubList{}
	if err := m.client.List(context.TODO(), multiClusterHubs); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if req.Operation == "CREATE" {
		err := m.validateCreate(req)
		if err != nil {
			log.Info("Create denied")
			return admission.Denied(err.Error())
		}
		log.Info("Create successful")
		return admission.Allowed("")
	}
	//If not create update
	if req.Operation == "UPDATE" {
		err := m.validateUpdate(req)
		if err != nil {
			log.Info("Update denied")
			return admission.Denied(err.Error())
		}
		log.Info("Update successful")
		return admission.Allowed("")
	}

	if req.Operation == "DELETE" {
		err := m.validateDelete(req)
		if err != nil {
			log.Info("Update denied")
			return admission.Denied(err.Error())
		}
		log.Info("Delete successful")
		return admission.Allowed("")
	}

	return admission.Denied("Operation not allowed on MultiClusterHub CR")
}

func (m *multiClusterHubValidator) validateCreate(req admission.Request) error {
	mch := &operatorsv1.MultiClusterHub{}
	err := m.decoder.DecodeRaw(req.Object, mch)
	if err != nil {
		return err
	}

	multiClusterHubs := &operatorsv1.MultiClusterHubList{}
	if err := m.client.List(context.TODO(), multiClusterHubs); err != nil {
		return fmt.Errorf("unable to list MultiClusterHubs: %s", err)
	}

	// Standalone MCH must exist before a hosted MCH can be created
	if (len(multiClusterHubs.Items) == 0) && mch.IsInHostedMode() {
		return fmt.Errorf("A Hosted Mode MCH can only be created once a non-hosted MCH is present")
	}

	// Prevent two standalone MCH's
	for _, existing := range multiClusterHubs.Items {
		existingMCH := existing
		if !mch.IsInHostedMode() && !existingMCH.IsInHostedMode() {
			return fmt.Errorf("MultiClusterHub in Standalone mode already exists: `%s`. Only one resource may exist in Standalone mode.", existingMCH.Name)
		}
	}

	// Validate components
	if mch.Spec.Overrides != nil {
		for _, c := range mch.Spec.Overrides.Components {
			if !operatorsv1.ValidComponent(c) {
				return errors.New(fmt.Sprintf("invalid component config: %s is not a known component", c.Name))
			}
		}
	}

	return nil
}

func (m *multiClusterHubValidator) validateUpdate(req admission.Request) error {

	// Parse existing and new MultiClusterHub resources
	existingMCH := &operatorsv1.MultiClusterHub{}
	err := m.decoder.DecodeRaw(req.OldObject, existingMCH)
	if err != nil {
		return err
	}
	newMCH := &operatorsv1.MultiClusterHub{}
	err = m.decoder.DecodeRaw(req.Object, newMCH)
	if err != nil {
		return err
	}
	if existingMCH.Spec.SeparateCertificateManagement != newMCH.Spec.SeparateCertificateManagement {
		return errors.New("Updating SeparateCertificateManagement is forbidden")
	}

	if existingMCH.IsInHostedMode() != newMCH.IsInHostedMode() {
		return fmt.Errorf("Changes cannot be made to DeploymentMode")
	}

	if !reflect.DeepEqual(existingMCH.Spec.Hive, newMCH.Spec.Hive) {
		return errors.New("Hive updates are forbidden")
	}

	if !utils.AvailabilityConfigIsValid(newMCH.Spec.AvailabilityConfig) && newMCH.Spec.AvailabilityConfig != "" {
		return errors.New("Invalid AvailabilityConfig given")
	}

	// Validate components
	if newMCH.Spec.Overrides != nil {
		for _, c := range newMCH.Spec.Overrides.Components {
			if !operatorsv1.ValidComponent(c) {
				return errors.New(fmt.Sprintf("invalid component config: %s is not a known component", c.Name))
			}
		}
	}

	return nil
}

func (m *multiClusterHubValidator) validateDelete(req admission.Request) error {

	ctx := context.Background()

	mch := &operatorsv1.MultiClusterHub{}
	err := m.decoder.DecodeRaw(req.OldObject, mch)
	if err != nil {
		return err
	}

	// Do not block delete of hosted mode, which does not spawn the resources
	if mch.IsInHostedMode() {
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
			if err := m.client.List(ctx, list); err != nil {
				return fmt.Errorf("unable to list %s: %s", resource.Name, err)
			}
			// If there are any unexpected resources, deny deletion
			if len(list.Items) > resource.ExceptionTotal {
				return fmt.Errorf("Cannot delete MultiClusterHub resource because %s resource(s) exist", resource.Name)
			}
			// if exception resources are present, check if they are the same as the exception resources
			if resource.ExceptionTotal > 0 {
				for _, item := range list.Items {
					if !contains(resource.Exceptions, item.GetName()) {
						return fmt.Errorf("Cannot delete MultiClusterHub resource because %s resource(s) exist", resource.Name)
					}
				}
			}
		}
	}

	return nil
}

// multiClusterHubValidator implements inject.Client.
// A client will be automatically injected.

// InjectClient injects the client.
func (m *multiClusterHubValidator) InjectClient(c client.Client) error {
	m.client = c
	return nil
}

// multiClusterHubValidator implements admission.DecoderInjector.
// A decoder will be automatically injected.

// InjectDecoder injects the decoder.
func (m *multiClusterHubValidator) InjectDecoder(d *admission.Decoder) error {
	m.decoder = d
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
