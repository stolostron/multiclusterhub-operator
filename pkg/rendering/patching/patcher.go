package patching

import (
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/v3/k8sdeps/kunstruct"
	"sigs.k8s.io/kustomize/v3/pkg/ifc"
	"sigs.k8s.io/kustomize/v3/pkg/resource"
	"sigs.k8s.io/yaml"

	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
)

type patchGenerateFn func(res *resource.Resource, multipleClusterHub *operatorsv1alpha1.MultiClusterHub) (ifc.Kunstructured, error)

func ApplyGlobalPatches(res *resource.Resource, multipleClusterHub *operatorsv1alpha1.MultiClusterHub) error {

	for _, generate := range []patchGenerateFn{
		generateImagePatch,
		generateImagePullSecretsPatch,
		generateNodeSelectorPatch,
	} {
		patch, err := generate(res, multipleClusterHub)
		if err != nil {
			return err
		}
		if patch == nil {
			continue
		}
		if err = res.Patch(patch); err != nil {
			return err
		}
	}
	return nil
}

func ApplyAPIServerPatches(res *resource.Resource, multipleClusterHub *operatorsv1alpha1.MultiClusterHub) error {
	replicasPatch := generateReplicasPatch(*multipleClusterHub.Spec.ReplicaCount)
	if err := res.Patch(replicasPatch); err != nil {
		return err
	}

	etcdServer := fmt.Sprintf("http://etcd-cluster.%s.svc.cluster.local:2379", multipleClusterHub.Namespace)
	args := make(map[string]string)
	args["etcd-servers"] = etcdServer

	args["mongo-host"] = utils.MongoEndpoints
	args["mongo-replicaset"] = utils.MongoReplicaSet
	envVars, volumes, volumeMounts := generateMongoSecrets(multipleClusterHub)
	if err := applySecretPatches(res, envVars, volumes, volumeMounts); err != nil {
		return err
	}

	if err := applySecretPatches(
		res,
		[]corev1.EnvVar{},
		[]corev1.Volume{{
			Name: "apiserver-certs",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: utils.APIServerSecretName},
			},
		}},
		[]corev1.VolumeMount{{Name: "apiserver-certs", MountPath: "/var/run/apiserver"}},
	); err != nil {
		return err
	}

	if err := applySecretPatches(
		res,
		[]corev1.EnvVar{},
		[]corev1.Volume{{
			Name: "klusterlet-certs",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: utils.KlusterletSecretName},
			},
		}},
		[]corev1.VolumeMount{{Name: "klusterlet-certs", MountPath: "/var/run/klusterlet"}},
	); err != nil {
		return err
	}

	argsPatch, err := generateContainerArgsPatch(res, args)
	if err != nil {
		return err
	}
	return res.Patch(argsPatch)
}

func ApplyControllerPatches(res *resource.Resource, multipleClusterHub *operatorsv1alpha1.MultiClusterHub) error {
	replicasPatch := generateReplicasPatch(*multipleClusterHub.Spec.ReplicaCount)
	if err := res.Patch(replicasPatch); err != nil {
		return err
	}
	return nil
}

func ApplyWebhookPatches(res *resource.Resource, multipleClusterHub *operatorsv1alpha1.MultiClusterHub) error {
	if err := applySecretPatches(
		res,
		[]corev1.EnvVar{},
		[]corev1.Volume{{
			Name: "webhook-cert",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: "mcm-webhook-secret"},
			},
		}},
		[]corev1.VolumeMount{{
			Name:      "webhook-cert",
			MountPath: "/var/run/mcm-webhook",
		}},
	); err != nil {
		return err
	}

	return nil
}

func applySecretPatches(
	res *resource.Resource, envVars []corev1.EnvVar, volumes []corev1.Volume, volumeMounts []corev1.VolumeMount) error {
	if len(envVars) > 0 {
		envVarPatch, err := generateEnvVarsPatch(res, envVars)
		if err != nil {
			return err
		}
		if err := res.Patch(envVarPatch); err != nil {
			return err
		}
	}

	if len(volumes) > 0 {
		volumesPatch, err := generateVolumesPatch(res, volumes)
		if err != nil {
			return err
		}
		if err := res.Patch(volumesPatch); err != nil {
			return err
		}
	}

	if len(volumeMounts) > 0 {
		volumeMountPatch, err := generateVolumeMountPatch(res, volumeMounts)
		if err != nil {
			return err
		}
		if err := res.Patch(volumeMountPatch); err != nil {
			return err
		}
	}

	return nil
}

