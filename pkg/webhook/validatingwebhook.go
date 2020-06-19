// Copyright (c) 2020 Red Hat, Inc.

package webhook

import (
	"context"
	"errors"
	"net/http"
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	operatorsv1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operator/v1"
)

type multiClusterHubValidator struct {
	client  client.Client
	decoder *admission.Decoder
}

// Handle set the default values to every incoming MultiClusterHub cr.
// Currently only handles create/update
func (m *multiClusterHubValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	multiClusterHubs := &operatorsv1.MultiClusterHubList{}
	if err := m.client.List(context.TODO(), multiClusterHubs); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
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

	return admission.Denied("Operation not allowed on MultiClusterHub CR")
}

func (m *multiClusterHubValidator) validateCreate(req admission.Request) error {

	creatingMCH := &operatorsv1.MultiClusterHub{}
	err := m.decoder.DecodeRaw(req.Object, creatingMCH)
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

	if existingMCH.Spec.Etcd.Storage != "" && existingMCH.Spec.Etcd.Storage != newMCH.Spec.Etcd.Storage {
		return errors.New("Updating Etcd storage is forbidden")
	}
	if existingMCH.Spec.Etcd.StorageClass != "" && existingMCH.Spec.Etcd.StorageClass != newMCH.Spec.Etcd.StorageClass {
		return errors.New("Updating Etcd storageClass is forbidden")
	}

	if existingMCH.Spec.Mongo.Storage != "" && existingMCH.Spec.Mongo.Storage != newMCH.Spec.Mongo.Storage {
		return errors.New("Updating Mongo storage is forbidden")
	}
	if existingMCH.Spec.Mongo.StorageClass != "" && existingMCH.Spec.Mongo.StorageClass != newMCH.Spec.Mongo.StorageClass {
		return errors.New("Updating Mongo storageClass is forbidden")
	}

	if !reflect.DeepEqual(existingMCH.Spec.Hive, newMCH.Spec.Hive) {
		return errors.New("Hive updates are forbidden")
	}

	if existingMCH.Spec.IPv6 != newMCH.Spec.IPv6 {
		return errors.New("IPv6 update is forbidden")
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
