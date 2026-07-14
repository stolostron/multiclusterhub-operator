// Copyright Contributors to the Open Cluster Management project

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TeardownPhase represents the current high-level phase of the teardown process.
type TeardownPhase string

const (
	TeardownPhaseDryRun                  TeardownPhase = "DryRun"
	TeardownPhaseGateOLMSubscription     TeardownPhase = "GateOLMSubscription"
	TeardownPhaseRemoveBlockingCRs       TeardownPhase = "RemoveBlockingCRs"
	TeardownPhaseDisableAddons           TeardownPhase = "DisableAddons"
	TeardownPhaseDeleteInfrastructureCRs TeardownPhase = "DeleteInfrastructureCRs"
	TeardownPhaseDetachManagedClusters   TeardownPhase = "DetachManagedClusters"
	TeardownPhaseDeleteMCH               TeardownPhase = "DeleteMCH"
	TeardownPhaseMonitorMCEChain         TeardownPhase = "MonitorMCEChain"
	TeardownPhaseCleanOrphans            TeardownPhase = "CleanOrphans"
	TeardownPhaseDeleteACMCRDs           TeardownPhase = "DeleteACMCRDs"
	TeardownPhaseRemoveOLMOperator       TeardownPhase = "RemoveOLMOperator"
	TeardownPhaseComplete                TeardownPhase = "Complete"
	TeardownPhaseFailed                  TeardownPhase = "Failed"
)

// TeardownPhaseState tracks whether an individual phase has completed.
type TeardownPhaseState string

const (
	PhaseStatePending    TeardownPhaseState = "Pending"
	PhaseStateInProgress TeardownPhaseState = "InProgress"
	PhaseStateComplete   TeardownPhaseState = "Complete"
	PhaseStateFailed     TeardownPhaseState = "Failed"
	PhaseStateSkipped    TeardownPhaseState = "Skipped"
)

// HubTeardownSpec defines the desired state of HubTeardown.
type HubTeardownSpec struct {
	// DryRun when true (the default) causes the controller to scan and report
	// without mutating any cluster resources. Set to false to begin teardown.
	// Can be toggled back to true at any time to pause and re-scan.
	// +kubebuilder:default=true
	DryRun bool `json:"dryRun"`

	// ApprovedDestructiveActions enables Tier 2 (non-cloud) finalizer
	// patching for stuck resources. Only evaluated when dryRun is false.
	ApprovedDestructiveActions bool `json:"approvedDestructiveActions,omitempty"`

	// AcknowledgeCloudResourceRisk enables Tier 1 (cloud-protecting) finalizer
	// patching. Setting this to true means the admin accepts that cloud
	// infrastructure (VMs, LBs, storage, DNS) may be orphaned and will
	// require manual cleanup in the cloud provider console.
	AcknowledgeCloudResourceRisk bool `json:"acknowledgeCloudResourceRisk,omitempty"`

	// ForceFinalizerTimeout is the duration to wait for a controller to
	// process its own finalizer before the teardown controller intervenes.
	// Defaults to 5m.
	// +optional
	ForceFinalizerTimeout *metav1.Duration `json:"forceFinalizerTimeout,omitempty"`

	// MaxDuration is the maximum wall-clock time allowed for the entire
	// teardown. If exceeded, the controller sets a Stalled condition with
	// guidance on what is blocking. Defaults to 2h.
	// +optional
	MaxDuration *metav1.Duration `json:"maxDuration,omitempty"`
}

