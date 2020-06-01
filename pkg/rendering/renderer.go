// Copyright (c) 2020 Red Hat, Inc.

package rendering

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"strconv"

	"github.com/fatih/structs"
	operatorsv1beta1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1beta1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/mcm"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/rendering/templates"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/kustomize/v3/pkg/resource"
)

const (
	apiserviceName      = "mcm-apiserver"
	controllerName      = "mcm-controller"
	webhookName         = "mcm-webhook"
	metadataErr         = "failed to find metadata field"
	proxyApiServiceName = "v1beta1.proxy.open-cluster-management.io"
)

var log = logf.Log.WithName("renderer")

type renderFn func(*resource.Resource) (*unstructured.Unstructured, error)

// Renderer is a Kustomizee Renderer Factory
type Renderer struct {
	cr        *operatorsv1beta1.MultiClusterHub
	renderFns map[string]renderFn
}

// NewRenderer Initializes a Kustomize Renderer Factory
func NewRenderer(multipleClusterHub *operatorsv1beta1.MultiClusterHub) *Renderer {
	renderer := &Renderer{cr: multipleClusterHub}
	renderer.renderFns = map[string]renderFn{
		"APIService":                   renderer.renderAPIServices,
		"Deployment":                   renderer.renderNamespace,
		"Service":                      renderer.renderNamespace,
		"ServiceAccount":               renderer.renderNamespace,
		"ConfigMap":                    renderer.renderNamespace,
		"ClusterRoleBinding":           renderer.renderClusterRoleBinding,
		"ClusterRole":                  renderer.renderClusterRole,
		"MutatingWebhookConfiguration": renderer.renderMutatingWebhookConfiguration,
		"Secret":                       renderer.renderSecret,
		"Subscription":                 renderer.renderNamespace,
		"EtcdCluster":                  renderer.renderEtcdCluster,
		"StatefulSet":                  renderer.renderNamespace,
		"Channel":                      renderer.renderNamespace,
		"HiveConfig":                   renderer.renderHiveConfig,
		"SecurityContextConstraints":   renderer.renderSecContextConstraints,
		"CustomResourceDefinition":     renderer.renderCRD,
	}
	return renderer
}

// Render renders Templates under TEMPLATES_PATH
func (r *Renderer) Render(c runtimeclient.Client) ([]*unstructured.Unstructured, error) {
	templates, err := templates.GetTemplateRenderer().GetTemplates()
	if err != nil {
		return nil, err
	}
	resources, err := r.renderTemplates(templates)
	if err != nil {
		return nil, err
	}
	return resources, nil
}

func (r *Renderer) renderTemplates(templates []*resource.Resource) ([]*unstructured.Unstructured, error) {
	uobjs := []*unstructured.Unstructured{}
	for _, template := range templates {
		render, ok := r.renderFns[template.GetKind()]
		if !ok {
			uobjs = append(uobjs, &unstructured.Unstructured{Object: template.Map()})
			continue
		}
		uobj, err := render(template.DeepCopy())
		if err != nil {
			return []*unstructured.Unstructured{}, err
		}
		if uobj == nil {
			continue
		}
		uobjs = append(uobjs, uobj)

	}

	// mcm-mutating-webhook has a section `webhooks.clientConfig.caBundle` created in secret mcm-webhook-secrets.
	// Current render mechanism cannot handle this dependence scenario. So re-render template dependence in the end.
	uobjs, err := reRenderDependence(uobjs)
	return uobjs, err
}

func (r *Renderer) renderAPIServices(res *resource.Resource) (*unstructured.Unstructured, error) {
	u := &unstructured.Unstructured{Object: res.Map()}
	spec, ok := u.Object["spec"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to find apiservices spec field")
	}
	metadata, ok := u.Object["metadata"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to find metadata field")
	}
	if metadata["name"] == proxyApiServiceName {
		spec["service"] = map[string]interface{}{
			"namespace": r.cr.Namespace,
			"name":      mcm.ACMProxyServerName,
		}
	} else {
		spec["service"] = map[string]interface{}{
			"namespace": r.cr.Namespace,
			"name":      apiserviceName,
		}
	}
	utils.AddInstallerLabel(u, r.cr.GetName(), r.cr.GetNamespace())
	return u, nil
}

