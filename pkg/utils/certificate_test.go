// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package utils

import (
	"os"
	"testing"

	k8scertutil "k8s.io/client-go/util/cert"
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
