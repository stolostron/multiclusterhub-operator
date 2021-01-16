// Copyright (c) 2020 Red Hat, Inc.

package multiclusterhub

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/semver"
	operatorsv1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operator/v1"
	"github.com/open-cluster-management/multicloudhub-operator/version"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/jsonpath"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// UpgradeHubSelfMgmtHackRequired checks the the current version and if hub self management is enabled
// to determine if special upgrade logic is required
func (r *ReconcileMultiClusterHub) UpgradeHubSelfMgmtHackRequired(mch *operatorsv1.MultiClusterHub) (bool, error) {
	currentVersionConstraint, err := semver.NewConstraint("< 2.1.2, >= 2.1.0")
	if err != nil {
		return false, fmt.Errorf("Error setting semver current version constraint < 2.1.2, >=2.1.0")
	}

	desiredVersionConstraint, err := semver.NewConstraint(">= 2.1.2")
	if err != nil {
		return false, fmt.Errorf("Error setting semver desired version constraint = 2.1.2")
	}
	if mch.Status.DesiredVersion != version.Version {
		return false, fmt.Errorf("Error checking desired version. Expected %s, but got %s.", version.Version, mch.Status.CurrentVersion)
	}

	if mch.Status.CurrentVersion == "" || mch.Status.DesiredVersion == "" {
		// Current Version is not available yet
		return false, nil
	}

	currentVersion, err := semver.NewVersion(mch.Status.CurrentVersion)
	if err != nil {
		return false, fmt.Errorf("Error setting semver currentversion: %s", mch.Status.CurrentVersion)
	}

	desiredVersion, err := semver.NewVersion(mch.Status.DesiredVersion)
	if err != nil {
		return false, fmt.Errorf("Error setting semver currentversion: %s", mch.Status.CurrentVersion)
	}

	currVersionValidation := currentVersionConstraint.Check(currentVersion)
	desVersionValidation := desiredVersionConstraint.Check(desiredVersion)
	if currVersionValidation && !mch.Spec.DisableHubSelfManagement && desVersionValidation {
		return true, nil
	}
	return false, nil
}

// ensureKlusterletAddonConfigPausedStatus makes sure if the klusterletaddonconfig pause status matches wantPaused
func ensureKlusterletAddonConfigPausedStatus(c client.Client, name string, namespace string, wantPaused bool) error {
	klusterletAddonConfigAnnotationPause := "klusterletaddonconfig-pause"
	klusterletaddonconfig := &unstructured.Unstructured{}
	klusterletaddonconfig.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "agent.open-cluster-management.io",
		Version: "v1",
		Kind:    "KlusterletAddonConfig",
	})
	err := c.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, klusterletaddonconfig)
	if err != nil {
		return err
	}
	isPaused := false
	annotations := klusterletaddonconfig.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if val, ok := annotations[klusterletAddonConfigAnnotationPause]; ok {
		isPaused = strings.EqualFold(val, "true")
	}
	// current status matches what we are expecting, no update is needed
	if isPaused == wantPaused {
		return nil
	}
	// need to update klusterletaddonconfig
	if wantPaused {
		// add pause
		annotations[klusterletAddonConfigAnnotationPause] = "true"
		klusterletaddonconfig.SetAnnotations(annotations)
	} else {
		// filter out pause
		newAnnotations := make(map[string]string)
		for k, v := range annotations {
			if k == klusterletAddonConfigAnnotationPause {
				continue
			}
			newAnnotations[k] = v
		}
		klusterletaddonconfig.SetAnnotations(newAnnotations)
	}

	return c.Update(context.TODO(), klusterletaddonconfig)
}

// getJSONPath generate a string with the given object by applying the template
func getJSONPath(obj interface{}, template string) (string, error) {
	j := jsonpath.New("jsonPath")
	if err := j.Parse(template); err != nil {
		return "", err
	}
	buf := bytes.NewBuffer([]byte{})
	if err := j.Execute(buf, obj); err != nil {
		// no more items to find
		return "", err
	}
	out := buf.String()
	return out, nil
}

