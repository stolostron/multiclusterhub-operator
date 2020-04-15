package multiclusterhub

import (
	"encoding/json"
	"os"
	"path"
	"strings"

	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
)

// This does not include every available attribute from manifest file, just the attributes we need
type manifestImage struct {
	Name           string `json:"name"`
	ManifestSha256 string `json:"manifest-sha256"`
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

// GetImageShaDigest Reads and formats image sha digest values from image manifest file.
func (r *ReconcileMultiClusterHub) GetImageShaDigest(version string) (map[string]string, error) {

	manifestData, err := readManifestFile(version)

	if err != nil {
		return nil, err
	}

	imageShaDigests, err := formatImageShaDigests(manifestData)

	if err != nil {
		return nil, err
	}

	return imageShaDigests, nil
}

// ManifestFileExists Has an image manifest file been provided?
func (r *ReconcileMultiClusterHub) ManifestFileExists(version string) bool {
	filepath, err := getManifestFilePath(version)
	if err != nil {
		return false
	}
	return fileExists(filepath)
}

func formatImageShaDigests(manifestData []byte) (map[string]string, error) {
	manifestFile := []manifestImage{}
	err := json.Unmarshal(manifestData, &manifestFile)
	if err != nil {
		return nil, err
	}

	imageShaDigests := make(map[string]string)
	for _, img := range manifestFile {
		if img.Name != "" && img.ManifestSha256 != "" {
			imageShaDigests[strings.ReplaceAll(img.Name, "-", "_")] = img.ManifestSha256
		}
	}
	return imageShaDigests, nil
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