func (r *Renderer) renderNamespace(res *resource.Resource) (*unstructured.Unstructured, error) {
	u := &unstructured.Unstructured{Object: res.Map()}

	if UpdateNamespace(u) {
		res.SetNamespace(r.cr.Namespace)
	}

	return &unstructured.Unstructured{Object: res.Map()}, nil
}

func (r *Renderer) renderClusterRole(res *resource.Resource) (*unstructured.Unstructured, error) {
	u := &unstructured.Unstructured{Object: res.Map()}
	utils.AddInstallerLabel(u, r.cr.GetName(), r.cr.GetNamespace())
	return u, nil
}

func (r *Renderer) renderClusterRoleBinding(res *resource.Resource) (*unstructured.Unstructured, error) {
	u := &unstructured.Unstructured{Object: res.Map()}

	utils.AddInstallerLabel(u, r.cr.GetName(), r.cr.GetNamespace())

	var clusterRoleBinding v1.ClusterRoleBinding
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &clusterRoleBinding)
	if err != nil {
		log.Error(err, "Failed to unmarshal clusterrolebindding")
		return nil, err
	}

	subject := clusterRoleBinding.Subjects[0]
	if subject.Kind == "Group" {
		return u, nil
	}

	if UpdateNamespace(u) {
		clusterRoleBinding.Subjects[0].Namespace = r.cr.Namespace
	}

	newCRB, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&clusterRoleBinding)
	if err != nil {
		log.Error(err, "Failed to unmarshal clusterrolebinding")
		return nil, err
	}

	return &unstructured.Unstructured{Object: newCRB}, nil
}

func (r *Renderer) renderCRD(res *resource.Resource) (*unstructured.Unstructured, error) {
	u := &unstructured.Unstructured{Object: res.Map()}
	utils.AddInstallerLabel(u, r.cr.GetName(), r.cr.GetNamespace())
	return u, nil
}

func (r *Renderer) renderMutatingWebhookConfiguration(res *resource.Resource) (*unstructured.Unstructured, error) {
	u := &unstructured.Unstructured{Object: res.Map()}
	webooks, ok := u.Object["webhooks"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to find webhooks spec field")
	}
	webhook := webooks[0].(map[string]interface{})
	clientConfig := webhook["clientConfig"].(map[string]interface{})
	service := clientConfig["service"].(map[string]interface{})

	service["namespace"] = r.cr.Namespace
	utils.AddInstallerLabel(u, r.cr.GetName(), r.cr.GetNamespace())
	return u, nil
}

func (r *Renderer) renderSecret(res *resource.Resource) (*unstructured.Unstructured, error) {
	caCert, tlsCert, tlsKey := "ca.crt", "tls.crt", "tls.key"
	u := &unstructured.Unstructured{Object: res.Map()}
	metadata, ok := u.Object["metadata"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to find metadata field")
	}
	data, ok := u.Object["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf(metadataErr)
	}

	metadata["namespace"] = r.cr.Namespace

	name := res.GetName()

	switch name {
	case "mcm-apiserver-self-signed-secrets":
		ca, err := utils.GenerateSelfSignedCACert("multiclusterhub-api")
		if err != nil {
			return nil, err
		}
		alternateDNS := []string{
			fmt.Sprintf("%s.%s", apiserviceName, r.cr.Namespace),
			fmt.Sprintf("%s.%s.svc", apiserviceName, r.cr.Namespace),
		}
		cert, err := utils.GenerateSignedCert(apiserviceName, alternateDNS, ca)
		if err != nil {
			return nil, err
		}
		data[caCert] = base64.StdEncoding.EncodeToString([]byte(ca.Cert))
		data[tlsCert] = base64.StdEncoding.EncodeToString([]byte(cert.Cert))
		data[tlsKey] = base64.StdEncoding.EncodeToString([]byte(cert.Key))

		return u, nil
	case "mcm-klusterlet-self-signed-secrets":
		ca, err := utils.GenerateSelfSignedCACert("multiclusterhub-klusterlet")
		if err != nil {
			return nil, err
		}
		cert, err := utils.GenerateSignedCert("multicluterhub-klusterlet", []string{}, ca)
		if err != nil {
			return nil, err
		}
		data[caCert] = base64.StdEncoding.EncodeToString([]byte(ca.Cert))
		data[tlsCert] = base64.StdEncoding.EncodeToString([]byte(cert.Cert))
		data[tlsKey] = base64.StdEncoding.EncodeToString([]byte(cert.Key))
		return u, nil
	case "mcm-webhook-secret":
		cn := "mcm-webhook." + r.cr.Namespace + ".svc"
		ca, err := utils.GenerateSelfSignedCACert(cn)
		if err != nil {
			return nil, err
		}
		cert, err := utils.GenerateSignedCert(cn, []string{}, ca)
		if err != nil {
			return nil, err
		}
		data[caCert] = base64.StdEncoding.EncodeToString([]byte(ca.Cert))
		data[tlsCert] = base64.StdEncoding.EncodeToString([]byte(cert.Cert))
		data[tlsKey] = base64.StdEncoding.EncodeToString([]byte(cert.Key))
		return u, nil
	}

	return u, nil
}

