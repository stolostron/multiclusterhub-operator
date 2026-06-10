#!/usr/bin/env bash

# Copyright Contributors to the Open Cluster Management project

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
MCH_NAMESPACE="${MCH_NAMESPACE:-open-cluster-management}"
MCH_NAME="${MCH_NAME:-multiclusterhub}"
PULL_SECRET_NAME="${PULL_SECRET_NAME:-pull-secret}"
SKIP_PULL_SECRET="${SKIP_PULL_SECRET:-false}"

# Script mode: detect, install, verify, uninstall
MODE="${1:-detect}"

#######################################
# Print colored message
# Arguments:
#   $1 - Color (RED, GREEN, YELLOW, BLUE)
#   $2 - Message
#######################################
print_msg() {
    local color=$1
    shift
    echo -e "${!color}$*${NC}"
}

#######################################
# Detect OLM version on cluster
# Returns:
#   "v0" if OLM v0 detected
#   "v1" if OLM v1 detected
#   "" if no OLM detected
#######################################
detect_olm_version() {
    local olm_version=""

    # Check for OLM v1 (ClusterExtension CRD)
    if kubectl get crd clusterextensions.olm.operatorframework.io &>/dev/null; then
        olm_version="v1"
    # Check for OLM v0 (CSV CRD)
    elif kubectl get crd clusterserviceversions.operators.coreos.com &>/dev/null; then
        olm_version="v0"
    fi

    echo "$olm_version"
}

#######################################
# Check prerequisites for OLM v0
#######################################
check_prerequisites_v0() {
    print_msg BLUE "Checking OLM v0 prerequisites..."

    local errors=0

    # Check for CatalogSource CRD
    if ! kubectl get crd catalogsources.operators.coreos.com &>/dev/null; then
        print_msg RED "✗ CatalogSource CRD not found"
        ((errors++))
    else
        print_msg GREEN "✓ CatalogSource CRD exists"
    fi

    # Check for available catalogs
    if ! kubectl get catalogsource -n openshift-marketplace &>/dev/null; then
        print_msg YELLOW "⚠ No CatalogSources found in openshift-marketplace"
        print_msg YELLOW "  Custom catalog may be needed"
    else
        local catalog_count
        catalog_count=$(kubectl get catalogsource -n openshift-marketplace --no-headers 2>/dev/null | wc -l)
        print_msg GREEN "✓ Found $catalog_count CatalogSource(s)"

        # List catalogs
        kubectl get catalogsource -n openshift-marketplace -o custom-columns=NAME:.metadata.name,DISPLAY:.spec.displayName,PUBLISHER:.spec.publisher --no-headers 2>/dev/null | while read -r line; do
            echo "    $line"
        done
    fi

    # Check OLM operator
    if ! kubectl get deployment -n openshift-operator-lifecycle-manager olm-operator &>/dev/null; then
        print_msg RED "✗ OLM operator not found"
        ((errors++))
    else
        print_msg GREEN "✓ OLM operator running"
    fi

    return $errors
}

#######################################
# Check prerequisites for OLM v1
#######################################
check_prerequisites_v1() {
    print_msg BLUE "Checking OLM v1 prerequisites..."

    local errors=0

    # Check for ClusterCatalog CRD
    if ! kubectl get crd clustercatalogs.olm.operatorframework.io &>/dev/null; then
        print_msg RED "✗ ClusterCatalog CRD not found"
        ((errors++))
    else
        print_msg GREEN "✓ ClusterCatalog CRD exists"
    fi

    # Check for serving ClusterCatalogs
    if ! kubectl get clustercatalog &>/dev/null; then
        print_msg YELLOW "⚠ No ClusterCatalogs found"
        print_msg YELLOW "  Custom catalog may be needed"
    else
        local total_catalogs
        local serving_catalogs
        total_catalogs=$(kubectl get clustercatalog --no-headers 2>/dev/null | wc -l)
        serving_catalogs=$(kubectl get clustercatalog -o json 2>/dev/null | \
            jq -r '.items[] | select(.status.conditions[]? | select(.type=="Serving" and .status=="True")) | .metadata.name' | wc -l)

        print_msg GREEN "✓ Found $total_catalogs ClusterCatalog(s), $serving_catalogs serving"

        # List catalogs with serving status
        kubectl get clustercatalog -o custom-columns=NAME:.metadata.name,PRIORITY:.spec.priority --no-headers 2>/dev/null | while read -r line; do
            echo "    $line"
        done
    fi

    # Check OLM v1 operator controller
    if ! kubectl get deployment -n olm-operator-controller-system operator-controller-controller-manager &>/dev/null; then
        print_msg RED "✗ OLM v1 operator controller not found"
        ((errors++))
    else
        print_msg GREEN "✓ OLM v1 operator controller running"
    fi

    return $errors
}

