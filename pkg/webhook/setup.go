// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package webhook

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	admissionregistration "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	clustermanager "open-cluster-management.io/api/operator/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
)

var log = logf.Log.WithName("multiclusterhub_webhook")

const (
	resourceName          = "multiclusterhubs"
	clusterManagerName    = "clustermanagers"
	operatorName          = "multiclusterhub-operator"
	validatingWebhookName = "multiclusterhub.validating-webhook.open-cluster-management.io"
	validatingCfgName     = "multiclusterhub-operator-validating-webhook"
	webhookSecretName     = "multiclusterhub-operator-webhook"
	crdName               = "multiclusterhubs.operator.open-cluster-management.io"
)

var (
	certDir        = filepath.Join("/tmp", "webhookcert")
	validatingPath = "/validate-v1-multiclusterhub"
)

func Setup(mgr manager.Manager) error {
	ns, err := utils.FindNamespace()
	if err != nil {
		return err
	}

	go func() {
		log.Info("calling createOrUpdateWebhookService")
		createOrUpdateWebhookService(mgr.GetClient(), ns)
		log.Info("calling updateCertDir")
		lastResourceVersion := updateCertDir(mgr.GetClient(), ns, "")
		log.Info("calling registerWebhook")
		registerWebhook(mgr, ns)
		log.Info("calling createOrUpdateValidatingWebhook")
		createOrUpdateValidatingWebhook(mgr.GetClient(), ns, validatingPath, certDir)

		// Check for changes to the webhook secret every minute
		ticker := time.NewTicker(time.Minute)
		for {
			select {
			case <-ticker.C:
				lastResourceVersion = updateCertDir(mgr.GetClient(), ns, lastResourceVersion)
			}
		}
	}()

	return nil
}

// registerWebhook adds the webhook server to the manager
func registerWebhook(mgr manager.Manager, ns string) {
	hookServer := &webhook.Server{
		Port:    8443,
		CertDir: certDir,
	}

	log.Info("Add the webhook server.")
	if err := mgr.Add(hookServer); err != nil {
		// Failure to add the webhook should cause the container to fail
		log.Error(err, "failed to add the webhook server to the manager")
		os.Exit(1)
	}

	log.Info("Registering webhooks to the webhook server.")
	hookServer.Register(validatingPath, &webhook.Admission{Handler: &multiClusterHubValidator{}})
}

// createOrUpdateWebhookService creates or updates a service with the Openshift self-serving-cert
func createOrUpdateWebhookService(c client.Client, namespace string) {
	service := &corev1.Service{}
	key := types.NamespacedName{Name: utils.WebhookServiceName, Namespace: namespace}
	for {
		if err := c.Get(context.TODO(), key, service); err != nil {
			if errors.IsNotFound(err) {

				service := newWebhookService(namespace)
				setOwnerReferences(c, namespace, service)
				if err := c.Create(context.TODO(), service); err != nil {
					log.Error(err, fmt.Sprintf("Failed to create %s/%s service", namespace, utils.WebhookServiceName))
					return
				}
				log.Info(fmt.Sprintf("Create %s/%s service", namespace, utils.WebhookServiceName))

				return
			}
			switch err.(type) {
			case *cache.ErrCacheNotStarted:
				time.Sleep(time.Second)
				continue
			default:
				log.Error(err, fmt.Sprintf("Failed to get %s/%s service", namespace, utils.WebhookServiceName))
				return
			}
		}
		metav1.SetMetaDataAnnotation(&service.ObjectMeta, "service.beta.openshift.io/serving-cert-secret-name", "multiclusterhub-operator-webhook")
		if err := c.Update(context.TODO(), service); err != nil {
			log.Error(err, fmt.Sprintf("Failed to update service %s", utils.WebhookServiceName))
			return
		}
		return
	}
}

func createOrUpdateValidatingWebhook(c client.Client, namespace, path string, certDir string) {
	validator := &admissionregistration.ValidatingWebhookConfiguration{}
	key := types.NamespacedName{Name: validatingCfgName}
	for {
		if err := c.Get(context.TODO(), key, validator); err != nil {
			if errors.IsNotFound(err) {
				cfg := newValidatingWebhookCfg(namespace, path)
				setOwnerReferences(c, namespace, cfg)
				if err := c.Create(context.TODO(), cfg); err != nil {
					log.Error(err, fmt.Sprintf("Failed to create validating webhook %s", validatingCfgName))
					return
				}
				log.Info(fmt.Sprintf("Create validating webhook %s", validatingCfgName))
				return
			}
			switch err.(type) {
			case *cache.ErrCacheNotStarted:
				time.Sleep(time.Second)
				continue
			default:
				log.Error(err, fmt.Sprintf("Failed to get validating webhook %s", validatingCfgName))
				return
			}
		}

		metav1.SetMetaDataAnnotation(&validator.ObjectMeta, "service.beta.openshift.io/inject-cabundle", "true")
		if err := c.Update(context.TODO(), validator); err != nil {
			log.Error(err, fmt.Sprintf("Failed to update validating webhook %s", validatingCfgName))
			return
		}
		log.Info(fmt.Sprintf("Update validating webhook %s", validatingCfgName))
		return
	}
}

