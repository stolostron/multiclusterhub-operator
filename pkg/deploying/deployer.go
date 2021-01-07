// Copyright (c) 2020 Red Hat, Inc.

package deploying

import (
	"context"
	"crypto/sha1" // #nosec G505 (not using sha for private encryption)
	"encoding/hex"

	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

var log = logf.Log.WithName("deployer")
var hashAnnotation = utils.AnnotationConfiguration

// Deploy attempts to create or update the obj resource depending on whether it exists.
// Returns true if deploy does try to create a new resource
func Deploy(c runtimeclient.Client, obj *unstructured.Unstructured) (error, bool) {
	found := &unstructured.Unstructured{}
	found.SetGroupVersionKind(obj.GroupVersionKind())
	err := c.Get(context.TODO(), types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, found)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Creating resource", "Kind", obj.GetKind(), "Name", obj.GetName())
			if kind := found.GetKind(); kind == "ServiceAccount" || kind == "CustomResourceDefinition" {
				annotate(obj)
			}
			return c.Create(context.TODO(), obj), true
		}
		return err, false
	}

	// Do not update webhook configurations or cert secrets
	if kind := found.GetKind(); kind == "MutatingWebhookConfiguration" || kind == "ValidatingWebhookConfiguration" {
		if name := found.GetName(); name == "ocm-mutating-webhook" || name == "ocm-validating-webhook" {
			return nil, false
		}
	}
	if kind := found.GetKind(); kind == "Secret" {
		if name := found.GetName(); name == "ocm-klusterlet-self-signed-secrets" || name == "ocm-webhook-secret" {
			return nil, false
		}
	}

	// Update if hash doesn't match
	if kind := found.GetKind(); kind == "ServiceAccount" || kind == "CustomResourceDefinition" {
		if shasMatch(found, obj) {
			return nil, false
		}
		annotate(obj)
	}

	// If resources exists, update it with current config
	obj.SetResourceVersion(found.GetResourceVersion())
	return c.Update(context.TODO(), obj), false
}

func ListDeployments(c runtimeclient.Client, namespace string) (bool, []appsv1.Deployment, error) {
	deployments := []appsv1.Deployment{}
	deployList := &appsv1.DeploymentList{}
	if err := c.List(context.TODO(), deployList, runtimeclient.InNamespace(namespace)); err != nil {
		return false, deployments, err
	}
	ready := true
	for _, deploy := range deployList.Items {
		if deploy.Name == "multiclusterhub-operator" {
			continue
		}
		if deploy.Status.UnavailableReplicas != 0 {
			ready = false
		}
		deployments = append(deployments, deploy)

	}
	return ready, deployments, nil
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

// annotated modifies a deployment and sets an annotation with the hash of the deployment spec
func annotate(u *unstructured.Unstructured) {
	var log = logf.Log.WithValues("Namespace", u.GetNamespace(), "Name", u.GetName())

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

func shasMatch(found, want *unstructured.Unstructured) bool {
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
