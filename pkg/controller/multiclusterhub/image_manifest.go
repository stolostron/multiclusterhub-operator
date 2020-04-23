package multiclusterhub

import (
	"encoding/json"
	"fmt"
	"os"
	"path"

	operatorsv1beta1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1beta1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
)

// This does not include every available attribute from manifest file, just the attributes we need
type manifestImage struct {
	ImageKey     string `json:"image-key"`
	ImageName    string `json:"image-name"`
	ImageVersion string `json:"image-version:"`
	ImageRemote  string `json:"image-remote"`
	ImageDigest  string `json:"image-digest"`
}

// if naming convention for manifest file changes, update this file (and Dockerfile and constants in utils)
func getManifestFilePath(version string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Error(err, "Couldn't get user home directory")
		return "", err
	}
	return path.Join(home, utils.ImageManifestsDir, version+".json"), nil
}

// GetImageOverrides Reads and formats full image reference from image manifest file.
func GetImageOverrides(mch *operatorsv1beta1.MultiClusterHub) (map[string]string, error) {

	version := mch.Status.CurrentVersion
	manifestData, err := readManifestFile(version)

	if err != nil {
		return nil, err
	}

	manifestImages := []manifestImage{}
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

// ManifestFileExists Has an image manifest file been provided?
func ManifestFileExists(version string) bool {
	filepath, err := getManifestFilePath(version)
	if err != nil {
		return false
	}
	return fileExists(filepath)
}

func buildFullImageReference(mch *operatorsv1beta1.MultiClusterHub, mi manifestImage) string {

	var useRegistry string
	if useRegistry = mi.ImageRemote; mch.Spec.Overrides.ImageRepository != "" {
		useRegistry = mch.Spec.Overrides.ImageRepository
	}
	imageRegistryAndName := fmt.Sprintf("%s/%s", useRegistry, mi.ImageName)
	var fullImageReference string
	if mch.Spec.Overrides.ImageTagSuffix != "" {
		fullImageReference = fmt.Sprintf("%s:%s-%s", imageRegistryAndName, mi.ImageVersion, mch.Spec.Overrides.ImageTagSuffix)
	} else {
		fullImageReference = fmt.Sprintf("%s@%s", imageRegistryAndName, mi.ImageDigest)
	}
	return fullImageReference
}

func formatImageOverrides(mch *operatorsv1beta1.MultiClusterHub, manifestImages []manifestImage) (map[string]string, error) {

	imageOverrides := make(map[string]string)
	for _, mi := range manifestImages {
		fullImageRef := buildFullImageReference(mch, mi)
		imageOverrides[mi.ImageKey] = fullImageRef
	}
	return imageOverrides, nil
}

func readManifestFile(version string) ([]byte, error) {

	filepath, err := getManifestFilePath(version)
	if err != nil {
		return nil, err
	}

	contents, err := readFileRaw(filepath)
	if err != nil {
		return nil, err
	}
	return contents, nil
}
