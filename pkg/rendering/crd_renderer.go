// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package rendering

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
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
func (r *CRDRenderer) Render() ([]*unstructured.Unstructured, []error) {
	var crds []*unstructured.Unstructured
	errs := []error{}

	// Read CRD files
	files, err := ioutil.ReadDir(r.directory)
	if err != nil {
		return nil, []error{err}
	}

	// Convert bytes to Unstructured resources
	for _, file := range files {
		if filepath.Ext(file.Name()) != ".yaml" {
			continue
		}

		filePath := path.Join(r.directory, file.Name())
		src, err := ioutil.ReadFile(filepath.Clean(filePath)) // #nosec G304 (filepath cleaned)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s - error reading file: %v", file.Name(), err.Error()))
			continue
		}

		crd := &unstructured.Unstructured{}
		if err = yaml.Unmarshal(src, crd); err != nil {
			errs = append(errs, fmt.Errorf("%s - error unmarshalling file to unstructured: %v", file.Name(), err.Error()))
			continue
		}

		// Check that it is actually a CRD
		crdKind, _, err := unstructured.NestedString(crd.Object, "spec", "names", "kind")
		if err != nil {
			errs = append(errs, fmt.Errorf("%s - error getting Kind field: %v", file.Name(), err.Error()))
			continue
		}
		crdGroup, _, err := unstructured.NestedString(crd.Object, "spec", "group")
		if err != nil {
			errs = append(errs, fmt.Errorf("%s - error getting Group field: %v", file.Name(), err.Error()))
			continue
		}

		if crd.GetKind() != "CustomResourceDefinition" || crdKind == "" || crdGroup == "" {
			errs = append(errs, fmt.Errorf("%s - CRD file bad format", file.Name()))
			continue
		}

		utils.AddInstallerLabel(crd, r.cr.GetName(), r.cr.GetNamespace())
		crds = append(crds, crd)
	}

	if len(errs) > 0 {
		return crds, errs
	}

	// Return resource list
	return crds, nil
}
