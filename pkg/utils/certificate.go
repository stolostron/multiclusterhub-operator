// licensed Materials - Property of IBM
// 5737-E67
// (C) Copyright IBM Corporation 2016, 2019 All Rights Reserved
// US Government Users Restricted Rights - Use, duplication or disclosure restricted by GSA ADP Schedule Contract with IBM Corp.

package utils

import (
	"bytes"
	"context"
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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
)

type certificate struct {
	Cert string
	Key  string
}

const (
	WebhookServiceName   = "multicloudhub-operator-webhook"
	APIServerSecretName  = "mcm-apiserver-self-signed-secrets"
	KlusterletSecretName = "mcm-klusterlet-self-signed-secrets"

	podNamespaceEnvVar = "POD_NAMESPACE"
	apiserviceName     = "mcm-apiserver"
	rsaKeySize         = 2048
	duration365d       = time.Hour * 24 * 365
)

func GenerateWebhookCerts(certDir string) (string, []byte, error) {
	namespace, err := findNamespace()
	if err != nil {
		return "", nil, err
	}

	ca, err := GenerateSelfSignedCACert("multicloudhub-webhook")
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

func GenerateAPIServerSecret(client runtimeclient.Client, multiCloudHub *operatorsv1alpha1.MultiCloudHub) error {
	name := multiCloudHub.Spec.Apiserver.ApiserverSecret
	if name != APIServerSecretName {
		return nil
	}
	namespace, err := findNamespace()
	if err != nil {
		return err
	}
	err = client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, &corev1.Secret{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			ca, err := GenerateSelfSignedCACert("multicloudhub-api")
			if err != nil {
				return err
			}

			alternateDNS := []string{
				fmt.Sprintf("%s.%s", apiserviceName, namespace),
				fmt.Sprintf("%s.%s.svc", apiserviceName, namespace),
			}
			cert, err := GenerateSignedCert(apiserviceName, alternateDNS, ca)
			if err != nil {
				return err
			}
			return client.Create(context.TODO(), &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					OwnerReferences: []metav1.OwnerReference{
						*metav1.NewControllerRef(multiCloudHub, multiCloudHub.GetObjectKind().GroupVersionKind())},
				},
				Data: map[string][]byte{
					"ca.crt":  []byte(ca.Cert),
					"tls.crt": []byte(cert.Cert),
					"tls.key": []byte(cert.Key),
				},
			})
		}
		return err
	}
	return nil
}

func GenerateKlusterletSecret(client runtimeclient.Client, multiCloudHub *operatorsv1alpha1.MultiCloudHub) error {
	name := multiCloudHub.Spec.Apiserver.KlusterletSecret
	if name != KlusterletSecretName {
		return nil
	}
	namespace, err := findNamespace()
	if err != nil {
		return err
	}
	err = client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, &corev1.Secret{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			ca, err := GenerateSelfSignedCACert("multicloudhub-klusterlet")
			if err != nil {
				return err
			}
			cert, err := GenerateSignedCert("multicloudhub-klusterlet", []string{}, ca)
			if err != nil {
				return err
			}
			return client.Create(context.TODO(), &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					OwnerReferences: []metav1.OwnerReference{
						*metav1.NewControllerRef(multiCloudHub, multiCloudHub.GetObjectKind().GroupVersionKind())},
				},
				Data: map[string][]byte{
					"ca.crt":  []byte(ca.Cert),
					"tls.crt": []byte(cert.Cert),
					"tls.key": []byte(cert.Key),
				},
			})
		}
		return err
	}
	return nil
}

func GenerateSelfSignedCACert(cn string) (certificate, error) {
	ca := certificate{}

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

func GenerateSignedCert(cn string, alternateDNS []string, ca certificate) (certificate, error) {
	cert := certificate{}

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

func findNamespace() (string, error) {
	ns, found := os.LookupEnv(podNamespaceEnvVar)
	if !found {
		return "", fmt.Errorf("%s envvar is not set", podNamespaceEnvVar)
	}
	return ns, nil
}
