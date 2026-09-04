# Architecture: multiclusterhub-operator

## Overview

The **MultiClusterHub (MCH) Operator** is the top-level operator for Red Hat
Advanced Cluster Management for Kubernetes (RHACM/ACM) — "the hub of the
hub." It manages installation and lifecycle of the full ACM hub stack on
OpenShift, centered on the `MultiClusterHub` custom resource (module
`github.com/stolostron/multiclusterhub-operator`, API group
`operator.open-cluster-management.io`).

The operator first deploys/adopts a **MultiClusterEngine (MCE)** instance
(via `backplane-operator`), which provides foundational CRDs and cluster
lifecycle capabilities, and then deploys ACM platform components on top:
console, governance/risk/compliance (GRC), observability, search,
application lifecycle, insights, cluster-lifecycle, submariner, volsync,
cluster-backup, and more.

## Repository Structure

| Path | Purpose |
|------|---------|
| `main.go` | Operator entrypoint: manager setup, scheme registration, webhook TLS, OLM detection, dynamic MCE watch. |
| `api/v1/` | `MultiClusterHub` and `InternalHubComponent` API types, webhook, deepcopy, helper methods. |
| `controllers/` | Reconciliation logic split by concern (~20 files): `reconcile.go`, `setup.go`, `components.go`, `templates.go`, `common.go`, `status.go`, `lifecycle.go`, `finalizers.go`, `sts.go`, `networkpolicy.go`, `internal_hub_components.go`, `managedcluster.go`. |
| `pkg/` | `deploying/`, `rendering/` (Helm chart engine), `manifest/`, `multiclusterengine/` (+ `olm/v0`, `olm/v1`), `overrides/`, `predicate/`, `templates/` (embedded charts/CRDs), `utils/`, `helpers/`, `version/`. |
| `config/` | Kustomize manifests: `crd/`, `default/`, `manager/`, `rbac/`, `webhook/`, `certmanager/`, `prometheus/`, `manifests/`. |
| `bundle/` | OLM bundle (CSV, CRD, webhook service, scorecard tests). |
| `build/` | `Dockerfile.rhtap` (Konflux), `Dockerfile.prow`, `Dockerfile.test.prow`. |
| `hack/` | `bundle-automation/` (Python chart/CSV generation), `catalog/`, `scripts/`. |
| `test/` | `function_tests/` (Ginkgo install/uninstall/update suites), `unit-tests/`. |
| `docs/` | Installation, configuration, available-components, disconnected/STS install guides. |
| `.tekton/`, `.github/workflows/` | Konflux pipelines and GitHub Actions CI. |

## Core Components

### The `MultiClusterHub` API (`api/v1/multiclusterhub_types.go`)

- **`Spec`**: `ImagePullSecret`, `AvailabilityConfig` (Basic/High), `NodeSelector`,
  `Tolerations`, `Overrides` (per-component config), `DisableHubSelfManagement`,
  `DisableUpdateClusterImageSets`, `LocalClusterName`, `NetworkPolicies`.
- **`Status`**: `Phase` (Pending/Paused/Running/Installing/Updating/
  Uninstalling/Error), `CurrentVersion`, `DesiredVersion`,
  `HubConditions`, `Components map[string]StatusCondition`,
  `MCEVersionCompliance`.
- **`InternalHubComponent`**: a marker CR used to signal component
  activation to operand teams.

### Managed components (`api/v1/multiclusterhub_methods.go`)

MCH-owned components include `app-lifecycle`, `cluster-backup`,
`cluster-lifecycle`, `console`, `fine-grained-rbac`, `grc`, `insights`,
`multicluster-engine`, `multicluster-observability`, `search`, `siteconfig`,
`submariner-addon`, and `volsync`. A separate `MCEComponents` list is passed
through to the managed MCE instance.

### Controllers

A single `MultiClusterHubReconciler` split across topic files:
`components.go` (ensure/ensureNo component logic and chart-location
mapping), `templates.go` (server-side apply with ownership/adoption
gating), `common.go` (MCE installation via OLM Subscription or
ClusterExtension), `status.go` (status/condition aggregation and MCE
version-compliance checks), `lifecycle.go` and `finalizers.go` (install/
uninstall), `sts.go` (AWS STS detection), `networkpolicy.go` (create-once
NetworkPolicies).

### Webhooks (`api/v1/multiclusterhub_webhook.go`)

Defaulting and validating webhooks handle annotation migration, block
deletion when protected resources exist (e.g. `MultiClusterObservability`,
`DiscoveryConfig`, `AgentServiceConfig`), and validate component names.
Webhook server TLS is configured dynamically from the OpenShift APIServer's
TLS profile.

## Data / Control Flow

### Startup (`main.go`)

Registers schemes for OCM, MCE (`backplane-operator` API), OLM v0/v1,
OpenShift, and Prometheus APIs; detects the OLM version in use
(`OperatorCondition` presence → v0, `ClusterExtension` CRD presence → v1,
neither → bare/pre-installed MCE); starts a background goroutine that adds
a dynamic watch on `MultiClusterEngine` once its CRD appears.

