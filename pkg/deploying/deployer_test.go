package deploying

import (
	"context"
	"encoding/json"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNewDeployment(t *testing.T) {
	fakeclient := fake.NewFakeClient()
	dep, err := toUnstructuredObj(newDeployment("dep", "ns", 1))
	if err != nil {
		t.Fatalf("failed to generate deployment %v", err)
	}
	err = Deploy(fakeclient, dep)
	if err != nil {
		t.Fatalf("failed to deploy deployment %v", err)
	}
	expected := &unstructured.Unstructured{}
	expected.SetGroupVersionKind(dep.GroupVersionKind())
	err = fakeclient.Get(context.TODO(), types.NamespacedName{Name: "dep", Namespace: "ns"}, expected)
	if err != nil {
		t.Fatalf("failed to find deployment %v", err)
	}
}

func TestUpdateDeployment(t *testing.T) {
	fakeclient := fake.NewFakeClient()
	olddep, _ := toUnstructuredObj(newDeployment("dep", "ns", 1))
	Deploy(fakeclient, olddep)

	newdep, _ := toUnstructuredObj(newDeployment("dep", "ns", 2))
	if err := Deploy(fakeclient, newdep); err != nil {
		t.Fatalf("failed to update deployment %v", err)
	}
	expected := &unstructured.Unstructured{}
	expected.SetGroupVersionKind(olddep.GroupVersionKind())
	fakeclient.Get(context.TODO(), types.NamespacedName{Name: "dep", Namespace: "ns"}, expected)
	spec, _, _ := unstructured.NestedMap(expected.Object, "spec")
	replicas, _, _ := unstructured.NestedInt64(spec, "replicas")
	if replicas != 2 {
		t.Fatalf("expect 2, but %d", replicas)
	}
}

func TestListDeployments(t *testing.T) {
	fakeclient := fake.NewFakeClient()
	fakeclient.Create(context.TODO(), newDeployment("multicloudhub-operator", "ns", 1))
	fakeclient.Create(context.TODO(), newDeployment("dep1", "ns", 1))
	fakeclient.Create(context.TODO(), newDeployment("dep2", "ns", 1))
	fakeclient.Create(context.TODO(), newDeployment("dep3", "ns1", 1))
	fakeclient.Create(context.TODO(), newDeployment("dep4", "ns2", 1))

	_, list, err := ListDeployments(fakeclient, "ns")
	if err != nil {
		t.Fatalf("failed with %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expect 2, but %d", len(list))
	}
}

func newDeployment(name, namespace string, replicas int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "extensions/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
	}
}

func toUnstructuredObj(obj runtime.Object) (*unstructured.Unstructured, error) {
	content, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	u := &unstructured.Unstructured{}
	err = u.UnmarshalJSON(content)
	return u, err
}
