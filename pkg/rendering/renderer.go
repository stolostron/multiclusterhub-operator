// Copyright Contributors to the Open Cluster Management project

// Package renderer provides Helm chart rendering functionality for
// MultiClusterHub components.
//
// This package handles template rendering, value injection, and chart version
// management for all ACM components deployed by the operator. It supports:
//   - Rendering Helm charts with custom values
//   - Injecting image overrides
//   - Managing chart dependencies
//   - CRD discovery and installation
//
// The main entry point is the RenderChart function, which takes a chart path
// and configuration and returns rendered Kubernetes manifests.
package renderer

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	loader "helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"

	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	v1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/helpers"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	"github.com/stolostron/multiclusterhub-operator/pkg/version"
	"helm.sh/helm/v3/pkg/engine"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

// defaults for the OADP subscription that will be created by the installer
const (
	// defaultOADPChannel specifies the minimum OADP channel version that should be installed by default.
	// This also represents the minimum expected version for installation.
	// Used by ACM installed on OCP 4.18 or older
	defaultOADPChannel = "stable-1.4"

	// defaultOADPChannel specifies the minimum OADP channel version that should be installed by default.
	// This also represents the minimum expected version for installation.
	// Used by ACM installed on OCP 4.19 or newer
	defaultOADPStableChannel = "stable"

	// defaultOADPName is the name of the OADP operator to be created by the installer.
	defaultOADPName = "redhat-oadp-operator"

	// defaultOADPInstallPlan defines the default approval policy for the OADP subscription's install plan.
	// Supported values are: "Automatic" or "Manual".
	defaultOADPInstallPlan = "Automatic"

	// defaultOADPCatalogSource specifies the default operator catalog source for the OADP subscription.
	defaultOADPCatalogSource = "redhat-operators"

	// defaultOADPCatalogSourceNamespace defines the default namespace where the OADP operator catalog source is located.
	defaultOADPCatalogSourceNamespace = "openshift-marketplace"
)

// defaults for the MTV subscription referenced by the mtv-operator AddOnTemplate workload manifest
const (
	// defaultMTVChannel specifies the MTV channel that the mtv-operator AddOnTemplate subscribes to by default.
	defaultMTVChannel = "release-v5.0"

	// defaultMTVPackageName is the name of the MTV operator package subscribed to by the AddOnTemplate.
	defaultMTVPackageName = "mtv-operator"

	// defaultMTVSubscriptionNamespace is the namespace the MTV subscription is created in on the managed cluster.
	defaultMTVSubscriptionNamespace = "openshift-mtv"

	// defaultMTVUpgradeApproval defines the default upgrade approval policy for the MTV subscription.
	// Supported values are: "Automatic" or "Manual".
	defaultMTVUpgradeApproval = "Automatic"
)

// defaults for the CNV subscription referenced by the kubevirt-hyperconverged AddOnTemplate workload manifest
const (
	// defaultCNVChannel specifies the CNV channel that the kubevirt-hyperconverged AddOnTemplate subscribes to by default.
	defaultCNVChannel = "stable"

	// defaultCNVPackageName is the name of the CNV operator package subscribed to by the AddOnTemplate.
	defaultCNVPackageName = "kubevirt-hyperconverged"

	// defaultCNVSubscriptionNamespace is the namespace the CNV subscription is created in on the managed cluster.
	defaultCNVSubscriptionNamespace = "openshift-cnv"

	// defaultCNVUpgradeApproval defines the default upgrade approval policy for the CNV subscription.
	// Supported values are: "Automatic" or "Manual".
	defaultCNVUpgradeApproval = "Automatic"
)

var log = logf.Log.WithName("reconcile")

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

