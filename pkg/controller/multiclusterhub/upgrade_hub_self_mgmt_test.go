package multiclusterhub

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_getImageFromManifestByKey(t *testing.T) {

	full_mch.Status.DesiredVersion = "2.1.2"
	tests := []struct {
		Name      string
		ImageKey  string
		ConfigMap *corev1.ConfigMap
		Result    string
	}{
		{
			Name:     "Proper image key given",
			ImageKey: "multicluster_operators_subscription",
			ConfigMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("mch-image-manifest-%s", full_mch.Status.DesiredVersion),
					Namespace: full_mch.Namespace,
				},
				Data: map[string]string{
					"multicluster_operators_subscription": "quay.io/rhibmcollab/multicluster-operators-subscription-image@sha256:test",
				},
			},
			Result: "quay.io/rhibmcollab/multicluster-operators-subscription-image@sha256:test",
		},
		{
			Name:     "Improper image key given",
			ImageKey: "nonexistant_image_key",
			ConfigMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("mch-image-manifest-%s", full_mch.Status.DesiredVersion),
					Namespace: full_mch.Namespace,
				},
				Data: map[string]string{
					"multicluster_operators_subscription": "quay.io/rhibmcollab/multicluster-operators-subscription-image@sha256:test",
				},
			},
			Result: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			r, err := getTestReconciler(full_mch)
			if err != nil {
				t.Fatalf("Failed to create test reconciler")
			}

			err = r.client.Create(context.TODO(), tt.ConfigMap)
			if err != nil {
				t.Fatalf("Err: %s", err)
			}

			image, err := r.getImageFromManifestByKey(full_mch, tt.ImageKey)
			if image != tt.Result {
				t.Fatalf("Unexpected image value returned")
			}
		})
	}
}
