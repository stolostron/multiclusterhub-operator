// Copyright (c) 2024 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package overrides

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/stolostron/multiclusterhub-operator/pkg/manifest"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var logf = log.Log.WithName("overrides")

const (
	// OSBSImagePrefix ...
	OSBSImagePrefix = "RELATED_IMAGE_"

	// OperandImagePrefix ...
	OperandImagePrefix = "OPERAND_IMAGE_"

	// TemplateOverridePrefix ...
	TemplateOverridePrefix = "TEMPLATE_OVERRIDE_"
)

/*
ConvertImageOverrides converts manifest images to overrides in a map. It iterates through the provided slice of
manifest images, constructs overrides based on digests or tags, and updates the given overrides map. It returns an
error if any image has an empty or missing ImageKey.
*/
func ConvertImageOverrides(overrides map[string]string, manifestImages []manifest.ManifestImage) error {
	for _, m := range manifestImages {
		// Check if the manifest image contains a ImageKey.
		if m.ImageKey == "" {
			return fmt.Errorf("unexpected manifest image format: missing or empty ImageKey %v", m)
		}

		// Check if either ImageDigest or ImageTag is provided.
		if m.ImageDigest != "" {
			overrides[m.ImageKey] = fmt.Sprintf("%s/%s@%s", m.ImageRemote, m.ImageName, m.ImageDigest)
		} else if m.ImageTag != "" {
			overrides[m.ImageKey] = fmt.Sprintf("%s/%s:%s", m.ImageRemote, m.ImageName, m.ImageTag)
		} else {
			return fmt.Errorf("unexpected manifest image format: neither ImageDigest nor ImageTag provided %v", m)
		}
	}
	return nil
}

/*
convertTemplateOverrides converts manifest templates to overrides in a map. It iterates through the provided
manifest template, converts each template override to a string, and updates the given overrides map. It returns an
error if there's any issue converting values.
*/
func ConvertTemplateOverrides(overrides map[string]string, manifestTemplate manifest.ManifestTemplate) error {
	for key, value := range manifestTemplate.TemplateOverrides {

		// Check if the key already exists.
		if overrides[key] != "" {
			continue // Skip processing if environment variable exists.
		}

		// Convert value to string if necessary.
		strValue, err := ConvertToString(value)
		if err != nil {
			return fmt.Errorf("error converting value for key %s: %w", key, err)
		}
		overrides[key] = strValue
	}
	return nil
}

// ConvertToString converts a value to a string.
func ConvertToString(value interface{}) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case int, int32, int64:
		return fmt.Sprintf("%d", v), nil
	case float32, float64:
		return fmt.Sprintf("%f", v), nil
	case bool:
		return strconv.FormatBool(v), nil
	default:
		return "", fmt.Errorf("unsupported type: %T", v)
	}
}

/*
GetOverridesFromConfigmap reads and formats image or template overrides from a ConfigMap. It fetches the specified
ConfigMap, parses the data, and returns overrides. If the ConfigMap is not found or has unexpected data, it returns
an error.
*/
func GetOverridesFromConfigmap(k8sClient client.Client, overrides map[string]string, namespace, configmapName string,
	isTemplate bool) (map[string]string, error) {

	objectType := "image"
	if isTemplate {
		objectType = "template"
	}

	logf.Info(fmt.Sprintf("Overriding %s from configmap: %s/%s", objectType, namespace, configmapName))

	configmap := &corev1.ConfigMap{}
	err := k8sClient.Get(context.TODO(), types.NamespacedName{
		Name:      configmapName,
		Namespace: namespace,
	}, configmap)

	if err != nil && errors.IsNotFound(err) {
		return overrides, err
	}

	if len(configmap.Data) != 1 {
		return overrides, fmt.Errorf(
			fmt.Sprintf("Unexpected number of keys in ConfigMap %s: expected 1 key, found %d keys", configmapName,
				len(configmap.Data)),
		)
	}

	for _, v := range configmap.Data {
		if isTemplate {
			var manifestTemplate manifest.ManifestTemplate
			if err := json.Unmarshal([]byte(v), &manifestTemplate); err != nil {
				return overrides, err
			}

			if err := ConvertTemplateOverrides(overrides, manifestTemplate); err != nil {
				return overrides, err
			}

		} else {
			var manifestImage []manifest.ManifestImage
			if err := json.Unmarshal([]byte(v), &manifestImage); err != nil {
				return overrides, err
			}

			if err := ConvertImageOverrides(overrides, manifestImage); err != nil {
				return overrides, err
			}
		}
	}

	return overrides, nil
}

/*
GetOverridesFromEnv reads and formats full image or template reference from environment variables.
*/
func GetOverridesFromEnv(prefix string) map[string]string {
	overrides := make(map[string]string)

	// Iterate through environment variables
	for _, e := range os.Environ() {
		key, value := parseEnvVarByPrefix(e, prefix)
		if key != "" && value != "" {
			overrides[key] = value
		}
	}

	if len(overrides) > 0 {
		logf.Info(fmt.Sprintf("Found overrides from environment variables set by %s prefix", prefix))
	}

	return overrides
}

/*
parseEnvVarByPrefix parses the environment variable and extracts key and value.
*/
func parseEnvVarByPrefix(envVar, prefix string) (key string, value string) {
	pair := strings.SplitN(envVar, "=", 2)

	if len(pair) == 2 && strings.HasPrefix(pair[0], prefix) {
		key = strings.ToLower(strings.TrimPrefix(pair[0], prefix))
		value = pair[1]
	}
	return key, value
}
