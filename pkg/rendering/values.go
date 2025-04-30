// Copyright Contributors to the Open Cluster Management project
package renderer

import (
	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	v1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
)

type Values struct {
	Global    Global    `json:"global" structs:"global"`
	HubConfig HubConfig `json:"hubconfig" structs:"hubconfig"`
	Org       string    `json:"org" structs:"org"`
}

type Global struct {
	ImageOverrides       map[string]string    `json:"imageOverrides" structs:"imageOverrides"`
	TemplateOverrides    map[string]string    `json:"templateOverrides" structs:"templateOverrides"`
	PullPolicy           string               `json:"pullPolicy" structs:"pullPolicy"`
	PullSecret           string               `json:"pullSecret" structs:"pullSecret"`
	Namespace            string               `json:"namespace" structs:"namespace"`
	ImageRepository      string               `json:"imageRepository" structs:"namespace"`
	Name                 string               `json:"name" structs:"name"`
	Channel              string               `json:"channel" structs:"channel"`
	MinOADPChannel       string               `json:"minOADPChannel" structs:"minOADPChannel"`
	MinOADPStableChannel string               `json:"MinOADPStableChannel" structs:"MinOADPStableChannel"`
	InstallPlanApproval  subv1alpha1.Approval `json:"installPlanApproval" structs:"installPlanApproval"`
	Source               string               `json:"source" structs:"source"`
	SourceNamespace      string               `json:"sourceNamespace" structs:"sourceNamespace"`
	HubSize              v1.HubSize           `json:"hubSize" structs:"hubSize" yaml:"hubSize"`
	APIUrl               string               `json:"apiUrl" structs:"apiUrl"`
	Target               string               `json:"target" structs:"target"`
	BaseDomain           string               `json:"baseDomain" structs:"baseDomain"`
	DeployOnOCP          bool                 `json:"deployOnOCP" structs:"deployOnOCP"`
	StorageClassName     string               `json:"storageClassName" structs:"storageClassName"`
	StartingCSV          string               `json:"startingCSV" structs:"startingCSV"`
}

type HubConfig struct {
	ClusterSTSEnabled bool              `json:"clusterSTSEnabled" structs:"clusterSTSEnabled"`
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
