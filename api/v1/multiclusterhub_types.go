// Copyright Contributors to the Open Cluster Management project

/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AvailabilityType ...
type AvailabilityType string

// DeploymentMode ...
type DeploymentMode string

const (
	// HABasic stands up most app subscriptions with a replicaCount of 1
	HABasic AvailabilityType = "Basic"
	// HAHigh stands up most app subscriptions with a replicaCount of 2
	HAHigh AvailabilityType = "High"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MultiClusterHubSpec defines the desired state of MultiClusterHub
type MultiClusterHubSpec struct {

	// Override pull secret for accessing MultiClusterHub operand and endpoint images
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Image Pull Secret",xDescriptors={"urn:alm:descriptor:io.kubernetes:Secret","urn:alm:descriptor:com.tectonic.ui:advanced"}
	ImagePullSecret string `json:"imagePullSecret,omitempty"`

	// Specifies deployment replication for improved availability. Options are: Basic and High (default)
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Availability Configuration",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced","urn:alm:descriptor:com.tectonic.ui:select:High","urn:alm:descriptor:com.tectonic.ui:select:Basic"}
	AvailabilityConfig AvailabilityType `json:"availabilityConfig,omitempty"`

	// Set the nodeselectors
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations causes all components to tolerate any taints.
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Developer Overrides
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Developer Overrides",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	Overrides *Overrides `json:"overrides,omitempty"`

	// Disable automatic import of the hub cluster as a managed cluster
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Disable Hub Self Management",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced","urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	DisableHubSelfManagement bool `json:"disableHubSelfManagement,omitempty"`

	// Disable automatic update of ClusterImageSets
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Disable Update ClusterImageSets",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:advanced","urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	DisableUpdateClusterImageSets bool `json:"disableUpdateClusterImageSets,omitempty"`

	// The name of the local-cluster resource
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Local Cluster Name",xDescriptors={"urn:alm:descriptor:io.kubernetes:text","urn:alm:descriptor:com.tectonic.ui:advanced"}
	//+kubebuilder:default="local-cluster"
	LocalClusterName string `json:"localClusterName,omitempty"`
}

// Overrides provides developer overrides for MCH installation
type Overrides struct {
	// Pull policy of the MultiCluster hub images
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// Provides optional configuration for components, the list of which can be found here: https://github.com/stolostron/multiclusterhub-operator/tree/main/docs/available-components.md
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Component Configuration",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	// +optional
	// +listType=map
	// +listMapKey=name
	Components []ComponentConfig `json:"components,omitempty"`
}

// ComponentConfig provides optional configuration items for individual components
type ComponentConfig struct {
	// Enabled specifies whether the component is enabled or disabled.
	Enabled bool `json:"enabled"`

	// Name denotes the name of the component being configured.
	Name string `json:"name"`

	// ConfigOverrides contains optional configuration overrides for deployments and containers.
	ConfigOverrides ConfigOverride `json:"configOverrides,omitempty"`
}

// ConfigOverride holds overrides for configurations specific to deployments and containers.
type ConfigOverride struct {
	// Deployments is a list of deployment specific configuration overrides.
	Deployments []DeploymentConfig `json:"deployments,omitempty"`
}

// DeploymentConfig provides configuration details for a specific deployment.
type DeploymentConfig struct {
	// Name specifies the name of the deployment being configured.
	Name string `json:"name"`

	// Containers is a list of container specific configurations within the deployment.
	Containers []ContainerConfig `json:"containers"`
}

// ContainerConfig holds configuration details for a specific container within a deployment.
type ContainerConfig struct {
	// Name specifies the name of the container being configured.
	Name string `json:"name"`

	// Env is a list of environment variable overrides for the container.
	Env []EnvConfig `json:"env"`
}

// EnvConfig represents an override for an environment variable within a container.
type EnvConfig struct {
	// Name specifies the name of the environment variable.
	Name string `json:"name,omitempty"`

	// Value specifies the value of the environment variable.
	Value string `json:"value,omitempty"`
}

type HubPhaseType string

const (
	HubPending         HubPhaseType = "Pending"
	HubPaused          HubPhaseType = "Paused"
	HubRunning         HubPhaseType = "Running"
	HubInstalling      HubPhaseType = "Installing"
	HubUpdating        HubPhaseType = "Updating"
	HubUninstalling    HubPhaseType = "Uninstalling"
	HubUpdatingBlocked HubPhaseType = "UpdatingBlocked"
	HubError           HubPhaseType = "Error"
)

