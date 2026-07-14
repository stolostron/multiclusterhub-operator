# HubTeardown: Orchestrated RHACM Uninstallation

HubTeardown provides a phased, observable, and safe alternative to the classic `kubectl delete mch` uninstall workflow. It scans the cluster for dependencies before making any changes, executes teardown in ordered phases with per-phase status tracking, and handles stuck finalizers with explicit admin approval gates.

## When to Use HubTeardown vs Classic Delete

| Scenario | Recommended Method |
|---|---|
| Simple hub with no managed clusters | Classic `kubectl delete mch` |
| Multi-cluster environment with managed clusters | HubTeardown |
| Hypershift / Hosted Control Plane clusters present | HubTeardown |
| Hive-provisioned clusters (ClusterDeployments) present | HubTeardown |
| History of stuck finalizers during uninstall | HubTeardown |
| Need to preview what will happen before committing | HubTeardown |
| Need to pause mid-teardown and resume later | HubTeardown |
| Automated pipeline with simple pass/fail | Classic delete |

HubTeardown's phase 6 (`DeleteMCH`) ultimately triggers the classic MCH finalizer path internally, so the two methods compose rather than compete.

## Prerequisites

- MultiClusterHub operator is installed and running.
- The `HubTeardown` CRD is present on the cluster (`hubteardowns.operator.open-cluster-management.io`).
- The user has RBAC permissions to create, patch, and delete `HubTeardown` resources in the hub namespace.

## Workflow

### Step 1: Create the HubTeardown CR in dry-run mode

```bash
cat <<EOF | oc apply -f -
apiVersion: operator.open-cluster-management.io/v1
kind: HubTeardown
metadata:
  name: teardown
  namespace: open-cluster-management
spec:
  dryRun: true
EOF
```

The controller will scan the cluster and populate the status with a full teardown preview. No resources are modified in dry-run mode.

### Step 2: Review the dry-run report

Check the overall summary:

```bash
oc get hubteardown teardown -n open-cluster-management
```

Review the detailed dry-run report:

```bash
oc get hubteardown teardown -n open-cluster-management -o jsonpath='{.status.dryRunReport.summary}'
```

Check for cloud resource warnings:

```bash
oc get hubteardown teardown -n open-cluster-management \
  -o jsonpath='{range .status.cloudResourceWarnings[*]}{.resource.kind}/{.resource.namespace}/{.resource.name}: {.riskSummary}{"\n"}{end}'
```

Review blocking resources:

```bash
oc get hubteardown teardown -n open-cluster-management \
  -o jsonpath='{range .status.blockingResources[*]}{.kind}/{.name}: {.reason}{"\n"}{end}'
```

### Step 3: Begin teardown

Once you have reviewed the dry-run report, disable dry-run to start the teardown:

```bash
oc patch hubteardown teardown -n open-cluster-management \
  --type=merge -p '{"spec":{"dryRun":false}}'
```

On activation, the controller re-scans the cluster and compares with the dry-run report. If blocking or cloud-risk resource counts have changed since the scan, a `DryRunReportStale` warning condition is set and the updated report is written to status.

During active teardown, the MCH reconciler automatically suspends component deployment to prevent conflicts with teardown phases.

### Step 4: Monitor progress

Watch phase progression:

```bash
oc get hubteardown teardown -n open-cluster-management -w
```

View per-phase detail:

```bash
oc get hubteardown teardown -n open-cluster-management \
  -o jsonpath='{range .status.phases[*]}{.phase}: {.state} - {.message}{"\n"}{end}'
```

Check events emitted by the controller:

```bash
oc get events -n open-cluster-management \
  --field-selector reason!=LeaderElection --sort-by='.lastTimestamp' | grep -i teardown
```

Check operator logs:

```bash
oc logs -n open-cluster-management deployment/multiclusterhub-operator \
  --tail=200 | grep -i hubteardown
```

### Step 5: Escalate flags if stuck

If teardown is stuck, the phase status message will tell you exactly which approval flag is needed. For example:

```
Waiting for 3 addons to terminate. Set spec.approvedDestructiveActions=true to patch Tier 2 (non-cloud) finalizers.
```

To approve Tier 2 (non-cloud) finalizer removal:

```bash
oc patch hubteardown teardown -n open-cluster-management \
  --type=merge -p '{"spec":{"approvedDestructiveActions":true}}'
```

To approve Tier 1 (cloud-protecting) finalizer removal — **this will orphan cloud infrastructure that requires manual cleanup**:

