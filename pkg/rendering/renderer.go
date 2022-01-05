// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package rendering

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/fatih/structs"
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/foundation"
	"github.com/stolostron/multiclusterhub-operator/pkg/rendering/templates"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/kustomize/v3/pkg/resource"
)

const (
	metadataErr         = "failed to find metadata field"
	proxyApiServiceName = "v1beta1.proxy.open-cluster-management.io"
)

var log = logf.Log.WithName("renderer")

type renderFn func(*resource.Resource) (*unstructured.Unstructured, error)

// Renderer is a Kustomizee Renderer Factory
type Renderer struct {
	cr        *operatorsv1.MultiClusterHub
	renderFns map[string]renderFn
}

// NewRenderer Initializes a Kustomize Renderer Factory
func NewRenderer(multipleClusterHub *operatorsv1.MultiClusterHub) *Renderer {
	renderer := &Renderer{
		cr: multipleClusterHub,
	}
	renderer.renderFns = map[string]renderFn{
		"APIService":                     renderer.renderAPIServices,
		"Deployment":                     renderer.renderNamespace,
		"Service":                        renderer.renderNamespace,
		"ServiceAccount":                 renderer.renderNamespace,
		"ConfigMap":                      renderer.renderNamespace,
		"ClusterRoleBinding":             renderer.renderClusterRoleBinding,
		"ClusterRole":                    renderer.renderClusterRole,
		"MutatingWebhookConfiguration":   renderer.renderMutatingWebhookConfiguration,
		"ValidatingWebhookConfiguration": renderer.renderValidatingWebhookConfiguration,
		"Subscription":                   renderer.renderNamespace,
		"StatefulSet":                    renderer.renderNamespace,
		"Channel":                        renderer.renderNamespace,
		"HiveConfig":                     renderer.renderHiveConfig,
		"CustomResourceDefinition":       renderer.renderCRD,
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

	return uobjs, nil
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
			"name":      foundation.OCMProxyServerName,
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

func (r *Renderer) renderValidatingWebhookConfiguration(res *resource.Resource) (*unstructured.Unstructured, error) {
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

func (r *Renderer) renderHiveConfig(res *resource.Resource) (*unstructured.Unstructured, error) {
	u := &unstructured.Unstructured{Object: res.Map()}
	HiveConfig := operatorsv1.HiveConfigSpec{}

	if r.cr.Spec.Hive != nil && !reflect.DeepEqual(structs.Map(r.cr.Spec.Hive), structs.Map(HiveConfig)) {
		u.Object["spec"] = structs.Map(r.cr.Spec.Hive)
	}
	utils.AddInstallerLabel(u, r.cr.GetName(), r.cr.GetNamespace())
	return u, nil
}

// UpdateNamespace checks for annotiation to update NS
func UpdateNamespace(u *unstructured.Unstructured) bool {
	metadata, ok := u.Object["metadata"].(map[string]interface{})
	updateNamespace := true
	if ok {
		annotations, ok := metadata["annotations"].(map[string]string)
		if ok {
			if annotations["update-namespace"] != "" {
				updateNamespace, _ = strconv.ParseBool(annotations["update-namespace"])
			}
		}
	}
	return updateNamespace
}
