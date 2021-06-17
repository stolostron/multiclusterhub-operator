// Copyright (c) 2020 Red Hat, Inc.

package multiclusterhub

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	subrelv1 "github.com/open-cluster-management/multicloud-operators-subscription-release/pkg/apis/apps/v1"
	operatorsv1 "github.com/open-cluster-management/multiclusterhub-operator/pkg/apis/operator/v1"
	"github.com/open-cluster-management/multiclusterhub-operator/pkg/channel"
	"github.com/open-cluster-management/multiclusterhub-operator/pkg/foundation"
	"github.com/open-cluster-management/multiclusterhub-operator/pkg/helmrepo"
	"github.com/open-cluster-management/multiclusterhub-operator/pkg/manifest"
	"github.com/open-cluster-management/multiclusterhub-operator/pkg/subscription"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func kind(kind string) schema.GroupKind {
	return schema.GroupKind{Group: "", Kind: kind}
}

func Test_ensureDeployment(t *testing.T) {
	r, err := getTestReconciler(full_mch)
	if err != nil {
		t.Fatalf("Failed to create test reconciler")
	}

	cacheSpec := CacheSpec{
		IngressDomain: "apps.smart-buck.dev01.red-chesterfield.com",
		ImageOverrides: map[string]string{
			"application_ui": "quay.io/open-cluster-management/application-ui@sha256:c740fc7bac067f003145ab909504287360564016b7a4a51b7ad4987aca123ac1",
			"console_api":    "quay.io/open-cluster-management/console-api@sha256:3ef1043b4e61a09b07ff37f9ad8fc6e707af9813936cf2c0d52f2fa0e489c75f",
			"rcm_controller": " quay.io/open-cluster-management/rcm-controller@sha256:8fab4d788241bf364dbc1b8c1ea5ccf18d3145a640dbd456b0dc7ba204e36819",
		},
	}

	tests := []struct {
		Name       string
		MCH        *operatorsv1.MultiClusterHub
		Deployment *appsv1.Deployment
		Result     error
	}{
		{
			Name:       "Test: EnsureDeployment - Multiclusterhub-repo",
			MCH:        full_mch,
			Deployment: helmrepo.Deployment(full_mch, cacheSpec.ImageOverrides),
			Result:     nil,
		},
		{
			Name:       "Test: EnsureDeployment - Webhook",
			MCH:        full_mch,
			Deployment: foundation.WebhookDeployment(full_mch, cacheSpec.ImageOverrides),
			Result:     nil,
		},
		{
			Name:       "Test: EnsureDeployment - Empty Deployment",
			MCH:        full_mch,
			Deployment: &appsv1.Deployment{},
			Result:     errors.NewInvalid(kind("Test"), "", nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			_, err = r.ensureDeployment(tt.MCH, tt.Deployment)

			if tt.Result != nil {
				// Check if error matches desired error

				if errors.ReasonForError(err) != errors.ReasonForError(tt.Result) {
					t.Fatalf("ensureDeployment() error = %v, wantErr %v", err, tt.Result)
				}
			} else {
				if err != nil {
					t.Fatalf("ensureDeployment() error = %v, wantErr %v", err, tt.Result)
				}

				deploy := &appsv1.Deployment{}
				err = r.client.Get(context.TODO(), types.NamespacedName{
					Name:      tt.Deployment.Name,
					Namespace: tt.Deployment.Namespace,
				}, deploy)

				if err != tt.Result {
					t.Fatalf("Could not find created '%s' deployment: %s", tt.Deployment.Name, err.Error())
				}
			}
		})
	}

}

