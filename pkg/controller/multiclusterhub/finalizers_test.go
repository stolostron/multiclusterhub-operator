// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package multiclusterhub

import (
	"context"
	"fmt"
	"os"
	"testing"

	operatorsv1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operator/v1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/channel"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/foundation"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/subscription"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

func Test_cleanupHiveConfigs(t *testing.T) {
	tests := []struct {
		Name       string
		MCH        *operatorsv1.MultiClusterHub
		HiveConfig *unstructured.Unstructured
		Result     error
	}{
		{
			Name:   "Installer Created HiveConfig",
			MCH:    full_mch,
			Result: nil,
		},
		{
			Name:   "Seperate HiveConfig",
			MCH:    empty_mch,
			Result: nil,
		},
	}

	reqLogger := log.WithValues("Request.Namespace", mch_namespace, "Request.Name", mch_name)

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// Objects to track in the fake client.
			r, err := getTestReconciler(tt.MCH)
			if err != nil {
				t.Fatalf("Failed to create test reconciler")
			}

			err = r.cleanupHiveConfigs(reqLogger, full_mch)
			if err != tt.Result {
				t.Fatal("Failed to cleanup Hive Config")
			}

		})
	}
}

func Test_cleanupAPIServices(t *testing.T) {

	APIService := &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testApiService",
			Namespace: mch_namespace,
		},
		Spec: apiregistrationv1.APIServiceSpec{
			Group:                 "proxy.open-cluster-management.io",
			Version:               "v1beta1",
			InsecureSkipTLSVerify: true,
			GroupPriorityMinimum:  1000,
			VersionPriority:       20,
		},
	}

	InstallerAPIService := APIService.DeepCopy()
	InstallerAPIService.SetLabels(map[string]string{
		"installer.name":      mch_name,
		"installer.namespace": mch_namespace,
	})

	tests := []struct {
		Name       string
		MCH        *operatorsv1.MultiClusterHub
		APIService *apiregistrationv1.APIService
		Result     error
	}{
		{
			Name:       "Without Labels",
			MCH:        full_mch,
			APIService: APIService,
			Result:     nil,
		},
		{
			Name:       "With Labels",
			MCH:        full_mch,
			APIService: InstallerAPIService,
			Result:     fmt.Errorf("apiservices.apiregistration.k8s.io \"testApiService\" not found"),
		},
	}

	reqLogger := log.WithValues("Request.Namespace", mch_namespace, "Request.Name", mch_name)

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// Objects to track in the fake client.
			r, err := getTestReconciler(tt.MCH)
			if err != nil {
				t.Fatalf("Failed to create test reconciler")
			}

			err = r.client.Create(context.TODO(), tt.APIService)
			if err != nil {
				t.Fatal(err.Error())
			}

			err = r.cleanupAPIServices(reqLogger, full_mch)
			if err != nil {
				t.Fatalf("Failed to cleanup API services: %s", err.Error())
			}

			emptyAPIService := &apiregistrationv1.APIService{}
			err = r.client.Get(context.TODO(), types.NamespacedName{
				Name:      tt.APIService.Name,
				Namespace: tt.APIService.Namespace,
			}, emptyAPIService)
			if !errorEquals(err, tt.Result) {
				t.Fatal(err.Error())
			}
		})
	}
}