// ensureAppmgrManifestWorkImage makes sure local-cluster appmgr's manifestwork is using correct image
func ensureAppmgrManifestWorkImage(c client.Client, clusterName string, imageKey string, imageValue string) error {
	manifestWork := &unstructured.Unstructured{}
	manifestWork.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "work.open-cluster-management.io",
		Version: "v1",
		Kind:    "ManifestWork",
	})
	// get manifestwork of manifestwork
	appmgrManifestWorkName := clusterName + "-klusterlet-addon-appmgr"
	if err := c.Get(context.TODO(), types.NamespacedName{
		Name:      appmgrManifestWorkName,
		Namespace: clusterName,
	}, manifestWork); err != nil {
		return err
	}
	// look through the manifests, and find the one with manifest
	index := -1
	// iterate through the manifests in case the order is changed
	// the generated manifest should not have more than 2 manifests, set 10 to be safe
	// the loop will be finished when i is out of array's bound or when find AppilcationManager manifest
	for i := 0; i < 10; i++ {
		// check if it's applicationmanager
		template := fmt.Sprintf("{.spec.workload.manifests[%d].kind}", i)
		kind, err := getJSONPath(manifestWork.Object, template)
		if err != nil {
			log.Error(err, "failed to find kind in manifest")
			break
		}
		if kind == "ApplicationManager" {
			index = i
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("given ManifestWork has no manifest of ApplicationManager")
	}
	// get the current image
	template := fmt.Sprintf("{.spec.workload.manifests[%d].spec.global.imageOverrides.%s}", index, imageKey)
	currImage, err := getJSONPath(manifestWork.Object, template)
	if err != nil {
		log.Error(err, "failed to find imageOverrides in Applicationmanager")
	}

	// update with json patch
	if currImage != imageValue {
		log.Info(fmt.Sprintf("current image: %s. Will update to %s\n", currImage, imageValue))
		jsonPathTemplate := fmt.Sprintf(
			`[{"op":"replace","path":"/spec/workload/manifests/%d/spec/global/imageOverrides/%s","value":"%s"}]`,
			index, imageKey, imageValue)
		log.Info("patch: " + jsonPathTemplate)
		return c.Patch(
			context.TODO(),
			manifestWork,
			client.RawPatch(types.JSONPatchType, []byte(jsonPathTemplate)))
	}

	return nil
}

// ensureAppmgrPodImage makes sure no appmgr pod is using an incorrect image.
// returns error if detected any running pods that are not using correct image.
func ensureAppmgrPodImage(c client.Client, imageValue string) error {
	podList := &corev1.PodList{}

	err := c.List(
		context.Background(),
		podList,
		client.InNamespace("open-cluster-management-agent-addon"),
		client.MatchingLabels{
			"app": "application-manager",
		},
	)
	if err != nil {
		log.Error(err, "failed to list pods in namespace open-cluster-management-agent-addon")
		return err
	}
	if len(podList.Items) == 0 {
		return nil
	}
	for _, pod := range podList.Items {
		hasOneMatch := false
		// check status skip not running pods
		if status, err := getJSONPath(
			pod,
			"{.status.phase}",
		); err == nil && status != string(corev1.PodRunning) {
			// ignore pods not running - pods not scheduled or has no container running
			continue
		} else if err != nil {
			log.Error(err, "failed to get pod status")
		}
		// at least one container is using the expected image
		for _, c := range pod.Spec.Containers {
			if c.Image == imageValue {
				hasOneMatch = true
			}
		}
		if !hasOneMatch {
			return fmt.Errorf(
				"pod %s in namespace open-cluster-management-agent-addon is not using the correct image",
				pod.ObjectMeta.Name,
			)
		}
	}
	return nil
}

// BeginEnsuringHubIsUpgradeable - beginning hook for ensuring the hub is upgradeable
// will make sure appmgr pod on hub is using the right image
func (r *ReconcileMultiClusterHub) BeginEnsuringHubIsUpgradeable(mch *operatorsv1.MultiClusterHub) (*reconcile.Result, error) {
	log.Info("Beginning Upgrade Specific Logic!")
	appmgrImageKey := "multicluster_operators_subscription"

	// get and sync klusterletaddonconfig, will ignore if not found
	log.Info("Stopping local-cluster klusterletaddonconfig")
	if err := ensureKlusterletAddonConfigPausedStatus(
		r.client,
		KlusterletAddonConfigName,
		ManagedClusterName,
		true,
	); err != nil && !errors.IsNotFound(err) {
		log.Error(err, "failed to pause klusterletaddonconfig")
		return &reconcile.Result{}, err
	}

	image, err := r.getImageFromManifestByKey(mch, appmgrImageKey)
	if err != nil {
		log.Error(err, "failed to get the image for appmgr")
		return nil, err
	}

	if err := ensureAppmgrManifestWorkImage(
		r.client,
		ManagedClusterName,
		appmgrImageKey,
		image,
	); err != nil && !errors.IsNotFound(err) {
		log.Error(err, "failed to sync appmgr ManifestWork with current image")
		return &reconcile.Result{}, err
	}
	log.Info(fmt.Sprintf("Check if the appmgr pod is using the correct image: %s", image))
	if err := ensureAppmgrPodImage(r.client, image); err != nil {
		log.Error(err, "failed to check if appmgr is using the correct image")
		return &reconcile.Result{}, err
	}

	return nil, nil
}

// getImageFromManifestByKey - Returns image associated with key for desiredVersion of MCH (retrieves new image)
func (r *ReconcileMultiClusterHub) getImageFromManifestByKey(mch *operatorsv1.MultiClusterHub, key string) (string, error) {
	log.Info(fmt.Sprintf("Checking for image associated with key: %s", key))
	configmap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("mch-image-manifest-%s", mch.Status.DesiredVersion),
			Namespace: mch.Namespace,
		},
	}

	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      configmap.Name,
		Namespace: configmap.Namespace,
	}, configmap)
	if err != nil {
		return "", err
	}

	if val, ok := configmap.Data[key]; ok {
		return val, nil
	}
	return "", fmt.Errorf("No image exists associated with key: %s", key)
}
