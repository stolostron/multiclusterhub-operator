// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	teardownResyncDryRun = 60 * time.Second
	teardownResyncActive = 15 * time.Second

	conditionTypeDryRunComplete      = "DryRunComplete"
	conditionTypeCloudResourcesRisk  = "CloudResourcesAtRisk"
	conditionTypeTeardownProgressing = "Progressing"
	conditionTypeTeardownComplete    = "Complete"
	conditionTypeTeardownStalled     = "Stalled"
	conditionTypeDryRunReportStale   = "DryRunReportStale"
	conditionTypeWaitingForApproval  = "WaitingForApproval"

	defaultMaxDuration = 2 * time.Hour

	annotationWarningsEmitted = "operator.open-cluster-management.io/warnings-emitted-generation"
)

// HubTeardownReconciler reconciles a HubTeardown object.
type HubTeardownReconciler struct {
	Client          client.Client
	UncachedClient  client.Client // reserved for strong-consistency reads (bypasses informer cache)
	Scheme          *runtime.Scheme
	Log             logr.Logger
	Recorder        record.EventRecorder
	DiscoveryClient discovery.DiscoveryInterface
}

//+kubebuilder:rbac:groups=operator.open-cluster-management.io,resources=hubteardowns,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.open-cluster-management.io,resources=hubteardowns/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.open-cluster-management.io,resources=hubteardowns/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch;delete
//+kubebuilder:rbac:groups=operators.coreos.com,resources=subscriptions;subscriptions/finalizers;clusterserviceversions,verbs=get;list;watch;update;patch;delete
//+kubebuilder:rbac:groups=hypershift.openshift.io,resources=hostedclusters;nodepools,verbs=get;list;watch;delete;patch
//+kubebuilder:rbac:groups=hive.openshift.io,resources=clusterdeployments;clusterpools,verbs=get;list;watch;delete;patch
//+kubebuilder:rbac:groups=agent-install.openshift.io,resources=infraenvs,verbs=get;list;watch;delete;patch
//+kubebuilder:rbac:groups=apps,resources=deployments;deployments/scale,verbs=get;list;patch
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations;mutatingwebhookconfigurations,verbs=get;list;delete
//+kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list
//+kubebuilder:rbac:groups="",resources=pods,verbs=list
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings,verbs=list;delete
//+kubebuilder:rbac:groups=addon.open-cluster-management.io,resources=clustermanagementaddons,verbs=get;list;watch;patch
//+kubebuilder:rbac:groups=cluster.open-cluster-management.io,resources=managedclustersets,verbs=get;list;watch;patch

func (r *HubTeardownReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx).WithName("hubteardown")

	teardown := &operatorv1.HubTeardown{}
	if err := r.Client.Get(ctx, req.NamespacedName, teardown); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Singleton guard: only one HubTeardown CR is allowed per namespace.
	// If multiple exist, refuse to reconcile any except the oldest.
	allTeardowns := &operatorv1.HubTeardownList{}
	if err := r.Client.List(ctx, allTeardowns, client.InNamespace(req.Namespace)); err != nil {
		return ctrl.Result{}, err
	}
	if len(allTeardowns.Items) > 1 {
		var oldest *operatorv1.HubTeardown
		for i := range allTeardowns.Items {
			td := &allTeardowns.Items[i]
			if td.GetDeletionTimestamp() != nil {
				continue
			}
			if oldest == nil || td.CreationTimestamp.Before(&oldest.CreationTimestamp) {
				oldest = td
			}
		}
		if oldest != nil && teardown.Name != oldest.Name {
			log.Info("Rejecting duplicate HubTeardown — only the oldest CR is active",
				"active", oldest.Name, "rejected", teardown.Name)
			r.setCondition(teardown, conditionTypeTeardownProgressing, metav1.ConditionFalse, "DuplicateRejected",
				fmt.Sprintf("Another HubTeardown %q already exists and is active. Delete this CR or the other one.", oldest.Name))
			if statusErr := r.Client.Status().Update(ctx, teardown); statusErr != nil {
				log.Error(statusErr, "Failed to update rejected teardown status")
			}
			return ctrl.Result{}, nil
		}
	}

	// Handle CR deletion: remove the finalizer so the CR can be garbage-collected.
	if teardown.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(teardown, teardownJobFinalizer) {
			if err := r.cleanupTeardownJob(ctx, log, teardown); err != nil {
				log.Error(err, "Failed to cleanup teardown Job during CR deletion")
			}
			controllerutil.RemoveFinalizer(teardown, teardownJobFinalizer)
			if err := r.Client.Update(ctx, teardown); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Ensure the finalizer is present to protect the CR while teardown is active.
	if !controllerutil.ContainsFinalizer(teardown, teardownJobFinalizer) {
		controllerutil.AddFinalizer(teardown, teardownJobFinalizer)
		if err := r.Client.Update(ctx, teardown); err != nil {
			return ctrl.Result{}, err
		}
	}

	if teardown.Spec.DryRun {
		return r.reconcileDryRun(ctx, log, teardown)
	}

	return r.reconcileActive(ctx, log, teardown)
}