func Test_cleanupClusterRoles(t *testing.T) {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-clusterrole",
			Namespace: mch_namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"cluster.open-cluster-management.io"},
				Resources: []string{"managedclusters"},
				Verbs:     []string{"get", "list", "watch", "create"},
			},
		},
	}
	installerClusterRole := clusterRole.DeepCopy()
	installerClusterRole.SetLabels(map[string]string{
		"installer.name":      mch_name,
		"installer.namespace": mch_namespace,
	})

	tests := []struct {
		Name        string
		MCH         *operatorsv1.MultiClusterHub
		ClusterRole *rbacv1.ClusterRole
		Result      error
	}{
		{
			Name:        "Without Labels",
			MCH:         full_mch,
			ClusterRole: clusterRole,
			Result:      nil,
		},
		{
			Name:        "With Labels",
			MCH:         full_mch,
			ClusterRole: installerClusterRole,
			Result:      fmt.Errorf("clusterroles.rbac.authorization.k8s.io \"test-clusterrole\" not found"),
		},
	}

	reqLogger := log.WithValues("Request.Namespace", mch_namespace, "Request.Name", mch_name)

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// Objects to track in the fake client.
			r, err := getTestReconciler(tt.MCH)
			if err != nil {
				t.Fatalf("Failed to create test reconciler")
			}

			err = r.client.Create(context.TODO(), tt.ClusterRole)
			if err != nil {
				t.Fatal(err.Error())
			}

			err = r.cleanupClusterRoles(reqLogger, full_mch)
			if err != nil {
				t.Fatal("Failed to cleanup clusterroles")
			}

			emptyClusterRole := &rbacv1.ClusterRole{}
			err = r.client.Get(context.TODO(), types.NamespacedName{
				Name:      tt.ClusterRole.Name,
				Namespace: tt.ClusterRole.Namespace,
			}, emptyClusterRole)
			if !errorEquals(err, tt.Result) {
				t.Fatal(err.Error())
			}
		})
	}
}

func Test_cleanupClusterRoleBindings(t *testing.T) {
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-clusterrolebinding",
			Namespace: mch_namespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "ServiceAccount",
				Name: "ocm-foundation-sa",
			},
		},
	}

	installerClusterRoleBinding := clusterRoleBinding.DeepCopy()
	installerClusterRoleBinding.SetLabels(map[string]string{
		"installer.name":      mch_name,
		"installer.namespace": mch_namespace,
	})

	tests := []struct {
		Name   string
		MCH    *operatorsv1.MultiClusterHub
		CRB    *rbacv1.ClusterRoleBinding
		Result error
	}{
		{
			Name:   "Without Labels",
			MCH:    full_mch,
			CRB:    clusterRoleBinding,
			Result: nil,
		},
		{
			Name:   "With Labels",
			MCH:    empty_mch,
			CRB:    installerClusterRoleBinding,
			Result: fmt.Errorf("clusterrolebindings.rbac.authorization.k8s.io \"test-clusterrolebinding\" not found"),
		},
	}

	reqLogger := log.WithValues("Request.Namespace", mch_namespace, "Request.Name", mch_name)

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// Objects to track in the fake client.
			r, err := getTestReconciler(tt.MCH)
			if err != nil {
				t.Fatalf("Failed to create test reconciler")
			}

			err = r.client.Create(context.TODO(), tt.CRB)
			if err != nil {
				t.Fatal(err.Error())
			}

			err = r.cleanupClusterRoleBindings(reqLogger, full_mch)
			if err != nil {
				t.Fatalf("Failed to cleanup clusterrolebindings: %s", err.Error())
			}

			emptyClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
			err = r.client.Get(context.TODO(), types.NamespacedName{
				Name:      tt.CRB.Name,
				Namespace: tt.CRB.Namespace,
			}, emptyClusterRoleBinding)
			if !errorEquals(err, tt.Result) {
				t.Fatal(err.Error())
			}
		})
	}
}