// parseProbeConfigFromAnnotations reads probe config from MCH annotations
func parseProbeConfigFromAnnotations(mch *v1.MultiClusterHub) *ProbeConfig {
	if mch.Annotations == nil {
		return nil
	}

	var config ProbeConfig
	hasAny := false

	if val, ok := mch.Annotations[utils.AnnotationProbeTimeoutSeconds]; ok {
		timeout, err := strconv.ParseInt(val, 10, 32)
		if err != nil {
			log.Info(fmt.Sprintf("Invalid probe-timeout-seconds annotation value %q: %v", val, err))
		} else if timeout <= 0 {
			log.Info(fmt.Sprintf("Invalid probe-timeout-seconds annotation value %q: must be positive", val))
		} else {
			timeout32 := int32(timeout)
			config.TimeoutSeconds = &timeout32
			hasAny = true
		}
	}

	if val, ok := mch.Annotations[utils.AnnotationProbeFailureThreshold]; ok {
		threshold, err := strconv.ParseInt(val, 10, 32)
		if err != nil {
			log.Info(fmt.Sprintf("Invalid probe-failure-threshold annotation value %q: %v", val, err))
		} else if threshold <= 0 {
			log.Info(fmt.Sprintf("Invalid probe-failure-threshold annotation value %q: must be positive", val))
		} else {
			threshold32 := int32(threshold)
			config.FailureThreshold = &threshold32
			hasAny = true
		}
	}

	if val, ok := mch.Annotations[utils.AnnotationProbeSuccessThreshold]; ok {
		threshold, err := strconv.ParseInt(val, 10, 32)
		if err != nil {
			log.Info(fmt.Sprintf("Invalid probe-success-threshold annotation value %q: %v", val, err))
		} else if threshold <= 0 {
			log.Info(fmt.Sprintf("Invalid probe-success-threshold annotation value %q: must be positive", val))
		} else {
			threshold32 := int32(threshold)
			config.SuccessThreshold = &threshold32
			hasAny = true
		}
	}

	if !hasAny {
		return nil
	}

	return &config
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

func RenderCRDs(crdDir string, mch *v1.MultiClusterHub) ([]*unstructured.Unstructured, []error) {
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

		bytesFile, e := os.ReadFile(filepath.Clean(path))
		if e != nil {
			errs = append(errs, fmt.Errorf("%s - error reading file: %v", info.Name(), err))
		}

		if err = yaml.Unmarshal(bytesFile, crd); err != nil {
			errs = append(errs, fmt.Errorf("%s - error unmarshalling file to unstructured: %v", info.Name(), err.Error()))
		}
		if mch != nil {
			_, conversion, _ := unstructured.NestedMap(crd.Object, "spec", "conversion", "webhook", "clientConfig", "service")
			if conversion {
				crd.Object["spec"].(map[string]interface{})["conversion"].(map[string]interface{})["webhook"].(map[string]interface{})["clientConfig"].(map[string]interface{})["service"].(map[string]interface{})["namespace"] = mch.Namespace
			}
		}
		crds = append(crds, crd)
		return nil
	})
	if err != nil {
		return crds, errs
	}

	return crds, errs
}

