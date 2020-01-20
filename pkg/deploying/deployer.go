package deploying

import (
	"context"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func Deploy(c runtimeclient.Client, obj *unstructured.Unstructured) error {
	found := &unstructured.Unstructured{}
	found.SetGroupVersionKind(obj.GroupVersionKind())
	err := c.Get(context.TODO(), types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, found)
	if err != nil {
		if errors.IsNotFound(err) {
			return c.Create(context.TODO(), obj)
		}
		return err
	}

	if found.GetKind() != "Deployment" {
		return nil
	}

	oldSpec, oldSpecFound := found.Object["spec"]
	newSpec, newSpecFound := obj.Object["spec"]
	if !oldSpecFound || !newSpecFound {
		return nil
	}
	if !reflect.DeepEqual(oldSpec, newSpec) {
		newObj := found.DeepCopy()
		newObj.Object["spec"] = newSpec
		return c.Update(context.TODO(), newObj)
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
		if deploy.Name == "multicloudhub-operator" {
			continue
		}
		if deploy.Status.UnavailableReplicas != 0 {
			ready = false
		}
		deployments = append(deployments, deploy)

	}
	return ready, deployments, nil
}
