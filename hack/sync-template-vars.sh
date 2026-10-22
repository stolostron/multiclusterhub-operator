#!/bin/bash
# sync-template-vars.sh
# Automatically syncs template variables with config.yaml escape-template-variables
# Scans all chart templates and ensures any {{UPPERCASE_VAR}} patterns are declared
# in the corresponding component's escape-template-variables list.
#
# Usage:
#   ./hack/sync-template-vars.sh [REGENERATE_TARGET]
#
# Arguments:
#   REGENERATE_TARGET - The make target to run for regeneration (default: regenerate-charts-from-bundles)
#                       Options: regenerate-charts-from-bundles, regenerate-charts, copy-charts

set -e

CHARTS_DIR="pkg/templates/charts/toggle"
NEEDS_REGENERATE=false
COMPONENTS_TO_REGENERATE=()

# Get regeneration target from argument or use default
REGENERATE_TARGET="${1:-regenerate-charts-from-bundles}"

# Determine which config file to use based on the regeneration target
case "$REGENERATE_TARGET" in
    regenerate-charts-from-bundles)
        CONFIG_FILE="hack/bundle-automation/config.yaml"
        CONFIG_TYPE="bundles"  # Uses .components[].operators[]
        ;;
    regenerate-charts)
        CONFIG_FILE="hack/bundle-automation/charts-config.yaml"
        CONFIG_TYPE="charts"   # Uses .components[].charts[]
        ;;
    copy-charts)
        CONFIG_FILE="hack/bundle-automation/copy-config.yaml"
        CONFIG_TYPE="copy"     # Uses root array with .charts[]
        ;;
    *)
        echo "Unknown regeneration target: $REGENERATE_TARGET"
        exit 1
        ;;
esac

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🔍 Scanning all component templates for variables...${NC}"
echo -e "${BLUE}   Regeneration target: ${REGENERATE_TARGET}${NC}"
echo -e "${BLUE}   Config file: ${CONFIG_FILE}${NC}"
echo -e "${BLUE}   Config type: ${CONFIG_TYPE}${NC}"

# Check if yq is installed
if ! command -v yq &> /dev/null; then
    echo -e "${RED}❌ Error: yq is required but not installed.${NC}"
    echo "Install with: brew install yq"
    exit 1
fi