// reconcileDryRun performs a read-only scan and populates the dry-run report.
func (r *HubTeardownReconciler) reconcileDryRun(ctx context.Context, log logr.Logger, td *operatorv1.HubTeardown) (ctrl.Result, error) {
	log.Info("Dry-run mode: scanning cluster state")

	graph, err := r.buildDependencyGraph(ctx, log)
	if err != nil {
		r.setCondition(td, conditionTypeDryRunComplete, metav1.ConditionFalse, "ScanFailed",
			fmt.Sprintf("Dependency graph scan failed: %v", err))
		if statusErr := r.Client.Status().Update(ctx, td); statusErr != nil {
			log.Error(statusErr, "Failed to update status after scan error")
		}
		return ctrl.Result{RequeueAfter: teardownResyncDryRun}, nil
	}

	report := r.buildDryRunReport(graph)
	td.Status.Phase = operatorv1.TeardownPhaseDryRun
	td.Status.DryRunReport = report
	td.Status.BlockingResources = graph.BlockingResources
	td.Status.CloudResourceWarnings = r.buildCloudWarnings(ctx, log, graph)

	r.setCondition(td, conditionTypeDryRunComplete, metav1.ConditionTrue, "ScanComplete",
		fmt.Sprintf("Dry-run analysis complete. Found %d blocking resources, %d cloud-risk resources. Set spec.dryRun=false to begin teardown.",
			report.TotalBlockingResources, report.TotalCloudRiskResources))

	if len(td.Status.CloudResourceWarnings) > 0 {
		r.setCondition(td, conditionTypeCloudResourcesRisk, metav1.ConditionTrue, "UnacknowledgedWarnings",
			fmt.Sprintf("%d resources have cloud-protecting finalizers. Review status.cloudResourceWarnings and set spec.acknowledgeCloudResourceRisk=true after verifying cloud state.",
				len(td.Status.CloudResourceWarnings)))
	} else {
		r.setCondition(td, conditionTypeCloudResourcesRisk, metav1.ConditionFalse, "NoCloudRisk",
			"No resources with cloud-protecting finalizers found.")
	}

	if err := r.Client.Status().Update(ctx, td); err != nil {
		log.Error(err, "Failed to update dry-run status")
		return ctrl.Result{}, err
	}

	// Emit cloud warning events only on the first scan (when the condition is freshly set).
	// Subsequent resyncs skip event emission to avoid flooding the event stream;
	// the warnings remain in status.cloudResourceWarnings for inspection.
	if !r.hasEmittedWarnings(td) {
		r.emitCloudWarningEvents(td)
		r.markWarningsEmitted(ctx, td)
	}

	return ctrl.Result{RequeueAfter: teardownResyncDryRun}, nil
}

