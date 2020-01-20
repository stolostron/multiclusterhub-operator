package v1alpha1

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MultiCloudHubSpec defines the desired state of MultiCloudHub
// +k8s:openapi-gen=true
type MultiCloudHubSpec struct {
	// Version of the MultiCloud hub
	Version string `json:"version"`

	// Repository of the MultiCloud hub images
	ImageRepository string `json:"imageRepository"`

	// Pull policy of the MultiCloud hub images
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy"`

	// Pull secret of the MultiCloud hub images
	// +optional
	ImagePullSecret string `json:"imagePullSecret,omitempty"`

	// Spec of NodeSelector
	// +optional
	NodeSelector *NodeSelector `json:"nodeSelector,omitempty"`

	// Spec of foundation
	Foundation `json:"foundation"`

	// Spec of etcd
	Etcd `json:"etcd"`

	// Spec of mongo
	Mongo `json:"mongo"`
}

// NodeSelector defines the desired state of NodeSelector
type NodeSelector struct {
	// Spec of OS
	// +optional
	OS string `json:"os,omitempty"`

	// Spec of CustomLabelSelector
	// +optional
	CustomLabelSelector string `json:"customLabelSelector,omitempty"`

	// Spec of CustomLabelValue
	// +optional
	CustomLabelValue string `json:"customLabelValue,omitempty"`
}

// Foundation defines the desired state of MultiCloudHub foundation components
type Foundation struct {
	// Spec of apiserver
	// +optional
	Apiserver `json:"apiserver,omitempty"`

	// Spec of controller
	// +optional
	Controller `json:"controller,omitempty"`
}

type Apiserver struct {
	// Number of desired pods. This is a pointer to distinguish between explicit
	// zero and not specified. Defaults to 1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Certificates of API server
	// +optional
	ApiserverSecret string `json:"apiserverSecret,omitempty"`

	// Certificates of Klusterlet
	// +optional
	KlusterletSecret string `json:"klusterletSecret,omitempty"`

	// Configuration of the pod
	// +optional
	Configuration map[string]string `json:"configuration,omitempty"`
}

type Controller struct {
	// Number of desired pods. This is a pointer to distinguish between explicit
	// zero and not specified. Defaults to 1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Configuration of the pod
	// +optional
	Configuration map[string]string `json:"configuration,omitempty"`
}

// Etcd defines the desired state of etcd
type Etcd struct {
	// Endpoints of etcd
	Endpoints string `json:"endpoints"`

	// Secret of etcd
	// +optional
	Secret string `json:"secret,omitempty"`
}

// Mongo defines the desired state of mongo
type Mongo struct {
	// Endpoints of mongo
	Endpoints string `json:"endpoints"`

	// Replica set of mongo
	ReplicaSet string `json:"replicaSet"`

	// User secret of mongo
	// +optional
	UserSecret string `json:"userSecret,omitempty"`

	// TLS secret of mongo
	// +optional
	TLSSecret string `json:"tlsSecret,omitempty"`

	// CA secret of mongo
	// +optional
	CASecret string `json:"caSecret,omitempty"`
}

// MultiCloudHubStatus defines the observed state of MultiCloudHub
// +k8s:openapi-gen=true
type MultiCloudHubStatus struct {
	// Represents the running phase of the MultiCloudHub
	Phase string `json:"phase"`

	// Represents the status of each deployment
	// +optional
	Deployments []DeploymentResult `json:"deployments,omitempty"`
}

// DeploymentResult defines the observed state of Deployment
type DeploymentResult struct {
	// Name of the deployment
	Name string `json:"name"`

	// The most recently observed status of the Deployment
	Status appsv1.DeploymentStatus `json:"status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MultiCloudHub is the Schema for the multicloudhubs API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=multicloudhubs,scope=Namespaced
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="Multicloudhub Operator"
type MultiCloudHub struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MultiCloudHubSpec   `json:"spec,omitempty"`
	Status MultiCloudHubStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MultiCloudHubList contains a list of MultiCloudHub
type MultiCloudHubList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MultiCloudHub `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MultiCloudHub{}, &MultiCloudHubList{})
}
