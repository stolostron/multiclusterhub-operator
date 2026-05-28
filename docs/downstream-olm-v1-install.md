# Installing ACM with Downstream Builds (OLM v1)

This guide explains how to install ACM using downstream development snapshots on clusters with OLM v1 (OpenShift 5.x+).

**Target Audience:** ACM developers and QE testing downstream builds.

**Not for production use.** For production installations, use official Red Hat operator catalogs.

---

## Prerequisites

- OpenShift cluster with OLM v1 (verify: `oc get crd clusterextensions.olm.operatorframework.io`)
- Access to downstream registry (e.g., `quay.io:443/acm-d`)
- Registry credentials for pulling images
- `kubectl` or `oc` CLI configured

---

## What Gets Installed

The installation creates:

1. **ImageContentSourcePolicy** - Mirrors registry.redhat.io to downstream registry
2. **Global pull secret** - Updated with downstream registry credentials
3. **ClusterCatalogs** - ACM and MCE dev catalogs from snapshot builds
4. **Namespace** - `open-cluster-management` (configurable)
5. **ServiceAccount** - `acm-installer` with cluster-admin permissions
6. **ClusterExtension** - Installs ACM operator via OLM v1
7. **MultiClusterHub CR** - Triggers ACM installation

---

## Configuration Variables

Edit these at the top of the script or export before running:

| Variable | Default | Description |
|----------|---------|-------------|
| `MCH_NAMESPACE` | `open-cluster-management` | Namespace for ACM operator |
| `MCH_NAME` | `multiclusterhub` | Name of MultiClusterHub CR |
| `REGISTRY_MIRROR` | `quay.io:443/acm-d` | Downstream registry mirror |
| `PULL_SECRET_NAME` | `multiclusterhub-operator-pull-secret` | Pull secret name |
| `ACM_SNAPSHOT` | `latest-5.0` | ACM snapshot version (e.g., `5.0.0-DOWNSTREAM-2026-05-28-11-50-04`) |
| `MCE_SNAPSHOT` | `latest-5.0` | MCE snapshot version (e.g., `5.0.0-DOWNSTREAM-2026-05-28-11-50-04`) |

**Example:**
```bash
export ACM_SNAPSHOT="5.0.0-DOWNSTREAM-2026-05-28-11-50-04"
export MCE_SNAPSHOT="5.0.0-DOWNSTREAM-2026-05-28-11-50-04"
```

---

## Installation Steps

### Step 1: Setup ICSP (ImageContentSourcePolicy)

Mirrors Red Hat registries to downstream registry:

```yaml
apiVersion: operator.openshift.io/v1alpha1
kind: ImageContentSourcePolicy
metadata:
  name: rhacm-repo
spec:
  repositoryDigestMirrors:
  - mirrors:
    - quay.io:443/acm-d
    source: registry.redhat.io/rhacm2
  - mirrors:
    - quay.io:443/acm-d
    source: registry.redhat.io/multicluster-engine
```

**Note:** ICSP triggers node reboot to apply mirror configuration.

### Step 2: Update Global Pull Secret

Adds downstream registry credentials to global pull secret in `openshift-config` namespace:

```bash
oc get secret/pull-secret -n openshift-config -o jsonpath='{.data.\.dockerconfigjson}' | base64 -d > pull_secret.yaml
oc registry login --registry="quay.io:443" --auth-basic="$USER:$PASS" --to=pull_secret.yaml
oc set data secret/pull-secret -n openshift-config --from-file=.dockerconfigjson=pull_secret.yaml
```

Script prompts for credentials interactively and saves to `registry_creds.txt` for reuse.

### Step 3: Create ClusterCatalogs

Creates two ClusterCatalogs pointing to downstream snapshot builds:

```yaml
apiVersion: olm.operatorframework.io/v1
kind: ClusterCatalog
metadata:
  name: acm-dev-catalog
spec:
  source:
    type: Image
    image:
      ref: quay.io:443/acm-d/acm-dev-catalog:latest-5.0
```

```yaml
apiVersion: olm.operatorframework.io/v1
kind: ClusterCatalog
metadata:
  name: mce-dev-catalog
spec:
  source:
    type: Image
    image:
      ref: quay.io:443/acm-d/mce-dev-catalog:latest-5.0
```

Script waits for catalogs to reach `Serving` state before continuing.

### Step 4: Create Namespace and Pull Secret