// MCEVersionComplianceStatus tracks MultiClusterEngine version compliance against required channel
type MCEVersionComplianceStatus struct {
	// RequiredChannel is the channel version that MCE should meet or exceed
	RequiredChannel string `json:"requiredChannel,omitempty"`

	// CurrentVersion is the actual version of the MCE that is currently installed
	CurrentVersion string `json:"currentVersion,omitempty"`

	// IsCompliant indicates whether the current MCE version meets or exceeds the required channel version
	IsCompliant bool `json:"isCompliant,omitempty"`

	// Message provides additional details about the compliance status
	Message string `json:"message,omitempty"`
}

// MultiClusterHubStatus defines the observed state of MultiClusterHub
type MultiClusterHubStatus struct {

	// Represents the running phase of the MultiClusterHub
	// +optional
	Phase HubPhaseType `json:"phase"`

	// CurrentVersion indicates the current version
	CurrentVersion string `json:"currentVersion,omitempty"`

	// DesiredVersion indicates the desired version
	DesiredVersion string `json:"desiredVersion,omitempty"`

	// Conditions contains the different condition statuses for the MultiClusterHub
	HubConditions []HubCondition `json:"conditions,omitempty"`

	// Components []ComponentCondition `json:"manifests,omitempty"`
	Components map[string]StatusCondition `json:"components,omitempty"`

	// MCEVersionCompliance tracks whether the MCE version meets the required channel version
	MCEVersionCompliance *MCEVersionComplianceStatus `json:"mceVersionCompliance,omitempty"`
}

// StatusCondition contains condition information.
type StatusCondition struct {
	// The component name
	Name string `json:"name,omitempty"`

	// The resource kind this condition represents
	Kind string `json:"kind,omitempty"`

	// Available indicates whether this component is considered properly running
	Available bool `json:"-"`

	// Type is the type of the cluster condition.
	// +required
	Type string `json:"type,omitempty"`

	// Status is the status of the condition. One of True, False, Unknown.
	// +required
	Status metav1.ConditionStatus `json:"status,omitempty"`

	// The last time this condition was updated.
	LastUpdateTime metav1.Time `json:"-"`

	// LastTransitionTime is the last time the condition changed from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`

	// Reason is a (brief) reason for the condition's last status change.
	// +required
	Reason string `json:"reason,omitempty"`

	// Message is a human-readable message indicating details about the last status change.
	// +required
	Message string `json:"message,omitempty"`
}

type HubConditionType string

const (
	// Progressing means the deployment is progressing.
	Progressing HubConditionType = "Progressing"

	// Complete means that all desired components are configured and in a running state.
	Complete HubConditionType = "Complete"

	// Terminating means that the multiclusterhub has been deleted and is cleaning up.
	Terminating HubConditionType = "Terminating"

	// Bocked means there is something preventing an update from occurring
	Blocked HubConditionType = "Blocked"

	// ComponentFailure means a deployment failed during an Apply
	ComponentFailure HubConditionType = "ComponentFailure"
)

// StatusCondition contains condition information.
type HubCondition struct {
	// Type is the type of the cluster condition.
	// +required
	Type HubConditionType `json:"type,omitempty"`

	// Status is the status of the condition. One of True, False, Unknown.
	// +required
	Status metav1.ConditionStatus `json:"status,omitempty"`

	// The last time this condition was updated.
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`

	// LastTransitionTime is the last time the condition changed from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`

	// Reason is a (brief) reason for the condition's last status change.
	// +required
	Reason string `json:"reason,omitempty"`

	// Message is a human-readable message indicating details about the last status change.
	// +required
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=multiclusterhubs,scope=Namespaced,shortName=mch

// MulticlusterHub defines the configuration
// for an instance of a multicluster hub, a central point for managing multiple
// Kubernetes-based clusters. The deployment of multicluster hub components
// is determined based on the configuration that is defined in this resource.
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase",description="The overall status of the MultiClusterHub"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="CurrentVersion",type="string",JSONPath=".status.currentVersion",description="The current version of the MultiClusterHub"
// +kubebuilder:printcolumn:name="DesiredVersion",type="string",JSONPath=".status.desiredVersion",description="The desired version of the MultiClusterHub"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[-1:].message",description="Message from the most recent condition"
// +operator-sdk:csv:customresourcedefinitions:displayName="MultiClusterHub"
type MultiClusterHub struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MultiClusterHubSpec   `json:"spec,omitempty"`
	Status MultiClusterHubStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MultiClusterHubList contains a list of MultiClusterHub
type MultiClusterHubList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MultiClusterHub `json:"items"`
}

// +kubebuilder:object:root=true
// +operator-sdk:csv:customresourcedefinitions:displayName="InternalHubComponent"
type InternalHubComponent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              InternalHubComponentSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true
type InternalHubComponentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InternalHubComponent `json:"items"`
}

type InternalHubComponentSpec struct{}

func init() {
	SchemeBuilder.Register(&MultiClusterHub{}, &MultiClusterHubList{})
	SchemeBuilder.Register(&InternalHubComponent{}, &InternalHubComponentList{})
}