// RenderCharts renders all Helm charts in the specified directory.
// olmVersion: OLM version detected at runtime ("v0", "v1", or "" for no OLM).
// Passed to chart templates as .Values.global.olmVersion for conditional rendering.
func RenderCharts(chartDir string, mch *v1.MultiClusterHub, images map[string]string, tpl map[string]string,
	isSTSEnabled bool, olmVersion string) ([]*unstructured.Unstructured, []error) {

	var templates []*unstructured.Unstructured
	errs := []error{}

	if val, ok := os.LookupEnv("DIRECTORY_OVERRIDE"); ok {
		chartDir = path.Join(val, chartDir)
	} else {
		value, _ := os.LookupEnv("TEMPLATES_PATH")
		chartDir = path.Join(value, chartDir)
	}

	charts, err := os.ReadDir(chartDir)
	if err != nil {
		errs = append(errs, err)
	}

	for _, chart := range charts {
		chartPath := filepath.Join(chartDir, chart.Name())
		chartTemplates, errs := renderTemplates(chartPath, mch, images, tpl, isSTSEnabled, olmVersion)
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

// RenderChart renders a single Helm chart from the specified path.
// olmVersion: OLM version detected at runtime ("v0", "v1", or "" for no OLM).
// Passed to chart templates as .Values.global.olmVersion for conditional rendering.
func RenderChart(chartPath string, mch *v1.MultiClusterHub, images map[string]string, templates map[string]string,
	isSTSEnabled bool, olmVersion string) ([]*unstructured.Unstructured, []error) {

	if val, ok := os.LookupEnv("DIRECTORY_OVERRIDE"); ok {
		chartPath = path.Join(val, chartPath)
	} else {
		value, _ := os.LookupEnv("TEMPLATES_PATH")
		chartPath = path.Join(value, chartPath)

	}

	chartTemplates, errs := renderTemplates(chartPath, mch, images, templates, isSTSEnabled, olmVersion)
	if len(errs) > 0 {
		for _, err := range errs {
			log.Info(err.Error())
		}
		return nil, errs
	}

	return chartTemplates, nil

}

func renderTemplates(chartPath string, mch *v1.MultiClusterHub, images map[string]string, tpl map[string]string,
	isSTSEnabled bool, olmVersion string) ([]*unstructured.Unstructured, []error) {

	var templates []*unstructured.Unstructured
	errs := []error{}

	chart, err := loader.Load(chartPath)
	if err != nil {
		log.Info("error loading chart")
		return nil, append(errs, err)
	}

	valuesYaml := &Values{}
	injectValuesOverrides(valuesYaml, mch, images, tpl, isSTSEnabled, olmVersion)
	helmEngine := engine.Engine{
		Strict:   true,
		LintMode: false,
	}

	vals, err := valuesYaml.ToValues()
	if err != nil {
		log.Info(fmt.Sprintf("error rendering chart: %s", chart.Name()))
		return nil, append(errs, err)
	}

	// Merge chart values.yaml defaults (base layer) with Go-injected overrides.
	// CoalesceValues applies the following priority order:
	//   1. Go-injected vals (highest) — runtime values from MCH spec, env vars, annotations
	//   2. chart values.yaml defaults (lowest) — shipped defaults, fills gaps Go doesn't set
	// This makes values.yaml the authoritative source for shipped defaults so chart
	// maintainers can update defaults (e.g. MTV channel) without requiring a Go constant change.
	coalesced, err := chartutil.CoalesceValues(chart, vals.AsMap())
	if err != nil {
		log.Info(fmt.Sprintf("error coalescing chart values: %s", chart.Name()))
		return nil, append(errs, err)
	}

	rawTemplates, err := helmEngine.Render(chart, chartutil.Values{"Values": coalesced.AsMap()})
	if err != nil {
		log.Info(fmt.Sprintf("error rendering chart: %s", chart.Name()))
		return nil, append(errs, err)
	}

	for fileName, templateFile := range rawTemplates {
		// Split multi-doc YAML (documents separated by --- with optional whitespace)
		// Helm may output "---\n" or "\n---\n" or even "---" at start of file
		docs := strings.Split(templateFile, "\n---")
		for i, doc := range docs {
			// First doc may not have leading ---, rest will have it stripped by split
			if i > 0 {
				// Remove leading newline if present after split point
				doc = strings.TrimPrefix(doc, "\n")
			}
			doc = strings.TrimSpace(doc)
			if doc == "" || doc == "---" {
				continue
			}

			unstructured := &unstructured.Unstructured{}
			if err = yaml.Unmarshal([]byte(doc), unstructured); err != nil {
				return nil, append(errs, fmt.Errorf("error converting file %s to unstructured: %v", fileName, err))
			}

			// Skip empty documents (helm comments/whitespace only)
			if unstructured.GetKind() == "" {
				continue
			}

			// Add namespace to namespaced resources
			switch unstructured.GetKind() {
			case "Deployment", "ServiceAccount", "Role", "RoleBinding", "Service", "ConfigMap", "Ingress", "Channel", "Subscription", "NetworkPolicy":
				if unstructured.GetNamespace() == "" {
					unstructured.SetNamespace(mch.Namespace)
				}
			}
			utils.AddInstallerLabel(unstructured, mch.Name, mch.Namespace)
			templates = append(templates, unstructured)
		}
	}

	return templates, errs
}

func injectValuesOverrides(values *Values, mch *v1.MultiClusterHub, images map[string]string,
	templates map[string]string, isSTSEnabled bool, olmVersion string) {

	values.Global.ImageOverrides = images

	values.Global.TemplateOverrides = templates

	values.Global.PullPolicy = string(utils.GetImagePullPolicy(mch))

	values.Global.Namespace = mch.Namespace

	values.Global.PullSecret = mch.Spec.ImagePullSecret

	values.Global.ImageRepository = utils.GetImageRepository(mch)

	values.Global.StorageClassName = os.Getenv(helpers.DefaultStorageClassName)

	values.Global.DeployOnOCP = true

	// Default to v0 if olmVersion not specified to maintain fallback compatibility
	if olmVersion == "" {
		values.Global.OLMVersion = "v0"
	} else {
		values.Global.OLMVersion = olmVersion
	}

	// OADP doesn't yet publish OLM v1-compatible bundles (AllNamespaces install mode
	// not declared, and package missing from some OCP 5.0 EC catalog indexes).
	// Force v0 Subscription for OADP until OADP team ships v1-ready bundles.
	// TODO: set to values.Global.OLMVersion once OADP bundles are v1-ready
	values.Global.OADPOLMVersion = "v0"

	values.HubConfig.ClusterSTSEnabled = isSTSEnabled

	values.HubConfig.ReplicaCount = utils.DefaultReplicaCount(mch)

	values.HubConfig.NodeSelector = mch.Spec.NodeSelector

	values.HubConfig.Tolerations = convertTolerations(utils.GetTolerations(mch))

	values.HubConfig.ProbeConfig = parseProbeConfigFromAnnotations(mch)

	values.HubConfig.OCPVersion = os.Getenv("ACM_HUB_OCP_VERSION")

	values.HubConfig.HubVersion = version.Version

	values.HubConfig.OCPIngress = os.Getenv("INGRESS_DOMAIN")

	values.Global.BaseDomain = os.Getenv("INGRESS_DOMAIN")

	values.Global.APIUrl = os.Getenv("API_URL")

	values.Global.Target = "acm"

	values.HubConfig.SubscriptionPause = utils.GetDisableClusterImageSets(mch)

	values.Org = "open-cluster-management"

	if utils.ProxyEnvVarsAreSet() {
		proxyVar := map[string]string{}
		proxyVar["HTTP_PROXY"] = os.Getenv("HTTP_PROXY")
		proxyVar["HTTPS_PROXY"] = os.Getenv("HTTPS_PROXY")
		proxyVar["NO_PROXY"] = os.Getenv("NO_PROXY")
		values.HubConfig.ProxyConfigs = proxyVar
	}

	values.Global.Name, values.Global.Channel, values.Global.InstallPlanApproval, values.Global.Source, values.Global.SourceNamespace, values.Global.StartingCSV = GetOADPConfig(mch)

	values.Global.MinOADPChannel = defaultOADPChannel
	values.Global.MinOADPStableChannel = defaultOADPStableChannel

	// MTV and CNV are only injected when an annotation override is present.
	// When nil, the fields are omitted from the Go values map (omitempty) and
	// CoalesceValues fills them from the chart's values.yaml defaults instead.
	values.Global.MTVOperator = GetMTVConfig(mch)
	values.Global.KubevirtHyperconvergedOperator = GetCNVConfig(mch)

	// NetworkPolicies configuration - default enabled to true
	networkPoliciesEnabled := true
	if mch.Spec.NetworkPolicies != nil {
		networkPoliciesEnabled = mch.Spec.NetworkPolicies.Enabled
	}
	values.Global.NetworkPolicies = NetworkPoliciesValue{
		Enabled: networkPoliciesEnabled,
	}

	// Apply OADP ClusterExtension overrides (OLM v1) if annotation present and effective OADP OLM version is v1
	if values.Global.OADPOLMVersion == "v1" {
		if overrides := parseOADPClusterExtensionAnnotation(mch); overrides != nil {
			// Override catalog settings for OLM v1
			if len(overrides.Channels) > 0 {
				// Use first channel from override
				values.Global.Channel = overrides.Channels[0]
			}
			if overrides.Version != "" {
				values.Global.StartingCSV = overrides.Version
			}
			if overrides.Source != "" {
				values.Global.Source = overrides.Source
			}
		}
	}

	// TODO: Define all overrides
}

// OADPClusterExtensionOverrides contains mutable fields that can be overridden via annotation for OADP (OLM v1).
type OADPClusterExtensionOverrides struct {
	// Channels is an optional list of channels to constrain upgrades
	Channels []string `json:"channels,omitempty"`
	// Version is an optional semver constraint for version selection
	Version string `json:"version,omitempty"`
	// Source is the catalog source name
	Source string `json:"source,omitempty"`
}

// parseOADPClusterExtensionAnnotation unmarshals the OADP ClusterExtension annotation from a MultiClusterHub.
// Returns nil if no annotation is present or if unmarshaling fails.
// The annotation key is installer.open-cluster-management.io/oadp-clusterextension-spec (OLM v1).
func parseOADPClusterExtensionAnnotation(m *v1.MultiClusterHub) *OADPClusterExtensionOverrides {
	oadpAnnotationOverrides := utils.GetOADPClusterExtensionAnnotationOverrides(m)
	if oadpAnnotationOverrides == "" {
		return nil
	}
	overrides := &OADPClusterExtensionOverrides{}
	err := json.Unmarshal([]byte(oadpAnnotationOverrides), overrides)
	if err != nil {
		log.Info(fmt.Sprintf("Failed to unmarshal OADP ClusterExtension annotation: %s. Error: %v", oadpAnnotationOverrides, err))
		return nil
	}
	return overrides
}

// parseOADPAnnotation unmarshals the OADP subscription annotation from a MultiClusterHub.
// Returns an empty SubscriptionSpec if no annotation is present or if unmarshaling fails.
// The annotation key is installer.open-cluster-management.io/oadp-subscription-spec (OLM v0).
func parseOADPAnnotation(m *v1.MultiClusterHub) *subv1alpha1.SubscriptionSpec {
	sub := &subv1alpha1.SubscriptionSpec{}
	oadpSpec := utils.GetOADPAnnotationOverrides(m)
	if oadpSpec == "" {
		return sub
	}

	if err := json.Unmarshal([]byte(oadpSpec), sub); err != nil {
		log.Info(fmt.Sprintf("Failed to unmarshal OADP annotation: %s.", oadpSpec))
	}
	return sub
}

// getOADPChannel determines the OADP operator channel based on annotation override or OCP version.
// If override is provided (from annotation), it takes precedence.
// Otherwise, returns stable channel for OCP 4.19+ or unknown versions, and stable-1.4 for OCP 4.18 and earlier.

// The ACM_HUB_OCP_VERSION environment variable is used to detect the OpenShift version.
func getOADPChannel(override string) string {
	if override != "" {
		return override
	}

	ocpVersion := os.Getenv("ACM_HUB_OCP_VERSION")
	isOCP419orNewer := ocpVersion == "" || strings.HasPrefix(ocpVersion, "4.19") ||
		!strings.HasPrefix(ocpVersion, "4.1")

	if isOCP419orNewer {
		return defaultOADPStableChannel
	}
	return defaultOADPChannel
}

// valueOrDefault returns value if it is non-empty, otherwise returns defaultValue.
// Helper function to reduce boilerplate for conditional default assignment.
func valueOrDefault(value, defaultValue string) string {
	if value != "" {
		return value
	}
	return defaultValue
}

func GetOADPConfig(m *v1.MultiClusterHub) (string, string, subv1alpha1.Approval, string, string, string) {
	sub := parseOADPAnnotation(m)

	name := valueOrDefault(sub.Package, defaultOADPName)
	channel := getOADPChannel(sub.Channel)
	source := valueOrDefault(sub.CatalogSource, defaultOADPCatalogSource)
	sourceNamespace := valueOrDefault(sub.CatalogSourceNamespace, defaultOADPCatalogSourceNamespace)
	startingCSV := sub.StartingCSV

	installPlan := subv1alpha1.Approval(defaultOADPInstallPlan)
	if sub.InstallPlanApproval != "" {
		installPlan = sub.InstallPlanApproval
	}

	return name, channel, installPlan, source, sourceNamespace, startingCSV
}

/*
parseMTVAnnotation unmarshals the MTV subscription annotation from a MultiClusterHub.
Returns nil if no annotation is present so callers can distinguish "not set" from
"set but empty". Returns an empty SubscriptionSpec (not nil) if unmarshaling fails,
so partial annotations degrade gracefully.
The annotation key is installer.open-cluster-management.io/mtv-subscription-spec.
*/
func parseMTVAnnotation(m *v1.MultiClusterHub) *subv1alpha1.SubscriptionSpec {
	mtvSpec := utils.GetMTVAnnotationOverrides(m)
	if mtvSpec == "" {
		return nil
	}

	sub := &subv1alpha1.SubscriptionSpec{}
	if err := json.Unmarshal([]byte(mtvSpec), sub); err != nil {
		log.Info(fmt.Sprintf("Failed to unmarshal MTV annotation: %s.", mtvSpec))
	}
	return sub
}

/*
parseCNVAnnotation unmarshals the CNV subscription annotation from a MultiClusterHub.
Returns nil if no annotation is present so callers can distinguish "not set" from
"set but empty". Returns an empty SubscriptionSpec (not nil) if unmarshaling fails,
so partial annotations degrade gracefully.
The annotation key is installer.open-cluster-management.io/cnv-subscription-spec.
*/
func parseCNVAnnotation(m *v1.MultiClusterHub) *subv1alpha1.SubscriptionSpec {
	cnvSpec := utils.GetCNVAnnotationOverrides(m)
	if cnvSpec == "" {
		return nil
	}

	sub := &subv1alpha1.SubscriptionSpec{}
	if err := json.Unmarshal([]byte(cnvSpec), sub); err != nil {
		log.Info(fmt.Sprintf("Failed to unmarshal CNV annotation: %s.", cnvSpec))
	}
	return sub
}

/*
operatorPolicyValue maps a parsed subscription annotation onto the chart values
block for an embedded OperatorPolicy. Any field left empty in the annotation falls
back to the shipped Go default (itself a safety net for partial annotations —
the full defaults live in values.yaml and are applied via CoalesceValues when
no annotation is present at all).
*/
func operatorPolicyValue(sub *subv1alpha1.SubscriptionSpec, defaults operatorPolicyDefaults) OperatorPolicyValue {
	return OperatorPolicyValue{
		Subscription: OperatorPolicySubscriptionValue{
			Channel:   valueOrDefault(sub.Channel, defaults.channel),
			Name:      valueOrDefault(sub.Package, defaults.packageName),
			Namespace: defaults.namespace,
			// The catalog fields are passed through untouched rather than
			// defaulted: leaving them empty is what lets the OperatorPolicy
			// controller inherit the catalog/CSV already resolved on the
			// managed cluster.
			Source:          sub.CatalogSource,
			SourceNamespace: sub.CatalogSourceNamespace,
			StartingCSV:     sub.StartingCSV,
		},
		UpgradeApproval: valueOrDefault(string(sub.InstallPlanApproval), defaults.upgradeApproval),
	}
}

// operatorPolicyDefaults holds fallback values used only when an annotation is
// present but leaves individual fields unset (partial override). When no
// annotation is present at all, values.yaml defaults are used instead via
// CoalesceValues — these constants are not consulted in that path.
type operatorPolicyDefaults struct {
	channel         string
	packageName     string
	namespace       string
	upgradeApproval string
}

/*
GetMTVConfig resolves the MTV subscription fields used by the mtv-operator AddOnTemplate.

Returns nil when no mtv-subscription-spec annotation is set — the caller omits the
field entirely so CoalesceValues can apply the values.yaml defaults unchanged.

When an annotation is present (even partially), the annotation fields take priority
and any unset annotation fields fall back to the Go defaults below, which mirror
the values.yaml shipped defaults and act as a safety net for partial overrides.
*/
func GetMTVConfig(m *v1.MultiClusterHub) *OperatorPolicyValue {
	sub := parseMTVAnnotation(m)
	if sub == nil {
		// No annotation — let values.yaml defaults flow through via CoalesceValues.
		return nil
	}
	result := operatorPolicyValue(sub, operatorPolicyDefaults{
		channel:         defaultMTVChannel,
		packageName:     defaultMTVPackageName,
		namespace:       defaultMTVSubscriptionNamespace,
		upgradeApproval: defaultMTVUpgradeApproval,
	})
	return &result
}

/*
GetCNVConfig resolves the CNV subscription fields used by the
kubevirt-hyperconverged AddOnTemplate.

Returns nil when no cnv-subscription-spec annotation is set — the caller omits the
field entirely so CoalesceValues can apply the values.yaml defaults unchanged.

When an annotation is present (even partially), the annotation fields take priority
and any unset annotation fields fall back to the Go defaults below, which mirror
the values.yaml shipped defaults and act as a safety net for partial overrides.
*/
func GetCNVConfig(m *v1.MultiClusterHub) *OperatorPolicyValue {
	sub := parseCNVAnnotation(m)
	if sub == nil {
		// No annotation — let values.yaml defaults flow through via CoalesceValues.
		return nil
	}
	result := operatorPolicyValue(sub, operatorPolicyDefaults{
		channel:         defaultCNVChannel,
		packageName:     defaultCNVPackageName,
		namespace:       defaultCNVSubscriptionNamespace,
		upgradeApproval: defaultCNVUpgradeApproval,
	})
	return &result
}
