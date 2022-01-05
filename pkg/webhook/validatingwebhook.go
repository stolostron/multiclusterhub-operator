// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package webhook

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	clustermanager "open-cluster-management.io/api/operator/v1"
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
	blockCreationResources = []struct {
		Name string
		GVK  schema.GroupVersionKind
	}{
		{
			Name: "MultiClusterEngine",
			GVK: schema.GroupVersionKind{
				Group:   "multicluster.openshift.io",
				Version: "v1alpha1",
				Kind:    "MultiClusterEngineList",
			},
		},
	}

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
			Name: "BareMetalAsset",
			GVK: schema.GroupVersionKind{
				Group:   "inventory.open-cluster-management.io",
				Version: "v1alpha1",
				Kind:    "BareMetalAssetList",
			},
			ExceptionTotal: 0,
			Exceptions:     []string{},
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
				Version: "v1alpha1",
				Kind:    "DiscoveryConfigList",
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

	if req.Kind.Kind == "ClusterManager" {
		if req.Operation == "DELETE" {
			err := m.validateClusterManagerDelete(req, multiClusterHubs)
			if err != nil {
				log.Info("Delete denied")
				return admission.Denied(err.Error())
			}
			log.Info("Delete successful")
			return admission.Allowed("")
		}
		// No other paths should exist
		return admission.Allowed("")
	}

	if req.Operation == "CREATE" {
		if len(multiClusterHubs.Items) == 0 {
			err := m.validateCreate(req)
			if err != nil {
				log.Info("Create denied")
				return admission.Denied(err.Error())
			}
			log.Info("Create successful")
			return admission.Allowed("")
		}
		return admission.Denied("The MultiClusterHub CR already exists")
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

	ctx := context.Background()
	log.Info("validate create", "name", req.Name)

	cfg, err := config.GetConfig()
	if err != nil {
		return err
	}

	c, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return err
	}

	for _, resource := range blockCreationResources {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(resource.GVK)
		err := discovery.ServerSupportsVersion(c, list.GroupVersionKind().GroupVersion())
		if err == nil {
			if err := m.client.List(ctx, list); err != nil {
				// Server may support Group Version, but explicitly also exempt Kind
				if strings.Contains(err.Error(), "no matches for kind") || k8serrors.IsNotFound(err) {
					continue
				}
				return fmt.Errorf("unable to list %s: %s", resource.Name, err)
			}
			if len(list.Items) == 0 {
				continue
			}
			return fmt.Errorf("cannot create %s resource. Existing %s resources must first be deleted", mch.Name, resource.Name)
		}
	}

	creatingMCH := &operatorsv1.MultiClusterHub{}
	err = m.decoder.DecodeRaw(req.Object, creatingMCH)
	if err != nil {
		return err
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

	if !reflect.DeepEqual(existingMCH.Spec.Hive, newMCH.Spec.Hive) {
		return errors.New("Hive updates are forbidden")
	}

	if !utils.AvailabilityConfigIsValid(newMCH.Spec.AvailabilityConfig) && newMCH.Spec.AvailabilityConfig != "" {
		return errors.New("Invalid AvailabilityConfig given")
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

func (m *multiClusterHubValidator) validateClusterManagerDelete(req admission.Request, mchList *operatorsv1.MultiClusterHubList) error {
	clusterManager := &clustermanager.ClusterManager{}
	err := m.decoder.DecodeRaw(req.OldObject, clusterManager)
	if err != nil {
		return err
	}

	username := req.UserInfo.Username
	if username == "" {
		return fmt.Errorf("ValidatingWebhook admission request must include Userinfo username")
	}

	if len(mchList.Items) != 1 {
		return fmt.Errorf("Only one MCH resource can exist")
	}
	mchName := mchList.Items[0].Name
	mchNamespace := mchList.Items[0].Namespace
	nameLabel, nameLabelExists := clusterManager.Labels["installer.name"]
	namespaceLabel, namespaceLabelExists := clusterManager.Labels["installer.namespace"]

	if nameLabelExists && namespaceLabelExists && nameLabel == mchName && namespaceLabel == mchNamespace {
		if username == fmt.Sprintf("system:serviceaccount:%s:multiclusterhub-operator", mchNamespace) {
			log.Info("MultiClusterHub is being uninstalled, allow for deletion of clustermanager")
			return nil
		}
		return fmt.Errorf("Unauthorized deletion of clustermanager resource")
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
