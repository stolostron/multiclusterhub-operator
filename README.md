[comment]: # ( Copyright Contributors to the Open Cluster Management project )

# WORK IN PROGRESS

We are in the process of enabling this repo for community contribution. See wiki [here](https://open-cluster-management.io/concepts/architecture/).

# MultiClusterHub Operator

The MultiClusterHub operator manages the install of Open Cluster Management (OCM) on RedHat Openshift Container Platform

## Contributing

For steps on how to contribute and test the MultiClusterHub Operator component, see [CONTRIBUTING](./CONTRIBUTING.md) guidelines.

## Development Tools

### Disabling MultiClusterHub Operator

Once installed, the hub operator will monitor changes in the cluster that affect an instance of the multiclusterhub (mch) and reconcile deviations to maintain desired state. To stop the installer from making these changes you can apply an annotation to the mch instance.

```bash
kubectl annotate mch <mch-name> installer.open-cluster-management.io/pause=true
```

Remove or edit this annotation to resume installer operations

```bash
kubectl annotate mch <mch-name> installer.open-cluster-management.io/pause=false --overwrite
```

### Add Image Overrides Via Configmap

Developer image overrides can be added by specifiying a configmap containing the overrides for the MCH resource. This configmap must be in the same namespace as the MCH resource.

This is done by creating a configmap from a new [manifest](https://github.com/stolostron/pipeline/tree/2.7-integration/snapshots). A developer may use this to override any 1 or all images.

If overriding individual images, the minimum required parameters required to build the image reference are -

- `image-name`
- `image-remote`
- `image-key`
- `image-digest` or `image-tag`, both can optionally be provided, if so the `image-digest` will be preferred.

```bash
kubectl create configmap <my-config> --from-file=docs/examples/manifest-oneimage.json # Override 1 image example
kubectl create configmap <my-config> --from-file=docs/examples/manifest-allimages.json # Overriding all images example

kubectl annotate mch <mch-name> --overwrite installer.open-cluster-management.io/image-overrides-configmap=<my-config> # Provide the configmap as an override to the MCH
```

To remove this annotation to revert back to the original manifest

```bash
kubectl annotate mch <mch-name> installer.open-cluster-management.io/image-overrides-configmap --overwrite # Remove annotation
kubectl delete configmap <my-config> # Delete configmap
```

If editing the configmap directly instead of creating/deleting it each time, an operator reconcile may be necessary in order to get the changes to take effect. This can be done by cycling the MCH Operator pod

```bash
kubectl delete pod multiclusterhub-operator-xxxxx-xxxxx
```

### Overriding MultiCluster Engine Installation (OLM v0 vs OLM v1)

The multicluster engine is installed automatically as part of a standard MCH installation. The operator detects which OLM version is available and uses the appropriate method:

- **OLM v0** (OpenShift 4.x): Uses Subscription + CSV
- **OLM v1** (OpenShift 5.x+): Uses ClusterExtension

The installation spec can be overridden using version-specific annotations:

#### OLM v0: Subscription Override

```yaml
apiVersion: operator.open-cluster-management.io/v1
kind: MultiClusterHub
metadata:
  annotations:
    installer.open-cluster-management.io/mce-subscription-spec: |
      {
        "channel": "stable-2.6",
        "installPlanApproval": "Manual",
        "source": "redhat-operators",
        "sourceNamespace": "openshift-marketplace",
        "startingCSV": "multicluster-engine.v2.6.0"
      }
  name: multiclusterhub
spec: {}
```

#### OLM v1: ClusterExtension Override

```yaml
apiVersion: operator.open-cluster-management.io/v1
kind: MultiClusterHub
metadata:
  annotations:
    installer.open-cluster-management.io/mce-clusterextension-spec: |
      {
        "channels": ["stable-2.6"],
        "version": ">=2.6.0 <2.7.0",
        "crdUpgradeSafetyEnforcement": "Strict"
      }
  name: multiclusterhub
spec: {}
```

**Key Differences:**

| Field | OLM v0 (Subscription) | OLM v1 (ClusterExtension) |
|-------|-----------------------|---------------------------|
| Channel selection | `"channel": "stable-2.6"` | `"channels": ["stable-2.6"]` |
| Version pinning | `"startingCSV": "multicluster-engine.v2.6.0"` | `"version": ">=2.6.0 <2.7.0"` (semver) |
| Upgrade approval | `"installPlanApproval": "Manual"` | `"crdUpgradeSafetyEnforcement": "Strict"` |
| Catalog selection | `"source"` + `"sourceNamespace"` | Automatic (by priority) |
| Config structure | `"config": {...}` | `"config": {"inline": {...}}` |

**Important:** Using the wrong annotation for your OLM version will have no effect (silently ignored).

#### Detecting OLM Version

The operator auto-detects OLM version, but you can verify manually:

```bash
# Check for OLM v1 (ClusterExtension CRD exists)
oc get crd clusterextensions.olm.operatorframework.io

# Check for OLM v0 (look for CSV resources)
oc get csv -n openshift-marketplace
```

If ClusterExtension CRD exists, cluster has OLM v1. Otherwise, OLM v0.

#### Resources Created

**OLM v0 creates:**
- Namespace: `multicluster-engine`
- Subscription (in `multicluster-engine` namespace)
- OperatorGroup (in `multicluster-engine` namespace)
- ClusterServiceVersion (managed by OLM)

**OLM v1 creates:**
- Namespace: `multicluster-engine`
- ServiceAccount: `mce-installer` (in `multicluster-engine` namespace)
- ClusterRoleBinding: `mce-installer-admin` (cluster-scoped, grants cluster-admin)
- ClusterExtension: `multicluster-engine` (cluster-scoped)

#### Catalog Management

**OLM v0:**
- Uses CatalogSource (namespaced in `openshift-marketplace`)
- Specify catalog via annotation: `"source": "custom-catalog", "sourceNamespace": "openshift-marketplace"`

```bash
# List catalogs
oc get catalogsource -n openshift-marketplace
```

**OLM v1:**
- Uses ClusterCatalog (cluster-scoped)
- Auto-selects catalog by priority (cannot pin catalog in annotation)
- To prefer custom catalog, set higher priority in ClusterCatalog spec

```bash
# List catalogs
oc get clustercatalog
```

#### Troubleshooting

**OLM v1: ClusterCatalog not serving**
```bash
# Check catalog status
oc get clustercatalog -o wide

# Wait for catalog to be ready
oc wait --for=condition=Serving clustercatalog/redhat-operators --timeout=10m
```

**OLM v1: ServiceAccount missing**
```bash
# Check if ServiceAccount created
oc get sa -n multicluster-engine mce-installer

# If missing, check operator logs
oc logs -n open-cluster-management deployment/multiclusterhub-operator | grep ServiceAccount
```

**OLM v0: Subscription stuck**
```bash
# Check subscription status
oc get subscription -n multicluster-engine

# If UpgradePending, approve InstallPlan
oc get installplan -n multicluster-engine
oc patch installplan <plan-name> -n multicluster-engine --type merge --patch '{"spec":{"approved":true}}'
```

### Overriding OADP Operator Subscription

The OADP operator is installed from redhat-operators by the cluster-backup chart. The spec of the subscription can be overriden by providing the following annotation to the MCH resource. One or many parameters can be provided from the ones listed in the `installer.open-cluster-management.io/oadp-subscription-spec` annotation below

```yaml
apiVersion: operator.open-cluster-management.io/v1
kind: MultiClusterHub
metadata:
  annotations:
    installer.open-cluster-management.io/oadp-subscription-spec: '{"channel": "stable-1.0","installPlanApproval": "Automatic","name":
      "redhat-oadp-operator","source": "redhat-operators","sourceNamespace": "openshift-marketplace","startingCSV":
      "oadp-operator.v1.0.2"}'
  name: multiclusterhub
spec: {}
```

Setting OADP annotation via CLI

```bash
oc annotate mch multiclusterhub installer.open-cluster-management.io/oadp-subscription-spec='{"channel":"stable-1.0","installPlanApproval":"Automatic","name":"redhat-oadp-operator","source":"redhat-operators","sourceNamespace":"openshift-marketplace","startingCSV": "oadp-operator.v1.0.2"}'
```

### Ignore OCP Version Requirement

The operator defines a minimum version of OCP it can run in to avoid unexpected behavior. If the OCP environment is below this threshold then the MCH instance will report failure early on. This requirement can be ignored in the following two ways

1. Set `DISABLE_OCP_MIN_VERSION` as an environment variable. The presence of this variable in the container the operator runs will skip the check.
2. Set `installer.open-cluster-management.io/ignore-ocp-version` annotation in the MCH instance.

```bash
kubectl annotate mch <mch-name> installer.open-cluster-management.io/ignore-ocp-version=true
```

### Ignore MCE Version Requirement

After deploying MCE the operator waits for MCE install to complete and verifies it is running at a minimum version. To ignore this version check:

1. Set `DISABLE_MCE_MIN_VERSION` as an environment variable. With this set the operator will only check that MCE has set its currentVersion status.

### Other Development Documents

- [Installation Guide](/docs/installation.md)
- [Configuration Guide](/docs/configuration.md)
- [Deploy automation](https://github.com/stolostron/deploy)

Rebuild Image: Thu Jul 24 10:04:30 EDT 2025
