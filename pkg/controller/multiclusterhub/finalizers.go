// Copyright (c) 2020 Red Hat, Inc.

package multiclusterhub

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	operatorsv1beta1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1beta1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func (r *ReconcileMultiClusterHub) cleanupHiveConfigs(reqLogger logr.Logger, m *operatorsv1beta1.MultiClusterHub) error {
	hiveConfigRes := schema.GroupVersionResource{Group: "hive.openshift.io", Version: "v1", Resource: "hiveconfigs"}

	dc, err := createDynamicClient()
	if err != nil {
		reqLogger.Error(err, "Failed to create dynamic client")
		return err
	}

	labelSelector := fmt.Sprintf("installer.name=%s, installer.namespace=%s", m.GetName(), m.GetNamespace())
	listOptions := metav1.ListOptions{
		LabelSelector: labelSelector,
	}
	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	// Find all resources created by installer based on label
	hiveResList, err := dc.Resource(hiveConfigRes).List(listOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			// If Hiveconfig resource doesn't exist then move on
			reqLogger.Info("Hiveconfig resource not found. Continuing.")
			return nil
		}
		reqLogger.Error(err, "Error while listing hiveconfig instances")
		return err
	}

	// Delete all identified instances
	for _, hiveRes := range hiveResList.Items {
		reqLogger.Info("Deleting hiveconfig", "Resource.Name", hiveRes.GetName())
		if err := dc.Resource(hiveConfigRes).Delete(hiveRes.GetName(), &deleteOptions); err != nil {
			reqLogger.Error(err, "Error while deleting hiveconfig instances")
			return err
		}
	}

	reqLogger.Info("Hiveconfigs finalized")
	return nil
}

func (r *ReconcileMultiClusterHub) cleanupAPIServices(reqLogger logr.Logger, m *operatorsv1beta1.MultiClusterHub) error {
	err := r.client.DeleteAllOf(
		context.TODO(),
		&apiregistrationv1.APIService{},
		client.MatchingLabels{
			"installer.name":      m.GetName(),
			"installer.namespace": m.GetNamespace(),
		},
	)

	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("No matching API services to finalize. Continuing.")
			return nil
		}
		reqLogger.Error(err, "Error while deleting API services")
		return err
	}

	reqLogger.Info("API services finalized")
	return nil
}

func (r *ReconcileMultiClusterHub) cleanupClusterRoles(reqLogger logr.Logger, m *operatorsv1beta1.MultiClusterHub) error {
	config, err := config.GetConfig()
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	labelSelector := fmt.Sprintf("installer.name=%s, installer.namespace=%s", m.GetName(), m.GetNamespace())
	listOptions := metav1.ListOptions{
		LabelSelector: labelSelector,
	}
	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}
	err = clientset.RbacV1().ClusterRoles().DeleteCollection(&deleteOptions, listOptions)

	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("No matching clusterroles to finalize. Continuing.")
			return nil
		}
		reqLogger.Error(err, "Error while deleting clusterroles")
		return err
	}

	reqLogger.Info("Clusterroles finalized")
	return nil
}

func (r *ReconcileMultiClusterHub) cleanupClusterRoleBindings(reqLogger logr.Logger, m *operatorsv1beta1.MultiClusterHub) error {
	config, err := config.GetConfig()
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	labelSelector := fmt.Sprintf("installer.name=%s, installer.namespace=%s", m.GetName(), m.GetNamespace())
	listOptions := metav1.ListOptions{
		LabelSelector: labelSelector,
	}
	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	err = clientset.RbacV1().ClusterRoleBindings().DeleteCollection(&deleteOptions, listOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("No matching clusterrolebindings to finalize. Continuing.")
			return nil
		}
		reqLogger.Error(err, "Error while deleting clusterrolebindings")
		return err
	}

	reqLogger.Info("Clusterrolebindings finalized")
	return nil
}

func (r *ReconcileMultiClusterHub) cleanupMutatingWebhooks(reqLogger logr.Logger, m *operatorsv1beta1.MultiClusterHub) error {
	config, err := config.GetConfig()
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	labelSelector := fmt.Sprintf("installer.name=%s, installer.namespace=%s", m.GetName(), m.GetNamespace())
	listOptions := metav1.ListOptions{
		LabelSelector: labelSelector,
	}
	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	err = clientset.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().DeleteCollection(&deleteOptions, listOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("No matching MutatingWebhookConfigurations to finalize. Continuing.")
			return nil
		}
		reqLogger.Error(err, "Error while deleting MutatingWebhookConfigurations")
		return err
	}

	reqLogger.Info("MutatingWebhookConfigurations finalized")
	return nil
}

func (r *ReconcileMultiClusterHub) cleanupPullSecret(reqLogger logr.Logger, m *operatorsv1beta1.MultiClusterHub) error {
	config, err := config.GetConfig()
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	labelSelector := fmt.Sprintf("installer.name=%s, installer.namespace=%s", m.GetName(), m.GetNamespace())
	listOptions := metav1.ListOptions{
		LabelSelector: labelSelector,
	}
	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}
	err = clientset.CoreV1().Secrets(utils.CertManagerNS(m)).DeleteCollection(&deleteOptions, listOptions)

	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("No matching secrets to finalize. Continuing.")
			return nil
		}
		reqLogger.Error(err, "Error while deleting secrets")
		return err
	}

	reqLogger.Info(fmt.Sprintf("%s secrets finalized", utils.CertManagerNS(m)))
	return nil
}

func (r *ReconcileMultiClusterHub) cleanupCRDs(log logr.Logger, m *operatorsv1beta1.MultiClusterHub) error {
	err := r.client.DeleteAllOf(
		context.TODO(),
		&apixv1.CustomResourceDefinition{}
		client.MatchingLabels{
			"installer.name":      m.GetName(),
			"installer.namespace": m.GetNamespace(),
		},
	)

	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("No matching CRDs to finalize. Continuing.")
			return nil
		}
		log.Error(err, "Error while deleting CRDs")
		return err
	}

	log.Info("CRDs finalized")
	return nil
}
