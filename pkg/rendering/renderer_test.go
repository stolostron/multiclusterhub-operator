// Copyright (c) 2020 Red Hat, Inc.

package rendering

import (
	"os"
	"path"
	"testing"

	operatorsv1beta1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1beta1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/rendering/templates"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestRender(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir %v", err)
	}
	templatesPath := path.Join(path.Dir(path.Dir(wd)), "templates")
	os.Setenv(templates.TemplatesPathEnvVar, templatesPath)
	defer os.Unsetenv(templates.TemplatesPathEnvVar)

	mchcr := &operatorsv1beta1.MultiClusterHub{
		TypeMeta:   metav1.TypeMeta{Kind: "MultiClusterHub"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: operatorsv1beta1.MultiClusterHubSpec{
			ImagePullSecret: "test",
			Mongo:           operatorsv1beta1.Mongo{},
		},
	}

	imageOverrides := map[string]string{
		"multicloud-manager": "quay.io/open-cluster-management/multicloud-manager@sha256:image-digest-sha",
	}

	renderer := NewRenderer(mchcr, imageOverrides)
	objs, err := renderer.Render(nil)
	if err != nil {
		t.Fatalf("failed to render multiclusterhub %v", err)
	}

	printObjs(t, objs)
}

func printObjs(t *testing.T, objs []*unstructured.Unstructured) {
	for _, obj := range objs {
		t.Log(obj)
	}
}
