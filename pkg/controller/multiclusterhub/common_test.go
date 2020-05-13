// Copyright (c) 2020 Red Hat, Inc.

package multiclusterhub

import (
	"fmt"
	"testing"

	operatorsv1beta1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1beta1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/channel"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/helmrepo"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/mcm"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/subscription"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_ensureDeployment(t *testing.T) {
	r, err := getTestReconciler(full_mch)
	if err != nil {
		t.Fatalf("Failed to create test reconciler")
	}

	cacheSpec := utils.CacheSpec{
		IngressDomain: "apps.smart-buck.dev01.red-chesterfield.com",
		ImageOverrides: map[string]string{
			"application_ui": "quay.io/open-cluster-management/application-ui@sha256:c740fc7bac067f003145ab909504287360564016b7a4a51b7ad4987aca123ac1",
			"console_api":    "quay.io/open-cluster-management/console-api@sha256:3ef1043b4e61a09b07ff37f9ad8fc6e707af9813936cf2c0d52f2fa0e489c75f",
			"rcm_controller": " quay.io/open-cluster-management/rcm-controller@sha256:8fab4d788241bf364dbc1b8c1ea5ccf18d3145a640dbd456b0dc7ba204e36819",
		},
	}

	tests := []struct {
		Name       string
		MCH        *operatorsv1beta1.MultiClusterHub
		Deployment *appsv1.Deployment
		Result     error
	}{
		{
			Name:       "Test: EnsureDeployment - APIServer",
			MCH:        full_mch,
			Deployment: mcm.APIServerDeployment(full_mch, cacheSpec),
			Result:     nil,
		},
		{
			Name:       "Test: EnsureDeployment - Multiclusterhub-repo",
			MCH:        full_mch,
			Deployment: helmrepo.Deployment(full_mch, cacheSpec),
			Result:     nil,
		},
		{
			Name:       "Test: EnsureDeployment - Webhook",
			MCH:        full_mch,
			Deployment: mcm.WebhookDeployment(full_mch, cacheSpec),
			Result:     nil,
		},
		{
			Name:       "Test: EnsureDeployment - Webhook",
			MCH:        full_mch,
			Deployment: mcm.ControllerDeployment(full_mch, cacheSpec),
			Result:     nil,
		},
		{
			Name:       "Test: EnsureDeployment - Empty Deployment",
			MCH:        full_mch,
			Deployment: &appsv1.Deployment{},
			Result:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			_, err = r.ensureDeployment(tt.MCH, tt.Deployment)
			if err != nil {
				t.Fatalf("Failed to ensure deployment")
			}
		})
	}

}

func Test_ensureService(t *testing.T) {
	r, err := getTestReconciler(full_mch)
	if err != nil {
		t.Fatalf("Failed to create test reconciler")
	}

	tests := []struct {
		Name    string
		MCH     *operatorsv1beta1.MultiClusterHub
		Service *corev1.Service
		Result  error
	}{
		{
			Name:    "Test: ensureService - Multiclusterhub-repo",
			MCH:     full_mch,
			Service: helmrepo.Service(full_mch),
			Result:  nil,
		},
		{
			Name:    "Test: ensureService - APIServer",
			MCH:     full_mch,
			Service: mcm.APIServerService(full_mch),
			Result:  nil,
		},
		{
			Name:    "Test: ensureService - Webhook",
			MCH:     full_mch,
			Service: mcm.WebhookService(full_mch),
			Result:  nil,
		},
		{
			Name:    "Test: ensureService - Empty service",
			MCH:     full_mch,
			Service: &corev1.Service{},
			Result:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			_, err = r.ensureService(tt.MCH, tt.Service)
			if !errorEquals(err, tt.Result) {
				t.Fatalf("Failed to ensure service")
			}
		})
	}
}

func Test_ensureSecret(t *testing.T) {
	r, err := getTestReconciler(full_mch)
	if err != nil {
		t.Fatalf("Failed to create test reconciler")
	}

	tests := []struct {
		Name   string
		MCH    *operatorsv1beta1.MultiClusterHub
		Secret *corev1.Secret
		Result error
	}{
		{
			Name:   "Test: ensureSecret - Multiclusterhub-repo",
			MCH:    full_mch,
			Secret: r.mongoAuthSecret(full_mch),
			Result: nil,
		},
		{
			Name:   "Test: ensureSecret - Empty secret",
			MCH:    full_mch,
			Secret: &corev1.Secret{},
			Result: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			_, err = r.ensureSecret(tt.MCH, tt.Secret)
			if !errorEquals(err, tt.Result) {
				t.Fatalf("Failed to ensure secret")
			}
		})
	}
}

