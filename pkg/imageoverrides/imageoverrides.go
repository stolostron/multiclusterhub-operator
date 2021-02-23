// Copyright (c) 2020 Red Hat, Inc.

package imageoverrides

import (
	"os"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

var logf = log.Log.WithName("controller_multiclusterhub")

// imagePrefix ...
const imagePrefix = "RELATED_IMAGE_"

// GetImageOverrides Reads and formats full image reference from image manifest file.
func GetImageOverrides() map[string]string {
	imageOverrides := make(map[string]string)
	for _, e := range os.Environ() {
		keyValuePair := strings.SplitN(e, "=", 2)

		if strings.HasPrefix(keyValuePair[0], imagePrefix) {
			key := strings.ToLower(strings.Replace(keyValuePair[0], imagePrefix, "", -1))
			imageOverrides[key] = keyValuePair[1]
		}
	}

	if len(imageOverrides) > 0 {
		logf.Info("Found image overrides from environment variables")
	}

	return imageOverrides
}