// reconcileActive drives the actual teardown phases.
func (r *HubTeardownReconciler) reconcileActive(ctx context.Context, log logr.Logger, td *operatorv1.HubTeardown) (ctrl.Result, error) {
	log.Info("Active teardown mode", "phase", td.Status.Phase)

	// Track when active teardown started for overall stall detection.
	// On the first active reconcile, re-scan the cluster to detect
	// changes since the dry-run report was generated.
	if td.Status.ActiveSince == nil {
		if td.Status.DryRunReport != nil {
			graph, err := r.buildDependencyGraph(ctx, log)
			if err != nil {
				log.Error(err, "Failed to re-scan on activation, proceeding with stale report")
			} else {
				freshReport := r.buildDryRunReport(graph)
				oldBlocking := td.Status.DryRunReport.TotalBlockingResources
				oldCloudRisk := td.Status.DryRunReport.TotalCloudRiskResources
				if freshReport.TotalBlockingResources != oldBlocking || freshReport.TotalCloudRiskResources != oldCloudRisk {
					r.setCondition(td, conditionTypeDryRunReportStale, metav1.ConditionTrue, "ClusterStateChanged",
						fmt.Sprintf("Cluster state changed since dry-run scan: blocking resources %d→%d, cloud-risk resources %d→%d. Dry-run report updated.",
							oldBlocking, freshReport.TotalBlockingResources, oldCloudRisk, freshReport.TotalCloudRiskResources))
					r.Recorder.Eventf(td, corev1.EventTypeWarning, "DryRunReportStale",
						"Cluster state changed since dry-run: blocking %d→%d, cloud-risk %d→%d",
						oldBlocking, freshReport.TotalBlockingResources, oldCloudRisk, freshReport.TotalCloudRiskResources)
				} else {
					r.setCondition(td, conditionTypeDryRunReportStale, metav1.ConditionFalse, "ReportCurrent",
						"Cluster state matches dry-run report.")
				}
				td.Status.DryRunReport = freshReport
				td.Status.CloudResourceWarnings = r.buildCloudWarnings(ctx, log, graph)
			}
		}

		now := metav1.Now()
		td.Status.ActiveSince = &now
		if err := r.Client.Status().Update(ctx, td); err != nil {
			log.Error(err, "Failed to persist ActiveSince timestamp")
			return ctrl.Result{}, err
		}
	}

	// Check for overall teardown stall. Skip if the final phase is already complete.
	if td.Status.ActiveSince == nil {
		return ctrl.Result{RequeueAfter: teardownResyncActive}, nil
	}
	if r.isPhaseComplete(td, operatorv1.TeardownPhaseRemoveOLMOperator) {
		if r.isConditionTrue(td, conditionTypeTeardownStalled) {
			r.setCondition(td, conditionTypeTeardownStalled, metav1.ConditionFalse, "TeardownComplete",
				"Teardown completed successfully; stall condition cleared.")
			if err := r.Client.Status().Update(ctx, td); err != nil {
				log.Error(err, "Failed to clear Stalled condition after completion")
			}
		}
		return ctrl.Result{}, nil
	}
	maxDur := defaultMaxDuration
	if td.Spec.MaxDuration != nil {
		maxDur = td.Spec.MaxDuration.Duration
	}
	elapsed := time.Since(td.Status.ActiveSince.Time)
	wasStalled := r.isConditionTrue(td, conditionTypeTeardownStalled)
	if elapsed > maxDur {
		stalledMsg := fmt.Sprintf("Teardown has been running for %s (limit: %s), currently stuck at phase %s.",
			elapsed.Round(time.Second), maxDur.Round(time.Second), td.Status.Phase)
		for _, ps := range td.Status.Phases {
			if ps.State == operatorv1.PhaseStateInProgress && ps.Message != "" {
				stalledMsg += " " + ps.Message
				break
			}
		}
		if !td.Spec.ApprovedDestructiveActions {
			switch td.Status.Phase {
			case operatorv1.TeardownPhaseDisableAddons, operatorv1.TeardownPhaseDetachManagedClusters,
				operatorv1.TeardownPhaseCleanOrphans, operatorv1.TeardownPhaseDeleteMCH:
				stalledMsg += " Consider setting spec.approvedDestructiveActions=true to resolve stuck Tier 2 finalizers."
			}
		}
		if !td.Spec.AcknowledgeCloudResourceRisk {
			if td.Status.Phase == operatorv1.TeardownPhaseDeleteInfrastructureCRs {
				stalledMsg += " Consider setting spec.acknowledgeCloudResourceRisk=true to resolve stuck Tier 1 (cloud) finalizers."
			}
		}
		r.setCondition(td, conditionTypeTeardownStalled, metav1.ConditionTrue, "TeardownStalled", stalledMsg)
		if err := r.Client.Status().Update(ctx, td); err != nil {
			log.Error(err, "Failed to update Stalled condition")
		}
		if !wasStalled {
			r.Recorder.Event(td, corev1.EventTypeWarning, "TeardownStalled", stalledMsg)
		}
		log.Info("Teardown stalled", "elapsed", elapsed.Round(time.Second), "limit", maxDur.Round(time.Second))
	} else if wasStalled {
		r.setCondition(td, conditionTypeTeardownStalled, metav1.ConditionFalse, "NotStalled",
			"Teardown no longer stalled after maxDuration was increased.")
		if err := r.Client.Status().Update(ctx, td); err != nil {
			log.Error(err, "Failed to clear Stalled condition")
		}
		r.Recorder.Event(td, corev1.EventTypeNormal, "StallCleared", "Teardown no longer stalled")
	}

	if err := r.ensureTeardownJob(ctx, log, td); err != nil {
		log.Error(err, "Failed to ensure teardown Job (controller will continue without resilient Job)")
	}

	// Re-verify the OLM gate finalizer on every active reconcile. If an admin
	// or OLM catalog update removed it, re-add it to prevent the operator from
	// being deleted mid-teardown. Only skip if we're past CleanOrphans (gate
	// was intentionally released).
	if !r.isPhaseComplete(td, operatorv1.TeardownPhaseCleanOrphans) {
		if err := r.ensureOLMGate(ctx, log); err != nil {
			log.Error(err, "Failed to re-verify OLM gate finalizer")
		}
	}

	// Clear WaitingForApproval if the user has now granted the required approvals.
	if td.Spec.ApprovedDestructiveActions || td.Spec.AcknowledgeCloudResourceRisk {
		r.clearWaitingForApproval(td)
	}

	r.setCondition(td, conditionTypeTeardownProgressing, metav1.ConditionTrue, "TeardownActive",
		fmt.Sprintf("Teardown is executing phase: %s", td.Status.Phase))

	// Each phase persists its own completion status immediately, so there is
	// no batched status update here. Errors from phases or status writes
	// trigger a requeue.
	result, err := r.executePhases(ctx, log, td)
	if err != nil {
		if statusErr := r.Client.Status().Update(ctx, td); statusErr != nil {
			log.Error(statusErr, "Failed to persist phase failure status")
		}
		return ctrl.Result{RequeueAfter: teardownResyncActive}, nil
	}
	return result, nil
}