func Test_ensureService(t *testing.T) {
	r, err := getTestReconciler(full_mch)
	if err != nil {
		t.Fatalf("Failed to create test reconciler")
	}

	tests := []struct {
		Name    string
		MCH     *operatorsv1.MultiClusterHub
		Service *corev1.Service
		Result  error
	}{
		{
			Name:    "Test: ensureService - Multiclusterhub-repo",
			MCH:     full_mch,
			Service: helmrepo.Service(full_mch),
			Result:  nil,
		},
		{
			Name:    "Test: ensureService - Webhook",
			MCH:     full_mch,
			Service: foundation.WebhookService(full_mch),
			Result:  nil,
		},
		{
			Name:    "Test: ensureService - Empty service",
			MCH:     full_mch,
			Service: &corev1.Service{},
			Result:  errors.NewInvalid(kind("Test"), "", nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			_, err = r.ensureService(tt.MCH, tt.Service)

			if tt.Result != nil {
				// Check if error matches desired error
				if errors.ReasonForError(err) != errors.ReasonForError(tt.Result) {
					t.Fatalf("ensureService() error = %v, wantErr %v", err, tt.Result)
				}
			} else {
				if err != nil {
					t.Fatalf("ensureService() error = %v, wantErr %v", err, tt.Result)
				}

				service := &corev1.Service{}
				err = r.client.Get(context.TODO(), types.NamespacedName{
					Name:      tt.Service.Name,
					Namespace: tt.Service.Namespace,
				}, service)
				if err != tt.Result {
					t.Fatalf("Could not find created '%s' service: %s", tt.Service.Name, err.Error())
				}
			}
		})
	}
}

func Test_ensureChannel(t *testing.T) {
	r, err := getTestReconciler(full_mch)
	if err != nil {
		t.Fatalf("Failed to create test reconciler")
	}

	tests := []struct {
		Name    string
		MCH     *operatorsv1.MultiClusterHub
		Channel *unstructured.Unstructured
		Result  error
	}{
		{
			Name:    "Test: ensureChannel - Multiclusterhub-repo",
			MCH:     full_mch,
			Channel: channel.Channel(full_mch),
			Result:  nil,
		},
		{
			Name:    "Test: ensureChannel - Empty channel",
			MCH:     full_mch,
			Channel: &unstructured.Unstructured{},
			Result:  fmt.Errorf("Object 'Kind' is missing in 'unstructured object has no kind'"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			_, err = r.ensureChannel(tt.MCH, tt.Channel)
			if !errorEquals(err, tt.Result) {
				t.Errorf("ensureChannel() error = %v, wantErr %v", err, tt.Result)
			}
		})

		// TODO: Check Channel is created in the fake client
	}
}

func Test_ensureSubscription(t *testing.T) {
	os.Setenv("UNIT_TEST", "true")
	defer os.Unsetenv("UNIT_TEST")

	r, err := getTestReconciler(full_mch)
	if err != nil {
		t.Fatalf("Failed to create test reconciler")
	}

	cacheSpec := CacheSpec{
		IngressDomain: "apps.smart-buck.dev01.red-chesterfield.com",
		ImageOverrides: map[string]string{
			"application_ui": "quay.io/open-cluster-management/application-ui@sha256:c740fc7bac067f003145ab909504287360564016b7a4a51b7ad4987aca123ac1",
			"console_api":    "quay.io/open-cluster-management/console-api@sha256:3ef1043b4e61a09b07ff37f9ad8fc6e707af9813936cf2c0d52f2fa0e489c75f",
			"rcm_controller": " quay.io/open-cluster-management/rcm-controller@sha256:8fab4d788241bf364dbc1b8c1ea5ccf18d3145a640dbd456b0dc7ba204e36819",
		},
	}

	tests := []struct {
		Name         string
		MCH          *operatorsv1.MultiClusterHub
		Subscription *unstructured.Unstructured
		Result       error
	}{
		{
			Name:         "Test: ensureSubscription - Cert-manager",
			MCH:          full_mch,
			Subscription: subscription.CertManager(full_mch, cacheSpec.ImageOverrides),
			Result:       nil,
		},
		{
			Name:         "Test: ensureSubscription - Cert-webhook",
			MCH:          full_mch,
			Subscription: subscription.CertWebhook(full_mch, cacheSpec.ImageOverrides),
			Result:       nil,
		},
		{
			Name:         "Test: ensureSubscription - Config-watcher",
			MCH:          full_mch,
			Subscription: subscription.ConfigWatcher(full_mch, cacheSpec.ImageOverrides),
			Result:       nil,
		},
		{
			Name:         "Test: ensureSubscription - Management-ingress",
			MCH:          full_mch,
			Subscription: subscription.ManagementIngress(full_mch, cacheSpec.ImageOverrides, cacheSpec.IngressDomain),
			Result:       nil,
		},
		{
			Name:         "Test: ensureSubscription - Application-UI",
			MCH:          full_mch,
			Subscription: subscription.ApplicationUI(full_mch, cacheSpec.ImageOverrides),
			Result:       nil,
		},
		{
			Name:         "Test: ensureSubscription - Console",
			MCH:          full_mch,
			Subscription: subscription.Console(full_mch, cacheSpec.ImageOverrides, cacheSpec.IngressDomain),
			Result:       nil,
		},
		{
			Name:         "Test: ensureSubscription - GRC",
			MCH:          full_mch,
			Subscription: subscription.GRC(full_mch, cacheSpec.ImageOverrides),
			Result:       nil,
		},
		{
			Name:         "Test: ensureSubscription - KUI",
			MCH:          full_mch,
			Subscription: subscription.KUIWebTerminal(full_mch, cacheSpec.ImageOverrides, cacheSpec.IngressDomain),
			Result:       nil,
		},
		{
			Name:         "Test: ensureSubscription - RCM",
			MCH:          full_mch,
			Subscription: subscription.RCM(full_mch, cacheSpec.ImageOverrides),
			Result:       nil,
		},
		{
			Name:         "Test: ensureSubscription - Search",
			MCH:          full_mch,
			Subscription: subscription.Search(full_mch, cacheSpec.ImageOverrides),
			Result:       nil,
		},
		{
			Name:         "Test: ensureSubscription - Topology",
			MCH:          full_mch,
			Subscription: subscription.Topology(full_mch, cacheSpec.ImageOverrides),
			Result:       nil,
		},
		{
			Name:         "Test: ensureSubscription - Empty Sub",
			MCH:          full_mch,
			Subscription: &unstructured.Unstructured{},
			Result:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			_, err = r.ensureSubscription(tt.MCH, tt.Subscription)
			if !errorEquals(err, tt.Result) {
				t.Errorf("ensureSubscription() error = %v, wantErr %v", err, tt.Result)
			}

			// TODO: Check Subscription is created in the fake client
		})
	}
}