#######################################
# Detect and display OLM version
#######################################
detect_mode() {
    print_msg BLUE "=== OLM Version Detection ==="
    echo

    local olm_version
    olm_version=$(detect_olm_version)

    if [[ -z "$olm_version" ]]; then
        print_msg RED "No OLM detected on this cluster"
        print_msg YELLOW "MultiClusterHub can still be installed (standalone mode)"
        print_msg YELLOW "MultiClusterEngine will be installed via direct CR creation"
        return 1
    elif [[ "$olm_version" == "v0" ]]; then
        print_msg GREEN "OLM v0 detected (OpenShift 4.x)"
        echo
        check_prerequisites_v0 || true
        echo
        print_msg BLUE "Installation will use:"
        echo "  • Subscription (namespaced)"
        echo "  • ClusterServiceVersion (CSV)"
        echo "  • InstallPlan"
        echo "  • OperatorGroup"
    elif [[ "$olm_version" == "v1" ]]; then
        print_msg GREEN "OLM v1 detected (OpenShift 5.x+)"
        echo
        check_prerequisites_v1 || true
        echo
        print_msg BLUE "Installation will use:"
        echo "  • ClusterExtension (cluster-scoped)"
        echo "  • ServiceAccount (mce-installer)"
        echo "  • ClusterRoleBinding (mce-installer-admin)"
    fi

    echo
    print_msg BLUE "To install MultiClusterHub:"
    echo "  $0 install"
    echo
    print_msg BLUE "To verify existing installation:"
    echo "  $0 verify"
}

#######################################
# Create pull secret
#######################################
create_pull_secret() {
    local namespace=$1

    if [[ "$SKIP_PULL_SECRET" == "true" ]]; then
        print_msg YELLOW "⊘ Skipping pull secret creation (SKIP_PULL_SECRET=true)"
        return 0
    fi

    if kubectl get secret -n "$namespace" "$PULL_SECRET_NAME" &>/dev/null; then
        print_msg GREEN "✓ Pull secret already exists: $PULL_SECRET_NAME"
        return 0
    fi

    print_msg YELLOW "Pull secret not found: $PULL_SECRET_NAME"
    print_msg BLUE "Create pull secret using one of:"
    echo
    echo "  # From dockercfg file:"
    echo "  kubectl create secret generic $PULL_SECRET_NAME \\"
    echo "    --from-file=.dockerconfigjson=\$HOME/.docker/config.json \\"
    echo "    --type=kubernetes.io/dockerconfigjson \\"
    echo "    -n $namespace"
    echo
    echo "  # From credentials:"
    echo "  kubectl create secret docker-registry $PULL_SECRET_NAME \\"
    echo "    --docker-server=registry.example.com \\"
    echo "    --docker-username=myuser \\"
    echo "    --docker-password=mypassword \\"
    echo "    --docker-email=myuser@example.com \\"
    echo "    -n $namespace"
    echo
    echo "  # Or set SKIP_PULL_SECRET=true to skip this step"
    echo
    return 1
}