// executePhases runs through teardown phases in order, resuming from the last completed phase.
func (r *HubTeardownReconciler) executePhases(ctx context.Context, log logr.Logger, td *operatorv1.HubTeardown) (ctrl.Result, error) {
	phases := []struct {
		phase   operatorv1.TeardownPhase
		execute func(ctx context.Context, log logr.Logger, td *operatorv1.HubTeardown) (bool, error)
	}{
		{operatorv1.TeardownPhaseGateOLMSubscription, r.phaseGateOLMSubscription},
		{operatorv1.TeardownPhaseRemoveBlockingCRs, r.phaseRemoveBlockingCRs},
		{operatorv1.TeardownPhaseDisableAddons, r.phaseDisableAddons},
		{operatorv1.TeardownPhaseDeleteInfrastructureCRs, r.phaseDeleteInfrastructureCRs},
		{operatorv1.TeardownPhaseDetachManagedClusters, r.phaseDetachManagedClusters},
		{operatorv1.TeardownPhaseDeleteMCH, r.phaseDeleteMCH},
		{operatorv1.TeardownPhaseMonitorMCEChain, r.phaseMonitorMCEChain},
		{operatorv1.TeardownPhaseCleanOrphans, r.phaseCleanOrphans},
		{operatorv1.TeardownPhaseDeleteACMCRDs, r.phaseDeleteACMCRDs},
		{operatorv1.TeardownPhaseRemoveOLMOperator, r.phaseRemoveOLMOperator},
	}

	for _, p := range phases {
		if r.isPhaseComplete(td, p.phase) {
			continue
		}

		// Check if admin paused back to dry-run mid-execution
		if td.Spec.DryRun {
			log.Info("Admin re-enabled dry-run, pausing teardown")
			return ctrl.Result{}, nil
		}

		td.Status.Phase = p.phase
		r.setPhaseStatus(td, p.phase, operatorv1.PhaseStateInProgress, "Executing phase")

		done, err := p.execute(ctx, log, td)
		if err != nil {
			r.setPhaseStatus(td, p.phase, operatorv1.PhaseStateFailed, err.Error())
			r.Recorder.Eventf(td, corev1.EventTypeWarning, "PhaseFailed",
				"Phase %s failed: %v", p.phase, err)
			return ctrl.Result{}, err
		}

		if !done {
			r.setPhaseStatus(td, p.phase, operatorv1.PhaseStateInProgress, "Waiting for phase to complete")
			if err := r.Client.Status().Update(ctx, td); err != nil {
				log.Error(err, "Failed to persist mid-phase status", "phase", p.phase)
			}
			return ctrl.Result{RequeueAfter: teardownResyncActive}, nil
		}

		now := metav1.Now()
		r.setPhaseStatusWithTime(td, p.phase, operatorv1.PhaseStateComplete, "Phase completed", &now)
		r.Recorder.Eventf(td, corev1.EventTypeNormal, "PhaseComplete",
			"Phase %s completed successfully", p.phase)

		// Persist phase completion immediately so a controller crash between
		// phases doesn't lose progress and cause re-execution of completed work.
		if err := r.Client.Status().Update(ctx, td); err != nil {
			log.Error(err, "Failed to persist phase completion", "phase", p.phase)
			return ctrl.Result{}, err
		}
	}

	// phaseRemoveOLMOperator handles persisting Complete status, removing the
	// finalizer, and deleting the Subscription/CSV itself — all before the
	// self-destructive CSV deletion. By the time we reach here the CR is
	// already in its terminal state. Nothing left to do.
	return ctrl.Result{}, nil
}

