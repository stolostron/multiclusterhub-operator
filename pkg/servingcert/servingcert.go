package servingcert

import (
	"bytes"
	"context"
	"crypto/x509"
	"fmt"
	"github.com/openshift/library-go/pkg/certs"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"time"

	"github.com/openshift/library-go/pkg/crypto"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/util/cert"
)

var (
	signingCertValidity = time.Hour * 24 * 365
	targetCertValidity  = time.Hour * 24 * 30
)

type CertGenerator struct {
	Namespace             string
	CaBundleConfigmapName string
	SigningKeySecretName  string
	SignerNamePrefix      string
	Client                kubernetes.Interface
	ConfigmapLister       corev1listers.ConfigMapLister
	SecretLister          corev1listers.SecretLister
	EventRecorder         events.Recorder
}

func (r *CertGenerator) EnsureConfigMapCABundle(ctx context.Context, signingCertKeyPair *crypto.CA) ([]*x509.Certificate, error) {
	// by this point we have current signing cert/key pair.  We now need to make sure that the ca-bundle configmap has this cert and
	// doesn't have any expired certs

	// Check if configmap exists
	originalCABundleConfigMap, err := r.ConfigmapLister.ConfigMaps(r.Namespace).Get(r.CaBundleConfigmapName)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	caBundleConfigMap := originalCABundleConfigMap.DeepCopy()

	if apierrors.IsNotFound(err) {
		// create an empty one
		caBundleConfigMap = &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: r.Namespace, Name: r.CaBundleConfigmapName}}
	}
	if _, err = manageCABundleConfigMap(caBundleConfigMap, signingCertKeyPair.Config.Certs[0]); err != nil {
		return nil, err
	}
	if originalCABundleConfigMap == nil || originalCABundleConfigMap.Data == nil ||
		!equality.Semantic.DeepEqual(originalCABundleConfigMap.Data, caBundleConfigMap.Data) {
		actualCABundleConfigMap, _, err := resourceapply.ApplyConfigMap(ctx, r.Client.CoreV1(), r.EventRecorder, caBundleConfigMap)
		if err != nil {
			return nil, err
		}
		caBundleConfigMap = actualCABundleConfigMap
	}

	caBundle := caBundleConfigMap.Data["ca-bundle.crt"]
	if len(caBundle) == 0 {
		return nil, fmt.Errorf("configmap/%s -n%s missing ca-bundle.crt", caBundleConfigMap.Name, caBundleConfigMap.Namespace)
	}
	certificates, err := cert.ParseCertsPEM([]byte(caBundle))
	if err != nil {
		return nil, err
	}

	return certificates, nil
}

// manageCABundleConfigMap adds the new certificate to the list of cabundles, eliminates duplicates, and prunes the list of expired
// certs to trust as signers
func manageCABundleConfigMap(caBundleConfigMap *corev1.ConfigMap, currentSigner *x509.Certificate) ([]*x509.Certificate, error) {
	if caBundleConfigMap.Data == nil {
		caBundleConfigMap.Data = map[string]string{}
	}

	var certificates []*x509.Certificate
	caBundle := caBundleConfigMap.Data["ca-bundle.crt"]
	if len(caBundle) > 0 {
		var err error
		certificates, err = cert.ParseCertsPEM([]byte(caBundle))
		if err != nil {
			return nil, err
		}
	}
	certificates = append([]*x509.Certificate{currentSigner}, certificates...)
	certificates = crypto.FilterExpiredCerts(certificates...)

	var finalCertificates []*x509.Certificate
	// now check for duplicates. n^2, but super simple
	for i := range certificates {
		found := false
		for j := range finalCertificates {
			if reflect.DeepEqual(certificates[i].Raw, finalCertificates[j].Raw) {
				found = true
				break
			}
		}
		if !found {
			finalCertificates = append(finalCertificates, certificates[i])
		}
	}

	caBytes, err := crypto.EncodeCertificates(finalCertificates...)
	if err != nil {
		return nil, err
	}

	caBundleConfigMap.Data["ca-bundle.crt"] = string(caBytes)

	return finalCertificates, nil
}

