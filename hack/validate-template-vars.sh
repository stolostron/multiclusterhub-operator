#!/bin/bash
# validate-template-vars.sh
# Validates that all template variables are declared in config.yaml
# Returns exit code 1 if any variables are missing (for use in CI/pre-commit hooks)

set -e

CONFIG_FILE="hack/bundle-automation/config.yaml"
CHARTS_DIR="pkg/templates/charts/toggle"
HAS_ERRORS=false

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🔍 Validating template variables against config.yaml...${NC}"

# Check if yq is installed
if ! command -v yq &> /dev/null; then
    echo -e "${RED}❌ Error: yq is required but not installed.${NC}"
    echo "Install with: brew install yq"
    exit 1
fi

# Get all components from config.yaml
COMPONENTS=$(yq '.components[].repo_name' "$CONFIG_FILE")

for repo_name in $COMPONENTS; do
    # Get all operators for this component
    OPERATOR_COUNT=$(yq ".components[] | select(.repo_name == \"$repo_name\") | .operators | length" "$CONFIG_FILE")

    for ((i=0; i<OPERATOR_COUNT; i++)); do
        OPERATOR_NAME=$(yq ".components[] | select(.repo_name == \"$repo_name\") | .operators[$i].name" "$CONFIG_FILE")

        # Find corresponding chart directory
        CHART_DIR="$CHARTS_DIR/$OPERATOR_NAME"

        if [ ! -d "$CHART_DIR/templates" ]; then
            continue
        fi

        # Find all {{UPPERCASE_VAR}} patterns in templates
        TEMPLATE_VARS=$(grep -rh "{{[A-Z_][A-Z_0-9]*}}" "$CHART_DIR/templates/" 2>/dev/null | \
            grep -oE '{{[A-Z_][A-Z_0-9]*}}' | \
            sed 's/[{}]//g' | \
            sort -u || echo "")

        if [ -z "$TEMPLATE_VARS" ]; then
            continue
        fi

        # Get current escape-template-variables list
        CURRENT_VARS=$(yq ".components[] | select(.repo_name == \"$repo_name\") | .operators[$i].escape-template-variables[]" "$CONFIG_FILE" 2>/dev/null | sort || echo "")

        # Check each variable
        MISSING_VARS=()
        for var in $TEMPLATE_VARS; do
            if ! echo "$CURRENT_VARS" | grep -q "^${var}$"; then
                MISSING_VARS+=("$var")
            fi
        done

        if [ ${#MISSING_VARS[@]} -gt 0 ]; then
            echo -e "${RED}❌ Component: ${repo_name} / Operator: ${OPERATOR_NAME}${NC}"
            echo -e "${RED}   Missing variables in escape-template-variables:${NC}"
            printf "${YELLOW}   - %s${NC}\n" "${MISSING_VARS[@]}"
            HAS_ERRORS=true
        fi
    done
done

echo ""
if [ "$HAS_ERRORS" = true ]; then
    echo -e "${RED}❌ Validation failed!${NC}"
    echo -e "${BLUE}💡 Run './hack/sync-template-vars.sh' to automatically fix${NC}"
    exit 1
else
    echo -e "${GREEN}✅ All template variables are properly declared in config.yaml${NC}"
    exit 0
fi
