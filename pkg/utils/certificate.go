// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package utils

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Certificate ...
type Certificate struct {
	Cert string
	Key  string
}

// GenerateWebhookCerts ...
func GenerateWebhookCerts(certDir string) (string, []byte, error) {
	namespace, err := FindNamespace()
	if err != nil {
		return "", nil, err
	}

	ca, err := GenerateSelfSignedCACert("multiclusterhub-webhook")
	if err != nil {
		return "", nil, err
	}

	alternateDNS := []string{
		fmt.Sprintf("%s.%s", WebhookServiceName, namespace),
		fmt.Sprintf("%s.%s.svc", WebhookServiceName, namespace),
	}
	cert, err := GenerateSignedCert(WebhookServiceName, alternateDNS, ca)
	if err != nil {
		return "", nil, err
	}

	if err := os.MkdirAll(certDir, os.ModePerm); err != nil {
		return "", nil, err
	}

	if err := ioutil.WriteFile(filepath.Join(certDir, "tls.crt"), []byte(cert.Cert), os.FileMode(0644)); err != nil {
		return "", nil, err
	}
	if err := ioutil.WriteFile(filepath.Join(certDir, "tls.key"), []byte(cert.Key), os.FileMode(0644)); err != nil {
		return "", nil, err
	}

	return namespace, []byte(ca.Cert), nil
}

// GenerateSelfSignedCACert ...
func GenerateSelfSignedCACert(cn string) (Certificate, error) {
	ca := Certificate{}

	template, err := generateBaseTemplateCert(cn, []string{})
	if err != nil {
		return ca, err
	}
	// Override KeyUsage and IsCA
	template.KeyUsage = x509.KeyUsageKeyEncipherment |
		x509.KeyUsageDigitalSignature |
		x509.KeyUsageCertSign
	template.IsCA = true

	priv, err := rsa.GenerateKey(rand.Reader, rsaKeySize)
	if err != nil {
		return ca, fmt.Errorf("error generating rsa key: %s", err)
	}

	ca.Cert, ca.Key, err = getCertAndKey(template, priv, template, priv)

	return ca, err
}

// GenerateSignedCert ...
func GenerateSignedCert(cn string, alternateDNS []string, ca Certificate) (Certificate, error) {
	cert := Certificate{}

	decodedSignerCert, _ := pem.Decode([]byte(ca.Cert))
	if decodedSignerCert == nil {
		return cert, errors.New("unable to decode certificate")
	}
	signerCert, err := x509.ParseCertificate(decodedSignerCert.Bytes)
	if err != nil {
		return cert, fmt.Errorf(
			"error parsing certificate: decodedSignerCert.Bytes: %s",
			err,
		)
	}
	decodedSignerKey, _ := pem.Decode([]byte(ca.Key))
	if decodedSignerKey == nil {
		return cert, errors.New("unable to decode key")
	}
	signerKey, err := x509.ParsePKCS1PrivateKey(decodedSignerKey.Bytes)
	if err != nil {
		return cert, fmt.Errorf(
			"error parsing prive key: decodedSignerKey.Bytes: %s",
			err,
		)
	}

	template, err := generateBaseTemplateCert(cn, alternateDNS)
	if err != nil {
		return cert, err
	}

	priv, err := rsa.GenerateKey(rand.Reader, rsaKeySize)
	if err != nil {
		return cert, fmt.Errorf("error generating rsa key: %s", err)
	}

	cert.Cert, cert.Key, err = getCertAndKey(template, priv, signerCert, signerKey)

	return cert, err
}

func generateBaseTemplateCert(cn string, alternateDNS []string) (*x509.Certificate, error) {
	serialNumberUpperBound := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberUpperBound)
	if err != nil {
		return nil, err
	}
	return &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: cn,
		},
		IPAddresses: []net.IP{},
		DNSNames:    alternateDNS,
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(duration365d),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
		BasicConstraintsValid: true,
	}, nil
}

func getCertAndKey(
	template *x509.Certificate,
	signeeKey *rsa.PrivateKey,
	parent *x509.Certificate,
	signingKey *rsa.PrivateKey,
) (string, string, error) {
	derBytes, err := x509.CreateCertificate(
		rand.Reader,
		template,
		parent,
		&signeeKey.PublicKey,
		signingKey,
	)
	if err != nil {
		return "", "", fmt.Errorf("error creating certificate: %s", err)
	}

	certBuffer := bytes.Buffer{}
	if err := pem.Encode(
		&certBuffer,
		&pem.Block{Type: "CERTIFICATE", Bytes: derBytes},
	); err != nil {
		return "", "", fmt.Errorf("error pem-encoding certificate: %s", err)
	}

	keyBuffer := bytes.Buffer{}
	if err := pem.Encode(
		&keyBuffer,
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(signeeKey),
		},
	); err != nil {
		return "", "", fmt.Errorf("error pem-encoding key: %s", err)
	}

	return certBuffer.String(), keyBuffer.String(), nil
}

func FindNamespace() (string, error) {
	ns, found := os.LookupEnv(podNamespaceEnvVar)
	if !found {
		return "", fmt.Errorf("%s envvar is not set", podNamespaceEnvVar)
	}
	return ns, nil
}