func (r *CertGenerator) EnsureSigningCertKeyPair(ctx context.Context) (*crypto.CA, error) {
	originalSigningCertKeyPairSecret, err := r.SecretLister.Secrets(r.Namespace).Get(r.SigningKeySecretName)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	signingCertKeyPairSecret := originalSigningCertKeyPairSecret.DeepCopy()
	if apierrors.IsNotFound(err) {
		// create an empty one
		klog.Infof("not found creating signing cert key")
		signingCertKeyPairSecret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: r.Namespace, Name: r.SigningKeySecretName}}
	}
	signingCertKeyPairSecret.Type = corev1.SecretTypeTLS

	if reason := needNewSigningCertKeyPair(signingCertKeyPairSecret); len(reason) > 0 {
		klog.Infof("reason: %s", reason)
		if err := setSigningCertKeyPairSecret(signingCertKeyPairSecret, r.SignerNamePrefix); err != nil {
			return nil, err
		}

		actualSigningCertKeyPairSecret, _, err := resourceapply.ApplySecret(ctx, r.Client.CoreV1(), r.EventRecorder, signingCertKeyPairSecret)
		if err != nil {
			return nil, err
		}
		signingCertKeyPairSecret = actualSigningCertKeyPairSecret
	}
	// at this point, the secret has the correct signer, so we should read that signer to be able to sign
	signingCertKeyPair, err := crypto.GetCAFromBytes(signingCertKeyPairSecret.Data["tls.crt"], signingCertKeyPairSecret.Data["tls.key"])
	if err != nil {
		return nil, err
	}

	return signingCertKeyPair, nil
}

func needNewSigningCertKeyPair(secret *corev1.Secret) string {
	certData := secret.Data["tls.crt"]
	if len(certData) == 0 {
		return "missing tls.crt"
	}

	certificates, err := cert.ParseCertsPEM(certData)
	if err != nil {
		return "bad certificate"
	}
	if len(certificates) == 0 {
		return "missing certificate"
	}

	certdata := certificates[0]
	if time.Now().After(certdata.NotAfter) {
		return "already expired"
	}

	maxWait := certdata.NotAfter.Sub(certdata.NotBefore) / 5
	latestTime := certdata.NotAfter.Add(-maxWait)
	now := time.Now()
	if now.After(latestTime) {
		return fmt.Sprintf("expired in %6.3f seconds", certdata.NotAfter.Sub(now).Seconds())
	}

	return ""
}

// setSigningCertKeyPairSecret creates a new signing cert/key pair and sets them in the secret
func setSigningCertKeyPairSecret(signingCertKeyPairSecret *corev1.Secret, singerNamePrefix string) error {
	signerName := fmt.Sprintf("%s@%d", singerNamePrefix, time.Now().Unix())
	ca, err := crypto.MakeSelfSignedCAConfigForDuration(signerName, signingCertValidity)
	if err != nil {
		return err
	}

	certBytes := &bytes.Buffer{}
	keyBytes := &bytes.Buffer{}
	if err := ca.WriteCertConfig(certBytes, keyBytes); err != nil {
		return err
	}

	if signingCertKeyPairSecret.Data == nil {
		signingCertKeyPairSecret.Data = map[string][]byte{}
	}
	signingCertKeyPairSecret.Data["tls.crt"] = certBytes.Bytes()
	signingCertKeyPairSecret.Data["tls.key"] = keyBytes.Bytes()

	return nil
}

func (r *CertGenerator) EnsureTargetCertKeyPair(ctx context.Context, signingCertKeyPair *crypto.CA,
	caBundleCerts []*x509.Certificate, name, serviceName string) error {
	originalTargetCertKeyPairSecret, err := r.SecretLister.Secrets(r.Namespace).Get(name)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	targetCertKeyPairSecret := originalTargetCertKeyPairSecret.DeepCopy()
	if apierrors.IsNotFound(err) {
		// create an empty one
		targetCertKeyPairSecret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: r.Namespace, Name: name}}
	}
	targetCertKeyPairSecret.Type = corev1.SecretTypeTLS

	reason := needNewTargetCertKeyPair(targetCertKeyPairSecret, caBundleCerts)
	if len(reason) == 0 {
		return nil
	}
	hostNames := []string{fmt.Sprintf("%s.%s.svc", serviceName, r.Namespace)}
	if err := setTargetCertKeyPairSecret(targetCertKeyPairSecret, hostNames, signingCertKeyPair); err != nil {
		return err
	}

	if _, _, err = resourceapply.ApplySecret(ctx, r.Client.CoreV1(), r.EventRecorder, targetCertKeyPairSecret); err != nil {
		return err
	}

	return err
}