func Test_ensureUnstructuredResource(t *testing.T) {
	r, err := getTestReconciler(full_mch)
	if err != nil {
		t.Fatalf("Failed to create test reconciler")
	}

	imageOverrides := map[string]string{
		"registration": "quay.io/open-cluster-management/registration@sha256:fe95bca419976ca8ffe608bc66afcead6ef333b863f22be55df57c89ded75dda",
	}

	tests := []struct {
		Name           string
		MCH            *operatorsv1.MultiClusterHub
		ClusterManager *unstructured.Unstructured
		Result         error
	}{
		{
			Name:           "Test: ensureUnstructuredResource - ClusterManager",
			MCH:            full_mch,
			ClusterManager: foundation.ClusterManager(full_mch, imageOverrides),
			Result:         nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			_, err = r.ensureUnstructuredResource(tt.MCH, tt.ClusterManager)
			if !errorEquals(err, tt.Result) {
				t.Fatalf("Failed to ensure ClusterManager: %s", err)
			}
		})
	}
}

func Test_OverrideImagesFromConfigmap(t *testing.T) {
	os.Setenv("MANIFESTS_PATH", "../../../image-manifests")
	defer os.Unsetenv("MANIFESTS_PATH")

	r, err := getTestReconciler(full_mch)
	if err != nil {
		t.Fatalf("Failed to create test reconciler")
	}

	annotatedMCH := full_mch.DeepCopy()
	annotatedMCH.SetAnnotations(map[string]string{
		"image-overrides-configmap": "my-config",
	})

	tests := []struct {
		Name          string
		MCH           *operatorsv1.MultiClusterHub
		CreateCM      bool
		ConfigMap     *corev1.ConfigMap
		ManifestImage manifest.ManifestImage
		Result        error
	}{
		{
			Name:      "Test: OverrideImagesFromConfigmap - Nonexistant configmap",
			MCH:       annotatedMCH,
			CreateCM:  false,
			ConfigMap: &corev1.ConfigMap{},
			Result:    fmt.Errorf(`configmaps "" not found`),
		},
		{
			Name:     "Test: OverrideImagesFromConfigmap - Override repo image",
			MCH:      annotatedMCH,
			CreateCM: true,
			ConfigMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-config",
					Namespace: full_mch.GetNamespace(),
				},
				Data: map[string]string{
					"overrides.json:": `[
						{
						  "image-name": "multiclusterhub-repo",
						  "image-tag": "2.2.5-test",
						  "image-remote": "quay.io/open-cluster-management",
						  "image-key": "multiclusterhub_repo"
						}
					  ]`,
				},
			},
			ManifestImage: manifest.ManifestImage{
				ImageKey:    "multiclusterhub_repo",
				ImageRemote: "quay.io/open-cluster-management",
				ImageTag:    "2.2.5-test",
				ImageName:   "multiclusterhub-repo",
			},
			Result: nil,
		},
		{
			Name:     "Test: OverrideImagesFromConfigmap - New image added from Configmap",
			MCH:      annotatedMCH,
			CreateCM: true,
			ConfigMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-config",
					Namespace: full_mch.GetNamespace(),
				},
				Data: map[string]string{
					"overrides.json:": `[
						{
						  "image-name": "non-existent-image",
						  "image-digest": "sha256:e728a4cdf4a11b78b927b7b280d75babca7b3880450fbf190d80b194f7d064b6",
						  "image-remote": "quay.io/open-cluster-management",
						  "image-key": "non_existent_image"
						}
					  ]`,
				},
			},
			ManifestImage: manifest.ManifestImage{
				ImageKey:    "non_existent_image",
				ImageRemote: "quay.io/open-cluster-management",
				ImageDigest: "sha256:e728a4cdf4a11b78b927b7b280d75babca7b3880450fbf190d80b194f7d064b6",
				ImageName:   "non-existent-image",
			},
			Result: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			if tt.CreateCM {
				err := r.client.Create(context.TODO(), tt.ConfigMap)
				if err != nil {
					t.Fatalf("Failed to create configmap: %s", err)
				}
				defer r.client.Delete(context.TODO(), tt.ConfigMap)
			}

			imagesOverrides, err := manifest.GetImageOverrides(tt.MCH)
			if err != nil {
				t.Fatalf("Failed to get image overrides: %s", err)
			}
			imagesOverrides, err = r.OverrideImagesFromConfigmap(imagesOverrides, tt.MCH.GetNamespace(), tt.ConfigMap.GetName())
			if !errorEquals(err, tt.Result) {
				t.Fatalf("Failed to get image overrides from configmap : %s", err)
			}
			if tt.CreateCM {
				if imagesOverrides[tt.ManifestImage.ImageKey] != fmt.Sprintf("%s/%s:%s", tt.ManifestImage.ImageRemote, tt.ManifestImage.ImageName, tt.ManifestImage.ImageTag) &&
					imagesOverrides[tt.ManifestImage.ImageKey] != fmt.Sprintf("%s/%s@%s", tt.ManifestImage.ImageRemote, tt.ManifestImage.ImageName, tt.ManifestImage.ImageDigest) {
					t.Fatalf("Unexpected image override")
				}
			}
		})
	}
}