func (r *Renderer) renderHiveConfig(res *resource.Resource) (*unstructured.Unstructured, error) {
	u := &unstructured.Unstructured{Object: res.Map()}
	HiveConfig := operatorsv1beta1.HiveConfigSpec{}

	if !reflect.DeepEqual(structs.Map(r.cr.Spec.Hive), structs.Map(HiveConfig)) {
		u.Object["spec"] = structs.Map(r.cr.Spec.Hive)
	}
	utils.AddInstallerLabel(u, r.cr.GetName(), r.cr.GetNamespace())
	return u, nil
}

func reRenderDependence(objs []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
	var ca interface{}
	var mutatingConfig *unstructured.Unstructured
	for _, obj := range objs {
		if obj.GetKind() == "Secret" && obj.GetName() == "mcm-webhook-secret" {
			data, ok := obj.Object["data"].(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("failed to get ca in mcm-webhook-secrets")
			}
			ca = data["ca.crt"]
		}

		if obj.GetKind() == "MutatingWebhookConfiguration" && obj.GetName() == "mcm-mutating-webhook" {
			mutatingConfig = obj
		}
	}

	if ca != nil && mutatingConfig != nil {
		webooks, ok := mutatingConfig.Object["webhooks"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("failed to find webhooks spec field")
		}
		webhook := webooks[0].(map[string]interface{})
		clientConfig := webhook["clientConfig"].(map[string]interface{})
		clientConfig["caBundle"] = ca
	}

	return objs, nil
}

func (r *Renderer) renderSecContextConstraints(res *resource.Resource) (*unstructured.Unstructured, error) {
	u := &unstructured.Unstructured{Object: res.Map()}
	users, ok := u.Object["users"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to find users field")
	}
	ns := r.cr.Namespace
	users[0] = fmt.Sprintf("system:serviceaccount:%s:default", ns)
	return u, nil
}

// UpdateNamespace checks for annotiation to update NS
func UpdateNamespace(u *unstructured.Unstructured) bool {
	metadata, ok := u.Object["metadata"].(map[string]interface{})
	updateNamespace := true
	if ok {
		annotations, ok := metadata["annotations"].(map[string]interface{})
		if ok {
			if annotations["update-namespace"] != "" {
				updateNamespace, _ = strconv.ParseBool(annotations["update-namespace"].(string))
			}
		}
	}
	return updateNamespace
}

func (r *Renderer) renderEtcdCluster(res *resource.Resource) (*unstructured.Unstructured, error) {
	r.renderNamespace(res)
	u := &unstructured.Unstructured{Object: res.Map()}
	spec, ok := u.Object["spec"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to find Etcd spec field")
	}

	pod, ok := spec["pod"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to find Etcd spec pod field")
	}
	persistentVolumeClaimSpec, ok := pod["persistentVolumeClaimSpec"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to find Etcd spec pod persistentVolumeClaimSpec field")
	}
	persistentVolumeClaimSpec["storageClassName"] = r.cr.Spec.Etcd.StorageClass

	resources, ok := persistentVolumeClaimSpec["resources"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to find Etcd spec pod persistentVolumeClaimSpec resources field")
	}
	requests, ok := resources["requests"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to find Etcd spec pod persistentVolumeClaimSpec resources requests field")
	}
	requests["storage"] = r.cr.Spec.Etcd.Storage
	return u, nil
}