func Test_cleanupMutatingWebhooks(t *testing.T) {
	MWC := &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-mutatingwebhookconfiguration",
			Namespace: mch_namespace,
		},
		Webhooks: []admissionregistrationv1.MutatingWebhook{
			{
				Name: "ocm.mutating.webhook.admission.open-cluster-management.io",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					Service: &admissionregistrationv1.ServiceReference{
						Name: "ocm-webhook",
					},
				},
			},
		},
	}

	installerMWC := MWC.DeepCopy()
	installerMWC.SetLabels(map[string]string{
		"installer.name":      mch_name,
		"installer.namespace": mch_namespace,
	})

	tests := []struct {
		Name   string
		MCH    *operatorsv1.MultiClusterHub
		MWC    *admissionregistrationv1.MutatingWebhookConfiguration
		Result error
	}{
		{
			Name:   "Without Labels",
			MCH:    full_mch,
			MWC:    MWC,
			Result: nil,
		},
		{
			Name:   "With Labels",
			MCH:    empty_mch,
			MWC:    installerMWC,
			Result: fmt.Errorf("mutatingwebhookconfigurations.admissionregistration.k8s.io \"test-mutatingwebhookconfiguration\" not found"),
		},
	}

	reqLogger := log.WithValues("Request.Namespace", mch_namespace, "Request.Name", mch_name)

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// Objects to track in the fake client.
			r, err := getTestReconciler(tt.MCH)
			if err != nil {
				t.Fatalf("Failed to create test reconciler")
			}

			err = r.client.Create(context.TODO(), tt.MWC)
			if err != nil {
				t.Fatal(err.Error())
			}

			err = r.cleanupMutatingWebhooks(reqLogger, full_mch)
			if err != nil {
				t.Fatal("Failed to cleanup mutatingwebhookconfigurations")
			}

			emptyMWC := &admissionregistrationv1.MutatingWebhookConfiguration{}
			err = r.client.Get(context.TODO(), types.NamespacedName{
				Name:      tt.MWC.Name,
				Namespace: tt.MWC.Namespace,
			}, emptyMWC)
			if !errorEquals(err, tt.Result) {
				t.Fatal(err.Error())
			}
		})
	}
}

func Test_cleanupValidatingWebhooks(t *testing.T) {
	MWC := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-validatingwebhookconfiguration",
			Namespace: mch_namespace,
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				Name: "ocm.validating.webhook.admission.open-cluster-management.io",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					Service: &admissionregistrationv1.ServiceReference{
						Name: "ocm-webhook",
					},
				},
			},
		},
	}

	installerMWC := MWC.DeepCopy()
	installerMWC.SetLabels(map[string]string{
		"installer.name":      mch_name,
		"installer.namespace": mch_namespace,
	})

	tests := []struct {
		Name   string
		MCH    *operatorsv1.MultiClusterHub
		MWC    *admissionregistrationv1.ValidatingWebhookConfiguration
		Result error
	}{
		{
			Name:   "Without Labels",
			MCH:    full_mch,
			MWC:    MWC,
			Result: nil,
		},
		{
			Name:   "With Labels",
			MCH:    empty_mch,
			MWC:    installerMWC,
			Result: fmt.Errorf("validatingwebhookconfigurations.admissionregistration.k8s.io \"test-validatingwebhookconfiguration\" not found"),
		},
	}

	reqLogger := log.WithValues("Request.Namespace", mch_namespace, "Request.Name", mch_name)

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// Objects to track in the fake client.
			r, err := getTestReconciler(tt.MCH)
			if err != nil {
				t.Fatalf("Failed to create test reconciler")
			}

			err = r.client.Create(context.TODO(), tt.MWC)
			if err != nil {
				t.Fatal(err.Error())
			}

			err = r.cleanupValidatingWebhooks(reqLogger, full_mch)
			if err != nil {
				t.Fatal("Failed to cleanup validatingwebhookconfiguration")
			}

			emptyMWC := &admissionregistrationv1.ValidatingWebhookConfiguration{}
			err = r.client.Get(context.TODO(), types.NamespacedName{
				Name:      tt.MWC.Name,
				Namespace: tt.MWC.Namespace,
			}, emptyMWC)
			if !errorEquals(err, tt.Result) {
				t.Fatal(err.Error())
			}
		})
	}
}