Creates `open-cluster-management` namespace and copies global pull secret:

```bash
kubectl create namespace open-cluster-management
kubectl get secret pull-secret -n openshift-config -o yaml | \
  sed "s/namespace: openshift-config/namespace: open-cluster-management/" | \
  sed "s/name: pull-secret/name: multiclusterhub-operator-pull-secret/" | \
  kubectl apply -f -
```

### Step 5: Create ServiceAccount and RBAC

OLM v1 requires explicit ServiceAccount with permissions:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: acm-installer
  namespace: open-cluster-management
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: acm-installer-admin
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: acm-installer
  namespace: open-cluster-management
```

### Step 6: Create ClusterExtension

Installs ACM operator via ClusterExtension (OLM v1 resource):

```yaml
apiVersion: olm.operatorframework.io/v1
kind: ClusterExtension
metadata:
  name: advanced-cluster-management
spec:
  namespace: open-cluster-management
  serviceAccount:
    name: acm-installer
  source:
    sourceType: Catalog
    catalog:
      packageName: advanced-cluster-management
  config:
    configType: Inline
    inline:
      watchNamespace: open-cluster-management
```

Script waits for `Installed` condition before proceeding.

### Step 7: Create MultiClusterHub CR

Triggers ACM installation:

```yaml
apiVersion: operator.open-cluster-management.io/v1
kind: MultiClusterHub
metadata:
  name: multiclusterhub
  namespace: open-cluster-management
spec:
  imagePullSecret: multiclusterhub-operator-pull-secret
  availabilityConfig: High
```

---

## Installation Script

Save as `install-downstream-olmv1.sh`:

```bash
#!/usr/bin/env bash

# OLM v1 Installation Script for ACM/MCE
# For downstream development builds

set -e

# Configuration
MCH_NAMESPACE="${MCH_NAMESPACE:-open-cluster-management}"
MCH_NAME="${MCH_NAME:-multiclusterhub}"
REGISTRY_MIRROR="${REGISTRY_MIRROR:-quay.io:443/acm-d}"
PULL_SECRET_NAME="${PULL_SECRET_NAME:-multiclusterhub-operator-pull-secret}"
REGISTRY_CREDS_FILE="${REGISTRY_CREDS_FILE:-registry_creds.txt}"
PULL_SECRET_DATA_FILE="${PULL_SECRET_DATA_FILE:-pull_secret_data.txt}"

# Snapshot versions
ACM_SNAPSHOT="${ACM_SNAPSHOT:-latest-5.0}"
MCE_SNAPSHOT="${MCE_SNAPSHOT:-latest-5.0}"

echo "Installing ACM via OLM v1"
echo "ACM: $ACM_SNAPSHOT"
echo "MCE: $MCE_SNAPSHOT"
echo ""

#######################################
# Get or create registry credentials
#######################################
get_registry_creds() {
    local registry_host=$(echo "$REGISTRY_MIRROR" | cut -d'/' -f1)

    if [[ -f "$REGISTRY_CREDS_FILE" ]]; then
        echo "Found existing registry credentials: $REGISTRY_CREDS_FILE"
        read -p "Replace current credentials? (yes/no) [default: no]: " CONFIRM
        CONFIRM=${CONFIRM:-no}

        if [[ "$CONFIRM" =~ ^[Yy]([Ee][Ss])?$ ]]; then
            echo "Replacing existing credentials..."
            rm "$REGISTRY_CREDS_FILE"
            read -p "Enter username for $registry_host: " REGISTRY_USER
            read -sp "Enter password for $registry_host: " REGISTRY_PASS
            echo
            echo "$REGISTRY_USER:$REGISTRY_PASS" > "$REGISTRY_CREDS_FILE"
            echo "Credentials saved to $REGISTRY_CREDS_FILE"
        else
            echo "Using existing credentials from $REGISTRY_CREDS_FILE"
            local creds=$(cat "$REGISTRY_CREDS_FILE")
            REGISTRY_USER=$(echo "$creds" | cut -d':' -f1)
            REGISTRY_PASS=$(echo "$creds" | cut -d':' -f2-)
        fi
    else
        echo "No existing credentials found. Creating new ones..."
        read -p "Enter username for $registry_host: " REGISTRY_USER
        read -sp "Enter password for $registry_host: " REGISTRY_PASS
        echo
        echo "$REGISTRY_USER:$REGISTRY_PASS" > "$REGISTRY_CREDS_FILE"
        echo "Credentials saved to $REGISTRY_CREDS_FILE"
    fi
}

