// Copyright Contributors to the Open Cluster Management project
package renderer

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strconv"

	loader "helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"

	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	v1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	"github.com/stolostron/multiclusterhub-operator/pkg/version"
	"helm.sh/helm/v3/pkg/engine"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	log "k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

type Values struct {
	Global    Global    `json:"global" structs:"global"`
	HubConfig HubConfig `json:"hubconfig" structs:"hubconfig"`
	Org       string    `json:"org" structs:"org"`
}

type Global struct {
	ImageOverrides      map[string]string    `json:"imageOverrides" structs:"imageOverrides"`
	PullPolicy          string               `json:"pullPolicy" structs:"pullPolicy"`
	PullSecret          string               `json:"pullSecret" structs:"pullSecret"`
	Namespace           string               `json:"namespace" structs:"namespace"`
	ImageRepository     string               `json:"imageRepository" structs:"namespace"`
	Name                string               `json:"name" structs:"name"`
	Channel             string               `json:"channel" structs:"Channel"`
	InstallPlanApproval subv1alpha1.Approval `json:"installPlanApproval" structs:"installPlanApproval"`
	Source              string               `json:"source" structs:"source"`
	SourceNamespace     string               `json:"sourceNamespace" structs:"sourceNamespace"`
}

type HubConfig struct {
	NodeSelector      map[string]string `json:"nodeSelector" structs:"nodeSelector"`
	ProxyConfigs      map[string]string `json:"proxyConfigs" structs:"proxyConfigs"`
	ReplicaCount      int               `json:"replicaCount" structs:"replicaCount"`
	Tolerations       []Toleration      `json:"tolerations" structs:"tolerations"`
	OCPVersion        string            `json:"ocpVersion" structs:"ocpVersion"`
	HubVersion        string            `json:"hubVersion" structs:"hubVersion"`
	OCPIngress        string            `json:"ocpIngress" structs:"ocpIngress"`
	SubscriptionPause string            `json:"subscriptionPause" structs:"subscriptionPause"`
}

type Toleration struct {
	Key               string                    `json:"Key" protobuf:"bytes,1,opt,name=key"`
	Operator          corev1.TolerationOperator `json:"Operator" protobuf:"bytes,2,opt,name=operator,casttype=TolerationOperator"`
	Value             string                    `json:"Value" protobuf:"bytes,3,opt,name=value"`
	Effect            corev1.TaintEffect        `json:"Effect" protobuf:"bytes,4,opt,name=effect,casttype=TaintEffect"`
	TolerationSeconds *int64                    `json:"TolerationSeconds" protobuf:"varint,5,opt,name=tolerationSeconds"`
}

func convertTolerations(tols []corev1.Toleration) []Toleration {
	var tolerations []Toleration
	for _, t := range tols {
		tolerations = append(tolerations, Toleration{
			Key:               t.Key,
			Operator:          t.Operator,
			Value:             t.Value,
			Effect:            t.Effect,
			TolerationSeconds: t.TolerationSeconds,
		})
	}
	return tolerations
}

func (u *Toleration) MarshalJSON() ([]byte, error) {

	v := reflect.ValueOf(u)
	values := make([]string, reflect.Indirect(v).NumField())
	var operator corev1.TolerationOperator = u.Operator
	var effect corev1.TaintEffect = u.Effect

	//Marshal all Toleration fields that are a number or true/false into a string
	for i := 0; i < reflect.Indirect(v).NumField(); i++ {
		switch reflect.Indirect(v).Field(i).Kind() {
		case reflect.String:
			if str, ok := reflect.Indirect(v).Field(i).Interface().(string); ok {
				if _, err := strconv.Atoi(str); err == nil {
					values[i] = fmt.Sprintf("'%s'", str)
				} else if _, err := strconv.ParseFloat(str, 64); err == nil {
					values[i] = fmt.Sprintf("'%s'", str)
				} else if _, err := strconv.ParseBool(str); err == nil {
					values[i] = fmt.Sprintf("'%s'", str)
				} else {
					values[i] = str
				}
			}
			if tol, ok := reflect.Indirect(v).Field(i).Interface().(corev1.TolerationOperator); ok {
				str := string(tol)
				if _, err := strconv.Atoi(str); err == nil {
					operator = corev1.TolerationOperator(fmt.Sprintf("'%s'", str))
				} else if _, err := strconv.ParseFloat(str, 64); err == nil {
					operator = corev1.TolerationOperator(fmt.Sprintf("'%s'", str))
				} else if _, err := strconv.ParseBool(str); err == nil {
					operator = corev1.TolerationOperator(fmt.Sprintf("'%s'", str))
				}
			}
			if eff, ok := reflect.Indirect(v).Field(i).Interface().(corev1.TaintEffect); ok {
				str := string(eff)
				if _, err := strconv.Atoi(str); err == nil {
					effect = corev1.TaintEffect(fmt.Sprintf("'%s'", str))
				} else if _, err := strconv.ParseFloat(str, 64); err == nil {
					effect = corev1.TaintEffect(fmt.Sprintf("'%s'", str))
				} else if _, err := strconv.ParseBool(str); err == nil {
					effect = corev1.TaintEffect(fmt.Sprintf("'%s'", str))
				}
			}
		}

	}

	return json.Marshal(&struct {
		Key               string                    `json:"Key" protobuf:"bytes,1,opt,name=key"`
		Operator          corev1.TolerationOperator `json:"Operator" protobuf:"bytes,2,opt,name=operator,casttype=TolerationOperator"`
		Value             string                    `json:"Value" protobuf:"bytes,3,opt,name=value"`
		Effect            corev1.TaintEffect        `json:"Effect" protobuf:"bytes,4,opt,name=effect,casttype=TaintEffect"`
		TolerationSeconds *int64                    `json:"TolerationSeconds" protobuf:"varint,5,opt,name=tolerationSeconds"`
	}{
		Key:               values[0],
		Operator:          operator,
		Value:             values[2],
		Effect:            effect,
		TolerationSeconds: u.TolerationSeconds,
	})
}

