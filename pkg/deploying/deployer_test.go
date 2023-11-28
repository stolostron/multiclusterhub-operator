// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package deploying

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

/*
TestNewDeployment tests the creation and deployment of a new Kubernetes Deployment.
It uses a fake client for testing purposes and verifies that the deployment is successfully created and deployed.
This test checks the entire process from creation to deployment and retrieval.
*/
func TestNewDeployment(t *testing.T) {
	fakeclient := fake.NewClientBuilder().Build()
	dep, err := toUnstructuredObj(newDeployment("dep", "ns", 1))
	if err != nil {
		t.Fatalf("failed to generate deployment %v", err)
	}
	err, _ = Deploy(fakeclient, dep)
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

/*
TestListDeployments tests the listing of Deployments in a specific namespace.
It uses a fake client for testing purposes and creates multiple Deployments with different names and namespaces.
The test ensures that the ListDeployments function returns the expected number of Deployments in the specified namespace.
*/
func TestListDeployments(t *testing.T) {
	fakeclient := fake.NewClientBuilder().Build()
	fakeclient.Create(context.TODO(), newDeployment("multiclusterhub-operator", "ns", 1))
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

/*
newSA creates and returns a new Kubernetes ServiceAccount as an unstructured object.
The ServiceAccount is named "test" and belongs to the namespace "test".
*/
func newSA() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("v1")
	u.SetKind("ServiceAccount")
	u.SetName("test")
	u.SetNamespace("test")
	return u
}

/*
TestRepeatedDeploy tests the repeated deployment of a Kubernetes resource.
It uses a fake client for testing purposes and deploys a ServiceAccount multiple times, checking for expected behavior.
The test also verifies that annotations are properly added and updated during the repeated deployments.
*/
func TestRepeatedDeploy(t *testing.T) {
	fakeclient := fake.NewClientBuilder().Build()

	err, new := Deploy(fakeclient, newSA())
	if err != nil {
		t.Fatalf("failed to deploy service account: %v", err)
	}
	if new != true {
		t.Fatalf("Deploy() didn't create service account")
	}

	err, new = Deploy(fakeclient, newSA())
	if err != nil {
		t.Fatalf("failed to deploy service account: %v", err)
	}
	if new != false {
		t.Fatalf("Deploy() shouldn't create service account twice")
	}

	expected := &unstructured.Unstructured{}
	expected.SetGroupVersionKind(schema.GroupVersionKind{
		Kind:    "ServiceAccount",
		Version: "v1",
	})

	err = fakeclient.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "test"}, expected)
	if err != nil {
		t.Errorf("failed to find service account %v", err)
	}
	firstHash := expected.GetAnnotations()[utils.AnnotationConfiguration]
	if firstHash == "" {
		t.Errorf("service account has no sha annotation")
	}

	// Change resource and deploy again
	annotatedSA := newSA()
	annotatedSA.SetAnnotations(map[string]string{"foo": "bar"})
	err, _ = Deploy(fakeclient, annotatedSA)
	if err != nil {
		t.Fatalf("failed to deploy service account: %v", err)
	}

	expected2 := &unstructured.Unstructured{}
	expected2.SetGroupVersionKind(schema.GroupVersionKind{
		Kind:    "ServiceAccount",
		Version: "v1",
	})

	err = fakeclient.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "test"}, expected2)
	if err != nil {
		t.Errorf("failed to find service account %v", err)
	}
	secondHash := expected2.GetAnnotations()[utils.AnnotationConfiguration]
	if secondHash == firstHash {
		t.Errorf("Hash should not match; %s == %s", firstHash, secondHash)
	}

	if expected2.GetAnnotations()["foo"] != "bar" {
		t.Errorf("Annotation no longer present: got %s, wanted %s", expected2.GetAnnotations()["foo"], "bar")
	}

}

/*
newDeployment creates and returns a new Kubernetes Deployment with the specified name, namespace, and replicas.
The API version is set to "extensions/v1beta1".
*/
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

/*
toUnstructuredObj converts a Kubernetes runtime.Object to an unstructured.Unstructured object.
It uses JSON marshaling and unmarshaling to perform the conversion.
Returns the unstructured object and any error encountered during the process.
*/

func toUnstructuredObj(obj runtime.Object) (*unstructured.Unstructured, error) {
	content, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	u := &unstructured.Unstructured{}
	err = u.UnmarshalJSON(content)
	return u, err
}

/*
Test_shasMatch tests the shasMatch function, which compares the SHA annotations of two unstructured objects.
It creates test cases with unstructured objects having matching and non-matching SHA annotations.
The test ensures that shasMatch produces the expected results for different scenarios.
*/
func Test_shasMatch(t *testing.T) {
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind": "Pod", "apiVersion": "v1", "metadata": map[string]interface{}{"name": "test"},
		},
	}
	podSha, _ := hash(pod)

	rightSha := pod.DeepCopy()
	rightSha.SetAnnotations(map[string]string{utils.AnnotationConfiguration: podSha})

	wrongSha := pod.DeepCopy()
	wrongSha.SetAnnotations(map[string]string{utils.AnnotationConfiguration: "123abc"})

	tests := []struct {
		name     string
		found    *unstructured.Unstructured
		want     *unstructured.Unstructured
		expected bool
	}{
		{
			name:     "Matching shas",
			found:    rightSha,
			want:     pod,
			expected: true,
		},
		{
			name:     "Matching shas",
			found:    wrongSha,
			want:     pod,
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shasMatch(tt.found, tt.want); got != tt.expected {
				t.Errorf("shasMatch() = %v, want %v", got, tt.expected)
			}
		})
	}
}

/*
Test_annotate tests the annotate function, which adds default annotations to a Kubernetes resource.
It checks that existing annotations are preserved, and new annotations are added.
The test ensures that the annotate function behaves as expected.
*/
func Test_annotate(t *testing.T) {
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind": "Pod", "apiVersion": "v1", "metadata": map[string]interface{}{"name": "test"},
		},
	}
	pod.SetAnnotations(map[string]string{"foo": "bar"})

	t.Run("Keep existing annotations", func(t *testing.T) {
		annotate(pod)
		if got := pod.GetAnnotations()["foo"]; got != "bar" {
			t.Errorf("Expected annotation to equal %s; got %s", "bar", got)
		}
		if len(pod.GetAnnotations()) != 2 {
			t.Errorf("Expected 2 annotations; got %d", len(pod.GetAnnotations()))
		}
	})
}
