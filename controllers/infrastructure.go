// Copyright Contributors to the Open Cluster Management project

/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"os"

	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"

	configv1 "github.com/openshift/api/config/v1"
	pkgerrors "github.com/pkg/errors"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *MultiClusterHubReconciler) ensureOpenShiftNamespaceLabel(ctx context.Context, m *operatorv1.MultiClusterHub) (
	ctrl.Result, error,
) {
	existingNs := &corev1.Namespace{}

	err := r.Client.Get(ctx, types.NamespacedName{Name: m.GetNamespace()}, existingNs)
	if err != nil || errors.IsNotFound(err) {
		log.Error(err, fmt.Sprintf("Failed to find namespace for MultiClusterHub: %s", m.GetNamespace()))
		return ctrl.Result{}, err
	}

	if existingNs.Labels == nil || len(existingNs.Labels) == 0 {
		existingNs.Labels = make(map[string]string)
	}

	if _, ok := existingNs.Labels[utils.OpenShiftClusterMonitoringLabel]; !ok {
		r.Log.Info(fmt.Sprintf("Adding label: %s to namespace: %s", utils.OpenShiftClusterMonitoringLabel,
			m.GetNamespace()))
		existingNs.Labels[utils.OpenShiftClusterMonitoringLabel] = "true"

		err = r.Client.Update(ctx, existingNs)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to update namespace for MultiClusterHub: %s with the label: %s",
				m.GetNamespace(), utils.OpenShiftClusterMonitoringLabel))
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// createCAconfigmap creates a configmap that will be injected with the
// trusted CA bundle for use with the OCP cluster wide proxy
func (r *MultiClusterHubReconciler) createTrustBundleConfigmap(ctx context.Context, mch *operatorv1.MultiClusterHub) (
	ctrl.Result, error,
) {
	// Get Trusted Bundle configmap name
	trustBundleName := defaultTrustBundleName
	trustBundleNamespace := mch.Namespace
	if name, ok := os.LookupEnv(trustBundleNameEnvVar); ok && name != "" {
		trustBundleName = name
	}
	namespacedName := types.NamespacedName{
		Name:      trustBundleName,
		Namespace: trustBundleNamespace,
	}
	log.Info(fmt.Sprintf("using trust bundle configmap %s/%s", trustBundleNamespace, trustBundleName))

	// Check if configmap exists
	cm := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, namespacedName, cm)
	if err != nil && !errors.IsNotFound(err) {
		// Unknown error. Requeue
		msg := fmt.Sprintf("error while getting trust bundle configmap %s/%s", trustBundleNamespace, trustBundleName)
		log.Error(err, msg)
		return ctrl.Result{}, err
	} else if err == nil {
		// configmap exists
		return ctrl.Result{}, nil
	}

	// Create configmap
	cm = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      trustBundleName,
			Namespace: trustBundleNamespace,
			Labels: map[string]string{
				"config.openshift.io/inject-trusted-cabundle": "true",
			},
		},
	}
	err = ctrl.SetControllerReference(mch, cm, r.Scheme)
	if err != nil {
		return ctrl.Result{}, pkgerrors.Wrapf(
			err, "Error setting controller reference on trust bundle configmap %s",
			trustBundleName,
		)
	}
	err = r.Client.Create(ctx, cm)
	if err != nil {
		// Error creating configmap
		log.Info(fmt.Sprintf("error creating trust bundle configmap %s: %s", trustBundleName, err))
		return ctrl.Result{}, err
	}
	// Configmap created successfully
	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) createMetricsService(ctx context.Context, m *operatorv1.MultiClusterHub) (
	ctrl.Result, error,
) {
	const Port = 8383

	sName := utils.MCHOperatorMetricsServiceName
	sNamespace := m.GetNamespace()

	namespacedName := types.NamespacedName{
		Name:      sName,
		Namespace: sNamespace,
	}

	// Check if service exists
	if err := r.Client.Get(ctx, namespacedName, &corev1.Service{}); err != nil {
		if !errors.IsNotFound(err) {
			// Unknown error. Requeue
			log.Error(err, fmt.Sprintf("error while getting multiclusterhub metrics service: %s/%s", sNamespace, sName))
			return ctrl.Result{}, err
		}

		// Create metrics service
		s := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sName,
				Namespace: sNamespace,
				Labels: map[string]string{
					"name": operatorv1.MCH,
				},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:       "metrics",
						Port:       int32(Port),
						Protocol:   "TCP",
						TargetPort: intstr.FromInt(Port),
					},
				},
				Selector: map[string]string{
					"name": operatorv1.MCH,
				},
			},
		}

		if err = ctrl.SetControllerReference(m, s, r.Scheme); err != nil {
			return ctrl.Result{}, pkgerrors.Wrapf(
				err, "error setting controller reference on metrics service: %s", sName,
			)
		}

		if err = r.Client.Create(ctx, s); err != nil {
			// Error creating metrics service
			log.Error(err, fmt.Sprintf("error creating multiclusterhub metrics service: %s", sName))
			return ctrl.Result{}, err
		}

		log.Info(fmt.Sprintf("Created multiclusterhub metrics service: %s", sName))
	}

	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) createMetricsServiceMonitor(ctx context.Context, m *operatorv1.MultiClusterHub) (
	ctrl.Result, error,
) {
	smName := utils.MCHOperatorMetricsServiceMonitorName
	smNamespace := m.GetNamespace()

	namespacedName := types.NamespacedName{
		Name:      smName,
		Namespace: smNamespace,
	}

	// Check if service exists
	if err := r.Client.Get(ctx, namespacedName, &promv1.ServiceMonitor{}); err != nil {
		if !errors.IsNotFound(err) {
			// Unknown error. Requeue
			log.Error(err, fmt.Sprintf("error while getting multiclusterhub metrics service: %s/%s", smNamespace, smName))
			return ctrl.Result{}, err
		}

		// Create metrics service
		sm := &promv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      smName,
				Namespace: smNamespace,
				Labels: map[string]string{
					"name": operatorv1.MCH,
				},
			},
			Spec: promv1.ServiceMonitorSpec{
				Endpoints: []promv1.Endpoint{
					{
						BearerTokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
						BearerTokenSecret: &corev1.SecretKeySelector{
							Key: "",
						},
						Port: "metrics",
					},
				},
				NamespaceSelector: promv1.NamespaceSelector{
					MatchNames: []string{
						m.GetNamespace(),
					},
				},
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"name": operatorv1.MCH,
					},
				},
			},
		}

		if err = ctrl.SetControllerReference(m, sm, r.Scheme); err != nil {
			return ctrl.Result{}, pkgerrors.Wrapf(
				err, "error setting controller reference on multiclusterhub metrics servicemonitor: %s", smName)
		}

		if err = r.Client.Create(ctx, sm); err != nil {
			// Error creating metrics servicemonitor
			log.Error(err, fmt.Sprintf("error creating metrics servicemonitor: %s", smName))
			return ctrl.Result{}, err
		}

		logf.Log.Info(fmt.Sprintf("Created multiclusterhub metrics servicemonitor: %s", smName))
	}

	return ctrl.Result{}, nil
}

