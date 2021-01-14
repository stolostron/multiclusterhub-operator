// Copyright (c) 2020 Red Hat, Inc.

package multiclusterhub

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_getImageFromManifestByKey(t *testing.T) {

	full_mch.Status.DesiredVersion = "2.1.2"
	tests := []struct {
		Name      string
		ImageKey  string
		ConfigMap *corev1.ConfigMap
		Result    string
	}{
		{
			Name:     "Proper image key given",
			ImageKey: "multicluster_operators_subscription",
			ConfigMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("mch-image-manifest-%s", full_mch.Status.DesiredVersion),
					Namespace: full_mch.Namespace,
				},
				Data: map[string]string{
					"multicluster_operators_subscription": "quay.io/rhibmcollab/multicluster-operators-subscription-image@sha256:test",
				},
			},
			Result: "quay.io/rhibmcollab/multicluster-operators-subscription-image@sha256:test",
		},
		{
			Name:     "Improper image key given",
			ImageKey: "nonexistant_image_key",
			ConfigMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("mch-image-manifest-%s", full_mch.Status.DesiredVersion),
					Namespace: full_mch.Namespace,
				},
				Data: map[string]string{
					"multicluster_operators_subscription": "quay.io/rhibmcollab/multicluster-operators-subscription-image@sha256:test",
				},
			},
			Result: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			r, err := getTestReconciler(full_mch)
			if err != nil {
				t.Fatalf("Failed to create test reconciler")
			}

			err = r.client.Create(context.TODO(), tt.ConfigMap)
			if err != nil {
				t.Fatalf("Err: %s", err)
			}

			image, err := r.getImageFromManifestByKey(full_mch, tt.ImageKey)
			if image != tt.Result {
				t.Fatalf("Unexpected image value returned")
			}
		})
	}
}

func Test_ensureKlusterletAddonConfigPausedStatus(t *testing.T) {
	klusterletaddonconfig := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "agent.open-cluster-management.io/v1",
			"kind":       "KlusterletAddonConfig",
			"metadata": map[string]interface{}{
				"name":      "test-name",
				"namespace": "test-namespace",
			},
			"spec": map[string]interface{}{},
		},
	}
	klusterletaddonconfigPaused := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "agent.open-cluster-management.io/v1",
			"kind":       "KlusterletAddonConfig",
			"metadata": map[string]interface{}{
				"name":      "test-name",
				"namespace": "test-namespace",
				"annotations": map[string]interface{}{
					"klusterletaddonconfig-pause": "true",
				},
			},
			"spec": map[string]interface{}{},
		},
	}
	type args struct {
		client     client.Client
		name       string
		namespace  string
		wantPaused bool
	}
	tests := []struct {
		name       string
		args       args
		wantErr    bool
		wantPaused bool // will check the pause annotation when error is nil
	}{
		{
			name: "should return error if not found",
			args: args{
				client:     fake.NewFakeClient(),
				name:       "test-name",
				namespace:  "test-namespace",
				wantPaused: true,
			},
			wantErr:    true,
			wantPaused: false,
		},
		{
			name: "should set pause if want pause and not in pause staus",
			args: args{
				client:     fake.NewFakeClient(klusterletaddonconfig),
				name:       "test-name",
				namespace:  "test-namespace",
				wantPaused: true,
			},
			wantErr:    false,
			wantPaused: true,
		},
		{
			name: "should set pause if want pause and not in pause staus",
			args: args{
				client:     fake.NewFakeClient(klusterletaddonconfig),
				name:       "test-name",
				namespace:  "test-namespace",
				wantPaused: true,
			},
			wantErr:    false,
			wantPaused: true,
		},
		{
			name: "should do nothing if want pause and already in pause",
			args: args{
				client:     fake.NewFakeClient(klusterletaddonconfigPaused),
				name:       "test-name",
				namespace:  "test-namespace",
				wantPaused: true,
			},
			wantErr:    false,
			wantPaused: true,
		},
		{
			name: "should resume if don't want pause and already in pause",
			args: args{
				client:     fake.NewFakeClient(klusterletaddonconfigPaused),
				name:       "test-name",
				namespace:  "test-namespace",
				wantPaused: false,
			},
			wantErr:    false,
			wantPaused: false,
		},
		{
			name: "should do nothing if don't want pause and not in pause",
			args: args{
				client:     fake.NewFakeClient(klusterletaddonconfig),
				name:       "test-name",
				namespace:  "test-namespace",
				wantPaused: false,
			},
			wantErr:    false,
			wantPaused: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ensureKlusterletAddonConfigPausedStatus(tt.args.client, tt.args.name, tt.args.namespace, tt.args.wantPaused)
			if (err != nil) != tt.wantErr {
				t.Fatalf("want error: %t, error: %v", tt.wantErr, err)
			}
			// check paused
			tempKAC := klusterletaddonconfig.DeepCopy()

			_ = tt.args.client.Get(context.TODO(),
				types.NamespacedName{Name: tt.args.name, Namespace: tt.args.namespace},
				tempKAC,
			)
			if err == nil {
				a := tempKAC.GetAnnotations()
				if (tt.wantPaused && (a == nil || a["klusterletaddonconfig-pause"] != "true")) ||
					(!tt.wantPaused && (a != nil && a["klusterletaddonconfig-pause"] == "true")) {
					t.Fatalf("want pause: %t, annotations: %v", tt.wantPaused, a)
				}
			}
		})
	}
}