// isPhaseComplete checks if a phase has already been completed.
func (r *HubTeardownReconciler) isPhaseComplete(td *operatorv1.HubTeardown, phase operatorv1.TeardownPhase) bool {
	for _, ps := range td.Status.Phases {
		if ps.Phase == phase && (ps.State == operatorv1.PhaseStateComplete || ps.State == operatorv1.PhaseStateSkipped) {
			return true
		}
	}
	return false
}

// setPhaseStatus updates or appends a phase status entry.
func (r *HubTeardownReconciler) setPhaseStatus(td *operatorv1.HubTeardown, phase operatorv1.TeardownPhase, state operatorv1.TeardownPhaseState, msg string) {
	now := metav1.Now()
	for i, ps := range td.Status.Phases {
		if ps.Phase == phase {
			td.Status.Phases[i].State = state
			td.Status.Phases[i].Message = msg
			if state == operatorv1.PhaseStateInProgress && ps.StartTime == nil {
				td.Status.Phases[i].StartTime = &now
			}
			return
		}
	}
	entry := operatorv1.TeardownPhaseStatus{
		Phase:   phase,
		State:   state,
		Message: msg,
	}
	if state == operatorv1.PhaseStateInProgress {
		entry.StartTime = &now
	}
	td.Status.Phases = append(td.Status.Phases, entry)
}

