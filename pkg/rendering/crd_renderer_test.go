// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package rendering

import (
	"errors"
	"os"
	"path"
	"testing"

	operatorsv1 "github.com/open-cluster-management/multiclusterhub-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCRDRender(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir %v", err)
	}
	testCrdsPath := path.Join(wd, "testdata")

	mch := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
	}

	t.Run("New renderer", func(t *testing.T) {
		os.Setenv(CRDsPathEnvVar, testCrdsPath)
		defer os.Unsetenv(CRDsPathEnvVar)

		_, err := NewCRDRenderer(mch)
		if err != nil {
			t.Errorf("NewCRDRenderer() error = %v, wantErr %v", err, nil)
		}
	})

	t.Run("Missing env variable", func(t *testing.T) {
		os.Unsetenv(CRDsPathEnvVar)

		_, err := NewCRDRenderer(mch)
		if err == nil {
			wantErr := errors.New("CRDS_PATH environment variable is required")
			t.Errorf("CRDRenderer.Render() error = %v, wantErr %v", err, wantErr)
		}
	})

	t.Run("Render successfully", func(t *testing.T) {
		renderer := &CRDRenderer{
			directory: path.Join(testCrdsPath, "success"),
			cr:        mch,
		}
		_, errs := renderer.Render()
		if errs != nil {
			t.Errorf("CRDRenderer.Render() error = %v, wantErr %v", err, nil)
		}
	})

	t.Run("Render error", func(t *testing.T) {
		renderer := &CRDRenderer{
			directory: path.Join(testCrdsPath, "failure"),
			cr:        mch,
		}
		_, errs := renderer.Render()
		if len(errs) != 2 {
			t.Errorf("CRDRenderer.Render() error = %v, wanted %d errors", err, 2)
		}
	})
}