```bash
oc patch hubteardown teardown -n open-cluster-management \
  --type=merge -p '{"spec":{"acknowledgeCloudResourceRisk":true}}'
```

If teardown exceeds `maxDuration` (default 2 hours), a `Stalled` condition is set with guidance on what is blocking and which approval flags to consider.

### Step 6: Pause at any time

You can pause teardown and re-scan at any point by toggling dry-run back on:

```bash
oc patch hubteardown teardown -n open-cluster-management \
  --type=merge -p '{"spec":{"dryRun":true}}'
```

Completed phases are not re-executed on resume.

### Step 7: Post-teardown verification

After teardown completes, check the final status:

```bash
oc get hubteardown teardown -n open-cluster-management -o yaml
```

If cloud resources were orphaned (Tier 1 finalizers were patched), check the events for a summary of what needs manual cleanup:

```bash
oc get events -n open-cluster-management \
  --field-selector reason=CloudResourcesOrphaned
```

Verify cloud resources in the relevant provider console (AWS, Azure, GCP) using the resource details from the events.

## Spec Reference

| Field | Type | Default | Description |
|---|---|---|---|
| `dryRun` | `bool` | `true` | When true, the controller scans and reports without mutating any resources. Set to false to begin teardown. Can be toggled back to true at any time to pause. |
| `approvedDestructiveActions` | `bool` | `false` | Enables Tier 2 (non-cloud) finalizer patching for stuck resources. Only evaluated when `dryRun` is false. |
| `acknowledgeCloudResourceRisk` | `bool` | `false` | Enables Tier 1 (cloud-protecting) finalizer patching. Setting this means you accept that cloud infrastructure (VMs, LBs, storage, DNS) may be orphaned and require manual cleanup. |
| `forceFinalizerTimeout` | `duration` | `5m` | How long to wait for a controller to process its own finalizer before the teardown controller intervenes. |
| `maxDuration` | `duration` | `2h` | Maximum wall-clock time for the entire teardown. If exceeded, a `Stalled` condition is set with guidance on what is blocking. |

## Phase Reference

HubTeardown executes 10 phases in order. Each phase reports its status (Pending, InProgress, Complete, Failed, Skipped) with start and completion timestamps.

### 1. GateOLMSubscription

Adds a finalizer (`operator.open-cluster-management.io/teardown-gate`) to the ACM OLM Subscription. This prevents the operator from being removed by OLM before teardown completes. The finalizer is re-added on every reconcile if manually removed. Skipped if no ACM Subscription is found.

### 2. RemoveBlockingCRs

Deletes resources that block the MCH validating webhook:
- `MultiClusterObservability`
- `DiscoveryConfig`
- `AgentServiceConfig`
- `SearchCustomization`
- Orphaned admission webhooks whose backing services are gone

Waits for each to be fully removed before proceeding.

### 3. DisableAddons

Clears addon placements from `ClusterManagementAddOn` resources, scales down addon controllers, and deletes `ManagedClusterAddOn` resources for all non-local ManagedClusters. Resolves stuck addon finalizers based on approval flags. Local-cluster addons are skipped (handled in phase 6). If stuck, the status message indicates which approval flag is needed.

### 4. DeleteInfrastructureCRs

Deletes infrastructure provisioning CRs while their controllers are still running:
- `HostedCluster` and `NodePool` (HyperShift)
- `ClusterDeployment` and `ClusterPool` (Hive)
- `InfraEnv` (assisted-installer)
- `ClusterInstance` (siteconfig)

**Cloud risk gate:** This phase requires `acknowledgeCloudResourceRisk: true` when infrastructure CRs exist. For Hive-provisioned clusters, the delete triggers Hive's deprovision pod which cleans up cloud resources (VMs, VPCs, DNS, security groups). For HyperShift, the delete triggers the HyperShift operator's cleanup. The phase waits for all infrastructure CRs to fully terminate before proceeding.

If infrastructure CRs are stuck finalizing, the status message indicates which approval flag is needed.

### 5. DetachManagedClusters

Deletes all non-local `ManagedCluster` resources. If a ManagedCluster has been stuck deleting longer than `forceFinalizerTimeout`, the controller resolves stuck finalizers (subject to approval flags). Unreachable clusters have their addon finalizers force-stripped after timeout. `local-cluster` is always preserved.

### 6. DeleteMCH

Deletes the `MultiClusterHub` CR. This triggers the classic MCH finalizer path internally (`finalizeHub`), which handles app subscriptions, HelmRelease cleanup, namespace teardown, ClusterRole/Binding cleanup, and MCE removal. Resolves blocking MCH finalizers (including submariner and ClusterManagementAddOn finalizers) after timeout.

