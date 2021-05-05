// Copyright (c) 2020 Red Hat, Inc.

package rendering

import (
	"errors"
	"os"
	"path"
	"testing"

	operatorsv1 "github.com/open-cluster-management/multiclusterhub-operator/pkg/apis/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCRDRender(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir %v", err)
	}
	crdsPath := path.Join(path.Dir(path.Dir(wd)), "crds")

	mch := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
	}

	t.Run("Missing env variable", func(t *testing.T) {
		os.Unsetenv(CRDsPathEnvVar)

		_, err := NewCRDRenderer(mch)
		if err == nil {
			wantErr := errors.New("CRDS_PATH environment variable is required")
			t.Errorf("CRDRenderer.Render() error = %v, wantErr %v", err, wantErr)
		}
	})

	t.Run("Render successfully", func(t *testing.T) {
		os.Setenv(CRDsPathEnvVar, crdsPath)
		defer os.Unsetenv(CRDsPathEnvVar)

		renderer, err := NewCRDRenderer(mch)
		_, err = renderer.Render()
		if err != nil {
			t.Errorf("CRDRenderer.Render() error = %v, wantErr %v", err, nil)
		}
	})
}
