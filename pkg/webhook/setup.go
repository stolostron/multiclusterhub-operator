package webhook

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	admissionregistration "k8s.io/api/admissionregistration/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
)

var log = logf.Log.WithName("multiclusterhub_webhook")

const (
	resourceName          = "multiclusterhubs"
	operatorName          = "multiclusterhub-operator"
	mutatingWebhookName   = "multiclusterhub.mutating-webhook.open-cluster-management.io"
	mutatingCfgName       = "multiclusterhub-operator-mutating-webhook"
	validatingWebhookName = "multiclusterhub.validating-webhook.open-cluster-management.io"
	validatingCfgName     = "multiclusterhub-operator-validating-webhook"
)

func Setup(mgr manager.Manager) error {
	certDir := filepath.Join("/tmp", "webhookcert")
	ns, ca, err := utils.GenerateWebhookCerts(certDir)
	if err != nil {
		return err
	}

	hookServer := &webhook.Server{
		Port:    8443,
		CertDir: certDir,
	}

	log.Info("Add the webhook server.")
	if err := mgr.Add(hookServer); err != nil {
		return err
	}

	log.Info("Registering webhooks to the webhook server.")
	mutatingPath := "/mutate-v1alpha1-multiclusterhub"
	hookServer.Register(mutatingPath, &webhook.Admission{Handler: &multiClusterHubMutator{}})
	validatingPath := "/validate-v1alpha1-multiclusterhub"
	hookServer.Register(validatingPath, &webhook.Admission{Handler: &multiClusterHubValidator{}})

	go createWebhookService(mgr.GetClient(), ns)
	go createOrUpdateMutatingWebhook(mgr.GetClient(), ns, mutatingPath, ca)
	go createOrUpdateValiatingWebhook(mgr.GetClient(), ns, validatingPath, ca)

	return nil
}

func createWebhookService(c client.Client, namespace string) {
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
		log.Info(fmt.Sprintf("%s/%s service is found", namespace, utils.WebhookServiceName))
		return
	}
}

func createOrUpdateMutatingWebhook(c client.Client, namespace, path string, ca []byte) {
	mutator := &admissionregistration.MutatingWebhookConfiguration{}
	key := types.NamespacedName{Name: mutatingCfgName}
	for {
		if err := c.Get(context.TODO(), key, mutator); err != nil {
			if errors.IsNotFound(err) {
				cfg := newMutatingWebhookCfg(namespace, path, ca)
				setOwnerReferences(c, namespace, cfg)
				if err := c.Create(context.TODO(), cfg); err != nil {
					log.Error(err, fmt.Sprintf("Failed to create mutating webhook %s", mutatingCfgName))
					return
				}
				log.Info(fmt.Sprintf("Create mutating webhook %s", mutatingCfgName))
				return
			}
			switch err.(type) {
			case *cache.ErrCacheNotStarted:
				time.Sleep(time.Second)
				continue
			default:
				log.Error(err, fmt.Sprintf("Failed to get mutating webhook %s", mutatingCfgName))
				return
			}
		}

		mutator.Webhooks[0].ClientConfig.Service.Namespace = namespace
		mutator.Webhooks[0].ClientConfig.CABundle = ca
		if err := c.Update(context.TODO(), mutator); err != nil {
			log.Error(err, fmt.Sprintf("Failed to update mutating webhook %s", mutatingCfgName))
			return
		}
		log.Info(fmt.Sprintf("Update mutating webhook %s", mutatingCfgName))
		return
	}
}

func createOrUpdateValiatingWebhook(c client.Client, namespace, path string, ca []byte) {
	validator := &admissionregistration.ValidatingWebhookConfiguration{}
	key := types.NamespacedName{Name: validatingCfgName}
	for {
		if err := c.Get(context.TODO(), key, validator); err != nil {
			if errors.IsNotFound(err) {
				cfg := newValidatingWebhookCfg(namespace, path, ca)
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

		validator.Webhooks[0].ClientConfig.Service.Namespace = namespace
		validator.Webhooks[0].ClientConfig.CABundle = ca
		if err := c.Update(context.TODO(), validator); err != nil {
			log.Error(err, fmt.Sprintf("Failed to update validating webhook %s", validatingCfgName))
			return
		}
		log.Info(fmt.Sprintf("Update validating webhook %s", validatingCfgName))
		return
	}
}

func setOwnerReferences(c client.Client, namespace string, obj metav1.Object) {
	key := types.NamespacedName{Name: operatorName, Namespace: namespace}
	owner := &appsv1.Deployment{}
	if err := c.Get(context.TODO(), key, owner); err != nil {
		log.Error(err, fmt.Sprintf("Failed to set ownew references for %s", obj.GetName()))
		return
	}

	obj.SetOwnerReferences([]metav1.OwnerReference{
		*metav1.NewControllerRef(owner, owner.GetObjectKind().GroupVersionKind())})
}

func newWebhookService(namespace string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.WebhookServiceName,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports:    []corev1.ServicePort{{Port: 443, TargetPort: intstr.FromInt(8443)}},
			Selector: map[string]string{"name": operatorName},
		},
	}
}

func newMutatingWebhookCfg(namespace, path string, ca []byte) *admissionregistration.MutatingWebhookConfiguration {
	return &admissionregistration.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: mutatingCfgName,
		},
		Webhooks: []admissionregistration.MutatingWebhook{{
			Name: mutatingWebhookName,
			Rules: []admissionregistration.RuleWithOperations{{
				Rule: admissionregistration.Rule{
					APIGroups:   []string{operatorsv1alpha1.SchemeGroupVersion.Group},
					APIVersions: []string{operatorsv1alpha1.SchemeGroupVersion.Version},
					Resources:   []string{resourceName},
				},
				Operations: []admissionregistration.OperationType{
					admissionregistration.Create,
					admissionregistration.Update,
				},
			}},
			ClientConfig: admissionregistration.WebhookClientConfig{
				Service: &admissionregistration.ServiceReference{
					Name:      utils.WebhookServiceName,
					Namespace: namespace,
					Path:      &path,
				},
				CABundle: ca,
			},
		}},
	}
}

func newValidatingWebhookCfg(namespace, path string, ca []byte) *admissionregistration.ValidatingWebhookConfiguration {
	return &admissionregistration.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: validatingCfgName,
		},
		Webhooks: []admissionregistration.ValidatingWebhook{{
			Name: validatingWebhookName,
			Rules: []admissionregistration.RuleWithOperations{{
				Rule: admissionregistration.Rule{
					APIGroups:   []string{operatorsv1alpha1.SchemeGroupVersion.Group},
					APIVersions: []string{operatorsv1alpha1.SchemeGroupVersion.Version},
					Resources:   []string{resourceName},
				},
				Operations: []admissionregistration.OperationType{
					admissionregistration.Create,
				},
			}},
			ClientConfig: admissionregistration.WebhookClientConfig{
				Service: &admissionregistration.ServiceReference{
					Name:      utils.WebhookServiceName,
					Namespace: namespace,
					Path:      &path,
				},
				CABundle: ca,
			},
		}},
	}
}