func generateMongoSecrets(mch *operatorsv1alpha1.MultiClusterHub) ([]corev1.EnvVar, []corev1.Volume, []corev1.VolumeMount) {

	var mode int32 = 420
	envs := []corev1.EnvVar{}
	volumeMounts := []corev1.VolumeMount{}
	volumes := []corev1.Volume{}
	envs = append(envs, corev1.EnvVar{
		Name: "MONGO_USERNAME",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				Key: "user",
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "mongodb-admin",
				},
			},
		},
	}, corev1.EnvVar{
		Name: "MONGO_PASSWORD",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				Key: "password",
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "mongodb-admin",
				},
			},
		},
	})
	envs = append(envs, corev1.EnvVar{Name: "MONGO_SSLCA", Value: "/certs/mongodb-ca/tls.crt"})
	volumes = append(volumes, corev1.Volume{
		Name: "mongodb-ca-cert",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{DefaultMode: &mode, SecretName: utils.MongoCaSecret},
		},
	})
	volumeMounts = append(volumeMounts, corev1.VolumeMount{MountPath: "/certs/mongodb-ca", Name: "mongodb-ca-cert"})
	envs = append(
		envs,
		corev1.EnvVar{Name: "MONGO_SSLCERT", Value: "/certs/mongodb-client/tls.crt"},
		corev1.EnvVar{Name: "MONGO_SSLKEY", Value: "/certs/mongodb-client/tls.key"},
	)
	volumes = append(volumes, corev1.Volume{
		Name: "mongodb-client-cert",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{DefaultMode: &mode, SecretName: utils.MongoTLSSecret},
		},
	})
	volumeMounts = append(volumeMounts, corev1.VolumeMount{MountPath: "/certs/mongodb-client", Name: "mongodb-client-cert"})

	return envs, volumes, volumeMounts
}

func generateReplicasPatch(replicas int32) ifc.Kunstructured {
	return kunstruct.NewKunstructuredFactoryImpl().FromMap(map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": replicas,
		},
	})
}

func generateImagePatch(res *resource.Resource, mch *operatorsv1alpha1.MultiClusterHub) (ifc.Kunstructured, error) {
	imageFromTemplate, err := res.GetString("spec.template.spec.containers[0].image") // need to loop through all images
	if err != nil {
		return nil, err
	}
	imageRepo := mch.Spec.ImageRepository
	imageTagSuffix := mch.Spec.ImageTagSuffix
	if imageTagSuffix != "" {
		imageTagSuffix = "-" + imageTagSuffix
	}
	generatedImage := fmt.Sprintf("%s/%s%s", imageRepo, imageFromTemplate, imageTagSuffix)

	container, _ := res.GetFieldValue("spec.template.spec.containers[0]") // need to loop through all images
	containerMap, _ := container.(map[string]interface{})
	containerMap["image"] = generatedImage
	containerMap["imagePullPolicy"] = mch.Spec.ImagePullPolicy

	return kunstruct.NewKunstructuredFactoryImpl().FromMap(map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{containerMap},
				},
			},
		},
	}), nil
}

const imagePullSecretsTemplate = `
kind: __kind__
spec:
  template:
    spec:
      imagePullSecrets:
      - name: __pullsecrets__
` // #nosec G101 (no actual secrets within)

func generateImagePullSecretsPatch(res *resource.Resource, mch *operatorsv1alpha1.MultiClusterHub) (ifc.Kunstructured, error) {
	pullSecret := mch.Spec.ImagePullSecret
	if pullSecret == "" {
		return nil, nil
	}
	template := strings.Replace(imagePullSecretsTemplate, "__kind__", res.GetKind(), 1)
	template = strings.Replace(template, "__pullsecrets__", pullSecret, 1)
	json, err := yaml.YAMLToJSON([]byte(template))
	if err != nil {
		return nil, err
	}
	var u unstructured.Unstructured
	err = u.UnmarshalJSON(json)
	return &kunstruct.UnstructAdapter{Unstructured: u}, err
}

func generateContainerArgsPatch(r *resource.Resource, newArgs map[string]string) (ifc.Kunstructured, error) {
	originalArgs, err := r.Kunstructured.GetStringSlice("spec.template.spec.containers[0].args")
	if err != nil {
		return nil, err
	}

	cmd, originalArgs := splitArgs(originalArgs)

	argsMap := toArgsMap(originalArgs)

	for newkey, newval := range newArgs {
		argsMap[fmt.Sprintf("--%s", newkey)] = newval
	}

	args := []string{}
	for k, v := range argsMap {
		arg := fmt.Sprintf("%s=%s", k, v)
		if v == "" {
			arg = k
		}
		args = append(args, arg)
	}
	sort.Strings(args)
	if cmd != "" {
		args = append([]string{cmd}, args...)
	}

	container, _ := r.GetFieldValue("spec.template.spec.containers[0]")
	containerMap, _ := container.(map[string]interface{})
	containerMap["args"] = args

	return kunstruct.NewKunstructuredFactoryImpl().FromMap(map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{containerMap},
				},
			},
		},
	}), nil
}

const nodeSelectorTemplate = `
kind: __kind__
spec:
  template:
    spec:
      nodeSelector: {__selector__}
`

