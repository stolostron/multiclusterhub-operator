// Copyright Contributors to the Open Cluster Management project
package renderer

import (
	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

type Values struct {
	Global    Global    `json:"global" structs:"global"`
	HubConfig HubConfig `json:"hubconfig" structs:"hubconfig"`
	Org       string    `json:"org" structs:"org"`
}

type Global struct {
	ImageOverrides                 map[string]string    `json:"imageOverrides" structs:"imageOverrides"`
	TemplateOverrides              map[string]string    `json:"templateOverrides" structs:"templateOverrides"`
	PullPolicy                     string               `json:"pullPolicy" structs:"pullPolicy"`
	PullSecret                     string               `json:"pullSecret" structs:"pullSecret"`
	Namespace                      string               `json:"namespace" structs:"namespace"`
	ImageRepository                string               `json:"imageRepository" structs:"namespace"`
	Name                           string               `json:"name" structs:"name"`
	Channel                        string               `json:"channel" structs:"channel"`
	MinOADPChannel                 string               `json:"minOADPChannel" structs:"minOADPChannel"`
	MinOADPStableChannel           string               `json:"MinOADPStableChannel" structs:"MinOADPStableChannel"`
	InstallPlanApproval            subv1alpha1.Approval `json:"installPlanApproval" structs:"installPlanApproval"`
	Source                         string               `json:"source" structs:"source"`
	SourceNamespace                string               `json:"sourceNamespace" structs:"sourceNamespace"`
	APIUrl                         string               `json:"apiUrl" structs:"apiUrl"`
	Target                         string               `json:"target" structs:"target"`
	BaseDomain                     string               `json:"baseDomain" structs:"baseDomain"`
	DeployOnOCP                    bool                 `json:"deployOnOCP" structs:"deployOnOCP"`
	StorageClassName               string               `json:"storageClassName" structs:"storageClassName"`
	StartingCSV                    string               `json:"startingCSV" structs:"startingCSV"`
	OLMVersion                     string               `json:"olmVersion" structs:"olmVersion"`         // "v0" or "v1" - detected at runtime by main.go detectOLMVersion
	OADPOLMVersion                 string               `json:"oadpOlmVersion" structs:"oadpOlmVersion"` // forced to v0 until OADP ships OLM v1-ready bundles
	MTVOperator                    OperatorPolicyValue  `json:"mtvOperator" structs:"mtvOperator"`
	KubevirtHyperconvergedOperator OperatorPolicyValue  `json:"kubevirtHyperconvergedOperator" structs:"kubevirtHyperconvergedOperator"`
	NetworkPolicies                NetworkPoliciesValue `json:"networkPolicies" structs:"networkPolicies"`
}

// OperatorPolicyValue mirrors a templated OperatorPolicy block of the
// mtv-integrations chart values.yaml. The outer key is derived from the
// OperatorPolicy name by the chart generator, so the Go field name has to stay
// in step with what generate-charts.py produces.
type OperatorPolicyValue struct {
	Subscription    OperatorPolicySubscriptionValue `json:"subscription" structs:"subscription"`
	UpgradeApproval string                          `json:"upgradeApproval" structs:"upgradeApproval"`
}

// OperatorPolicySubscriptionValue mirrors an OperatorPolicy subscription block.
//
// The catalog fields are deliberately left at their zero value when the
// annotation does not set them. Leaving them empty is what lets the policy
// controller inherit the catalog and CSV already resolved on the managed
// cluster, so an empty string is a valid unset marker rather than a missing
// value.
type OperatorPolicySubscriptionValue struct {
	Channel         string `json:"channel" structs:"channel"`
	Name            string `json:"name" structs:"name"`
	Namespace       string `json:"namespace" structs:"namespace"`
	Source          string `json:"source" structs:"source"`
	SourceNamespace string `json:"sourceNamespace" structs:"sourceNamespace"`
	StartingCSV     string `json:"startingCSV" structs:"startingCSV"`
}

type NetworkPoliciesValue struct {
	Enabled bool `json:"enabled" structs:"enabled"`
}

type HubConfig struct {
	ClusterSTSEnabled bool              `json:"clusterSTSEnabled" structs:"clusterSTSEnabled"`
	NodeSelector      map[string]string `json:"nodeSelector" structs:"nodeSelector"`
	ProxyConfigs      map[string]string `json:"proxyConfigs" structs:"proxyConfigs"`
	ReplicaCount      int               `json:"replicaCount" structs:"replicaCount"`
	Tolerations       []Toleration      `json:"tolerations" structs:"tolerations"`
	ProbeConfig       *ProbeConfig      `json:"probeConfig" structs:"probeConfig"`
	OCPVersion        string            `json:"ocpVersion" structs:"ocpVersion"`
	HubVersion        string            `json:"hubVersion" structs:"hubVersion"`
	OCPIngress        string            `json:"ocpIngress" structs:"ocpIngress"`
	SubscriptionPause string            `json:"subscriptionPause" structs:"subscriptionPause"`
}

type ProbeConfig struct {
	TimeoutSeconds   *int32 `json:"timeoutSeconds,omitempty"`
	FailureThreshold *int32 `json:"failureThreshold,omitempty"`
	SuccessThreshold *int32 `json:"successThreshold,omitempty"`
}

type Toleration struct {
	Key               string                    `json:"Key" protobuf:"bytes,1,opt,name=key"`
	Operator          corev1.TolerationOperator `json:"Operator" protobuf:"bytes,2,opt,name=operator,casttype=TolerationOperator"`
	Value             string                    `json:"Value" protobuf:"bytes,3,opt,name=value"`
	Effect            corev1.TaintEffect        `json:"Effect" protobuf:"bytes,4,opt,name=effect,casttype=TaintEffect"`
	TolerationSeconds *int64                    `json:"TolerationSeconds" protobuf:"varint,5,opt,name=tolerationSeconds"`
}
