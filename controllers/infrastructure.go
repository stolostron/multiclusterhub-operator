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

// ensureOpenShiftNamespaceLabel adds the openshift.io/cluster-monitoring label to the MCH namespace.
// This label allows the OpenShift monitoring stack to scrape PrometheusRules and ServiceMonitors
// deployed by ACM, avoiding conflicts with the openshift-* namespace.
func (r *MultiClusterHubReconciler) ensureOpenShiftNamespaceLabel(ctx context.Context, m *operatorv1.MultiClusterHub) (
	ctrl.Result, error,
) {
	existingNs := &corev1.Namespace{}

	err := r.Client.Get(ctx, types.NamespacedName{Name: m.GetNamespace()}, existingNs)
	if err != nil || errors.IsNotFound(err) {
		log.Error(err, "Failed to find namespace for MultiClusterHub", "namespace", m.GetNamespace())
		return ctrl.Result{}, err
	}

	if len(existingNs.Labels) == 0 {
		existingNs.Labels = make(map[string]string)
	}

	if _, ok := existingNs.Labels[utils.OpenShiftClusterMonitoringLabel]; !ok {
		r.Log.Info("Adding monitoring label to namespace",
			"label", utils.OpenShiftClusterMonitoringLabel, "namespace", m.GetNamespace())
		existingNs.Labels[utils.OpenShiftClusterMonitoringLabel] = "true"

		err = r.Client.Update(ctx, existingNs)
		if err != nil {
			log.Error(err, "Failed to update namespace with monitoring label",
				"namespace", m.GetNamespace(), "label", utils.OpenShiftClusterMonitoringLabel)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// createTrustBundleConfigmap ensures a trusted CA bundle configmap exists in the MCH namespace.
// The configmap is labeled so OpenShift injects the cluster-wide proxy CA bundle into it.
// If the configmap already exists, this is a no-op.
func (r *MultiClusterHubReconciler) createTrustBundleConfigmap(ctx context.Context, mch *operatorv1.MultiClusterHub) (
	ctrl.Result, error) {
	trustBundleName := defaultTrustBundleName
	trustBundleNamespace := mch.Namespace
	if name, ok := os.LookupEnv(trustBundleNameEnvVar); ok && name != "" {
		trustBundleName = name
	}

	namespacedName := types.NamespacedName{
		Name:      trustBundleName,
		Namespace: trustBundleNamespace,
	}

	// Check if configmap already exists
	cm := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, namespacedName, cm)

	if err == nil {
		return ctrl.Result{}, nil
	}

	if !errors.IsNotFound(err) {
		log.Error(err, "Failed to get trust bundle configmap",
			"name", trustBundleName, "namespace", trustBundleNamespace)
		return ctrl.Result{}, err
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
			err, "Error setting controller reference on trust bundle configmap %s", trustBundleName)
	}

	err = r.Client.Create(ctx, cm)
	if err != nil {
		log.Error(err, "Failed to create trust bundle configmap",
			"name", trustBundleName, "namespace", trustBundleNamespace)
		return ctrl.Result{}, err
	}

	log.Info("Created trust bundle configmap",
		"name", trustBundleName, "namespace", trustBundleNamespace)
	return ctrl.Result{}, nil
}

/*
createMetricsService ensures the MCH operator's metrics service exists in the MCH namespace.

This service exposes the operator's metrics endpoint (port 8383) so that Prometheus can scrape
operator-specific metrics for monitoring and observability. The service is owned by the MCH CR
and will be automatically cleaned up when the MCH is deleted.

This is required for:
  - Monitoring operator health and performance
  - Alerting on operator issues
  - Providing visibility into MCH reconciliation metrics
*/
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
			log.Error(err, "Failed to get metrics service", "name", sName, "namespace", sNamespace)
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
			log.Error(err, "Failed to create metrics service", "name", sName, "namespace", sNamespace)
			return ctrl.Result{}, err
		}

		log.Info("Created metrics service", "name", sName, "namespace", sNamespace)
	}

	return ctrl.Result{}, nil
}

/*
createMetricsServiceMonitor ensures the MCH operator's ServiceMonitor exists in the MCH namespace.

A ServiceMonitor is a Prometheus Operator CRD that configures Prometheus to scrape metrics from
the MCH operator service. This enables automatic discovery and collection of operator metrics
without manual Prometheus configuration.

This is required for:
  - Automatic metrics collection by the OpenShift monitoring stack
  - Integration with the cluster-wide Prometheus instance
  - Consistent monitoring across all ACM components

The ServiceMonitor is owned by the MCH CR and will be automatically cleaned up when the MCH is deleted.
*/
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
			log.Error(err, "Failed to get metrics ServiceMonitor", "name", smName, "namespace", smNamespace)
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
			log.Error(err, "Failed to create metrics ServiceMonitor", "name", smName, "namespace", smNamespace)
			return ctrl.Result{}, err
		}

		logf.Log.Info("Created metrics ServiceMonitor", "name", smName, "namespace", smNamespace)
	}

	return ctrl.Result{}, nil
}

// ingressDomain discovers the cluster's ingress domain from the OpenShift Ingress config
// and caches it in CacheSpec. Sets the INGRESS_DOMAIN environment variable so Helm charts
// can reference the domain during rendering.
func (r *MultiClusterHubReconciler) ingressDomain(ctx context.Context, m *operatorv1.MultiClusterHub) (
	ctrl.Result, error) {
	ingress := &configv1.Ingress{}

	err := r.Client.Get(ctx, types.NamespacedName{Name: "cluster"}, ingress)
	if err != nil {
		r.Log.Error(err, "Failed to get Ingress")

		return ctrl.Result{}, err
	}

	domain := ingress.Spec.Domain
	if r.CacheSpec.IngressDomain != domain {
		if r.CacheSpec.IngressDomain != "" {
			r.Log.Info("Ingress domain changed, updating cached value",
				"previousDomain", r.CacheSpec.IngressDomain,
				"newDomain", domain)
		}

		r.Log.Info("Setting ingress domain", "domain", domain)
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

// openShiftApiUrl discovers the API server URL from the OpenShift Infrastructure config
// and sets it as the API_URL environment variable for chart rendering.
func (r *MultiClusterHubReconciler) openShiftApiUrl(ctx context.Context, m *operatorv1.MultiClusterHub) (
	ctrl.Result, error) {
	infrastructure := &configv1.Infrastructure{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: "cluster"}, infrastructure)
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
