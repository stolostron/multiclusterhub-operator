// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package imageoverrides

import (
	"os"
	"strings"
)

const (
	// TemplateOverridePrefix ...
	TemplateOverridePrefix = "TEMPLATE_OVERRIDE"
)

// GetTemplateOverrides Reads and formats full image reference from template manifest file.
func GetTemplateOverrides() map[string]string {
	templateOverrides := make(map[string]string)

	// First check for environment variables containing the 'OPERAND_LIMIT_' prefix
	for _, e := range os.Environ() {
		keyValuePair := strings.SplitN(e, "=", 2)
		if strings.HasPrefix(keyValuePair[0], TemplateOverridePrefix) {
			key := strings.ToLower(strings.Replace(keyValuePair[0], TemplateOverridePrefix, "", -1))
			templateOverrides[key] = keyValuePair[1]
		}
	}

	// If entries exist containing operand limit prefix, return
	if len(templateOverrides) > 0 {
		logf.Info("Found image overrides from environment variables set by operand image prefix")
		return templateOverrides
	}

	return templateOverrides
}