#######################################
# Setup global pull secret
#######################################
setup_global_pull_secret() {
    local registry_host=$(echo "$REGISTRY_MIRROR" | cut -d'/' -f1)

    echo "Updating global pull secret for $registry_host..."

    # Get registry credentials
    get_registry_creds

    # Get current pull secret
    oc get secret/pull-secret -n openshift-config --template='{{index .data ".dockerconfigjson" | base64decode}}' >pull_secret.yaml

    # Add registry credentials
    oc registry login --registry="$registry_host" --auth-basic="$REGISTRY_USER:$REGISTRY_PASS" --to=pull_secret.yaml
    oc set data secret/pull-secret -n openshift-config --from-file=.dockerconfigjson=pull_secret.yaml
    rm pull_secret.yaml

    echo "✓ Global pull secret updated"
}

#######################################
# Get or create pull secret data
#######################################
get_pull_secret_data() {
    if [[ -f "$PULL_SECRET_DATA_FILE" ]]; then
        echo "Found existing pull secret data file: $PULL_SECRET_DATA_FILE"
        read -p "Replace current value? (yes/no) [default: no]: " CONFIRM
        CONFIRM=${CONFIRM:-no}

        if [[ "$CONFIRM" =~ ^[Yy]([Ee][Ss])?$ ]]; then
            echo "Replacing existing pull secret data..."
            rm "$PULL_SECRET_DATA_FILE"
            echo ""
            echo "Enter .dockerconfigjson value from pull secret (base64 encoded):"
            echo "Example: kubectl get secret <name> -o jsonpath='{.data.\.dockerconfigjson}'"
            read -p "> " PULL_SECRET_DATA
            echo "$PULL_SECRET_DATA" > "$PULL_SECRET_DATA_FILE"
            echo "Pull secret data saved to $PULL_SECRET_DATA_FILE"
        else
            echo "Using existing pull secret data from $PULL_SECRET_DATA_FILE"
            PULL_SECRET_DATA=$(cat "$PULL_SECRET_DATA_FILE")
        fi
    else
        echo "No existing pull secret data file found. Creating new one..."
        echo ""
        echo "Enter .dockerconfigjson value from pull secret (base64 encoded):"
        echo "Example: kubectl get secret <name> -o jsonpath='{.data.\.dockerconfigjson}'"
        read -p "> " PULL_SECRET_DATA
        echo "$PULL_SECRET_DATA" > "$PULL_SECRET_DATA_FILE"
        echo "Pull secret data saved to $PULL_SECRET_DATA_FILE"
    fi

    export DOCKER_CONFIG="$PULL_SECRET_DATA"
    export QUAY_TOKEN=$(echo $DOCKER_CONFIG | base64 -d | sed "s/quay\.io/quay\.io:443/g" | base64)
}

#######################################
# Create ClusterCatalog
#######################################
create_catalog() {
    local name=$1
    local image=$2

    echo "Creating ClusterCatalog: $name"

    # Check if already serving before applying
    local existing_serving=$(kubectl get clustercatalog "$name" -o jsonpath='{.status.conditions[?(@.type=="Serving")].status}' 2>/dev/null || echo "")

    if [[ "$existing_serving" == "True" ]]; then
        echo "✓ Catalog already exists and serving"
        return 0
    fi

    kubectl apply -f - <<EOF
apiVersion: olm.operatorframework.io/v1
kind: ClusterCatalog
metadata:
  name: $name
spec:
  source:
    type: Image
    image:
      ref: $image
EOF

    # Give it a moment to reconcile
    echo "Waiting for catalog to serve..."
    sleep 10

    timeout=180
    elapsed=10

    while [[ $elapsed -lt $timeout ]]; do
        serving=$(kubectl get clustercatalog "$name" -o jsonpath='{.status.conditions[?(@.type=="Serving")].status}' 2>/dev/null || echo "")

        if [[ "$serving" == "True" ]]; then
            echo "✓ Catalog serving"
            return 0
        fi

        sleep 10
        ((elapsed+=10))
    done

    echo "⚠ Timeout waiting for catalog"
    return 1
}