#######################################
# Install MultiClusterHub
#######################################
install_mode() {
    print_msg BLUE "=== MultiClusterHub Installation ==="
    echo

    local olm_version
    olm_version=$(detect_olm_version)

    if [[ -n "$olm_version" ]]; then
        print_msg GREEN "OLM $olm_version detected"
    else
        print_msg YELLOW "No OLM detected (standalone installation)"
    fi

    # Check if already installed
    if kubectl get multiclusterhub -n "$MCH_NAMESPACE" "$MCH_NAME" &>/dev/null; then
        print_msg YELLOW "MultiClusterHub already exists: $MCH_NAMESPACE/$MCH_NAME"
        print_msg BLUE "To verify installation: $0 verify"
        return 1
    fi

    # Create namespace
    if ! kubectl get namespace "$MCH_NAMESPACE" &>/dev/null; then
        print_msg BLUE "Creating namespace: $MCH_NAMESPACE"
        kubectl create namespace "$MCH_NAMESPACE"
        print_msg GREEN "✓ Namespace created"
    else
        print_msg GREEN "✓ Namespace exists: $MCH_NAMESPACE"
    fi

    # Create or check pull secret
    if ! create_pull_secret "$MCH_NAMESPACE"; then
        return 1
    fi

    # Create MultiClusterHub CR
    print_msg BLUE "Creating MultiClusterHub CR..."

    local pull_secret_spec=""
    if [[ "$SKIP_PULL_SECRET" != "true" ]]; then
        pull_secret_spec="imagePullSecret: $PULL_SECRET_NAME"
    fi

    kubectl apply -f - <<EOF
apiVersion: operator.open-cluster-management.io/v1
kind: MultiClusterHub
metadata:
  name: $MCH_NAME
  namespace: $MCH_NAMESPACE
spec:
  $pull_secret_spec
  availabilityConfig: High
EOF

    print_msg GREEN "✓ MultiClusterHub CR created"
    echo

    # Show what will happen
    if [[ "$olm_version" == "v0" ]]; then
        print_msg BLUE "Operator will create (OLM v0):"
        echo "  1. Namespace: multicluster-engine"
        echo "  2. Pull secret (if configured)"
        echo "  3. OperatorGroup"
        echo "  4. Subscription"
        echo "  → OLM creates: InstallPlan, CSV, Operator deployment"
    elif [[ "$olm_version" == "v1" ]]; then
        print_msg BLUE "Operator will create (OLM v1):"
        echo "  1. Namespace: multicluster-engine"
        echo "  2. Pull secret (if configured)"
        echo "  3. ServiceAccount: mce-installer"
        echo "  4. ClusterRoleBinding: mce-installer-admin"
        echo "  5. ClusterExtension: multicluster-engine"
        echo "  → OLM creates: Operator deployment"
    else
        print_msg BLUE "Operator will create (standalone):"
        echo "  1. Namespace: multicluster-engine"
        echo "  2. Pull secret (if configured)"
        echo "  3. MultiClusterEngine CR directly"
    fi

    echo
    print_msg BLUE "Monitor installation:"
    echo "  kubectl get multiclusterhub -n $MCH_NAMESPACE $MCH_NAME -o yaml"
    echo "  kubectl get pods -n $MCH_NAMESPACE"
    echo "  kubectl get pods -n multicluster-engine"
    echo
    print_msg BLUE "Verify installation:"
    echo "  $0 verify"
}