// needNewTargetCertKeyPair returns a reason for creating a new target cert/key pair.
// Return empty if a valid cert/key pair is in place and no need to rotate it yet.
//
// We create a new target cert/key pair if
//  1. no cert/key pair exits
//  2. or the cert expired (then we are also pretty late)
//  3. or we are over the renewal percentage of the validity
//  4. or our old CA is gone from the bundle (then we are pretty late to the renewal party)
func needNewTargetCertKeyPair(secret *corev1.Secret, caBundleCerts []*x509.Certificate) string {
	certData := secret.Data["tls.crt"]
	if len(certData) == 0 {
		return "missing tls.crt"
	}

	certificates, err := cert.ParseCertsPEM(certData)
	if err != nil {
		return "bad certificate"
	}
	if len(certificates) == 0 {
		return "missing certificate"
	}

	cert := certificates[0]
	if time.Now().After(cert.NotAfter) {
		return "already expired"
	}

	maxWait := cert.NotAfter.Sub(cert.NotBefore) / 5
	latestTime := cert.NotAfter.Add(-maxWait)
	now := time.Now()
	if now.After(latestTime) {
		return fmt.Sprintf("expired in %6.3f seconds", cert.NotAfter.Sub(now).Seconds())
	}

	// check the signer common name against all the common names in our ca bundle, so we don't refresh early
	for _, caCert := range caBundleCerts {
		if cert.Issuer.CommonName == caCert.Subject.CommonName {
			return ""
		}
	}

	return fmt.Sprintf("issuer %q not in ca bundle:\n%s", cert.Issuer.CommonName, certs.CertificateBundleToString(caBundleCerts))
}

// setTargetCertKeyPairSecret creates a new cert/key pair and sets them in the secret.
func setTargetCertKeyPairSecret(targetCertKeyPairSecret *corev1.Secret, hostNames []string, signer *crypto.CA) error {
	if targetCertKeyPairSecret.Data == nil {
		targetCertKeyPairSecret.Data = map[string][]byte{}
	}

	// make sure that we don't specify something past our signer
	targetValidity := targetCertValidity
	// TODO: When creating a certificate, crypto.MakeServerCertForDuration accetps validity as input parameter,
	// It calls time.Now() as the current time to calculate NotBefore/NotAfter of new certificate, which might
	// be little later than the returned value of time.Now() call in the line below to get remainingSignerValidity.
	// 2 more seconds is added here as a buffer to make sure NotAfter of the new certificate does not past NotAfter
	// of the signing certificate. We may need a better way to handle this.
	remainingSignerValidity := signer.Config.Certs[0].NotAfter.Sub(time.Now().Add(time.Second * 2))
	if remainingSignerValidity < targetCertValidity {
		targetValidity = remainingSignerValidity
	}
	certKeyPair, err := NewCertificate(signer, targetValidity, hostNames)
	if err != nil {
		return err
	}
	targetCertKeyPairSecret.Data["tls.crt"], targetCertKeyPairSecret.Data["tls.key"], err = certKeyPair.GetPEMBytes()
	if err != nil {
		return err
	}

	return nil
}

func NewCertificate(signer *crypto.CA, validity time.Duration, hostNames []string) (*crypto.TLSCertificateConfig, error) {
	if len(hostNames) == 0 {
		return nil, fmt.Errorf("no hostnames set")
	}
	return signer.MakeServerCertForDuration(sets.Set[string](sets.NewString(hostNames...)), validity)
}

func NewEventRecorder(kubeClient kubernetes.Interface, controllerName, namespace string) events.Recorder {
	controllerRef, err := events.GetControllerReferenceForCurrentPod(context.TODO(), kubeClient, namespace, nil)
	if err != nil {
		klog.Warningf("unable to get owner reference (falling back to namespace): %v", err)
	}

	options := events.RecommendedClusterSingletonCorrelatorOptions()
	return events.NewKubeRecorderWithOptions(kubeClient.CoreV1().Events(namespace), options, controllerName, controllerRef)
}

func (r *CertGenerator) GenerateWebhookCertKey(ctx context.Context, name, serviceName string) error {
	signingCertKeyPair, err := r.EnsureSigningCertKeyPair(ctx)
	if err != nil {
		return err
	}
	cabundleCerts, err := r.EnsureConfigMapCABundle(ctx, signingCertKeyPair)
	if err != nil {
		return err
	}

	return r.EnsureTargetCertKeyPair(ctx, signingCertKeyPair, cabundleCerts, name, serviceName)
}