func Test_ensureAppmgrManifestWorkImage(t *testing.T) {
	manifestSecret := map[string]interface{}{
		"apiVersion": "",
		"kind":       "Secret",
		"metadata": map[string]interface{}{
			"name":      "test-secret",
			"namespace": "test-namespace",
		},
	}
	manifestAppMgr := map[string]interface{}{
		"apiVersion": "agent.open-cluster-management.io/v1",
		"kind":       "ApplicationManager",
		"metadata": map[string]interface{}{
			"name":      "test-appmgr",
			"namespace": "test-namespace",
		},
		"spec": map[string]interface{}{
			"global": map[string]interface{}{
				"imageOverrides": map[string]interface{}{
					"image-key":       "image-value",
					"other-image-key": "other-image-value",
				},
			},
		},
	}
	manifestAppMgrNew := map[string]interface{}{
		"apiVersion": "agent.open-cluster-management.io/v1",
		"kind":       "ApplicationManager",
		"metadata": map[string]interface{}{
			"name":      "test-appmgr",
			"namespace": "test-namespace",
		},
		"spec": map[string]interface{}{
			"global": map[string]interface{}{
				"imageOverrides": map[string]interface{}{
					"image-key":       "image-value-new",
					"other-image-key": "other-image-value",
				},
			},
		},
	}
	manifestWork := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "work.open-cluster-management.io/v1",
			"kind":       "ManifestWork",
			"metadata": map[string]interface{}{
				"name":      "test-cluster-klusterlet-addon-appmgr",
				"namespace": "test-cluster",
			},
			"spec": map[string]interface{}{
				"workload": map[string]interface{}{
					"manifests": []interface{}{
						manifestSecret,
						manifestAppMgr,
					},
				},
			},
		},
	}
	manifestWorkNew := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "work.open-cluster-management.io/v1",
			"kind":       "ManifestWork",
			"metadata": map[string]interface{}{
				"name":            "test-cluster-klusterlet-addon-appmgr",
				"namespace":       "test-cluster",
				"resourceVersion": "1",
			},
			"spec": map[string]interface{}{
				"workload": map[string]interface{}{
					"manifests": []interface{}{
						manifestSecret,
						manifestAppMgrNew,
					},
				},
			},
		},
	}
	manifestWorkNoAppmgr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "agent.open-cluster-management.io/v1",
			"kind":       "KlusterletAddonConfig",
			"metadata": map[string]interface{}{
				"name":      "test-cluster-klusterlet-addon-appmgr",
				"namespace": "test-cluster",
			},
			"spec": map[string]interface{}{
				"workload": map[string]interface{}{
					"manifests": []interface{}{
						manifestSecret,
					},
				},
			},
		},
	}
	type args struct {
		client      client.Client
		clusterName string
		imageKey    string
		imageValue  string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		wantObj *unstructured.Unstructured
	}{
		{
			name: "should return error if not found",
			args: args{
				client:      fake.NewFakeClient(),
				clusterName: "test-cluster",
				imageKey:    "abc",
				imageValue:  "def",
			},
			wantErr: true,
			wantObj: nil,
		},
		{
			name: "should do nothing if using correct image",
			args: args{
				client:      fake.NewFakeClient(manifestWork),
				clusterName: "test-cluster",
				imageKey:    "image-key",
				imageValue:  "image-value",
			},
			wantErr: false,
			wantObj: manifestWork,
		},
		{
			name: "should return error if there is no ApplicationManager",
			args: args{
				client:      fake.NewFakeClient(manifestWorkNoAppmgr),
				clusterName: "test-cluster",
				imageKey:    "image-key",
				imageValue:  "image-value",
			},
			wantErr: true,
			wantObj: nil,
		},
		{
			name: "should set image if image is not the same",
			args: args{
				client:      fake.NewFakeClient(manifestWork),
				clusterName: "test-cluster",
				imageKey:    "image-key",
				imageValue:  "image-value-new",
			},
			wantErr: false,
			wantObj: manifestWorkNew,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ensureAppmgrManifestWorkImage(tt.args.client, tt.args.clusterName, tt.args.imageKey, tt.args.imageValue)
			if (err != nil) != tt.wantErr {
				t.Fatalf("want error: %t, error: %v", tt.wantErr, err)
			}
			//check obj
			temp := manifestWork.DeepCopy()

			_ = tt.args.client.Get(context.TODO(),
				types.NamespacedName{
					Name:      tt.args.clusterName + "-klusterlet-addon-appmgr",
					Namespace: tt.args.clusterName,
				},
				temp,
			)
			if err == nil && !reflect.DeepEqual(tt.wantObj, temp) {
				t.Fatalf("expect: %#v\n got: %#v\n", tt.wantObj, temp)
			}
		})
	}

}

