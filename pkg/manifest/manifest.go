// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	operatorsv1 "github.com/stolostron/multiclusterhub-operator/pkg/apis/operator/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	"github.com/stolostron/multiclusterhub-operator/version"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("manifest")

const ManifestsPathEnvVar = "MANIFESTS_PATH"

// ManifestImage contains details for a specific image version
type ManifestImage struct {
	ImageKey     string `json:"image-key"`
	ImageName    string `json:"image-name"`
	ImageVersion string `json:"image-version"`

	// remote registry where image is stored
	ImageRemote string `json:"image-remote"`

	// immutable sha version identifier
	ImageDigest string `json:"image-digest"`

	ImageTag string `json:"image-tag"`
}

// GetImageOverrides Reads and formats full image reference from image manifest file.
func GetImageOverrides(mch *operatorsv1.MultiClusterHub) (map[string]string, error) {
	manifestData, err := readManifestFile(version.Version)
	if err != nil {
		return nil, err
	}

	var manifestImages []ManifestImage
	err = json.Unmarshal(manifestData, &manifestImages)
	if err != nil {
		return nil, err
	}

	imageOverrides, err := formatImageOverrides(mch, manifestImages)
	if err != nil {
		return nil, err
	}

	return imageOverrides, nil
}

func formatImageOverrides(mch *operatorsv1.MultiClusterHub, manifestImages []ManifestImage) (map[string]string, error) {
	imageOverrides := make(map[string]string)
	for _, img := range manifestImages {
		imageOverrides[img.ImageKey] = buildFullImageReference(mch, img)
	}
	return imageOverrides, nil
}

func buildFullImageReference(mch *operatorsv1.MultiClusterHub, mi ManifestImage) string {
	registry := mi.ImageRemote
	// Use ImageRepository if provided
	if reg := utils.GetImageRepository(mch); reg != "" {
		registry = reg
	}
	return manifestFormat(mi, registry)
}

func manifestFormat(mi ManifestImage, registry string) string {
	image := mi.ImageName
	digest := mi.ImageDigest
	return fmt.Sprintf("%s/%s@%s", registry, image, digest)
}

// readManifestFile returns the byte content of a versioned image manifest file
func readManifestFile(version string) ([]byte, error) {
	manifestsPath, found := os.LookupEnv(ManifestsPathEnvVar)
	if !found {
		missingEnvErr := errors.New("MANIFESTS_PATH environment variable is required")
		return nil, missingEnvErr
	}

	filePath := path.Join(manifestsPath, version+".json")
	contents, err := ioutil.ReadFile(filepath.Clean(filePath)) // #nosec G304 (filepath cleaned)
	if err != nil {
		log.Error(err, "Failed to read image manifest", "Path", filePath)
		return nil, err
	}
	return contents, nil
}