func Test_maintainImageManifestConfigmap(t *testing.T) {
	r, err := getTestReconciler(full_mch)
	if err != nil {
		t.Fatalf("Failed to create test reconciler")
	}

	r.CacheSpec = CacheSpec{
		ImageOverrides: map[string]string{
			"application_ui": "quay.io/open-cluster-management/application-ui@sha256:c740fc7bac067f003145ab909504287360564016b7a4a51b7ad4987aca123ac1",
			"console_api":    "quay.io/open-cluster-management/console-api@sha256:3ef1043b4e61a09b07ff37f9ad8fc6e707af9813936cf2c0d52f2fa0e489c75f",
			"rcm_controller": " quay.io/open-cluster-management/rcm-controller@sha256:8fab4d788241bf364dbc1b8c1ea5ccf18d3145a640dbd456b0dc7ba204e36819",
		},
		ManifestVersion: "2.2.5",
	}

	configmapName := fmt.Sprintf("mch-image-manifest-%s", r.CacheSpec.ManifestVersion)

	// Check configmap is created if it doesnt exist
	err = r.maintainImageManifestConfigmap(full_mch)
	if err != nil {
		t.Fatalf("Failed to store image overrides: %s", err)
	}
	configmap := &corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Name:      configmapName,
		Namespace: full_mch.Namespace,
	}, configmap)
	if err != nil {
		t.Fatalf("Failed to get overrides configmap: %s", err)
	}
	if !reflect.DeepEqual(configmap.Data, r.CacheSpec.ImageOverrides) {
		t.Fatalf("Failed to set configmap contents")
	}

	// Check configmap is updated if exists
	configmap.Data = make(map[string]string)
	err = r.client.Update(context.TODO(), configmap)
	if err != nil {
		t.Fatalf("Failed to find image overrides configmap: %s", err)
	}
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Name:      configmapName,
		Namespace: full_mch.Namespace,
	}, configmap)
	if err != nil {
		t.Fatalf("Failed to get overrides configmap: %s", err)
	}
	if !reflect.DeepEqual(configmap.Data, make(map[string]string)) {
		t.Fatalf("Failed to clear configmap contents")
	}
	err = r.maintainImageManifestConfigmap(full_mch)
	if err != nil {
		t.Fatalf("Failed to store image overrides: %s", err)
	}
	configmap = &corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Name:      configmapName,
		Namespace: full_mch.Namespace,
	}, configmap)
	if !reflect.DeepEqual(configmap.Data, r.CacheSpec.ImageOverrides) {
		t.Fatalf("Failed to update configmap")
	}
}

