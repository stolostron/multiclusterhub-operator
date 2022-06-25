// Copyright Contributors to the Open Cluster Management project
package renderer

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	loader "helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"

	"github.com/fatih/structs"
	v1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	"helm.sh/helm/v3/pkg/engine"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

type Values struct {
	Global    Global    `yaml:"global" structs:"global"`
	HubConfig HubConfig `yaml:"hubconfig" structs:"hubconfig"`
	Org       string    `yaml:"org" structs:"org"`
}

type Global struct {
	ImageOverrides map[string]string `yaml:"imageOverrides" structs:"imageOverrides"`
	PullPolicy     string            `yaml:"pullPolicy" structs:"pullPolicy"`
	PullSecret     string            `yaml:"pullSecret" structs:"pullSecret"`
	Namespace      string            `yaml:"namespace" structs:"namespace"`
}

type HubConfig struct {
	NodeSelector map[string]string   `yaml:"nodeSelector" structs:"nodeSelector"`
	ProxyConfigs map[string]string   `yaml:"proxyConfigs" structs:"proxyConfigs"`
	ReplicaCount int                 `yaml:"replicaCount" structs:"replicaCount"`
	Tolerations  []corev1.Toleration `yaml:"tolerations" structs:"tolerations"`
	OCPVersion   string              `yaml:"ocpVersion" structs:"ocpVersion"`
}

func RenderCRDs(crdDir string) ([]*unstructured.Unstructured, []error) {
	var crds []*unstructured.Unstructured
	errs := []error{}

	if val, ok := os.LookupEnv("DIRECTORY_OVERRIDE"); ok {
		crdDir = path.Join(val, crdDir)
	} else {
		value, _ := os.LookupEnv("TEMPLATES_PATH")
		crdDir = path.Join(value, crdDir)

	}

	// Read CRD files
	err := filepath.Walk(crdDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Println(err.Error())
			return err
		}
		crd := &unstructured.Unstructured{}
		if info == nil || info.IsDir() {
			return nil
		}

		bytesFile, e := ioutil.ReadFile(filepath.Clean(path))
		if e != nil {
			errs = append(errs, fmt.Errorf("%s - error reading file: %v", info.Name(), err.Error()))
		}
		if err = yaml.Unmarshal(bytesFile, crd); err != nil {
			errs = append(errs, fmt.Errorf("%s - error unmarshalling file to unstructured: %v", info.Name(), err.Error()))
		}
		crds = append(crds, crd)
		return nil
	})
	if err != nil {
		return crds, errs
	}

	return crds, errs
}

func RenderCharts(chartDir string, mch *v1.MultiClusterHub, images map[string]string) ([]*unstructured.Unstructured, []error) {
	log := log.FromContext(context.Background())
	var templates []*unstructured.Unstructured
	errs := []error{}
	if val, ok := os.LookupEnv("DIRECTORY_OVERRIDE"); ok {
		chartDir = path.Join(val, chartDir)
	} else {
		value, _ := os.LookupEnv("TEMPLATES_PATH")
		chartDir = path.Join(value, chartDir)
	}
	charts, err := ioutil.ReadDir(chartDir)
	if err != nil {
		errs = append(errs, err)
	}
	for _, chart := range charts {
		chartPath := filepath.Join(chartDir, chart.Name())
		chartTemplates, errs := renderTemplates(chartPath, mch, images)
		if len(errs) > 0 {
			for _, err := range errs {
				log.Info(err.Error())
			}
			return nil, errs
		}
		templates = append(templates, chartTemplates...)
	}
	return templates, nil
}

func RenderChart(chartPath string, mch *v1.MultiClusterHub, images map[string]string) ([]*unstructured.Unstructured, []error) {
	log := log.FromContext(context.Background())
	errs := []error{}
	if val, ok := os.LookupEnv("DIRECTORY_OVERRIDE"); ok {
		chartPath = path.Join(val, chartPath)
	} else {
		value, _ := os.LookupEnv("TEMPLATES_PATH")
		chartPath = path.Join(value, chartPath)

	}
	chartTemplates, errs := renderTemplates(chartPath, mch, images)
	if len(errs) > 0 {
		for _, err := range errs {
			log.Info(err.Error())
		}
		return nil, errs
	}
	return chartTemplates, nil

}

func renderTemplates(chartPath string, mch *v1.MultiClusterHub, images map[string]string) ([]*unstructured.Unstructured, []error) {
	log := log.FromContext(context.Background())
	var templates []*unstructured.Unstructured
	errs := []error{}
	chart, err := loader.Load(chartPath)
	if err != nil {
		log.Info(fmt.Sprintf("error loading chart:"))
		return nil, append(errs, err)
	}
	valuesYaml := &Values{}
	injectValuesOverrides(valuesYaml, mch, images)
	helmEngine := engine.Engine{
		Strict:   true,
		LintMode: false,
	}
	rawTemplates, err := helmEngine.Render(chart, chartutil.Values{"Values": structs.Map(valuesYaml)})
	if err != nil {
		log.Info(fmt.Sprintf("error rendering chart: "))
		return nil, append(errs, err)
	}

	for fileName, templateFile := range rawTemplates {
		unstructured := &unstructured.Unstructured{}
		if err = yaml.Unmarshal([]byte(templateFile), unstructured); err != nil {
			return nil, append(errs, fmt.Errorf("error converting file %s to unstructured", fileName))
		}

		// Add namespace to namespaced resources
		switch unstructured.GetKind() {
		case "Deployment", "ServiceAccount", "Role", "RoleBinding", "Service", "ConfigMap":
			unstructured.SetNamespace(mch.Namespace)
		}
		templates = append(templates, unstructured)
	}

	return templates, errs
}

func injectValuesOverrides(values *Values, mch *v1.MultiClusterHub, images map[string]string) {

	values.Global.ImageOverrides = images

	values.Global.PullPolicy = string(utils.GetImagePullPolicy(mch))

	values.Global.Namespace = mch.Namespace

	values.Global.PullSecret = mch.Spec.ImagePullSecret

	values.HubConfig.ReplicaCount = utils.DefaultReplicaCount(mch)

	values.HubConfig.NodeSelector = mch.Spec.NodeSelector

	values.HubConfig.Tolerations = utils.GetTolerations(mch)

	values.Org = "open-cluster-management"

	values.HubConfig.OCPVersion = os.Getenv("ACM_HUB_OCP_VERSION")

	if utils.ProxyEnvVarsAreSet() {
		proxyVar := map[string]string{}
		proxyVar["HTTP_PROXY"] = os.Getenv("HTTP_PROXY")
		proxyVar["HTTPS_PROXY"] = os.Getenv("HTTPS_PROXY")
		proxyVar["NO_PROXY"] = os.Getenv("NO_PROXY")
		values.HubConfig.ProxyConfigs = proxyVar
	}

	// TODO: Define all overrides
}
