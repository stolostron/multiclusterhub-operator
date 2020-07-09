// Copyright (c) 2020 Red Hat, Inc.

package utils

import (
	"context"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	k8scertutil "k8s.io/client-go/util/cert"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorsv1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operator/v1"
)

func TestGenerateSignedWebhookCertificates(t *testing.T) {
	os.Setenv(podNamespaceEnvVar, "test")
	certDir := "/tmp/tmp-cert"
	defer func() {
		os.RemoveAll(certDir)
		os.Unsetenv(podNamespaceEnvVar)
	}()

	ns, ca, err := GenerateWebhookCerts(certDir)
	if err != nil {
		t.Errorf("Generate signed certificate failed, %v", err)
	}
	if ns != "test" {
		t.Errorf("Generate signed certificate failed")
	}
	if ca == nil {
		t.Errorf("Generate signed certificate failed")
	}

	canReadCertAndKey, err := k8scertutil.CanReadCertAndKey("/tmp/tmp-cert/tls.crt", "/tmp/tmp-cert/tls.key")
	if err != nil {
		t.Errorf("Generate signed certificate failed, %v", err)
	}
	if !canReadCertAndKey {
		t.Errorf("Generate signed certificate failed")
	}
}

func TestGenerateKlusterletSecret(t *testing.T) {
	os.Setenv(podNamespaceEnvVar, "test")
	defer os.Unsetenv(podNamespaceEnvVar)

	fakeclient := fake.NewFakeClient()
	err := GenerateKlusterletSecret(fakeclient, &operatorsv1.MultiClusterHub{})
	if err != nil {
		t.Errorf("Expected nil, but failed %v", err)
	}

	err = GenerateKlusterletSecret(fakeclient, &operatorsv1.MultiClusterHub{
		Spec: operatorsv1.MultiClusterHubSpec{},
	})
	if err != nil {
		t.Errorf("Failed to generate secret, %v", err)
	}

	expected := &corev1.Secret{}
	err = fakeclient.Get(context.TODO(), types.NamespacedName{Name: KlusterletSecretName, Namespace: "test"}, expected)
	if err != nil {
		t.Errorf("Failed to generate secret, %v", err)
	}
}