func Test_filterDeploymentsByRelease(t *testing.T) {
	tests := []struct {
		name         string
		allDeps      []*appsv1.Deployment
		releaseLabel string
		want         []*appsv1.Deployment
	}{

		{
			name: "Deployment with release label",
			allDeps: []*appsv1.Deployment{
				{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"meta.helm.sh/release-name": "foo-123"}}},
				{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"meta.helm.sh/release-name": "bar-123"}}},
			},
			releaseLabel: "foo-123",
			want: []*appsv1.Deployment{
				{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"meta.helm.sh/release-name": "foo-123"}}},
			},
		},
		{
			name: "Deployments with no matching release label",
			allDeps: []*appsv1.Deployment{
				{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"meta.helm.sh/release-name": "bar-123"}}},
			},
			releaseLabel: "foo-123",
			want:         []*appsv1.Deployment{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filterDeploymentsByRelease(tt.allDeps, tt.releaseLabel); !reflect.DeepEqual(got, tt.want) {
				// Got desired if both are empty
				if !(len(got) == 0 && len(tt.want) == 0) {
					t.Errorf("filterDeploymentsByRelease() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func Test_addInstallerLabel(t *testing.T) {
	type args struct {
		d    *appsv1.Deployment
		name string
		ns   string
	}
	tests := []struct {
		name       string
		args       args
		want       bool
		wantDeploy *appsv1.Deployment
	}{
		{
			name: "Deployment has no labels",
			args: args{
				d:    &appsv1.Deployment{},
				name: "foo",
				ns:   "default",
			},
			want: true,
		},
		{
			name: "Deployment has desired labels",
			args: args{
				d: &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
						"installer.name":      "foo",
						"installer.namespace": "default",
					}},
				},
				name: "foo",
				ns:   "default",
			},
			want: false,
		},
		{
			name: "Wrong installer name and namespace",
			args: args{
				d: &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
						"installer.name":      "bar",
						"installer.namespace": "kube-system",
					}},
				},
				name: "foo",
				ns:   "default",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := addInstallerLabel(tt.args.d, tt.args.name, tt.args.ns); got != tt.want {
				t.Errorf("addInstallerLabel() = %v, want %v", got, tt.want)
			}
			labels := tt.args.d.Labels
			if gotName := labels["installer.name"]; gotName != tt.args.name {
				t.Errorf("Name label = %v, want %v", gotName, tt.args.name)
			}
			if gotNS := labels["installer.namespace"]; gotNS != tt.args.ns {
				t.Errorf("Namespace label = %v, want %v", gotNS, tt.args.ns)
			}
		})
	}
}

