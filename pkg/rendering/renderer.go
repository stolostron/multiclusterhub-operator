package rendering

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/v3/pkg/resource"

	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/rendering/patching"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/rendering/templates"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
)

const (
	apiserviceName        = "mcm-apiserver"
	controllerName        = "mcm-controller"
	webhookName           = "webhook-core-webhook"
	clusterControllerName = "multicloud-operators-cluster-controller"
)

type renderFn func(*resource.Resource) (*unstructured.Unstructured, error)

type Renderer struct {
	cr        *operatorsv1alpha1.MultiCloudHub
	renderFns map[string]renderFn
}

func NewRenderer(multipleCloudHub *operatorsv1alpha1.MultiCloudHub) *Renderer {
	renderer := &Renderer{cr: multipleCloudHub}
	renderer.renderFns = map[string]renderFn{
		"APIService":                   renderer.renderAPIServices,
		"Deployment":                   renderer.renderDeployments,
		"Service":                      renderer.renderNamespace,
		"ServiceAccount":               renderer.renderNamespace,
		"ClusterRoleBinding":           renderer.renderClusterRoleBinding,
		"MutatingWebhookConfiguration": renderer.renderMutatingWebhookConfiguration,
		"Secret":                       renderer.renderSecret,
		"Subscription":                 renderer.renderBaseMetadataNamespace,
		"EtcdCluster":                  renderer.renderBaseMetadataNamespace,
		"StatefulSet":                  renderer.renderBaseMetadataNamespace,
	}
	return renderer
}

func (r *Renderer) Render() ([]*unstructured.Unstructured, error) {
	templates, err := templates.GetTemplateRenderer().GetTemplates(r.cr)
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
	spec["service"] = map[string]interface{}{
		"namespace": r.cr.Namespace,
		"name":      apiserviceName,
	}
	return u, nil
}

func (r *Renderer) renderNamespace(res *resource.Resource) (*unstructured.Unstructured, error) {
	res.SetNamespace(r.cr.Namespace)
	return &unstructured.Unstructured{Object: res.Map()}, nil
}

func (r *Renderer) renderDeployments(res *resource.Resource) (*unstructured.Unstructured, error) {
	err := patching.ApplyGlobalPatches(res, r.cr)
	if err != nil {
		return nil, err
	}

	res.SetNamespace(r.cr.Namespace)

	name := res.GetName()
	switch name {
	case apiserviceName:
		if err := patching.ApplyAPIServerPatches(res, r.cr); err != nil {
			return nil, err
		}
		return &unstructured.Unstructured{Object: res.Map()}, nil
	case controllerName:
		if err := patching.ApplyControllerPatches(res, r.cr); err != nil {
			return nil, err
		}
		return &unstructured.Unstructured{Object: res.Map()}, nil
	case webhookName:
		return &unstructured.Unstructured{Object: res.Map()}, nil
	case clusterControllerName:
		return &unstructured.Unstructured{Object: res.Map()}, nil
	default:
		return nil, fmt.Errorf("unknown MultipleCloudHub deployment component %s", name)
	}
}

func (r *Renderer) renderClusterRoleBinding(res *resource.Resource) (*unstructured.Unstructured, error) {
	u := &unstructured.Unstructured{Object: res.Map()}
	subjects, ok := u.Object["subjects"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to find clusterrolebinding subjects field")
	}
	subject := subjects[0].(map[string]interface{})
	kind := subject["kind"]
	if kind == "Group" {
		return u, nil
	}
	subject["namespace"] = r.cr.Namespace
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
	return u, nil
}

func (r *Renderer) renderBaseMetadataNamespace(res *resource.Resource) (*unstructured.Unstructured, error) {
	u := &unstructured.Unstructured{Object: res.Map()}
	metadata, ok := u.Object["metadata"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to find metadata field")
	}

	metadata["namespace"] = r.cr.Namespace
	return u, nil
}

func (r *Renderer) renderSecret(res *resource.Resource) (*unstructured.Unstructured, error) {
	u := &unstructured.Unstructured{Object: res.Map()}
	metadata, ok := u.Object["metadata"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to find metadata field")
	}
	data, ok := u.Object["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to find data field")
	}

	metadata["namespace"] = r.cr.Namespace

	name := res.GetName()

	if name == "mcm-apiserver-self-signed-secrets" {
		ca, err := utils.GenerateSelfSignedCACert("multicloudhub-api")
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
		data["ca.crt"] = []byte(ca.Cert)
		data["tls.crt"] = []byte(cert.Cert)
		data["tls.key"] = []byte(cert.Key)
	} else if name == "mcm-klusterlet-self-signed-secrets" {
		ca, err := utils.GenerateSelfSignedCACert("multicloudhub-klusterlet")
		if err != nil {
			return nil, err
		}
		cert, err := utils.GenerateSignedCert("multicloudhub-klusterlet", []string{}, ca)
		if err != nil {
			return nil, err
		}
		data["ca.crt"] = []byte(ca.Cert)
		data["tls.crt"] = []byte(cert.Cert)
		data["tls.key"] = []byte(cert.Key)
	}

	return u, nil
}
