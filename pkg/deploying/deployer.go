// Copyright (c) 2026 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

// Package deploying handles component deployment operations.
//
// This package provides deployment orchestration for ACM components.
package deploying

import (
	"context"
	"crypto/sha1" // #nosec G505 (not using sha for private encryption)
	"encoding/hex"

	"github.com/go-logr/logr"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

var hashAnnotation = utils.AnnotationConfiguration

// Deploy attempts to create or update the obj resource depending on whether it exists.
// Returns true if deploy does try to create a new resource
func Deploy(log logr.Logger, c runtimeclient.Client, obj *unstructured.Unstructured) (error, bool) {
	found := &unstructured.Unstructured{}
	found.SetGroupVersionKind(obj.GroupVersionKind())
	err := c.Get(context.TODO(), types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, found)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Creating resource", "Kind", obj.GetKind(), "Name", obj.GetName())
			if kind := found.GetKind(); kind == "ServiceAccount" || kind == "CustomResourceDefinition" {
				annotate(log, obj)
			}
			return c.Create(context.TODO(), obj), true
		}
		return err, false
	}

	// Do not update cert secrets

	if kind := found.GetKind(); kind == "Secret" {
		if name := found.GetName(); name == "ocm-klusterlet-self-signed-secrets" {
			return nil, false
		}
	}
	// Update if hash doesn't match
	if kind := found.GetKind(); kind == "ServiceAccount" || kind == "CustomResourceDefinition" {
		if shasMatch(log, found, obj) {
			return nil, false
		}
		annotate(log, obj)
	}

	// If resources exists, update it with current config
	obj.SetResourceVersion(found.GetResourceVersion())
	return c.Update(context.TODO(), obj), false
}

func hash(u *unstructured.Unstructured) (string, error) {
	spec, err := yaml.Marshal(u.Object)
	if err != nil {
		return "", err
	}
	h := sha1.New() // #nosec G401 (not using sha for private encryption)
	_, err = h.Write(spec)
	if err != nil {
		return "", err
	}
	bs := h.Sum(nil)
	return hex.EncodeToString(bs), nil
}

func annotate(log logr.Logger, u *unstructured.Unstructured) {
	log = log.WithValues("Namespace", u.GetNamespace(), "Name", u.GetName())

	hx, err := hash(u)
	if err != nil {
		log.Error(err, "Couldn't marshal deployment spec. Hash not assigned.")
	}

	if anno := u.GetAnnotations(); anno == nil {
		u.SetAnnotations(map[string]string{hashAnnotation: hx})
	} else {
		anno[hashAnnotation] = hx
		u.SetAnnotations(anno)
	}
}

func shasMatch(log logr.Logger, found, want *unstructured.Unstructured) bool {
	hx, err := hash(want)
	if err != nil {
		log.Error(err, "Couldn't marshal object spec.", "Name", found.GetName())
	}

	if existing := found.GetAnnotations()[hashAnnotation]; existing != hx {
		log.Info("Hashes don't match. Update needed.", "Name", want.GetName(), "Existing sha", existing, "New sha", hx)
		return false
	} else {
		return true
	}
}
