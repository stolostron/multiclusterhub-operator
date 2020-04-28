// Copyright (c) 2020 Red Hat, Inc.

package deploying

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("deployer")

func Deploy(c runtimeclient.Client, obj *unstructured.Unstructured) error {
	found := &unstructured.Unstructured{}
	found.SetGroupVersionKind(obj.GroupVersionKind())
	err := c.Get(context.TODO(), types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, found)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Creating resource", "Kind", obj.GetKind(), "Name", obj.GetName())
			return c.Create(context.TODO(), obj)
		}
		return err
	}
	return nil
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