#######################################
# Setup ImageContentSourcePolicy
#######################################
setup_icsp() {
    echo "Setting up ImageContentSourcePolicy..."

    kubectl apply -f - <<EOF
apiVersion: operator.openshift.io/v1alpha1
kind: ImageContentSourcePolicy
metadata:
  name: rhacm-repo
spec:
  repositoryDigestMirrors:
  - mirrors:
    - $REGISTRY_MIRROR
    source: registry.redhat.io/rhacm2
  - mirrors:
    - $REGISTRY_MIRROR
    source: registry.redhat.io/multicluster-engine
  - mirrors:
    - $REGISTRY_MIRROR
    source: registry.redhat.io/openshift4/ose-cluster-api-rhel9
  - mirrors:
    - $REGISTRY_MIRROR
    source: registry.redhat.io/openshift4/ose-aws-cluster-api-controllers-rhel9
  - mirrors:
    - registry.redhat.io/openshift4/ose-oauth-proxy
    source: registry.access.redhat.com/openshift4/ose-oauth-proxy
EOF

    echo "✓ ICSP created"
}

#######################################
# Create namespace and pull secret
#######################################
create_namespace_and_secret() {
    local namespace=$1

    # Create namespace
    if ! kubectl get namespace "$namespace" &>/dev/null; then
        echo "Creating namespace: $namespace"
        kubectl create namespace "$namespace"
        echo "✓ Namespace created"
    else
        echo "✓ Namespace exists: $namespace"
    fi

    # Create pull secret
    if kubectl get secret -n "$namespace" "$PULL_SECRET_NAME" &>/dev/null; then
        echo "✓ Pull secret exists: $PULL_SECRET_NAME"
    else
        echo "Creating pull secret from global config..."

        # Copy from openshift-config
        kubectl get secret pull-secret -n openshift-config -o yaml | \
            sed "s/namespace: openshift-config/namespace: $namespace/" | \
            sed "s/name: pull-secret/name: $PULL_SECRET_NAME/" | \
            kubectl apply -f -

        echo "✓ Pull secret created"
    fi
}

#######################################
# Main install
#######################################

# Setup registry and pull secrets
echo "Step 1: ICSP Setup"
setup_icsp
echo ""

echo "Step 2: Global Pull Secret"
setup_global_pull_secret
echo ""

echo "Step 3: Pull Secret Data"
get_pull_secret_data
echo ""

# Create ACM catalog
echo "Step 4: ACM ClusterCatalog"
create_catalog "acm-dev-catalog" "$REGISTRY_MIRROR/acm-dev-catalog:$ACM_SNAPSHOT"
echo ""

# Create MCE catalog
echo "Step 5: MCE ClusterCatalog"
create_catalog "mce-dev-catalog" "$REGISTRY_MIRROR/mce-dev-catalog:$MCE_SNAPSHOT"
echo ""

# Create namespace and pull secret
echo "Step 6: Namespace and Pull Secret"
create_namespace_and_secret "$MCH_NAMESPACE"
echo ""

# Create ServiceAccount
echo "Step 7: ServiceAccount"
kubectl apply -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: acm-installer
  namespace: $MCH_NAMESPACE
EOF
echo ""

# Create ClusterRoleBinding
echo "Step 8: ClusterRoleBinding"
kubectl apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: acm-installer-admin
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: acm-installer
  namespace: $MCH_NAMESPACE
EOF
echo ""

# Create ClusterExtension
echo "Step 9: ClusterExtension for ACM operator"
kubectl apply -f - <<EOF
apiVersion: olm.operatorframework.io/v1
kind: ClusterExtension
metadata:
  name: advanced-cluster-management
spec:
  namespace: $MCH_NAMESPACE
  serviceAccount:
    name: acm-installer
  source:
    sourceType: Catalog
    catalog:
      packageName: advanced-cluster-management
  config:
    configType: Inline
    inline:
      watchNamespace: $MCH_NAMESPACE
EOF
echo ""

# Wait for operator
echo "Waiting for ACM operator (max 5min)..."
timeout=300
elapsed=0

while [[ $elapsed -lt $timeout ]]; do
    installed=$(kubectl get clusterextension advanced-cluster-management -o jsonpath='{.status.conditions[?(@.type=="Installed")].status}' 2>/dev/null || echo "")

    if [[ "$installed" == "True" ]]; then
        echo "✓ ACM operator installed"
        break
    fi

    sleep 10
    ((elapsed+=10))
done

