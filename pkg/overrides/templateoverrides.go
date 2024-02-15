// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package overrides

import (
	"os"
	"strings"
)

const (
	// TemplateOverridePrefix ...
	TemplateOverridePrefix = "TEMPLATE_OVERRIDE_"
)

// GetTemplateOverrides reads and formats full image reference from environment variables.
func GetTemplateOverrides() map[string]string {
	templateOverrides := make(map[string]string)

	// Iterate through environment variables
	for _, e := range os.Environ() {
		key, value := parseEnvVar(e)
		if key != "" && value != "" {
			templateOverrides[key] = value
		}
	}

	// Check if any overrides were found
	if len(templateOverrides) > 0 {
		logf.Info("Found image overrides from environment variables set by operand image prefix")
	}

	return templateOverrides
}

// parseEnvVar parses the environment variable and extracts key and value
func parseEnvVar(envVar string) (key string, value string) {
	pair := strings.SplitN(envVar, "=", 2)
	if len(pair) == 2 && strings.HasPrefix(pair[0], TemplateOverridePrefix) {
		key = strings.ToLower(strings.TrimPrefix(pair[0], TemplateOverridePrefix))
		value = pair[1]
	}
	return key, value
}