func Test_getAppSubOwnedHelmReleases(t *testing.T) {
	appsubs := []types.NamespacedName{
		{Name: "search-prod-sub", Namespace: "default"},
		{Name: "topology-sub", Namespace: "default"},
	}

	tests := []struct {
		name    string
		hrList  []*subrelv1.HelmRelease
		appsubs []types.NamespacedName
		want    []*subrelv1.HelmRelease
	}{
		{
			name: "No matches",
			hrList: []*subrelv1.HelmRelease{
				{ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps.open-cluster-management.io/v1",
							Kind:       "Subscription",
							Name:       "a-different-subscription",
						},
					},
				}},
			},
			appsubs: appsubs,
			want:    []*subrelv1.HelmRelease{},
		},
		{
			name: "One helmrelease owned by appsub",
			hrList: []*subrelv1.HelmRelease{
				{ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps.open-cluster-management.io/v1",
							Kind:       "Subscription",
							Name:       "topology-sub",
						},
					},
				}},
			},
			appsubs: appsubs,
			want: []*subrelv1.HelmRelease{
				{ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps.open-cluster-management.io/v1",
							Kind:       "Subscription",
							Name:       "topology-sub",
						},
					},
				}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getAppSubOwnedHelmReleases(tt.hrList, tt.appsubs); !reflect.DeepEqual(got, tt.want) {
				// Got desired if both are empty
				if !(len(got) == 0 && len(tt.want) == 0) {
					t.Errorf("getAppSubOwnedHelmReleases() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func Test_getHelmReleaseOwnedDeployments(t *testing.T) {
	allDeployments := []*appsv1.Deployment{
		{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"meta.helm.sh/release-name": "foo-123"}}},
		{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"meta.helm.sh/release-name": "foo-123"}}},
	}
	tests := []struct {
		name   string
		dList  []*appsv1.Deployment
		hrList []*subrelv1.HelmRelease
		want   []*appsv1.Deployment
	}{
		{
			name:  "No matches",
			dList: allDeployments,
			hrList: []*subrelv1.HelmRelease{
				{ObjectMeta: metav1.ObjectMeta{Name: "hr-of-a-different-name"}},
			},
			want: []*appsv1.Deployment{},
		},
		{
			name:  "Two deployments owned by same helmrelease",
			dList: allDeployments,
			hrList: []*subrelv1.HelmRelease{
				{ObjectMeta: metav1.ObjectMeta{Name: "foo-123"}},
			},
			want: allDeployments,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getHelmReleaseOwnedDeployments(tt.dList, tt.hrList); !reflect.DeepEqual(got, tt.want) {
				// Got desired if both are empty
				if !(len(got) == 0 && len(tt.want) == 0) {
					t.Errorf("getHelmReleaseOwnedDeployments() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func Test_ensureWebhookIsAvailable(t *testing.T) {
	availableWebhook := full_mch.DeepCopy()
	unavailableWebhook := full_mch.DeepCopy()

	statusConditionAvailable := operatorsv1.StatusCondition{
		Type:   "Deployed",
		Status: "True",
	}
	statusConditionUnavailable := operatorsv1.StatusCondition{
		Type:   "Error",
		Status: "False",
	}
	availableWebhook.Status.Components = make(map[string]operatorsv1.StatusCondition)
	unavailableWebhook.Status.Components = make(map[string]operatorsv1.StatusCondition)

	availableWebhook.Status.Components["cert-manager-webhook-sub"] = statusConditionAvailable
	unavailableWebhook.Status.Components["cert-manager-webhook-sub"] = statusConditionUnavailable

	tests := []struct {
		name   string
		mch    *operatorsv1.MultiClusterHub
		result error
	}{
		{
			name:   "No status",
			mch:    full_mch,
			result: fmt.Errorf("Waiting for cert-manager-webhook status"),
		},
		{
			name:   "Webhook is Unavailable",
			mch:    unavailableWebhook,
			result: fmt.Errorf("Waiting for cert-manager-webhook to be available"),
		},
		{
			name:   "Webhook is available",
			mch:    availableWebhook,
			result: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := getTestReconciler(full_mch)
			if err != nil {
				t.Fatalf("Failed to create test reconciler")
			}
			_, err = r.ensureWebhookIsAvailable(tt.mch)
			if !errorEquals(err, tt.result) {
				t.Fatalf("Improper webhook status")
			}
		})
	}
}

func Test_getDeploymentByName(t *testing.T) {
	foo := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
	bar := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "bar"}}
	allDeployments := []*appsv1.Deployment{foo, bar}
	tests := []struct {
		name          string
		allDeps       []*appsv1.Deployment
		desiredDeploy string
		want          *appsv1.Deployment
		want1         bool
	}{
		{
			name:          "Deployment found",
			allDeps:       allDeployments,
			desiredDeploy: "foo",
			want:          foo,
			want1:         true,
		},
		{
			name:          "Deployment not found",
			allDeps:       allDeployments,
			desiredDeploy: "baz",
			want:          nil,
			want1:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := getDeploymentByName(tt.allDeps, tt.desiredDeploy)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getDeploymentByName() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("getDeploymentByName() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_isOLMManaged(t *testing.T) {
	foo := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
	bar := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "bar",
			Labels: map[string]string{"bar": "bar"},
		},
	}
	olmDeploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "olm",
			Labels: map[string]string{"olm.owner": "foobar"},
		},
	}

	tests := []struct {
		name   string
		deploy *appsv1.Deployment
		want   bool
	}{
		{
			name:   "Not OLM managed, no labels",
			deploy: foo,
			want:   false,
		},
		{
			name:   "Not OLM managed, with labels",
			deploy: bar,
			want:   false,
		},
		{
			name:   "OLM installed",
			deploy: olmDeploy,
			want:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isOLMManaged(tt.deploy); got != tt.want {
				t.Errorf("isOLMManaged() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReconcileMultiClusterHub_ensureSubscriptionOperatorIsRunning(t *testing.T) {
	r, err := getTestReconciler(full_mch)
	if err != nil {
		t.Fatalf("Failed to create test reconciler")
	}

	mchOperator := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "multiclusterhub-operator",
			Labels: map[string]string{"olm.owner": "advanced-cluster-management.v2.1.2"},
		},
	}

	subOperator := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "multicluster-operators-standalone-subscription",
			Labels: map[string]string{"olm.owner": "advanced-cluster-management.v2.1.2"},
		},
	}

	subOperatorOld := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "multicluster-operators-standalone-subscription",
			Labels: map[string]string{"olm.owner": "advanced-cluster-management.v2.1.1"},
		},
	}

	tests := []struct {
		name       string
		mch        *operatorsv1.MultiClusterHub
		allDeps    []*appsv1.Deployment
		wantResult *reconcile.Result
		wantErr    error
	}{
		{
			name:       "Operators are in sync",
			mch:        full_mch,
			allDeps:    []*appsv1.Deployment{mchOperator, subOperator},
			wantResult: nil,
			wantErr:    nil,
		},
		{
			name:       "Subscription operator is not up-to-date",
			mch:        full_mch,
			allDeps:    []*appsv1.Deployment{mchOperator, subOperatorOld},
			wantResult: &reconcile.Result{RequeueAfter: time.Second * 10},
			wantErr:    nil,
		},
		{
			name:       "Not running MCH operator in deployment",
			mch:        full_mch,
			allDeps:    []*appsv1.Deployment{subOperatorOld},
			wantResult: nil,
			wantErr:    nil,
		},
		{
			name:       "Subscription operator is missing",
			mch:        full_mch,
			allDeps:    []*appsv1.Deployment{mchOperator},
			wantResult: &reconcile.Result{RequeueAfter: time.Second * 10},
			wantErr:    fmt.Errorf("Standalone subscription deployment not found"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r.ensureSubscriptionOperatorIsRunning(tt.mch, tt.allDeps)
			if tt.wantResult != nil && result == nil {
				t.Errorf("ensureSubscriptionOperatorIsRunning() got = %v, want %v", result, tt.wantResult)
			}
			if tt.wantResult == nil && result != nil {
				t.Errorf("ensureSubscriptionOperatorIsRunning() got = %v, want %v", result, tt.wantResult)
			}
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("ensureSubscriptionOperatorIsRunning() error = %v, wantErr %v", err, tt.wantErr)
				}
			} else {
				if err == nil {
					t.Fatalf("ensureSubscriptionOperatorIsRunning() error = %v, wantErr %v", err, tt.wantErr)
				}

			}
		})
	}

}
