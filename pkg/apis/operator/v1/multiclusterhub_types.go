// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AvailabilityType ...
type AvailabilityType string

const (
	// HABasic stands up most app subscriptions with a replicaCount of 1
	HABasic AvailabilityType = "Basic"
	// HAHigh stands up most app subscriptions with a replicaCount of 2
	HAHigh AvailabilityType = "High"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MultiClusterHubSpec defines the desired state of MultiClusterHub
// +k8s:openapi-gen=true
type MultiClusterHubSpec struct {

	// Override pull secret for accessing MultiClusterHub operand and endpoint images
	// +optional
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Image Pull Secret"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:io.kubernetes:Secret,urn:alm:descriptor:com.tectonic.ui:advanced"
	ImagePullSecret string `json:"imagePullSecret,omitempty"`

	// Specifies deployment replication for improved availability. Options are: Basic and High (default)
	// +optional
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Availability Configuration"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:advanced,urn:alm:descriptor:com.tectonic.ui:select:High,urn:alm:descriptor:com.tectonic.ui:select:Basic"
	AvailabilityConfig AvailabilityType `json:"availabilityConfig,omitempty"`

	// Install cert-manager into its own namespace
	// +optional
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Separate Certificate Management"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:advanced,urn:alm:descriptor:com.tectonic.ui:booleanSwitch"
	SeparateCertificateManagement bool `json:"separateCertificateManagement"`

	// Set the nodeselectors
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// (Deprecated) Overrides for the default HiveConfig spec
	// +optional
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Hive Config"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:advanced"
	Hive *HiveConfigSpec `json:"hive,omitempty"`

	// Configuration options for ingress management
	// +optional
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Ingress Management"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:advanced"
	Ingress IngressSpec `json:"ingress,omitempty"`

	// Developer Overrides
	// +optional
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Developer Overrides"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:hidden"
	Overrides *Overrides `json:"overrides,omitempty"`

	// Provide the customized OpenShift default ingress CA certificate to RHACM
	// +optional
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Custom CA Configmap"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:advanced,urn:alm:descriptor:io.kubernetes:ConfigMap"
	CustomCAConfigmap string `json:"customCAConfigmap,omitempty"`

	// Disable automatic import of the hub cluster as a managed cluster
	// +optional
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Disable Hub Self Management"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:advanced,urn:alm:descriptor:io.kubernetes:booleanSwitch"
	DisableHubSelfManagement bool `json:"disableHubSelfManagement,omitempty"`

	// Disable automatic update of ClusterImageSets
	// +optional
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Disable Update ClusterImageSets"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:advanced,urn:alm:descriptor:io.kubernetes:booleanSwitch"
	DisableUpdateClusterImageSets bool `json:"disableUpdateClusterImageSets,omitempty"`
}

// Overrides provides developer overrides for MCH installation
type Overrides struct {
	// Pull policy of the MultiCluster hub images
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
}

type HiveConfigSpec struct {

	// (Deprecated) ExternalDNS specifies configuration for external-dns if it is to be deployed by
	// Hive. If absent, external-dns will not be deployed.
	// +optional
	ExternalDNS *ExternalDNSConfig `json:"externalDNS,omitempty"`

	// (Deprecated) AdditionalCertificateAuthorities is a list of references to secrets in the
	// 'hive' namespace that contain an additional Certificate Authority to use when communicating
	// with target clusters. These certificate authorities will be used in addition to any self-signed
	// CA generated by each cluster on installation.
	// +optional
	AdditionalCertificateAuthorities []corev1.LocalObjectReference `json:"additionalCertificateAuthorities,omitempty"`

	// (Deprecated) GlobalPullSecret is used to specify a pull secret that will be used globally by all of the cluster deployments.
	// For each cluster deployment, the contents of GlobalPullSecret will be merged with the specific pull secret for
	// a cluster deployment(if specified), with precedence given to the contents of the pull secret for the cluster deployment.
	// +optional
	GlobalPullSecret *corev1.LocalObjectReference `json:"globalPullSecret,omitempty"`

	// (Deprecated) Backup specifies configuration for backup integration.
	// If absent, backup integration will be disabled.
	// +optional
	Backup BackupConfig `json:"backup,omitempty"`

	// (Deprecated) FailedProvisionConfig is used to configure settings related to handling provision failures.
	FailedProvisionConfig FailedProvisionConfig `json:"failedProvisionConfig"`

	// (Deprecated) MaintenanceMode can be set to true to disable the hive controllers in situations where we need to ensure
	// nothing is running that will add or act upon finalizers on Hive types. This should rarely be needed.
	// Sets replicas to 0 for the hive-controllers deployment to accomplish this.
	MaintenanceMode *bool `json:"maintenanceMode,omitempty"`
}

// HiveConfigStatus defines the observed state of Hive
type HiveConfigStatus struct {
	// (Deprecated) AggregatorClientCAHash keeps an md5 hash of the aggregator client CA
	// configmap data from the openshift-config-managed namespace. When the configmap changes,
	// admission is redeployed.
	AggregatorClientCAHash string `json:"aggregatorClientCAHash,omitempty"`
}

// BackupConfig contains settings for the Velero backup integration.
type BackupConfig struct {
	// (Deprecated) Velero specifies configuration for the Velero backup integration.
	// +optional
	Velero VeleroBackupConfig `json:"velero,omitempty"`

	// (Deprecated) MinBackupPeriodSeconds specifies that a minimum of MinBackupPeriodSeconds will occur in between each backup.
	// This is used to rate limit backups. This potentially batches together multiple changes into 1 backup.
	// No backups will be lost as changes that happen during this interval are queued up and will result in a
	// backup happening once the interval has been completed.
	// +optional
	MinBackupPeriodSeconds *int `json:"minBackupPeriodSeconds,omitempty"`
}

// VeleroBackupConfig contains settings for the Velero backup integration.
type VeleroBackupConfig struct {
	// (Deprecated) Enabled dictates if Velero backup integration is enabled.
	// If not specified, the default is disabled.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
}

// FailedProvisionConfig contains settings to control behavior undertaken by Hive when an installation attempt fails.
type FailedProvisionConfig struct {

	// (Deprecated) SkipGatherLogs disables functionality that attempts to gather full logs from the cluster if an installation
	// fails for any reason. The logs will be stored in a persistent volume for up to 7 days.
	SkipGatherLogs bool `json:"skipGatherLogs,omitempty"`
}

// ExternalDNSConfig contains settings for running external-dns in a Hive
// environment.
type ExternalDNSConfig struct {

	// (Deprecated) AWS contains AWS-specific settings for external DNS
	// +optional
	AWS *ExternalDNSAWSConfig `json:"aws,omitempty"`

	// (Deprecated) GCP contains GCP-specific settings for external DNS
	// +optional
	GCP *ExternalDNSGCPConfig `json:"gcp,omitempty"`

	// As other cloud providers are supported, additional fields will be
	// added for each of those cloud providers. Only a single cloud provider
	// may be configured at a time.
}

// ExternalDNSAWSConfig contains AWS-specific settings for external DNS
type ExternalDNSAWSConfig struct {
	// (Deprecated) Credentials references a secret that will be used to authenticate with
	// AWS Route53. It will need permission to manage entries in each of the
	// managed domains for this cluster.
	// Secret should have AWS keys named 'aws_access_key_id' and 'aws_secret_access_key'.
	// +optional
	Credentials corev1.LocalObjectReference `json:"credentials,omitempty"`
}

// ExternalDNSGCPConfig contains GCP-specific settings for external DNS
type ExternalDNSGCPConfig struct {
	// (Deprecated) Credentials references a secret that will be used to authenticate with
	// GCP DNS. It will need permission to manage entries in each of the
	// managed domains for this cluster.
	// Secret should have a key names 'osServiceAccount.json'.
	// The credentials must specify the project to use.
	// +optional
	Credentials corev1.LocalObjectReference `json:"credentials,omitempty"`
}

// IngressSpec specifies configuration options for ingress management
type IngressSpec struct {
	// List of SSL ciphers enabled for management ingress. Defaults to full list of supported ciphers
	// +optional
	SSLCiphers []string `json:"sslCiphers,omitempty"`
}

type HubPhaseType string

const (
	HubPending      HubPhaseType = "Pending"
	HubRunning      HubPhaseType = "Running"
	HubInstalling   HubPhaseType = "Installing"
	HubUpdating     HubPhaseType = "Updating"
	HubUninstalling HubPhaseType = "Uninstalling"
)

// MultiClusterHubStatus defines the observed state of MultiClusterHub
// +k8s:openapi-gen=true
type MultiClusterHubStatus struct {
	// Represents the running phase of the MultiClusterHub
	// +optional
	Phase HubPhaseType `json:"phase"`

	// CurrentVersion indicates the current version
	// +optional
	CurrentVersion string `json:"currentVersion,omitempty"`

	// DesiredVersion indicates the desired version
	// +optional
	DesiredVersion string `json:"desiredVersion,omitempty"`

	// Conditions contains the different condition statuses for the MultiClusterHub
	// +optional
	HubConditions []HubCondition `json:"conditions,omitempty"`

	// Components []ComponentCondition `json:"manifests,omitempty"`
	// +optional
	Components map[string]StatusCondition `json:"components,omitempty"`
}

// StatusCondition contains condition information.
type StatusCondition struct {
	// The resource kind this condition represents
	Kind string `json:"-"`

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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MultiClusterHub defines the configuration for an instance of the MultiCluster Hub
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=multiclusterhubs,scope=Namespaced,shortName=mch
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="MultiClusterHub"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase",description="The overall status of the multiclusterhub"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type MultiClusterHub struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MultiClusterHubSpec   `json:"spec,omitempty"`
	Status MultiClusterHubStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MultiClusterHubList contains a list of MultiClusterHub
type MultiClusterHubList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MultiClusterHub `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MultiClusterHub{}, &MultiClusterHubList{})
}
