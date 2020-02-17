package webhook

import (
	"context"
	"encoding/json"
	"net/http"

	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	DefaultRepository = "quay.io/default-repo"
	LatestVerison     = "latest"
)

type multiCloudHubMutator struct {
	client  client.Client
	decoder *admission.Decoder
}

// Handle set the default values to every incoming MultiCloudHub cr.
func (m *multiCloudHubMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	multiCloudHub := &operatorsv1alpha1.MultiCloudHub{}

	log.Info("Start to mutate MultiCloudHub ...")
	err := m.decoder.Decode(req, multiCloudHub)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if multiCloudHub.Spec.Version == "" {
		multiCloudHub.Spec.Version = LatestVerison
	}

	if multiCloudHub.Spec.ImageRepository == "" {
		multiCloudHub.Spec.ImageRepository = DefaultRepository
	}

	if multiCloudHub.Spec.ImagePullPolicy == "" {
		multiCloudHub.Spec.ImagePullPolicy = corev1.PullAlways
	}

	var replicas int32 = 1
	if multiCloudHub.Spec.Foundation.Apiserver.Replicas == nil {
		multiCloudHub.Spec.Foundation.Apiserver.Replicas = &replicas
	}

	if multiCloudHub.Spec.Foundation.Apiserver.ApiserverSecret == "" {
		multiCloudHub.Spec.Foundation.Apiserver.ApiserverSecret = utils.APIServerSecretName
	}

	if multiCloudHub.Spec.Foundation.Apiserver.KlusterletSecret == "" {
		multiCloudHub.Spec.Foundation.Apiserver.KlusterletSecret = utils.KlusterletSecretName
	}

	if len(multiCloudHub.Spec.Foundation.Apiserver.Configuration) == 0 {
		multiCloudHub.Spec.Foundation.Apiserver.Configuration = map[string]string{"http2-max-streams-per-connection": "1000"}
	}

	if multiCloudHub.Spec.Foundation.Controller.Replicas == nil {
		multiCloudHub.Spec.Foundation.Controller.Replicas = &replicas
	}

	if len(multiCloudHub.Spec.Foundation.Controller.Configuration) == 0 {
		multiCloudHub.Spec.Foundation.Controller.Configuration = map[string]string{
			"enable-rbac":             "true",
			"enable-service-registry": "true",
		}
	}

	marshaledMultiCloudHub, err := json.Marshal(multiCloudHub)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	log.Info("Finish to mutate MultiCloudHub.")
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledMultiCloudHub)
}

// multiCloudHubMutator implements inject.Client.
// A client will be automatically injected.

// InjectClient injects the client.
func (m *multiCloudHubMutator) InjectClient(c client.Client) error {
	m.client = c
	return nil
}

// multiCloudHubMutator implements admission.DecoderInjector.
// A decoder will be automatically injected.

// InjectDecoder injects the decoder.
func (m *multiCloudHubMutator) InjectDecoder(d *admission.Decoder) error {
	m.decoder = d
	return nil
}
