// Copyright (c) 2020 Red Hat, Inc.

package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	operatorsv1beta1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1beta1"
	"github.com/open-cluster-management/multicloudhub-operator/version"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("manifest")

const ManifestsPathEnvVar = "MANIFESTS_PATH"

// OverrideType is an enumeration of possible image format options
type OverrideType int

const (
	Unknown OverrideType = iota
	Manifest
	Suffix
)

// ManifestImage contains details for a specific image version
type ManifestImage struct {
	ImageKey     string `json:"image-key"`
	ImageName    string `json:"image-name"`
	ImageVersion string `json:"image-version"`

	// remote registry where image is stored
	ImageRemote string `json:"image-remote"`

	// immutable sha version identifier
	ImageDigest string `json:"image-digest"`
}

// GetImageOverrideType returns an image format type based on the MultiClusterHub
// object content
func GetImageOverrideType(m *operatorsv1beta1.MultiClusterHub) OverrideType {
	if m.Spec.Overrides.ImageTagSuffix == "" {
		return Manifest
	} else {
		return Suffix
	}
}

// GetImageOverrides Reads and formats full image reference from image manifest file.
func GetImageOverrides(mch *operatorsv1beta1.MultiClusterHub) (map[string]string, error) {
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

	// TODO: Remove hardcoded image once in pipeline
	if _, ok := imageOverrides["oauth_proxy"]; !ok {
		// image digest equivalent of `origin-oauth-proxy:4.5`
		imageOverrides["oauth_proxy"] = "quay.io/openshift/origin-oauth-proxy@sha256:8599745a5bf914a3e00efa08f9ddd2c409de91c551d330b80e683bca52aa2146"
	}

	return imageOverrides, nil
}

func formatImageOverrides(mch *operatorsv1beta1.MultiClusterHub, manifestImages []ManifestImage) (map[string]string, error) {
	imageOverrides := make(map[string]string)
	for _, img := range manifestImages {
		imageOverrides[img.ImageKey] = buildFullImageReference(mch, img)
	}
	return imageOverrides, nil
}

func buildFullImageReference(mch *operatorsv1beta1.MultiClusterHub, mi ManifestImage) string {
	registry := mi.ImageRemote
	// Use ImageRepository if provided
	if mch.Spec.Overrides.ImageRepository != "" {
		registry = mch.Spec.Overrides.ImageRepository
	}

	switch imageFormat := GetImageOverrideType(mch); imageFormat {
	case Suffix:
		suffix := mch.Spec.Overrides.ImageTagSuffix
		return suffixFormat(mi, registry, suffix)
	case Manifest:
		fallthrough
	default:
		// Default is Manifest format
		return manifestFormat(mi, registry)
	}
}

func manifestFormat(mi ManifestImage, registry string) string {
	image := mi.ImageName
	digest := mi.ImageDigest
	return fmt.Sprintf("%s/%s@%s", registry, image, digest)
}

func suffixFormat(mi ManifestImage, registry string, suffix string) string {
	image := mi.ImageName
	version := mi.ImageVersion
	return fmt.Sprintf("%s/%s:%s-%s", registry, image, version, suffix)
}

// readManifestFile returns the byte content of a versioned image manifest file
func readManifestFile(version string) ([]byte, error) {
	manifestsPath, found := os.LookupEnv(ManifestsPathEnvVar)
	if !found {
		missingEnvErr := errors.New("MANIFESTS_PATH environment variable is required")
		return nil, missingEnvErr
	}

	filePath := path.Join(manifestsPath, version+".json")
	contents, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Error(err, "Failed to read image manifest", "Path", filePath)
		return nil, err
	}
	return contents, nil
}