if [[ $elapsed -ge $timeout ]]; then
    echo "⚠ Timeout - check: kubectl get clusterextension advanced-cluster-management -o yaml"
    exit 1
fi

echo ""
echo "Waiting for webhook to be ready..."
sleep 30

# Create MultiClusterHub CR
echo "Step 10: MultiClusterHub CR"
kubectl apply -f - <<EOF
apiVersion: operator.open-cluster-management.io/v1
kind: MultiClusterHub
metadata:
  name: $MCH_NAME
  namespace: $MCH_NAMESPACE
spec:
  imagePullSecret: $PULL_SECRET_NAME
  availabilityConfig: High
EOF

echo ""
echo "✓ Installation started"
echo ""
echo "Monitor with:"
echo "  kubectl get multiclusterhub -n $MCH_NAMESPACE"
echo "  kubectl get clusterextension"
echo "  kubectl get pods -n $MCH_NAMESPACE"
echo "  kubectl get pods -n multicluster-engine"
```

---

## Running the Script

```bash
# Set snapshot versions (optional, defaults to latest-5.0)
export ACM_SNAPSHOT="5.0.0-DOWNSTREAM-2026-05-28-11-50-04"
export MCE_SNAPSHOT="5.0.0-DOWNSTREAM-2026-05-28-11-50-04"

# Run installation
chmod +x install-downstream-olmv1.sh
./install-downstream-olmv1.sh
```

Script prompts for registry credentials interactively and saves to `registry_creds.txt` for reuse.

---

## Monitoring Installation

```bash
# Check ClusterExtension status
kubectl get clusterextension advanced-cluster-management -o yaml

# Check MultiClusterHub status
kubectl get multiclusterhub -n open-cluster-management

# Check operator pods
kubectl get pods -n open-cluster-management
kubectl get pods -n multicluster-engine

# Check ClusterCatalog status
kubectl get clustercatalog
```

---

## Troubleshooting

### ClusterCatalog Not Serving

```bash
# Check catalog status
kubectl get clustercatalog acm-dev-catalog -o yaml

# Common issue: registry pull secret missing in olm namespace
kubectl get secret -n olm-operator-controller-system
```

### ClusterExtension Stuck in Progressing

```bash
# Check ClusterExtension conditions
kubectl get clusterextension advanced-cluster-management -o jsonpath='{.status.conditions}' | jq

# Check operator logs
kubectl logs -n olm-operator-controller-system deployment/catalogd-controller-manager
```

### ServiceAccount Missing Permissions

```bash
# Verify ClusterRoleBinding exists
kubectl get clusterrolebinding acm-installer-admin

# Check ServiceAccount
kubectl get sa -n open-cluster-management acm-installer
```

### ICSP Not Applied

ICSP requires node reboot. Check:

```bash
# Verify ICSP created
kubectl get imagecontentsourcepolicy rhacm-repo

# Check node status (should show SchedulingDisabled during reboot)
kubectl get nodes
```

---

## Cleanup

```bash
# Delete MultiClusterHub (triggers cleanup)
kubectl delete multiclusterhub multiclusterhub -n open-cluster-management

# Delete ClusterExtension
kubectl delete clusterextension advanced-cluster-management

# Delete ClusterCatalogs
kubectl delete clustercatalog acm-dev-catalog mce-dev-catalog

# Delete namespace
kubectl delete namespace open-cluster-management

# Delete ICSP (optional, triggers node reboot)
kubectl delete imagecontentsourcepolicy rhacm-repo

# Delete RBAC
kubectl delete clusterrolebinding acm-installer-admin
```

---

## Differences from Production Installation

| Aspect | Downstream (This Guide) | Production |
|--------|------------------------|------------|
| Catalogs | Custom ClusterCatalogs from snapshots | Red Hat official catalogs |
| Registry | Downstream mirror (quay.io:443/acm-d) | registry.redhat.io |
| ICSP | Required for registry mirror | Not needed |
| Versions | Snapshot builds (e.g., `latest-5.0`) | Released versions |
| Support | Development/testing only | Production supported |

---

## See Also

- [README.md](../README.md) - OLM v0 vs v1 annotation differences
- [CONTRIBUTING.md](../CONTRIBUTING.md) - Development workflow
- [OLM v1 Documentation](https://docs.openshift.com/container-platform/latest/operators/understanding/olm-packaging-format.html)
