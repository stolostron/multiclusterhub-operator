package utils

import (
	"encoding/base64"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// NewMultiClusterHub ...
func NewMultiClusterHub(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "operators.open-cluster-management.io/v1beta1",
			"kind":       "MultiClusterHub",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"imagePullSecret": "multiclusterhub-operator-pull-secret",
			},
		},
	}
}

// NewOperatorGroup ...
func NewOperatorGroup(namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "operators.coreos.com/v1",
			"kind":       "OperatorGroup",
			"metadata": map[string]interface{}{
				"name":      "default",
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"targetNamespaces": []string{namespace},
			},
		},
	}
}

// NewPullSecret ...
func NewPullSecret(name, namespace string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind: "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Type: "kubernetes.io/dockerconfigjson",
		StringData: map[string]string{
			".dockerconfigjson": fmt.Sprintf(`{"auths":{"quay.io":{"username":"%s","password":"%s","auth":"%s"}}}`, os.Getenv("DOCKER_USER"), os.Getenv("DOCKER_PASS"), base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", os.Getenv("DOCKER_USER"), os.Getenv("DOCKER_PASS"))))),
		},
	}
}

// NewNamespace ...
func NewNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// NewACMSubscription ...
func NewACMSubscription(namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "operators.coreos.com/v1alpha1",
			"kind":       "Subscription",
			"metadata": map[string]interface{}{
				"name":      "acm-operator-subscription",
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"sourceNamespace":     "openshift-marketplace",
				"source":              "redhat-operators",
				"channel":             "release-1.0",
				"installPlanApproval": "Automatic",
				"name":                "advanced-cluster-management",
			},
		},
	}
}
