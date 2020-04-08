package subscription

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_getOverrides(t *testing.T) {
	overrides := map[string]interface{}{
		"image": map[string]interface{}{
			"pullPolicy": "Always",
		},
	}

	sub := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps.open-cluster-management.io/v1",
			"kind":       "Subscription",
			"metadata": map[string]interface{}{
				"name":      "name",
				"namespace": "test",
			},
			"spec": map[string]interface{}{
				"channel": "channel",
				"name":    "name",
				"placement": map[string]interface{}{
					"local": true,
				},
				"packageOverrides": []map[string]interface{}{
					{
						"packageName": "name",
						"packageOverrides": []map[string]interface{}{
							{
								"path":  "spec",
								"value": overrides,
							},
						},
					},
				},
			},
		},
	}

	t.Run("Find overrides", func(t *testing.T) {
		got, err := getOverrides(sub)
		if err != nil {
			t.Errorf("getOverrides() error = %v", err)
		}
		if !reflect.DeepEqual(got, overrides) {
			t.Errorf("getOverrides() = %v, want %v", got, overrides)
		}
	})
}