### Reconcile loop (`controllers/reconcile.go`)

1. Fetch the `MultiClusterHub` CR; migrate deprecated annotations.
2. Resolve image/template overrides (environment, annotations, dev
   ConfigMaps); apply defaults.
3. Handle deletion via `finalizeHub`, or continue with installation.
4. Install CRDs; deploy the App Subscription operator component first;
   ensure the pull secret.
5. **`ensureMultiClusterEngine`** — deploy or adopt an MCE instance; this
   must precede other components since they depend on MCE-provided CRDs.
6. Configure ingress domain, trust bundle, NetworkPolicies (create-once),
   and MCH operator metrics.
7. **`waitForMCEReady`** — block until the managed MCE reports a compatible
   `CurrentVersion` before proceeding.
8. Loop over `MCHComponents`, calling `ensureComponentOrNoComponent` for
   each (skipping components that have been migrated into MCE).
9. Sync `KlusterletAddonConfig` for hub self-management (unless disabled).
10. Update status and requeue (~5 minutes).

### MCE integration (`pkg/multiclusterengine/`)

Branches on detected OLM version:
- **OLM v1**: creates a `ClusterExtension` (plus a dedicated installer
  ServiceAccount/ClusterRoleBinding).
- **OLM v0**: manages a `Subscription` + `OperatorGroup` against the
  `multicluster-engine` package.
- **None**: skips installation (MCE assumed pre-installed/external).

The `MultiClusterEngine` CR itself is rendered from the MCH spec
(local-cluster name, image pull secret, tolerations, node selector,
availability, target namespace, network policies, and the `MCEComponents`
override list).

### Component deployment

Charts are rendered via Helm v3 (`pkg/rendering/renderer.go`) with values
covering image/template overrides, proxy configuration, hub sizing, and
storage class; applied via server-side apply with field manager
`multiclusterhub-operator`, gated by ownership/adoption policy and a
release-version alignment annotation.

## Build, Test & Release

- **Makefile**: `manifests`/`generate` (controller-gen), `test` (envtest,
  coverage to `cover.out`), `build`/`run`, `docker-build`/`podman-build`,
  `install`/`deploy` (kustomize), `bundle`/`bundle-build`/`catalog-build`
  (OLM).
- **Dockerfiles**: `Dockerfile` (community, static Go binary → UBI9
  minimal, non-root); `build/Dockerfile.rhtap` (Konflux/hermetic,
  multi-arch).
- **CI**: `.tekton/` Konflux PipelineRuns for `release-acm-5.0`/`5.1`
  branches; `.github/workflows/` includes RBAC-generation verification,
  bundle/chart regeneration, image-key validation, and OWNERS resync.
- **Bundle**: consumed downstream by `stolostron/acm-operator-bundle` and
  the catalog tooling in `acm-mce-operator-catalogs`.
- **Testing**: envtest + Ginkgo/Gomega for controller/webhook suites;
  dedicated functional test suite (`test/function_tests`) covering
  install/uninstall/update flows.

## Dependencies & Integrations

- **controller-runtime**, **k8s.io** client libraries, **helm.sh/helm/v3**.
- **`github.com/stolostron/backplane-operator`** — the `MultiClusterEngine`
  API this operator deploys/adopts; the primary integration point.
- **`github.com/stolostron/search-v2-operator`**, **open-cluster-management.io/api**.
- **operator-framework/** (OLM v0/v1/v2 APIs, `operator-controller`,
  `operator-lib`).
- **openshift/api**, **prometheus-operator** monitoring APIs.
- Related repos: `acm-operator-bundle`, `installer-dev-tools` (bundle-chart
  generation tooling), `konflux-build-catalog` (CI pipelines).
- Deployed ACM operands (charts under `pkg/templates/charts/toggle/`):
  console, GRC, insights, cluster-lifecycle, cluster-backup,
  multicloud-operators-subscription (app-lifecycle), multicluster-observability,
  search-v2, siteconfig, submariner-addon, volsync, and more.

## Conventions & Patterns

- **Kubebuilder v4 layout**, single API group, single reconciler split into
  many topic files.
- **Versioning**: `OPERATOR_VERSION` env var mandatory at startup; minimum
  supported OCP and MCE versions are enforced (with development
  escape-hatch environment variables/annotations).
- **Reconciliation idioms**: deferred status sync; server-side apply with a
  dedicated field manager; installer labels drive watch enqueue and
  cleanup; create-once NetworkPolicy pattern.
- **OLM abstraction**: runtime detection of OLM v0 vs. v1 vs. none, with
  parallel implementations under `pkg/multiclusterengine/olm/{v0,v1}`.
- **Overrides & escape hatches**: pause annotation, image-repository/
  image-overrides-configmap/template-overrides-configmap annotations,
  minimum-version bypass environment variables.
- **Component migration handling**: components later folded into MCE are
  kept in the component list for webhook validation but pruned from active
  reconciliation.
- **DCO sign-off required** on all commits.