// ingressDomain is discovered from Openshift cluster configuration resources
func (r *MultiClusterHubReconciler) ingressDomain(
	ctx context.Context,
	m *operatorv1.MultiClusterHub,
) (ctrl.Result, error) {
	ingress := &configv1.Ingress{}
	err := r.Client.Get(ctx, types.NamespacedName{
		Name: "cluster",
	}, ingress)
	if err != nil {
		r.Log.Error(err, "Failed to get Ingress")

		return ctrl.Result{}, err
	}

	domain := ingress.Spec.Domain
	if r.CacheSpec.IngressDomain != domain {
		if r.CacheSpec.IngressDomain != "" {
			r.Log.Info("Detected ingress domain mismatch. Current value: " + r.CacheSpec.IngressDomain)
		}
		r.Log.Info("Setting ingress domain to: " + domain)
		r.CacheSpec.IngressDomain = domain
		// Set OCP version as env var, so that charts can render this value
		err = os.Setenv("INGRESS_DOMAIN", domain)
		if err != nil {
			r.Log.Error(err, "Failed to set INGRESS_DOMAIN environment variable")

			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// ingressDomain is discovered from Openshift cluster configuration resources
func (r *MultiClusterHubReconciler) openShiftApiUrl(ctx context.Context, m *operatorv1.MultiClusterHub) (
	ctrl.Result, error) {
	infrastructure := &configv1.Infrastructure{}
	err := r.Client.Get(ctx, types.NamespacedName{
		Name: "cluster",
	}, infrastructure)
	if err != nil {
		r.Log.Error(err, "Failed to get Infrastructure")

		return ctrl.Result{}, err
	}

	url := infrastructure.Status.APIServerURL
	err = os.Setenv("API_URL", url)
	if err != nil {
		r.Log.Error(err, "Failed to set API_URL environment variable")

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