#######################################
# Verify MultiClusterHub installation
#######################################
verify_mode() {
    print_msg BLUE "=== MultiClusterHub Verification ==="
    echo

    local olm_version
    olm_version=$(detect_olm_version)

    # Check MCH exists
    if ! kubectl get multiclusterhub -n "$MCH_NAMESPACE" "$MCH_NAME" &>/dev/null; then
        print_msg RED "✗ MultiClusterHub not found: $MCH_NAMESPACE/$MCH_NAME"
        print_msg BLUE "To install: $0 install"
        return 1
    fi

    print_msg GREEN "✓ MultiClusterHub exists: $MCH_NAMESPACE/$MCH_NAME"

    # Check phase
    local phase
    phase=$(kubectl get multiclusterhub -n "$MCH_NAMESPACE" "$MCH_NAME" -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")

    if [[ "$phase" == "Running" ]]; then
        print_msg GREEN "✓ Phase: $phase"
    else
        print_msg YELLOW "⚠ Phase: $phase"
    fi

    echo

    # OLM-specific verification
    if [[ "$olm_version" == "v0" ]]; then
        print_msg BLUE "OLM v0 Resources:"

        # Check Subscription
        if kubectl get subscription -n multicluster-engine multicluster-engine &>/dev/null; then
            local sub_state
            sub_state=$(kubectl get subscription -n multicluster-engine multicluster-engine -o jsonpath='{.status.state}' 2>/dev/null || echo "Unknown")
            if [[ "$sub_state" == "AtLatestKnown" ]]; then
                print_msg GREEN "  ✓ Subscription: $sub_state"
            else
                print_msg YELLOW "  ⚠ Subscription: $sub_state"
            fi
        else
            print_msg RED "  ✗ Subscription not found"
        fi

        # Check CSV
        if kubectl get csv -n multicluster-engine -l operators.coreos.com/multicluster-engine.multicluster-engine="" &>/dev/null; then
            local csv_name
            local csv_phase
            csv_name=$(kubectl get csv -n multicluster-engine -l operators.coreos.com/multicluster-engine.multicluster-engine="" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
            csv_phase=$(kubectl get csv -n multicluster-engine "$csv_name" -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
            if [[ "$csv_phase" == "Succeeded" ]]; then
                print_msg GREEN "  ✓ CSV: $csv_name ($csv_phase)"
            else
                print_msg YELLOW "  ⚠ CSV: $csv_name ($csv_phase)"
            fi
        else
            print_msg RED "  ✗ CSV not found"
        fi

    elif [[ "$olm_version" == "v1" ]]; then
        print_msg BLUE "OLM v1 Resources:"

        # Check ClusterExtension
        if kubectl get clusterextension multicluster-engine &>/dev/null; then
            local ce_installed
            ce_installed=$(kubectl get clusterextension multicluster-engine -o jsonpath='{.status.conditions[?(@.type=="Installed")].status}' 2>/dev/null || echo "Unknown")
            if [[ "$ce_installed" == "True" ]]; then
                print_msg GREEN "  ✓ ClusterExtension: Installed"
            else
                print_msg YELLOW "  ⚠ ClusterExtension: $ce_installed"
            fi
        else
            print_msg RED "  ✗ ClusterExtension not found"
        fi

        # Check ServiceAccount
        if kubectl get sa -n multicluster-engine mce-installer &>/dev/null; then
            print_msg GREEN "  ✓ ServiceAccount: mce-installer"
        else
            print_msg YELLOW "  ⚠ ServiceAccount not found"
        fi

        # Check ClusterRoleBinding
        if kubectl get clusterrolebinding mce-installer-admin &>/dev/null; then
            print_msg GREEN "  ✓ ClusterRoleBinding: mce-installer-admin"
        else
            print_msg YELLOW "  ⚠ ClusterRoleBinding not found"
        fi
    fi

    echo

    # Check MultiClusterEngine
    print_msg BLUE "MultiClusterEngine:"
    if kubectl get multiclusterengine multicluster-engine &>/dev/null; then
        local mce_phase
        mce_phase=$(kubectl get multiclusterengine multicluster-engine -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
        if [[ "$mce_phase" == "Available" ]]; then
            print_msg GREEN "  ✓ MultiClusterEngine: $mce_phase"
        else
            print_msg YELLOW "  ⚠ MultiClusterEngine: $mce_phase"
        fi
    else
        print_msg RED "  ✗ MultiClusterEngine not found"
    fi

    echo

    # Check operator pods
    print_msg BLUE "Operator Pods:"

    # MCH operator
    local mch_pods
    mch_pods=$(kubectl get pods -n "$MCH_NAMESPACE" -l name=multiclusterhub-operator --no-headers 2>/dev/null | wc -l)
    if [[ "$mch_pods" -gt 0 ]]; then
        print_msg GREEN "  ✓ MultiClusterHub operator: $mch_pods pod(s)"
    else
        print_msg RED "  ✗ MultiClusterHub operator: no pods"
    fi

    # MCE operator
    local mce_pods
    mce_pods=$(kubectl get pods -n multicluster-engine -l control-plane=backplane-operator --no-headers 2>/dev/null | wc -l)
    if [[ "$mce_pods" -gt 0 ]]; then
        print_msg GREEN "  ✓ MultiClusterEngine operator: $mce_pods pod(s)"
    else
        print_msg YELLOW "  ⚠ MultiClusterEngine operator: no pods"
    fi

    echo

    # Show component status
    print_msg BLUE "Component Status:"
    kubectl get multiclusterhub -n "$MCH_NAMESPACE" "$MCH_NAME" -o jsonpath='{range .status.components[*]}{.name}{"\t"}{.kind}{"\t"}{.status}{"\n"}{end}' 2>/dev/null | column -t -s $'\t' | while read -r line; do
        echo "  $line"
    done

    echo

    if [[ "$phase" == "Running" ]]; then
        print_msg GREEN "Installation complete and healthy"
    else
        print_msg YELLOW "Installation in progress or degraded"
        print_msg BLUE "Check logs:"
        echo "  kubectl logs -n $MCH_NAMESPACE deployment/multiclusterhub-operator"
        echo "  kubectl logs -n multicluster-engine deployment/backplane-operator"
    fi
}

#######################################
# Uninstall MultiClusterHub
#######################################
uninstall_mode() {
    print_msg BLUE "=== MultiClusterHub Uninstallation ==="
    echo

    if ! kubectl get multiclusterhub -n "$MCH_NAMESPACE" "$MCH_NAME" &>/dev/null; then
        print_msg YELLOW "MultiClusterHub not found: $MCH_NAMESPACE/$MCH_NAME"
        return 0
    fi

    print_msg YELLOW "This will delete MultiClusterHub: $MCH_NAMESPACE/$MCH_NAME"
    print_msg YELLOW "The operator will automatically clean up:"
    echo "  • MultiClusterEngine CR"
    echo "  • Subscription/ClusterExtension (depending on OLM version)"
    echo "  • ServiceAccount/ClusterRoleBinding (OLM v1)"
    echo "  • Pull secrets"
    echo

    read -rp "Continue? (yes/no): " confirm
    if [[ "$confirm" != "yes" ]]; then
        print_msg BLUE "Cancelled"
        return 1
    fi

    print_msg BLUE "Deleting MultiClusterHub..."
    kubectl delete multiclusterhub -n "$MCH_NAMESPACE" "$MCH_NAME"

    print_msg GREEN "✓ Deletion initiated"
    print_msg BLUE "Monitor cleanup:"
    echo "  kubectl get multiclusterhub -n $MCH_NAMESPACE"
    echo "  kubectl get subscription -n multicluster-engine"
    echo "  kubectl get clusterextension"
}

#######################################
# Show usage
#######################################
usage() {
    cat <<EOF
MultiClusterHub Installation Helper

USAGE:
  $0 [MODE]

MODES:
  detect      Detect OLM version and show prerequisites (default)
  install     Install MultiClusterHub
  verify      Verify existing MultiClusterHub installation
  uninstall   Uninstall MultiClusterHub

ENVIRONMENT VARIABLES:
  MCH_NAMESPACE       Namespace for MultiClusterHub (default: open-cluster-management)
  MCH_NAME            Name of MultiClusterHub CR (default: multiclusterhub)
  PULL_SECRET_NAME    Name of pull secret (default: pull-secret)
  SKIP_PULL_SECRET    Skip pull secret validation (default: false)

EXAMPLES:
  # Detect OLM version
  $0 detect

  # Install with custom namespace
  MCH_NAMESPACE=my-namespace $0 install

  # Install without pull secret
  SKIP_PULL_SECRET=true $0 install

  # Verify installation
  $0 verify

  # Uninstall
  $0 uninstall

EOF
}

#######################################
# Main
#######################################
main() {
    case "$MODE" in
        detect)
            detect_mode
            ;;
        install)
            install_mode
            ;;
        verify)
            verify_mode
            ;;
        uninstall)
            uninstall_mode
            ;;
        help|--help|-h)
            usage
            ;;
        *)
            print_msg RED "Unknown mode: $MODE"
            echo
            usage
            exit 1
            ;;
    esac
}

main "$@"