# Function to process a single chart/operator
process_chart() {
    local chart_name="$1"
    local repo_name="$2"
    local yq_path="$3"  # yq path to this chart in config file

    # Find corresponding chart directory
    CHART_DIR="$CHARTS_DIR/$chart_name"

    if [ ! -d "$CHART_DIR/templates" ]; then
        echo -e "${YELLOW}  ⚠️  Chart directory not found: $CHART_DIR/templates (skipping)${NC}"
        return
    fi

    echo -e "  📁 Scanning charts for: ${chart_name}"

    # Find all {{UPPERCASE_VAR}} patterns in templates
    TEMPLATE_VARS=$(grep -rh "{{[A-Z_][A-Z_0-9]*}}" "$CHART_DIR/templates/" 2>/dev/null | \
        grep -oE '{{[A-Z_][A-Z_0-9]*}}' | \
        sed 's/[{}]//g' | \
        sort -u || echo "")

    if [ -z "$TEMPLATE_VARS" ]; then
        echo -e "${GREEN}  ✅ No uppercase variables found in templates${NC}"
        return
    fi

    # Get current escape-template-variables list
    CURRENT_VARS=$(yq "${yq_path}.escape-template-variables[]" "$CONFIG_FILE" 2>/dev/null | sort || echo "")

    # Check each variable
    ADDED_VARS=()
    for var in $TEMPLATE_VARS; do
        if ! echo "$CURRENT_VARS" | grep -q "^${var}$"; then
            echo -e "${YELLOW}  ➕ Adding missing variable: ${var}${NC}"

            # Check if escape-template-variables key exists
            HAS_KEY=$(yq "${yq_path} | has(\"escape-template-variables\")" "$CONFIG_FILE")

            if [ "$HAS_KEY" = "false" ]; then
                # Create the key with first variable
                yq -i "(${yq_path}.escape-template-variables) = [\"$var\"]" "$CONFIG_FILE"
            else
                # Append to existing list
                yq -i "(${yq_path}.escape-template-variables) += [\"$var\"]" "$CONFIG_FILE"
            fi

            ADDED_VARS+=("$var")
            COMPONENT_MODIFIED=true
            NEEDS_REGENERATE=true
        fi
    done

    if [ ${#ADDED_VARS[@]} -eq 0 ]; then
        echo -e "${GREEN}  ✅ All variables already declared${NC}"
    else
        echo -e "${GREEN}  ✓ Added ${#ADDED_VARS[@]} variable(s) to ${CONFIG_FILE}${NC}"
    fi
}

# Process based on config type
if [ "$CONFIG_TYPE" = "bundles" ]; then
    # config.yaml structure: .components[].operators[]
    COMPONENTS=$(yq '.components[].repo_name' "$CONFIG_FILE")

    for repo_name in $COMPONENTS; do
        echo -e "\n${BLUE}Processing component: ${repo_name}${NC}"
        COMPONENT_MODIFIED=false

        OPERATOR_COUNT=$(yq ".components[] | select(.repo_name == \"$repo_name\") | .operators | length" "$CONFIG_FILE")

        for ((i=0; i<OPERATOR_COUNT; i++)); do
            OPERATOR_NAME=$(yq ".components[] | select(.repo_name == \"$repo_name\") | .operators[$i].name" "$CONFIG_FILE")
            YQ_PATH=".components[] | select(.repo_name == \"$repo_name\") | .operators[$i]"

            process_chart "$OPERATOR_NAME" "$repo_name" "$YQ_PATH"

            if [ "$COMPONENT_MODIFIED" = true ]; then
                COMPONENTS_TO_REGENERATE+=("$OPERATOR_NAME")
            fi
        done
    done

elif [ "$CONFIG_TYPE" = "charts" ]; then
    # charts-config.yaml structure: .components[].charts[]
    COMPONENTS=$(yq '.components[].repo_name' "$CONFIG_FILE")

    for repo_name in $COMPONENTS; do
        echo -e "\n${BLUE}Processing component: ${repo_name}${NC}"
        COMPONENT_MODIFIED=false

        CHART_COUNT=$(yq ".components[] | select(.repo_name == \"$repo_name\") | .charts | length" "$CONFIG_FILE")

        for ((i=0; i<CHART_COUNT; i++)); do
            CHART_NAME=$(yq ".components[] | select(.repo_name == \"$repo_name\") | .charts[$i].name" "$CONFIG_FILE")
            YQ_PATH=".components[] | select(.repo_name == \"$repo_name\") | .charts[$i]"

            process_chart "$CHART_NAME" "$repo_name" "$YQ_PATH"

            if [ "$COMPONENT_MODIFIED" = true ]; then
                COMPONENTS_TO_REGENERATE+=("$CHART_NAME")
            fi
        done
    done

elif [ "$CONFIG_TYPE" = "copy" ]; then
    # copy-config.yaml structure: root array with .charts[]
    COMPONENT_COUNT=$(yq '. | length' "$CONFIG_FILE")

    for ((comp_idx=0; comp_idx<COMPONENT_COUNT; comp_idx++)); do
        REPO_NAME=$(yq ".[$comp_idx].repo_name" "$CONFIG_FILE")
        echo -e "\n${BLUE}Processing component: ${REPO_NAME}${NC}"
        COMPONENT_MODIFIED=false

        CHART_COUNT=$(yq ".[$comp_idx].charts | length" "$CONFIG_FILE")

        for ((i=0; i<CHART_COUNT; i++)); do
            CHART_NAME=$(yq ".[$comp_idx].charts[$i].name" "$CONFIG_FILE")
            YQ_PATH=".[$comp_idx].charts[$i]"

            process_chart "$CHART_NAME" "$REPO_NAME" "$YQ_PATH"

            if [ "$COMPONENT_MODIFIED" = true ]; then
                COMPONENTS_TO_REGENERATE+=("$CHART_NAME")
            fi
        done
    done
fi

echo ""
if [ "$NEEDS_REGENERATE" = true ]; then
    echo -e "${BLUE}🔄 Changes detected in ${#COMPONENTS_TO_REGENERATE[@]} component(s)${NC}"
    echo -e "${BLUE}   Components to regenerate: ${COMPONENTS_TO_REGENERATE[*]}${NC}"
    echo ""

    # Get unique component repo names for the operators that were modified
    for operator_name in "${COMPONENTS_TO_REGENERATE[@]}"; do
        # Find the repo_name for this operator
        repo_name=$(yq ".components[] | select(.operators[].name == \"$operator_name\") | .repo_name" "$CONFIG_FILE" | head -1)

        if [ -n "$repo_name" ]; then
            echo -e "${BLUE}   Regenerating component: ${repo_name} (operator: ${operator_name})${NC}"
            echo -e "${BLUE}   Running: COMPONENT=\"$operator_name\" make -f Makefile.dev ${REGENERATE_TARGET}${NC}"
            COMPONENT="$operator_name" make -f Makefile.dev "${REGENERATE_TARGET}"
        fi
    done

    echo ""
    echo -e "${GREEN}✅ Templates updated with proper variable escaping${NC}"
    echo -e "${BLUE}💡 You can now run 'go generate ./...' to verify${NC}"
else
    echo -e "${GREEN}✅ No changes needed - all variables already declared in config.yaml${NC}"
fi
