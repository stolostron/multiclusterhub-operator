// Copyright (c) 2020 Red Hat, Inc.

package rendering

import (
	"errors"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	operatorsv1 "github.com/open-cluster-management/multiclusterhub-operator/pkg/apis/operator/v1"
	"github.com/open-cluster-management/multiclusterhub-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

const CRDsPathEnvVar = "CRDS_PATH"

type CRDRenderer struct {
	directory string
	cr        *operatorsv1.MultiClusterHub
}

func NewCRDRenderer(mch *operatorsv1.MultiClusterHub) (*CRDRenderer, error) {
	crdDir, found := os.LookupEnv(CRDsPathEnvVar)
	if !found {
		missingEnvErr := errors.New("CRDS_PATH environment variable is required")
		return nil, missingEnvErr
	}
	return &CRDRenderer{
		directory: crdDir,
		cr:        mch,
	}, nil
}

// Render renders Templates under TEMPLATES_PATH
func (r *CRDRenderer) Render() ([]*unstructured.Unstructured, error) {
	var crds []*unstructured.Unstructured

	// Read CRD files
	files, err := ioutil.ReadDir(r.directory)
	if err != nil {
		return nil, err
	}

	var crdBytes [][]byte
	for _, file := range files {
		if filepath.Ext(file.Name()) != ".yaml" {
			continue
		}
		filePath := path.Join(r.directory, file.Name())
		src, err := ioutil.ReadFile(filepath.Clean(filePath)) // #nosec G304 (filepath cleaned)
		if err != nil {
			return nil, err
		}
		crdBytes = append(crdBytes, src)
	}

	// Convert bytes to Unstructured resources
	for _, file := range crdBytes {
		crd := &unstructured.Unstructured{}
		if err = yaml.Unmarshal(file, crd); err != nil {
			return nil, err
		}

		// Check that it is actually a CRD
		crdKind, _, err := unstructured.NestedString(crd.Object, "spec", "names", "kind")
		if err != nil {
			return nil, err
		}
		crdGroup, _, err := unstructured.NestedString(crd.Object, "spec", "group")
		if err != nil {
			return nil, err
		}

		if crd.GetKind() != "CustomResourceDefinition" || crdKind == "" || crdGroup == "" {
			continue
		}

		utils.AddInstallerLabel(crd, r.cr.GetName(), r.cr.GetNamespace())
		crds = append(crds, crd)
	}

	// Return resource list
	return crds, nil
}
