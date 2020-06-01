// Copyright (c) 2020 Red Hat, Inc.

package templates

import (
	"os"
	"path"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorsv1beta1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1beta1"
)

func TestGetCoreTemplates(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir %v", err)
	}
	templatesPath := path.Join(path.Dir(path.Dir(path.Dir(wd))), "templates")
	os.Setenv(TemplatesPathEnvVar, templatesPath)
	defer os.Unsetenv(TemplatesPathEnvVar)

	mchcr := &operatorsv1beta1.MultiClusterHub{
		TypeMeta:   metav1.TypeMeta{Kind: "MultiClusterHub"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec:       operatorsv1beta1.MultiClusterHubSpec{},
		Status: operatorsv1beta1.MultiClusterHubStatus{
			CurrentVersion: "1.0.1",
		},
	}
	_, err = GetTemplateRenderer().GetTemplates(mchcr)

	if err != nil {
		t.Fatalf("failed to render core template %v", err)
	}
}