func (r *CertGenerator) DumpCertSecret(ctx context.Context, name, dir string) error {
	certKeySecret, err := r.Client.CoreV1().Secrets(r.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get secret %q: %v", name, err)
	}

	err = os.MkdirAll(dir, 0700)
	if err != nil {
		return fmt.Errorf("failed to create directory %q: %v", dir, err)
	}
	for key, data := range certKeySecret.Data {
		filename := path.Clean(path.Join(dir, key))
		lastData, err := os.ReadFile(filepath.Clean(filename))
		switch {
		case os.IsNotExist(err):
			// create file
			if err := os.WriteFile(filename, data, 0600); err != nil {
				return fmt.Errorf("unable to write file %q: %w", filename, err)
			}
		case err != nil:
			return fmt.Errorf("unable to write file %q: %w", filename, err)
		case bytes.Equal(lastData, data):
			// skip file without any change
			continue
		default:
			// update file
			if err := os.WriteFile(path.Clean(filename), data, 0600); err != nil {
				return fmt.Errorf("unable to write file %q: %w", filename, err)
			}
		}
	}

	return nil
}

func (r *CertGenerator) InjectCABundle(ctx context.Context, configmaps, validatingWebhooks, mutatingWebhooks []string) error {
	var errs []error
	caBundle := []byte("placeholder")
	caConfigmap, err := r.Client.CoreV1().ConfigMaps(r.Namespace).Get(ctx, r.CaBundleConfigmapName, metav1.GetOptions{})
	switch {
	case errors.IsNotFound(err):
		return nil
	case err != nil:
		return err
	default:
		if cb := caConfigmap.Data["ca-bundle.crt"]; len(cb) > 0 {
			caBundle = []byte(cb)
		}
	}

	for _, configmap := range configmaps {
		targetConfigmap, err := r.Client.CoreV1().ConfigMaps(r.Namespace).Get(ctx, configmap, metav1.GetOptions{})
		switch {
		case errors.IsNotFound(err):
			// do nothing
		case err != nil:
			errs = append(errs, err)
		default:
			copyTargetConfigmap := targetConfigmap.DeepCopy()
			if copyTargetConfigmap.Data == nil {
				copyTargetConfigmap.Data = map[string]string{"ca-bundle.crt": string(caBundle)}
			} else if cb := copyTargetConfigmap.Data["ca-bundle.crt"]; len(cb) == 0 || cb != string(caBundle) {
				copyTargetConfigmap.Data["ca-bundle.crt"] = string(caBundle)
				_, err = r.Client.CoreV1().ConfigMaps(r.Namespace).Update(ctx, copyTargetConfigmap, metav1.UpdateOptions{})
				if err != nil {
					errs = append(errs, err)
				}
			}
		}
	}

	for _, webhookName := range validatingWebhooks {
		validatingWebhook, err := r.Client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(ctx, webhookName, metav1.GetOptions{})
		switch {
		case errors.IsNotFound(err):
			// do nothing
		case err != nil:
			errs = append(errs, err)
		default:
			copyValidatingWebhook := validatingWebhook.DeepCopy()
			updated := false
			for key, webhook := range copyValidatingWebhook.Webhooks {
				if !bytes.Equal(webhook.ClientConfig.CABundle, caBundle) {
					copyValidatingWebhook.Webhooks[key].ClientConfig.CABundle = caBundle
					updated = true
				}
			}
			if updated {
				_, err = r.Client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Update(ctx, copyValidatingWebhook, metav1.UpdateOptions{})
				if err != nil {
					errs = append(errs, err)
				}
			}
		}
	}
	for _, webhookName := range mutatingWebhooks {
		mutatingWebhook, err := r.Client.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, webhookName, metav1.GetOptions{})
		switch {
		case errors.IsNotFound(err):
		// do nothing
		case err != nil:
			errs = append(errs, err)
		default:
			copyMutatingWebhook := mutatingWebhook.DeepCopy()
			updated := false
			for key, webhook := range copyMutatingWebhook.Webhooks {
				if !bytes.Equal(webhook.ClientConfig.CABundle, caBundle) {
					copyMutatingWebhook.Webhooks[key].ClientConfig.CABundle = caBundle
					updated = true
				}
			}
			if updated {
				_, err = r.Client.AdmissionregistrationV1().MutatingWebhookConfigurations().Update(ctx, copyMutatingWebhook, metav1.UpdateOptions{})
				if err != nil {
					errs = append(errs, err)
				}
			}
		}
	}

	return utilerrors.NewAggregate(errs)
}