func setOwnerReferences(c client.Client, namespace string, obj metav1.Object) {
	key := types.NamespacedName{Name: crdName}
	owner := &apixv1.CustomResourceDefinition{}
	if err := c.Get(context.TODO(), key, owner); err != nil {
		log.Error(err, fmt.Sprintf("Failed to set owner references for %s", obj.GetName()))
		return
	}

	obj.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion: owner.APIVersion,
			Kind:       owner.Kind,
			Name:       owner.Name,
			UID:        owner.UID,
		},
	})
}

// updateCertDir reads the webhook secret and saves the cert info to the cert directory if the resourceVersion is
// different from the one provided. It will retry on error until successful and returns the last resource version written.
func updateCertDir(c client.Client, namespace, lastResourceVersion string) string {
	secret := &corev1.Secret{}
	nn := types.NamespacedName{Name: webhookSecretName, Namespace: namespace}

	for {
		if err := c.Get(context.TODO(), nn, secret); err != nil {
			switch err.(type) {
			case *cache.ErrCacheNotStarted:
				time.Sleep(time.Second)
				continue
			default:
				time.Sleep(time.Second)
				log.Error(err, fmt.Sprintf("Fails to return secret"))
				continue
			}
		}

		if secret.ResourceVersion == lastResourceVersion {
			return lastResourceVersion
		}
		log.Info(fmt.Sprintf("resourceVersion of secret %s has changed. Updating certs.", webhookSecretName))

		if err := os.MkdirAll(certDir, os.ModePerm); err != nil {
			log.Error(err, fmt.Sprintf("trouble creating directory"))
			return ""
		}
		if err := ioutil.WriteFile(filepath.Join(certDir, "tls.crt"), secret.Data["tls.crt"], os.FileMode(0644)); err != nil {
			log.Error(err, fmt.Sprintf("trouble writing crt"))
			return ""
		}
		if err := ioutil.WriteFile(filepath.Join(certDir, "tls.key"), secret.Data["tls.key"], os.FileMode(0644)); err != nil {
			log.Error(err, fmt.Sprintf("trouble writing key"))
			return ""
		}
		return secret.ResourceVersion
	}
}

func newWebhookService(namespace string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        utils.WebhookServiceName,
			Namespace:   namespace,
			Annotations: map[string]string{"service.beta.openshift.io/serving-cert-secret-name": "multiclusterhub-operator-webhook"},
		},
		Spec: corev1.ServiceSpec{
			Ports:    []corev1.ServicePort{{Port: 443, TargetPort: intstr.FromInt(8443)}},
			Selector: map[string]string{"name": operatorName},
		},
	}
}

func newValidatingWebhookCfg(namespace, path string) *admissionregistration.ValidatingWebhookConfiguration {
	sideEffect := admissionregistration.SideEffectClassNone

	return &admissionregistration.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:        validatingCfgName,
			Annotations: map[string]string{"service.beta.openshift.io/inject-cabundle": "true"},
		},
		Webhooks: []admissionregistration.ValidatingWebhook{{
			AdmissionReviewVersions: []string{
				"v1",
				"v1beta1",
			},
			ClientConfig: admissionregistration.WebhookClientConfig{
				Service: &admissionregistration.ServiceReference{
					Name:      utils.WebhookServiceName,
					Namespace: namespace,
					Path:      &path,
				},
			},
			Name: validatingWebhookName,
			Rules: []admissionregistration.RuleWithOperations{{
				Rule: admissionregistration.Rule{
					APIGroups:   []string{operatorsv1.GroupVersion.Group},
					APIVersions: []string{operatorsv1.GroupVersion.Version},
					Resources:   []string{resourceName},
				},
				Operations: []admissionregistration.OperationType{
					admissionregistration.Create,
					admissionregistration.Update,
					admissionregistration.Delete,
				},
			}, {
				Rule: admissionregistration.Rule{
					APIGroups:   []string{clustermanager.SchemeGroupVersion.Group},
					APIVersions: []string{clustermanager.SchemeGroupVersion.Version},
					Resources:   []string{clusterManagerName},
				},
				Operations: []admissionregistration.OperationType{
					admissionregistration.Delete,
				},
			}},
			SideEffects: &sideEffect,
		}},
	}
}
