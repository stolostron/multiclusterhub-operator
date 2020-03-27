package patching

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/v3/k8sdeps/kunstruct"
	"sigs.k8s.io/kustomize/v3/pkg/resource"
	"sigs.k8s.io/yaml"

	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
)

var apiserver = `
kind: Deployment
apiVersion: apps/v1
metadata:
  name: mcm-apiserver
  labels:
    app: "mcm-apiserver"
spec:
  template:
    spec:
      volumes:
        - name: apiserver-cert
          secret:
            secretName: "test"
      containers:
      - name: mcm-apiserver
        image: "mcm-api"
        env:
          - name: MYHUBNAME
            value: test
        volumeMounts: []
        args:
          - "/mcm-apiserver"
          - "--enable-admission-plugins=HCMUserIdentity,KlusterletCA,NamespaceLifecycle"
`

var factory = resource.NewFactory(kunstruct.NewKunstructuredFactoryImpl())

func TestApplyGlobalPatches(t *testing.T) {
	json, err := yaml.YAMLToJSON([]byte(apiserver))
	if err != nil {
		t.Fatalf("failed to apply global patches %v", err)
	}
	var u unstructured.Unstructured
	u.UnmarshalJSON(json)
	apiserver := factory.FromMap(u.Object)

	mchcr := &operatorsv1alpha1.MultiClusterHub{
		TypeMeta:   metav1.TypeMeta{Kind: "MultiClusterHub"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: operatorsv1alpha1.MultiClusterHubSpec{
			ImageRepository: "quay.io/open-cluster-management",
			ImagePullPolicy: "Always",
			ImagePullSecret: "test",
			NodeSelector: &operatorsv1alpha1.NodeSelector{
				OS:                  "test",
				CustomLabelSelector: "test",
				CustomLabelValue:    "test",
			},
		},
	}

	err = ApplyGlobalPatches(apiserver, mchcr)
	if err != nil {
		t.Fatalf("failed to apply global patches %v", err)
	}
}

func TestApplyAPIServerPatches(t *testing.T) {
	json, err := yaml.YAMLToJSON([]byte(apiserver))
	if err != nil {
		t.Fatalf("failed to apply apiserver patches %v", err)
	}
	var u unstructured.Unstructured
	u.UnmarshalJSON(json)
	apiserver := factory.FromMap(u.Object)

	var replicas int32 = 1
	mchcr := &operatorsv1alpha1.MultiClusterHub{
		TypeMeta:   metav1.TypeMeta{Kind: "MultiClusterHub"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: operatorsv1alpha1.MultiClusterHubSpec{
			Foundation: operatorsv1alpha1.Foundation{
				Apiserver: operatorsv1alpha1.Apiserver{
					Replicas: &replicas,
					Configuration: map[string]string{
						"test": "test",
					},
				},
			},
			Mongo: operatorsv1alpha1.Mongo{
				Endpoints:  "test",
				ReplicaSet: "test",
			},
		},
	}

	err = ApplyAPIServerPatches(apiserver, mchcr)
	if err != nil {
		t.Fatalf("failed to apply apiserver patches %v", err)
	}
}

func TestApplyAPIServerPatchesWithSecret(t *testing.T) {
	json, err := yaml.YAMLToJSON([]byte(apiserver))
	if err != nil {
		t.Fatalf("failed to apply apiserver patches %v", err)
	}
	var u unstructured.Unstructured
	u.UnmarshalJSON(json)
	apiserver := factory.FromMap(u.Object)

	var replicas int32 = 1
	mchcr := &operatorsv1alpha1.MultiClusterHub{
		TypeMeta:   metav1.TypeMeta{Kind: "MultiClusterHub"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: operatorsv1alpha1.MultiClusterHubSpec{
			Foundation: operatorsv1alpha1.Foundation{
				Apiserver: operatorsv1alpha1.Apiserver{
					Replicas: &replicas,
					Configuration: map[string]string{
						"test": "test",
					},
				},
			},
			Mongo: operatorsv1alpha1.Mongo{
				Endpoints:  "test",
				ReplicaSet: "test",
				UserSecret: "test",
				CASecret:   "test",
				TLSSecret:  "test",
			},
		},
	}

	err = ApplyAPIServerPatches(apiserver, mchcr)
	if err != nil {
		t.Fatalf("failed to apply apiserver patches %v", err)
	}
}

var controller = `
kind: Deployment
apiVersion: apps/v1
metadata:
  name: mcm-controller
  labels:
    app: "mcm-controller"
spec:
  template:
    spec:
      containers:
      - name: mcm-controller
        image: "mcm-controller"
        args:
          - "/mcm-controller"
          - "--leader-elect=true"
`

func TestApplyControllerPatches(t *testing.T) {
	json, err := yaml.YAMLToJSON([]byte(controller))
	if err != nil {
		t.Fatalf("failed to apply controller patches %v", err)
	}
	var u unstructured.Unstructured
	u.UnmarshalJSON(json)
	controller := factory.FromMap(u.Object)

	var replicas int32 = 1
	mchcr := &operatorsv1alpha1.MultiClusterHub{
		TypeMeta:   metav1.TypeMeta{Kind: "MultiClusterHub"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: operatorsv1alpha1.MultiClusterHubSpec{
			Foundation: operatorsv1alpha1.Foundation{
				Controller: operatorsv1alpha1.Controller{
					Replicas: &replicas,
					Configuration: map[string]string{
						"test": "test",
					},
				},
			},
		},
	}

	err = ApplyControllerPatches(controller, mchcr)
	if err != nil {
		t.Fatalf("failed to apply controller patches %v", err)
	}
}

var webhook = `
kind: Deployment
apiVersion: apps/v1
metadata:
  name: mcm-webhook
  labels:
    app: "mcm-webhook"
spec:
  template:
    spec:
      containers:
      - name: mcm-webhook
        image: "multicluster-manager:0.0.1"
        volumeMounts: []
      volumes: []
`

func TestApplyWebhookPatches(t *testing.T) {
	json, err := yaml.YAMLToJSON([]byte(webhook))
	if err != nil {
		t.Fatalf("failed to apply webhook patches %v", err)
	}
	var u unstructured.Unstructured
	u.UnmarshalJSON(json)
	webhook := factory.FromMap(u.Object)

	var replicas int32 = 1
	mchcr := &operatorsv1alpha1.MultiClusterHub{
		TypeMeta:   metav1.TypeMeta{Kind: "MultiClusterHub"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: operatorsv1alpha1.MultiClusterHubSpec{
			Foundation: operatorsv1alpha1.Foundation{
				Controller: operatorsv1alpha1.Controller{
					Replicas: &replicas,
					Configuration: map[string]string{
						"test": "test",
					},
				},
			},
		},
	}

	err = ApplyWebhookPatches(webhook, mchcr)
	if err != nil {
		t.Fatalf("failed to apply webhook patches %v", err)
	}
}
