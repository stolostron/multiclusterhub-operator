// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package imageoverrides

import (
	"os"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

var logf = log.Log.WithName("controller_multiclusterhub")

const (
	// OSBSImagePrefix ...
	OSBSImagePrefix = "RELATED_IMAGE_"

	// OperandImagePrefix ...
	OperandImagePrefix = "OPERAND_IMAGE_"
)

// GetImageOverrides Reads and formats full image reference from image manifest file.
func GetImageOverrides() map[string]string {
	imageOverrides := make(map[string]string)

	// First check for environment variables containing the 'OPERAND_IMAGE_' prefix
	for _, e := range os.Environ() {
		keyValuePair := strings.SplitN(e, "=", 2)
		if strings.HasPrefix(keyValuePair[0], OperandImagePrefix) {
			key := strings.ToLower(strings.Replace(keyValuePair[0], OperandImagePrefix, "", -1))
			imageOverrides[key] = keyValuePair[1]
		}
	}

	// If entries exist containing operand image prefix, return
	if len(imageOverrides) > 0 {
		logf.Info("Found image overrides from environment variables set by operand image prefix")
		return imageOverrides
	}

	// If no image overrides found, check 'RELATED_IMAGE_' prefix
	for _, e := range os.Environ() {
		keyValuePair := strings.SplitN(e, "=", 2)
		if strings.HasPrefix(keyValuePair[0], OSBSImagePrefix) {
			key := strings.ToLower(strings.Replace(keyValuePair[0], OSBSImagePrefix, "", -1))
			imageOverrides[key] = keyValuePair[1]
		}
	}

	// If entries exist containing related image prefix, return
	if len(imageOverrides) > 0 {
		logf.Info("Found image overrides from environment variables set by related image prefix")
	}

	return imageOverrides
}