// setPhaseStatusWithTime updates a phase status with an explicit completion time.
func (r *HubTeardownReconciler) setPhaseStatusWithTime(td *operatorv1.HubTeardown, phase operatorv1.TeardownPhase, state operatorv1.TeardownPhaseState, msg string, completionTime *metav1.Time) {
	r.setPhaseStatus(td, phase, state, msg)
	for i, ps := range td.Status.Phases {
		if ps.Phase == phase {
			td.Status.Phases[i].CompletionTime = completionTime
			return
		}
	}
}

// setCondition sets a metav1.Condition on the HubTeardown status.
func (r *HubTeardownReconciler) setCondition(td *operatorv1.HubTeardown, condType string, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&td.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		ObservedGeneration: td.Generation,
		Reason:             reason,
		Message:            truncateMessage(message, 4096),
	})
}

// isConditionTrue checks whether a condition is currently set to True.
func (r *HubTeardownReconciler) isConditionTrue(td *operatorv1.HubTeardown, condType string) bool {
	for _, c := range td.Status.Conditions {
		if c.Type == condType {
			return c.Status == metav1.ConditionTrue
		}
	}
	return false
}

func truncateMessage(msg string, max int) string {
	if len(msg) > max {
		return msg[:max-15] + "... (truncated)"
	}
	return msg
}

// hasEmittedWarnings checks whether cloud warning events have already been emitted
// for the current spec generation, preventing duplicate event flooding on resyncs.
func (r *HubTeardownReconciler) hasEmittedWarnings(td *operatorv1.HubTeardown) bool {
	if td.Annotations == nil {
		return false
	}
	return td.Annotations[annotationWarningsEmitted] == fmt.Sprintf("%d", td.Generation)
}

// markWarningsEmitted stamps the CR annotation so subsequent resyncs skip event emission.
func (r *HubTeardownReconciler) markWarningsEmitted(ctx context.Context, td *operatorv1.HubTeardown) {
	if td.Annotations == nil {
		td.Annotations = make(map[string]string)
	}
	td.Annotations[annotationWarningsEmitted] = fmt.Sprintf("%d", td.Generation)
	if err := r.Client.Update(ctx, td); err != nil {
		ctrllog.FromContext(ctx).Error(err, "Failed to mark warnings as emitted")
	}
}

// ensureOLMGate re-verifies the teardown gate finalizer on the ACM Subscription.
// Called on every active reconcile to guard against manual removal or OLM
// catalog reconciliation recreating the Subscription without the gate.
func (r *HubTeardownReconciler) ensureOLMGate(ctx context.Context, log logr.Logger) error {
	sub, err := r.findACMSubscription(ctx)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	for _, f := range sub.GetFinalizers() {
		if f == teardownGateFinalizer {
			return nil
		}
	}
	patch := client.MergeFrom(sub.DeepCopy())
	sub.SetFinalizers(append(sub.GetFinalizers(), teardownGateFinalizer))
	if err := r.Client.Patch(ctx, sub, patch); err != nil {
		return fmt.Errorf("re-adding gate finalizer to Subscription: %w", err)
	}
	log.Info("Re-added OLM gate finalizer (was missing)", "subscription", sub.GetName())
	return nil
}

func (r *HubTeardownReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("hubteardown-controller")

	restConfig := mgr.GetConfig()
	dc, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create discovery client: %w", err)
	}
	r.DiscoveryClient = dc

	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1.HubTeardown{}).
		Named("hubteardown").
		Complete(r)
}
