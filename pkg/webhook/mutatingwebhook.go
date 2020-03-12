package webhook

import (
	"context"
	"encoding/json"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
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
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if multiClusterHub.Spec.Version == "" {
		multiClusterHub.Spec.Version = LatestVerison
	}

	if multiClusterHub.Spec.ImageRepository == "" {
		multiClusterHub.Spec.ImageRepository = DefaultRepository
	}

	if multiClusterHub.Spec.ImagePullPolicy == "" {
		multiClusterHub.Spec.ImagePullPolicy = corev1.PullAlways
	}

	var replicas int32 = 1
	if multiClusterHub.Spec.Foundation.Apiserver.Replicas == nil {
		multiClusterHub.Spec.Foundation.Apiserver.Replicas = &replicas
	}

	if multiClusterHub.Spec.Foundation.Apiserver.ApiserverSecret == "" {
		multiClusterHub.Spec.Foundation.Apiserver.ApiserverSecret = utils.APIServerSecretName
	}

	if multiClusterHub.Spec.Foundation.Apiserver.KlusterletSecret == "" {
		multiClusterHub.Spec.Foundation.Apiserver.KlusterletSecret = utils.KlusterletSecretName
	}

	if len(multiClusterHub.Spec.Foundation.Apiserver.Configuration) == 0 {
		multiClusterHub.Spec.Foundation.Apiserver.Configuration = map[string]string{"http2-max-streams-per-connection": "1000"}
	}

	if multiClusterHub.Spec.Foundation.Controller.Replicas == nil {
		multiClusterHub.Spec.Foundation.Controller.Replicas = &replicas
	}

	if len(multiClusterHub.Spec.Foundation.Controller.Configuration) == 0 {
		multiClusterHub.Spec.Foundation.Controller.Configuration = map[string]string{
			"enable-rbac":             "true",
			"enable-service-registry": "true",
		}
	}

	marshaledMultiClusterHub, err := json.Marshal(multiClusterHub)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	log.Info("Finish to mutate MultiClusterHub.")
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