func Test_ensureAppmgrPodImage(t *testing.T) {
	podOld := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app": "application-manager",
			},
			Namespace: "open-cluster-management-agent-addon",
			Name:      "pod-old",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Image: "image-old",
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
	podOldNotRunning := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app": "application-manager",
			},
			Namespace: "open-cluster-management-agent-addon",
			Name:      "pod-old",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Image: "image-old",
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodFailed,
		},
	}
	podNew1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app": "application-manager",
			},
			Namespace: "open-cluster-management-agent-addon",
			Name:      "pod-new-1",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Image: "image-new",
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
	podNew2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app": "application-manager",
			},
			Namespace: "open-cluster-management-agent-addon",
			Name:      "pod-new-2",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Image: "image-new",
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
		},
	}

	tests := []struct {
		name    string
		client  client.Client
		image   string
		wantErr bool
	}{
		{
			name:    "should do nothing if pod not found",
			client:  fake.NewFakeClient(),
			image:   "image-new",
			wantErr: false,
		},
		{
			name:    "should do nothing if old pod is not running",
			client:  fake.NewFakeClient(podOldNotRunning, podNew1),
			image:   "image-new",
			wantErr: false,
		},
		{
			name:    "should return error if old pod is running and using the old image",
			client:  fake.NewFakeClient(podOld, podNew1),
			image:   "image-new",
			wantErr: true,
		},
		{
			name:    "should do nothing if all pods are using the new image",
			client:  fake.NewFakeClient(podNew1, podNew2),
			image:   "image-new",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ensureAppmgrPodImage(tt.client, tt.image)
			if (err != nil) != tt.wantErr {
				t.Fatalf("want error: %t, error: %v", tt.wantErr, err)
			}
		})
	}
}