func Test_cleanupPullSecret(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      full_mch.Spec.ImagePullSecret,
			Namespace: utils.CertManagerNamespace,
		},
		StringData: map[string]string{
			"test": "data",
		},
	}

	tests := []struct {
		Name   string
		MCH    *operatorsv1.MultiClusterHub
		Secret *corev1.Secret
	}{
		{
			Name:   "Without Labels",
			MCH:    full_mch,
			Secret: secret,
		},
	}

	reqLogger := log.WithValues("Request.Namespace", mch_namespace, "Request.Name", mch_name)

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// Objects to track in the fake client.
			r, err := getTestReconciler(tt.MCH)
			if err != nil {
				t.Fatalf("Failed to create test reconciler")
			}

			err = r.client.Create(context.TODO(), tt.Secret)
			if err != nil {
				t.Fatal(err.Error())
			}

			err = r.cleanupPullSecret(reqLogger, full_mch)
			if err != nil {
				t.Fatal("Failed to cleanup pull secret")
			}

			emptySecret := &corev1.Secret{}
			err = r.client.Get(context.TODO(), types.NamespacedName{
				Name:      tt.Secret.Name,
				Namespace: tt.Secret.Namespace,
			}, emptySecret)

			if err == nil || !errors.IsNotFound(err) {
				t.Errorf("cleanupPullSecret() error = %v, wanted isNotFound error", err)
			}
		})
	}
}

func Test_cleanupCRDS(t *testing.T) {
	CRD := &apixv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-crd",
			Namespace: mch_namespace,
		},
		Spec: apixv1.CustomResourceDefinitionSpec{
			Group: "inventory.open-cluster-management.io",
			Names: apixv1.CustomResourceDefinitionNames{
				Plural:   "baremetalassets",
				Kind:     "BareMetalAsset",
				ListKind: "BareMetalAssetList",
				Singular: "baremetalasset",
			},
		},
	}

	installerCRD := CRD.DeepCopy()
	installerCRD.SetLabels(map[string]string{
		"installer.name":      mch_name,
		"installer.namespace": mch_namespace,
	})

	tests := []struct {
		Name   string
		MCH    *operatorsv1.MultiClusterHub
		CRD    *apixv1.CustomResourceDefinition
		Result error
	}{
		{
			Name:   "Without Labels",
			MCH:    full_mch,
			CRD:    CRD,
			Result: nil,
		},
		{
			Name:   "With Labels",
			MCH:    empty_mch,
			CRD:    installerCRD,
			Result: fmt.Errorf("customresourcedefinitions.apiextensions.k8s.io \"test-crd\" not found"),
		},
	}

	reqLogger := log.WithValues("Request.Namespace", mch_namespace, "Request.Name", mch_name)

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// Objects to track in the fake client.
			r, err := getTestReconciler(tt.MCH)
			if err != nil {
				t.Fatalf("Failed to create test reconciler")
			}

			err = r.client.Create(context.TODO(), tt.CRD)
			if err != nil {
				t.Fatal(err.Error())
			}

			err = r.cleanupCRDs(reqLogger, full_mch)
			if err != nil {
				t.Fatal("Failed to cleanup CRDs")
			}

			emptyCRD := &apixv1.CustomResourceDefinition{}
			err = r.client.Get(context.TODO(), types.NamespacedName{
				Name:      tt.CRD.Name,
				Namespace: tt.CRD.Namespace,
			}, emptyCRD)
			if !errorEquals(err, tt.Result) {
				t.Fatal(err.Error())
			}
		})
	}
}

func Test_cleanupClusterManagers(t *testing.T) {
	tests := []struct {
		Name           string
		MCH            *operatorsv1.MultiClusterHub
		ClusterManager *unstructured.Unstructured
		Result         error
	}{
		{
			Name:   "Installer Created ClusterManager",
			MCH:    full_mch,
			Result: nil,
		},
		{
			Name:   "Seperate ClusterManager",
			MCH:    empty_mch,
			Result: nil,
		},
	}

	reqLogger := log.WithValues("Request.Namespace", mch_namespace, "Request.Name", mch_name)

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// Objects to track in the fake client.
			r, err := getTestReconciler(tt.MCH)
			if err != nil {
				t.Fatalf("Failed to create test reconciler: %s", err)
			}

			err = r.cleanupClusterManagers(reqLogger, full_mch)
			if err != tt.Result {
				t.Fatalf("Failed to cleanup ClusterManager: %s", err)
			}

		})
	}
}