### 7. MonitorMCEChain

Monitors the downstream deletion chain: waits for `MultiClusterHub` and `MultiClusterEngine` to be fully removed. Resolves orphaned `ClusterManager` finalizer if the MCE operator deployment is already gone. This covers the MCE -> ClusterManager -> Hypershift operator cleanup cascade.

### 8. CleanOrphans

Re-scans the cluster for any stuck-terminating ACM/MCE resources left after the MCE chain completes. Resolves stuck finalizers based on tier and approval flags. Releases the OLM Subscription gate finalizer. Emits a summary event listing any orphaned cloud resources.

### 9. DeleteACMCRDs

Removes all ACM/MCE CRDs from the cluster.

### 10. RemoveOLMOperator

Persists `Complete` status, then deletes the OLM Subscription and ClusterServiceVersion. This is the point of no return — deleting the CSV terminates the operator pod. The teardown cleanup Job (if running) handles any remaining work.

## Safety Features

### MCH Reconciler Awareness

During active teardown, the MCH reconciler detects the HubTeardown CR and suspends all component deployment (`ensureComponent()` calls are skipped). This prevents the MCH reconciler from fighting teardown by re-creating addon deployments that teardown phase 3 just scaled down. A `TeardownInProgress` condition is set on the MCH so `oc get mch` shows the suspended state.

### Singleton Guard

Only one HubTeardown CR can be active at a time. If a second CR is created, it is immediately rejected with a `DuplicateRejected` condition.

### Dry-Run Report Freshness

When transitioning from `dryRun: true` to `dryRun: false`, the controller re-scans the cluster and compares the current state with the dry-run report. If blocking or cloud-risk resource counts have changed, a `DryRunReportStale` warning condition and event are emitted, and the dry-run report in status is updated to reflect current state.

### Overall Stall Detection

If teardown exceeds `maxDuration` (default 2h), a `Stalled` condition is set. The stall message includes:
- How long teardown has been running
- Which phase is stuck and why
- Which approval flag to set to unblock (phase-specific guidance)

### OLM Gate Persistence

The OLM Subscription gate finalizer is re-verified on every active reconcile. If an admin or OLM catalog update removes it, the controller re-adds it to prevent the operator from being deleted mid-teardown.

## Finalizer Tiers

The teardown controller classifies known finalizers into two tiers to control the risk of automatic removal.

### Tier 1: Cloud-Protecting

These finalizers guard cloud infrastructure. Removing them orphans resources (VMs, load balancers, DNS records, storage) that require manual cleanup in the cloud provider console.

| Finalizer | Description |
|---|---|
| `hypershift.openshift.io/finalizer` | Hypershift cluster lifecycle: VMs, LBs, security groups, DNS |
| `hypershift.io/aws-oidc-discovery` | AWS S3 OIDC discovery documents |
| `hypershift.openshift.io/control-plane-operator-finalizer` | AWS PrivateLink / GCP Private Service Connect |
| `hive.openshift.io/deprovision` | Full cluster infrastructure deprovisioning |
| `agentserviceconfig.agent-install.openshift.io/ai-deprovision` | Assisted-installer provisioned infrastructure |

**Requires:** `spec.acknowledgeCloudResourceRisk: true`

### Tier 2: Non-Cloud

These finalizers protect Kubernetes-level state only. Removing them skips graceful cleanup of in-cluster resources but does not orphan cloud infrastructure.

| Finalizer | Description |
|---|---|
| `managedcluster-import-controller.open-cluster-management.io/cleanup` | Import controller state cleanup |
| `addon.open-cluster-management.io/addon-pre-delete` | Addon pre-delete hook execution |
| `work.open-cluster-management.io/manifest-work-cleanup` | ManifestWork cleanup |
| `search.open-cluster-management.io/finalizer` | Search index cleanup |
| `uninstall-helm-release` | HelmRelease state cleanup |
| `operator.open-cluster-management.io/cluster-manager-cleanup` | ClusterManager CRD cleanup |
| `finalizer.operator.open-cluster-management.io` | MCH operator hub finalizer |
| `finalizer.multicluster.openshift.io` | MCE operator backplane finalizer |

**Requires:** `spec.approvedDestructiveActions: true`

### Unknown Finalizers

Finalizers not in either allowlist are never automatically removed. They require manual intervention.

## Resilient Teardown Job

When the teardown enters active mode, the controller creates a Kubernetes Job (`hubteardown-cleanup`) that runs the operator binary with `--teardown-mode`. This Job:

