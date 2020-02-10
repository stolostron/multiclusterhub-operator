package webhook

import (
	"context"
	"net/http"

	operatorsv1alpha1 "github.com/rh-ibm-synergy/multicloudhub-operator/pkg/apis/operators/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type multiCloudHubValidator struct {
	client  client.Client
	decoder *admission.Decoder
}

// Handle set the default values to every incoming MultiCloudHub cr.
func (m *multiCloudHubValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	multiCloudHubs := &operatorsv1alpha1.MultiCloudHubList{}
	if err := m.client.List(context.TODO(), multiCloudHubs); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if len(multiCloudHubs.Items) == 0 {
		return admission.Allowed("")
	}

	return admission.Denied("The MultiClodHub CR already exists")
}

// multiCloudHubMutator implements inject.Client.
// A client will be automatically injected.

// InjectClient injects the client.
func (m *multiCloudHubValidator) InjectClient(c client.Client) error {
	m.client = c
	return nil
}

// multiCloudHubMutator implements admission.DecoderInjector.
// A decoder will be automatically injected.

// InjectDecoder injects the decoder.
func (m *multiCloudHubValidator) InjectDecoder(d *admission.Decoder) error {
	m.decoder = d
	return nil
}