func generateNodeSelectorPatch(res *resource.Resource, mch *operatorsv1alpha1.MultiClusterHub) (ifc.Kunstructured, error) {
	nodeSelectorOptions := mch.Spec.NodeSelector
	if nodeSelectorOptions == nil {
		return nil, nil
	}
	template := strings.Replace(nodeSelectorTemplate, "__kind__", res.GetKind(), 1)
	selectormap := map[string]string{}
	if nodeSelectorOptions.OS != "" {
		selectormap["beta.kubernetes.io/os"] = nodeSelectorOptions.OS
	}
	if nodeSelectorOptions.CustomLabelSelector != "" && nodeSelectorOptions.CustomLabelValue != "" {
		selectormap[nodeSelectorOptions.CustomLabelSelector] = nodeSelectorOptions.CustomLabelValue
	}
	if len(selectormap) == 0 {
		return nil, nil
	}
	selectors := []string{}
	for k, v := range selectormap {
		selectors = append(selectors, fmt.Sprintf("\"%s\":\"%s\"", k, v))
	}
	template = strings.Replace(template, "__selector__", strings.Join(selectors, ","), 1)
	json, err := yaml.YAMLToJSON([]byte(template))
	if err != nil {
		return nil, err
	}
	var u unstructured.Unstructured
	err = u.UnmarshalJSON(json)
	return &kunstruct.UnstructAdapter{Unstructured: u}, err
}

func generateEnvVarsPatch(r *resource.Resource, newEnvs []corev1.EnvVar) (ifc.Kunstructured, error) {
	origianl, err := r.GetSlice("spec.template.spec.containers[0].env")
	if err != nil {
		return nil, err
	}

	envMap := toNamedObjsMap(origianl)
	for _, newEnv := range newEnvs {
		envMap[newEnv.Name] = newEnv
	}

	envs := []interface{}{}
	for _, envName := range getSortedKeys(envMap) {
		envs = append(envs, envMap[envName])
	}

	container, _ := r.GetFieldValue("spec.template.spec.containers[0]")
	containerMap, _ := container.(map[string]interface{})
	containerMap["env"] = envs

	return kunstruct.NewKunstructuredFactoryImpl().FromMap(map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{containerMap},
				},
			},
		},
	}), nil
}

func generateVolumesPatch(r *resource.Resource, newVolumes []corev1.Volume) (ifc.Kunstructured, error) {
	origianl, err := r.GetSlice("spec.template.spec.volumes")
	if err != nil {
		return nil, err
	}

	volumesMap := toNamedObjsMap(origianl)
	for _, newVolume := range newVolumes {
		volumesMap[newVolume.Name] = newVolume
	}

	volumes := []interface{}{}
	for _, volumeName := range getSortedKeys(volumesMap) {
		volumes = append(volumes, volumesMap[volumeName])
	}

	return kunstruct.NewKunstructuredFactoryImpl().FromMap(map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"volumes": volumes,
				},
			},
		},
	}), nil
}

func generateVolumeMountPatch(r *resource.Resource, newVolumeMounts []corev1.VolumeMount) (ifc.Kunstructured, error) {
	origianl, err := r.GetSlice("spec.template.spec.containers[0].volumeMounts")
	if err != nil {
		return nil, err
	}
	volumeMountMap := toNamedObjsMap(origianl)
	for _, newVolumeMount := range newVolumeMounts {
		volumeMountMap[newVolumeMount.Name] = newVolumeMount
	}
	envs := []interface{}{}
	for _, envName := range getSortedKeys(volumeMountMap) {
		envs = append(envs, volumeMountMap[envName])
	}

	container, _ := r.GetFieldValue("spec.template.spec.containers[0]")
	containerMap, _ := container.(map[string]interface{})
	containerMap["volumeMounts"] = envs

	return kunstruct.NewKunstructuredFactoryImpl().FromMap(map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{containerMap},
				},
			},
		},
	}), nil
}

func splitArgs(args []string) (string, []string) {
	cmd := args[0]
	if !strings.HasPrefix(cmd, "--") {
		return cmd, args[1:]
	}
	return "", args
}

func toArgsMap(args []string) map[string]string {
	argsmap := map[string]string{}
	for _, arg := range args {
		index := strings.Index(arg, "=")
		if index == -1 {
			argsmap[arg] = ""
			continue
		}
		argsmap[arg[0:strings.Index(arg, "=")]] = arg[strings.Index(arg, "=")+1:]
	}
	return argsmap
}

func toNamedObjsMap(objs []interface{}) map[string]interface{} {
	objsMap := map[string]interface{}{}
	for _, obj := range objs {
		objmap, ok := obj.(map[string]interface{})
		if !ok {
			continue
		}
		name, ok := objmap["name"]
		if !ok {
			continue
		}
		objsMap[fmt.Sprintf("%s", name)] = obj
	}
	return objsMap
}

func getSortedKeys(objMap map[string]interface{}) []string {
	keys := []string{}
	for k := range objMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