// HubTeardownStatus defines the observed state of HubTeardown.
type HubTeardownStatus struct {
	// Phase is the current high-level phase of the teardown process.
	Phase TeardownPhase `json:"phase,omitempty"`

	// DryRunReport contains the full teardown preview generated during
	// dry-run mode. Updated on each reconcile while dryRun is true.
	// +optional
	DryRunReport *DryRunReport `json:"dryRunReport,omitempty"`

	// BlockingResources lists resources that prevent MCH deletion via
	// the validating webhook.
	// +optional
	BlockingResources []BlockingResource `json:"blockingResources,omitempty"`

	// CloudResourceWarnings lists resources with cloud-protecting finalizers
	// and their platform-specific risk descriptions.
	// +optional
	CloudResourceWarnings []CloudResourceWarning `json:"cloudResourceWarnings,omitempty"`

	// ActiveSince is the timestamp when the teardown transitioned from
	// dry-run to active execution. Used to calculate overall stall timeout.
	// +optional
	ActiveSince *metav1.Time `json:"activeSince,omitempty"`

	// Phases tracks the status of each teardown phase.
	// +optional
	Phases []TeardownPhaseStatus `json:"phases,omitempty"`

	// Conditions reflect the current state of the teardown process.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// DryRunReport contains the full teardown preview.
type DryRunReport struct {
	// ScanTime is when the last scan was performed.
	ScanTime metav1.Time `json:"scanTime"`

	// Summary is a human-readable overview of the teardown plan.
	Summary string `json:"summary"`

	// PlannedPhases is an ordered list of phases with per-phase detail.
	PlannedPhases []PlannedPhase `json:"plannedPhases"`

	// TotalBlockingResources is the count of resources blocking uninstall.
	TotalBlockingResources int `json:"totalBlockingResources"`

	// TotalCloudRiskResources is the count of resources with Tier 1 finalizers.
	TotalCloudRiskResources int `json:"totalCloudRiskResources"`
}

// PlannedPhase describes what a single teardown phase will do.
type PlannedPhase struct {
	// Phase name (matches TeardownPhase enum).
	Phase TeardownPhase `json:"phase"`

	// Action is a short description of what this phase will do.
	Action string `json:"action"`

	// ResourcesAffected is the count of resources this phase touches.
	ResourcesAffected int `json:"resourcesAffected"`

	// CloudRiskResources is the count of Tier 1 resources in this phase.
	// +optional
	CloudRiskResources int `json:"cloudRiskResources,omitempty"`

	// Details is a per-resource breakdown of what will happen.
	// +optional
	Details []string `json:"details,omitempty"`
}

// BlockingResource identifies a resource that prevents MCH deletion.
type BlockingResource struct {
	// Group is the API group of the resource.
	Group string `json:"group"`

	// Kind is the resource kind.
	Kind string `json:"kind"`

	// Name is the resource name.
	Name string `json:"name"`

	// Namespace is the resource namespace (empty for cluster-scoped).
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Reason explains why this resource blocks uninstall.
	Reason string `json:"reason"`
}

// ResourceRef identifies a Kubernetes resource by GVK + namespace/name.
type ResourceRef struct {
	// Group is the API group of the resource.
	Group string `json:"group"`

	// Kind is the resource kind.
	Kind string `json:"kind"`

	// Namespace is the resource namespace (empty for cluster-scoped).
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name is the resource name.
	Name string `json:"name"`
}

// CloudResourceWarning describes a resource whose finalizer protects cloud infrastructure.
type CloudResourceWarning struct {
	// Resource identifies the Kubernetes resource.
	Resource ResourceRef `json:"resource"`

	// Finalizer is the specific finalizer string that protects cloud resources.
	Finalizer string `json:"finalizer"`

	// RiskSummary is a human-readable description of what cloud resources
	// will be orphaned if this finalizer is removed.
	RiskSummary string `json:"riskSummary"`

	// Platform is the cloud platform (AWS, Azure, GCP, etc.) if detectable.
	// +optional
	Platform string `json:"platform,omitempty"`

	// Acknowledged indicates whether the admin has reviewed this warning.
	Acknowledged bool `json:"acknowledged"`
}

// TeardownPhaseStatus tracks the execution state of one teardown phase.
type TeardownPhaseStatus struct {
	// Phase name.
	Phase TeardownPhase `json:"phase"`

	// State is the current execution state of this phase.
	State TeardownPhaseState `json:"state"`

	// Message provides human-readable detail about the phase state.
	// +optional
	Message string `json:"message,omitempty"`

	// StartTime is when this phase began executing.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is when this phase finished.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=hubteardowns,scope=Namespaced,shortName=htd
//+kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`,description="Current teardown phase"
//+kubebuilder:printcolumn:name="Dry-Run",type=boolean,JSONPath=`.spec.dryRun`,description="Whether dry-run mode is active"
//+kubebuilder:printcolumn:name="Blocking",type=integer,JSONPath=`.status.dryRunReport.totalBlockingResources`,description="Number of blocking resources"
//+kubebuilder:printcolumn:name="Cloud-Risk",type=integer,JSONPath=`.status.dryRunReport.totalCloudRiskResources`,description="Number of cloud-risk resources"
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// HubTeardown is the Schema for the hubteardowns API. It provides orchestrated,
// observable, and safe RHACM uninstallation with dependency graph scanning,
// ordered phase execution, and explicit cloud resource leak warnings.
type HubTeardown struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HubTeardownSpec   `json:"spec,omitempty"`
	Status HubTeardownStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HubTeardownList contains a list of HubTeardown.
type HubTeardownList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HubTeardown `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HubTeardown{}, &HubTeardownList{})
}