func Test_ensureChannel(t *testing.T) {
	r, err := getTestReconciler(full_mch)
	if err != nil {
		t.Fatalf("Failed to create test reconciler")
	}

	tests := []struct {
		Name    string
		MCH     *operatorsv1beta1.MultiClusterHub
		Channel *unstructured.Unstructured
		Result  error
	}{
		{
			Name:    "Test: ensureSecret - Multiclusterhub-repo",
			MCH:     full_mch,
			Channel: channel.Channel(full_mch),
			Result:  nil,
		},
		{
			Name:    "Test: ensureSecret - Empty channel",
			MCH:     full_mch,
			Channel: &unstructured.Unstructured{},
			Result:  fmt.Errorf("Object 'Kind' is missing in 'unstructured object has no kind'"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			_, err = r.ensureChannel(tt.MCH, tt.Channel)
			if !errorEquals(err, tt.Result) {
				t.Fatalf("Failed to ensure channel")
			}
		})
	}
}

func Test_ensureSubscription(t *testing.T) {
	r, err := getTestReconciler(full_mch)
	if err != nil {
		t.Fatalf("Failed to create test reconciler")
	}

	cacheSpec := utils.CacheSpec{
		IngressDomain: "apps.smart-buck.dev01.red-chesterfield.com",
		ImageOverrides: map[string]string{
			"application_ui": "quay.io/open-cluster-management/application-ui@sha256:c740fc7bac067f003145ab909504287360564016b7a4a51b7ad4987aca123ac1",
			"console_api":    "quay.io/open-cluster-management/console-api@sha256:3ef1043b4e61a09b07ff37f9ad8fc6e707af9813936cf2c0d52f2fa0e489c75f",
			"rcm_controller": " quay.io/open-cluster-management/rcm-controller@sha256:8fab4d788241bf364dbc1b8c1ea5ccf18d3145a640dbd456b0dc7ba204e36819",
		},
	}

	tests := []struct {
		Name         string
		MCH          *operatorsv1beta1.MultiClusterHub
		Subscription *unstructured.Unstructured
		Result       error
	}{
		{
			Name:         "Test: ensureSubscription - Cert-manager",
			MCH:          full_mch,
			Subscription: subscription.CertManager(full_mch, cacheSpec),
			Result:       nil,
		},
		{
			Name:         "Test: ensureSubscription - Cert-webhook",
			MCH:          full_mch,
			Subscription: subscription.CertWebhook(full_mch, cacheSpec),
			Result:       nil,
		},
		{
			Name:         "Test: ensureSubscription - Config-watcher",
			MCH:          full_mch,
			Subscription: subscription.ConfigWatcher(full_mch, cacheSpec),
			Result:       nil,
		},
		{
			Name:         "Test: ensureSubscription - Management-ingress",
			MCH:          full_mch,
			Subscription: subscription.ManagementIngress(full_mch, cacheSpec),
			Result:       nil,
		},
		{
			Name:         "Test: ensureSubscription - Application-UI",
			MCH:          full_mch,
			Subscription: subscription.ApplicationUI(full_mch, cacheSpec),
			Result:       nil,
		},
		{
			Name:         "Test: ensureSubscription - Console",
			MCH:          full_mch,
			Subscription: subscription.Console(full_mch, cacheSpec),
			Result:       nil,
		},
		{
			Name:         "Test: ensureSubscription - GRC",
			MCH:          full_mch,
			Subscription: subscription.GRC(full_mch, cacheSpec),
			Result:       nil,
		},
		{
			Name:         "Test: ensureSubscription - KUI",
			MCH:          full_mch,
			Subscription: subscription.KUIWebTerminal(full_mch, cacheSpec),
			Result:       nil,
		},
		{
			Name:         "Test: ensureSubscription - Mongo",
			MCH:          full_mch,
			Subscription: subscription.MongoDB(full_mch, cacheSpec),
			Result:       nil,
		},
		{
			Name:         "Test: ensureSubscription - RCM",
			MCH:          full_mch,
			Subscription: subscription.RCM(full_mch, cacheSpec),
			Result:       nil,
		},
		{
			Name:         "Test: ensureSubscription - Search",
			MCH:          full_mch,
			Subscription: subscription.Search(full_mch, cacheSpec),
			Result:       nil,
		},
		{
			Name:         "Test: ensureSubscription - Topology",
			MCH:          full_mch,
			Subscription: subscription.Topology(full_mch, cacheSpec),
			Result:       nil,
		},
		{
			Name:         "Test: ensureSubscription - Empty Sub",
			MCH:          full_mch,
			Subscription: &unstructured.Unstructured{},
			Result:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			_, err = r.ensureSubscription(tt.MCH, tt.Subscription)
			if !errorEquals(err, tt.Result) {
				t.Fatalf("Failed to ensure subscription")
			}
		})
	}
}