func (val *Values) ToValues() (chartutil.Values, error) {
	inrec, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}
	vals, err := chartutil.ReadValues(inrec)
	if err != nil {
		return vals, err
	}
	return vals, nil

}

func RenderCRDs(crdDir string) ([]*unstructured.Unstructured, []error) {
	var crds []*unstructured.Unstructured
	errs := []error{}

	// Read CRD files
	err := filepath.Walk(crdDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Println(err.Error())
			return err
		}
		crd := &unstructured.Unstructured{}
		if info == nil || info.IsDir() || filepath.Ext(path) != ".yaml" {
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

	vals, err := valuesYaml.ToValues()
	if err != nil {
		log.Info(fmt.Sprintf("error rendering chart: %s", chart.Name()))
		return nil, append(errs, err)
	}

	rawTemplates, err := helmEngine.Render(chart, chartutil.Values{"Values": vals.AsMap()})
	if err != nil {
		log.Info(fmt.Sprintf("error rendering chart: %s", chart.Name()))
		return nil, append(errs, err)
	}

	for fileName, templateFile := range rawTemplates {
		unstructured := &unstructured.Unstructured{}
		if err = yaml.Unmarshal([]byte(templateFile), unstructured); err != nil {
			return nil, append(errs, fmt.Errorf("error converting file %s to unstructured", fileName))
		}

		// Add namespace to namespaced resources
		switch unstructured.GetKind() {
		case "Deployment", "ServiceAccount", "Role", "RoleBinding", "Service", "ConfigMap", "Ingress", "Channel", "Subscription":
			if unstructured.GetNamespace() == "" {
				unstructured.SetNamespace(mch.Namespace)
			}
		}
		utils.AddInstallerLabel(unstructured, mch.Name, mch.Namespace)
		templates = append(templates, unstructured)
	}

	return templates, errs
}

func injectValuesOverrides(values *Values, mch *v1.MultiClusterHub, images map[string]string) {

	values.Global.ImageOverrides = images

	values.Global.PullPolicy = string(utils.GetImagePullPolicy(mch))

	values.Global.Namespace = mch.Namespace

	values.Global.PullSecret = mch.Spec.ImagePullSecret

	values.Global.ImageRepository = utils.GetImageRepository(mch)

	values.HubConfig.ReplicaCount = utils.DefaultReplicaCount(mch)

	values.HubConfig.NodeSelector = mch.Spec.NodeSelector

	values.HubConfig.Tolerations = convertTolerations(utils.GetTolerations(mch))

	values.HubConfig.OCPVersion = os.Getenv("ACM_HUB_OCP_VERSION")

	values.HubConfig.HubVersion = version.Version

	values.HubConfig.OCPIngress = os.Getenv("INGRESS_DOMAIN")

	values.HubConfig.SubscriptionPause = utils.GetDisableClusterImageSets(mch)

	values.Org = "open-cluster-management"

	if utils.ProxyEnvVarsAreSet() {
		proxyVar := map[string]string{}
		proxyVar["HTTP_PROXY"] = os.Getenv("HTTP_PROXY")
		proxyVar["HTTPS_PROXY"] = os.Getenv("HTTPS_PROXY")
		proxyVar["NO_PROXY"] = os.Getenv("NO_PROXY")
		values.HubConfig.ProxyConfigs = proxyVar
	}

	values.Global.Name, values.Global.Channel, values.Global.InstallPlanApproval, values.Global.Source, values.Global.SourceNamespace = GetOADPConfig(mch)

	// TODO: Define all overrides
}

func GetOADPConfig(m *v1.MultiClusterHub) (string, string, subv1alpha1.Approval, string, string) {
	sub := &subv1alpha1.SubscriptionSpec{}
	var name, channel, source, sourceNamespace string
	var installPlan subv1alpha1.Approval
	if oadpSpec := utils.GetOADPAnnotationOverrides(m); oadpSpec != "" {

		err := json.Unmarshal([]byte(oadpSpec), sub)
		if err != nil {
			log.Info(fmt.Sprintf("Failed to unmarshal OADP annotation: %s.", oadpSpec))
			return "", "", "", "", ""
		}
	}

	if sub.Package != "" {
		name = sub.Package
	} else {
		name = "redhat-oadp-operator"
	}

	if sub.Channel != "" {
		channel = sub.Channel
	} else {
		channel = "stable-1.2"
	}

	if sub.InstallPlanApproval != "" {
		installPlan = sub.InstallPlanApproval
	} else {
		installPlan = "Automatic"
	}

	if sub.CatalogSource != "" {
		source = sub.CatalogSource
	} else {
		source = "redhat-operators"
	}

	if sub.CatalogSourceNamespace != "" {
		sourceNamespace = sub.CatalogSourceNamespace
	} else {
		sourceNamespace = "openshift-marketplace"
	}
	return name, channel, installPlan, source, sourceNamespace

}