func Test_cleanupAppSubscriptions(t *testing.T) {
	os.Setenv("UNIT_TEST", "true")
	defer os.Unsetenv("UNIT_TEST")

	tests := []struct {
		Name           string
		MCH            *operatorsv1.MultiClusterHub
		ClusterManager *unstructured.Unstructured
		Result         error
	}{
		{
			Name:   "Installer Created Appsubscriptions",
			MCH:    full_mch,
			Result: nil,
		},
	}

	reqLogger := log.WithValues("Request.Namespace", mch_namespace, "Request.Name", mch_name)

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// Objects to track in the fake client.
			r, err := getTestReconciler(tt.MCH)
			if err != nil {
				t.Fatalf("Failed to create test reconciler: %s", err)
			}

			var emptyOverrides map[string]string

			result, err := r.ensureSubscription(tt.MCH, subscription.CertWebhook(tt.MCH, emptyOverrides))
			if result != nil {
				t.Fatalf("Failed to ensure foundation resource: %s", err)
			}

			result, err = r.ensureSubscription(tt.MCH, subscription.ConfigWatcher(tt.MCH, emptyOverrides))
			if result != nil {
				t.Fatalf("Failed to ensure foundation resource: %s", err)
			}

			result, err = r.ensureSubscription(tt.MCH, subscription.Search(tt.MCH, emptyOverrides))
			if result != nil {
				t.Fatalf("Failed to ensure foundation resource: %s", err)
			}

			err = r.cleanupAppSubscriptions(reqLogger, tt.MCH)
			if err != tt.Result {
				t.Fatalf("Failed to cleanup appsubscription: %s", err)
			}

		})
	}
}

func Test_cleanupFoundation(t *testing.T) {
	tests := []struct {
		Name           string
		MCH            *operatorsv1.MultiClusterHub
		ClusterManager *unstructured.Unstructured
		Result         error
	}{
		{
			Name:   "Installer Foundation Artefacts",
			MCH:    full_mch,
			Result: nil,
		},
	}

	reqLogger := log.WithValues("Request.Namespace", mch_namespace, "Request.Name", mch_name)

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// Objects to track in the fake client.
			r, err := getTestReconciler(tt.MCH)
			if err != nil {
				t.Fatalf("Failed to create test reconciler: %s", err)
			}

			var emptyOverrides map[string]string

			result, err := r.ensureChannel(tt.MCH, channel.Channel(tt.MCH))
			if result != nil {
				t.Fatalf("Failed to ensure foundation resource: %s", err)
			}

			//OCM proxy server service
			result, err = r.ensureService(tt.MCH, foundation.OCMProxyServerService(tt.MCH))
			if result != nil {
				t.Fatalf("Failed to ensure foundation resource: %s", err)
			}

			// OCM apiServices
			result, err = r.ensureAPIService(tt.MCH, foundation.OCMProxyAPIService(tt.MCH))
			if result != nil {
				t.Fatalf("Failed to ensure foundation resource: %s", err)
			}
			result, err = r.ensureAPIService(tt.MCH, foundation.OCMClusterViewV1APIService(tt.MCH))
			if result != nil {
				t.Fatalf("Failed to ensure foundation resource: %s", err)
			}
			result, err = r.ensureAPIService(tt.MCH, foundation.OCMClusterViewV1alpha1APIService(tt.MCH))
			if result != nil {
				t.Fatalf("Failed to ensure foundation resource: %s", err)
			}

			//OCM controller deployment
			result, err = r.ensureDeployment(tt.MCH, foundation.OCMControllerDeployment(tt.MCH, emptyOverrides))
			if result != nil {
				t.Fatalf("Failed to ensure foundation resource: %s", err)
			}

			err = r.cleanupFoundation(reqLogger, tt.MCH)
			if err != tt.Result {
				t.Fatalf("Failed to cleanup foundation: %s", err)
			}

		})
	}
}
