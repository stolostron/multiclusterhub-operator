package webhook

import (
	"context"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
)

type multiClusterHubValidator struct {
	client  client.Client
	decoder *admission.Decoder
}

// Handle set the default values to every incoming MultiClusterHub cr.
func (m *multiClusterHubValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	multiClusterHubs := &operatorsv1alpha1.MultiClusterHubList{}
	if err := m.client.List(context.TODO(), multiClusterHubs); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if len(multiClusterHubs.Items) == 0 {
		return admission.Allowed("")
	}

	return admission.Denied("The MultiClodHub CR already exists")
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