- Survives operator pod deletion (e.g., if OLM removes the operator deployment).
- Runs a minimal manager with only the HubTeardown controller (no webhooks, no leader election).
- Uses the same operator image and service account.
- Is automatically cleaned up when teardown completes.
- Has a backoff limit of 6 retries and a TTL of 1 hour after completion.

The Job is a safety net. The primary controller path works independently. If `OPERATOR_IMAGE` is not set in the operator environment, the Job is skipped and teardown relies on the controller only.

## Troubleshooting

### Teardown stuck in a phase

Check which phase is stuck and what it needs:

```bash
oc get hubteardown teardown -n open-cluster-management \
  -o jsonpath='{range .status.phases[*]}{.phase}: {.state} - {.message}{"\n"}{end}'
```

The phase status message will indicate exactly which approval flag is needed. Common causes:

- **RemoveBlockingCRs**: A blocking CR has its own finalizer that is not completing. Check the resource directly.
- **DisableAddons**: Addon finalizers are stuck. The message will say whether to set `approvedDestructiveActions` or `acknowledgeCloudResourceRisk`.
- **DeleteInfrastructureCRs**: Hive deprovision pods or HyperShift cleanup is still running (can take 15-45 minutes per cluster). If stuck, the message will indicate which approval flag is needed.
- **DetachManagedClusters**: ManagedClusters have finalizers that the responsible controller is not processing. Consider setting `approvedDestructiveActions: true` after verifying the resources.
- **DeleteMCH**: The classic MCH finalizer path is stuck. Check the operator logs for `Finalizing:` messages.
- **MonitorMCEChain**: MCE has a finalizer that is not being processed. Check the MCE operator logs.
- **CleanOrphans**: Stuck resources need approval flags. The phase status message will indicate which flags to set.

### Stalled condition

If the `Stalled` condition appears, teardown has exceeded `maxDuration`. The condition message includes which phase is stuck and which approval flag to set. You can increase `maxDuration` to allow more time:

```bash
oc patch hubteardown teardown -n open-cluster-management \
  --type=merge -p '{"spec":{"maxDuration":"4h"}}'
```

### Dry-run report changed on activation

If a `DryRunReportStale` condition appears when you set `dryRun: false`, the cluster state changed since the dry-run scan. Review the updated report in `status.dryRunReport` and the condition message for what changed (e.g., "blocking resources 2→3, cloud-risk resources 0→1").

### Cloud resource warnings

If `status.cloudResourceWarnings` is populated, review each warning carefully before setting `acknowledgeCloudResourceRisk: true`. The warnings include:
- The specific cloud resources at risk (by platform: AWS, Azure, GCP).
- The current state of the resource (phase, whether it is already terminating).
- Instructions for manual verification in the cloud provider console.

### Resuming after a pause

If you paused teardown by setting `dryRun: true`, phases that already completed will not re-run. Setting `dryRun: false` resumes from the next incomplete phase.

### The teardown Job

If the operator pod is removed and the teardown Job takes over, you can monitor it:

```bash
oc get job hubteardown-cleanup -n open-cluster-management
oc logs job/hubteardown-cleanup -n open-cluster-management
```

The Job runs the same controller logic and updates the same `HubTeardown` CR status.

## Architecture

```
HubTeardown CR (dryRun: true)  -> scan -> dryRunReport
HubTeardown CR (dryRun: false) -> re-scan (staleness check)
                               -> GateOLMSubscription
                               -> RemoveBlockingCRs
                               -> DisableAddons
                               -> DeleteInfrastructureCRs (cloud risk gate)
                               -> DetachManagedClusters
                               -> DeleteMCH (triggers finalizeHub)
                               -> MonitorMCEChain
                               -> CleanOrphans + release OLM gate
                               -> DeleteACMCRDs
                               -> RemoveOLMOperator (self-destruct)
                               -> Complete
```

### Key files

| File | Purpose |
|---|---|
| `api/v1/hubteardown_types.go` | CRD types, spec, status, phase enums |
| `controllers/hubteardown_controller.go` | Main reconciler, dry-run scan, stall detection, phase dispatch |
| `controllers/teardown_execute.go` | Phase implementations (10 phases) |
| `controllers/teardown_finalizers.go` | Tier 1/2 finalizer classification and resolution |
| `controllers/teardown_graph.go` | Dependency graph builder for dry-run report |
| `controllers/teardown_job.go` | Resilient cleanup Job lifecycle |
| `controllers/teardown_warnings.go` | Cloud resource warning builder |
| `controllers/teardown_test.go` | Unit tests (20 tests) |
