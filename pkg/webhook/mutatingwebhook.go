package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	storv1 "k8s.io/api/storage/v1"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
)

const (
	DefaultRepository = "quay.io/open-cluster-management"
	LatestVerison     = "latest"
)

type multiClusterHubMutator struct {
	client  client.Client
	decoder *admission.Decoder
}

// Handle set the default values to every incoming MultiClusterHub cr.
func (m *multiClusterHubMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	multiClusterHub := &operatorsv1alpha1.MultiClusterHub{}

	log.Info("Start to mutate MultiClusterHub ...")
	err := m.decoder.Decode(req, multiClusterHub)
	log.Info("Erroor")
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	log.Info("mutate made it past error")
	if multiClusterHub.Spec.Version == "" {
		multiClusterHub.Spec.Version = LatestVerison
	}

	if multiClusterHub.Spec.ImageRepository == "" {
		multiClusterHub.Spec.ImageRepository = DefaultRepository
	}

	if multiClusterHub.Spec.ImagePullPolicy == "" {
		multiClusterHub.Spec.ImagePullPolicy = corev1.PullAlways
	}

	if multiClusterHub.Spec.Mongo.Storage == "" {
		multiClusterHub.Spec.Mongo.Storage = "1Gi"
	}

	if multiClusterHub.Spec.Mongo.StorageClass == "" {
		storageClass, err := m.getStorageClass()
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		multiClusterHub.Spec.Mongo.StorageClass = storageClass
	}

	if multiClusterHub.Spec.Etcd.Storage == "" {
		multiClusterHub.Spec.Etcd.Storage = "1Gi"
	}

	if multiClusterHub.Spec.Etcd.StorageClass == "" {
		storageClass, err := m.getStorageClass()
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		multiClusterHub.Spec.Etcd.StorageClass = storageClass
	}

	marshaledMultiClusterHub, err := json.Marshal(multiClusterHub)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	log.Info("Finish mutating MultiClusterHub.")
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledMultiClusterHub)
}

// multiClusterHubMutator implements inject.Client.
// A client will be automatically injected.

// InjectClient injects the client.
func (m *multiClusterHubMutator) InjectClient(c client.Client) error {
	m.client = c
	return nil
}

// multiClusterHubMutator implements admission.DecoderInjector.
// A decoder will be automatically injected.

// InjectDecoder injects the decoder.
func (m *multiClusterHubMutator) InjectDecoder(d *admission.Decoder) error {
	m.decoder = d
	return nil
}

func (m *multiClusterHubMutator) getStorageClass() (string, error) {
	scList := &storv1.StorageClassList{}
	if err := m.client.List(context.TODO(), scList); err != nil {
		return "", err
	}
	for _, sc := range scList.Items {
		if sc.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
			return sc.GetName(), nil
		}
	}
	return "", fmt.Errorf("failed to find default storageclass")
}
