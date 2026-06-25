#!/bin/bash
# validate-template-vars.sh
# Validates that all template variables are declared in config files
# Returns exit code 1 if any variables are missing (for use in CI/pre-commit hooks)

set -e

CHARTS_DIR="pkg/templates/charts/toggle"
HAS_ERRORS=false

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🔍 Validating template variables against all config files...${NC}"

# Check if yq is installed
if ! command -v yq &> /dev/null; then
    echo -e "${RED}❌ Error: yq is required but not installed.${NC}"
    echo "Install with: brew install yq"
    exit 1
fi

# Validate each config file
validate_config() {
    local config_file="$1"
    local config_type="$2"
    local config_label="$3"

    echo -e "\n${BLUE}Checking ${config_label}...${NC}"

    if [ ! -f "$config_file" ]; then
        echo -e "${YELLOW}  ⚠️  Config file not found: ${config_file} (skipping)${NC}"
        return
    fi

    if [ "$config_type" = "bundles" ]; then
        # config.yaml: .components[].operators[]
        local components=$(yq '.components[].repo_name' "$config_file" 2>/dev/null || echo "")
        for repo_name in $components; do
            local op_count=$(yq ".components[] | select(.repo_name == \"$repo_name\") | .operators | length" "$config_file")
            for ((i=0; i<op_count; i++)); do
                local name=$(yq ".components[] | select(.repo_name == \"$repo_name\") | .operators[$i].name" "$config_file")
                local vars=$(yq ".components[] | select(.repo_name == \"$repo_name\") | .operators[$i].escape-template-variables[]" "$config_file" 2>/dev/null | sort || echo "")
                check_template_vars "$name" "$repo_name" "$vars" "$config_file"
            done
        done

    elif [ "$config_type" = "charts" ]; then
        # charts-config.yaml: .components[].charts[]
        local components=$(yq '.components[].repo_name' "$config_file" 2>/dev/null || echo "")
        for repo_name in $components; do
            local chart_count=$(yq ".components[] | select(.repo_name == \"$repo_name\") | .charts | length" "$config_file")
            for ((i=0; i<chart_count; i++)); do
                local name=$(yq ".components[] | select(.repo_name == \"$repo_name\") | .charts[$i].name" "$config_file")
                local vars=$(yq ".components[] | select(.repo_name == \"$repo_name\") | .charts[$i].escape-template-variables[]" "$config_file" 2>/dev/null | sort || echo "")
                check_template_vars "$name" "$repo_name" "$vars" "$config_file"
            done
        done

    elif [ "$config_type" = "copy" ]; then
        # copy-config.yaml: root array with .charts[]
        local comp_count=$(yq '. | length' "$config_file")
        for ((comp_idx=0; comp_idx<comp_count; comp_idx++)); do
            local repo_name=$(yq ".[$comp_idx].repo_name" "$config_file")
            local chart_count=$(yq ".[$comp_idx].charts | length" "$config_file")
            for ((i=0; i<chart_count; i++)); do
                local name=$(yq ".[$comp_idx].charts[$i].name" "$config_file")
                local vars=$(yq ".[$comp_idx].charts[$i].escape-template-variables[]" "$config_file" 2>/dev/null | sort || echo "")
                check_template_vars "$name" "$repo_name" "$vars" "$config_file"
            done
        done
    fi
}

check_template_vars() {
    local name="$1"
    local repo_name="$2"
    local current_vars="$3"
    local config_file="$4"

    local chart_dir="$CHARTS_DIR/$name"

    if [ ! -d "$chart_dir/templates" ]; then
        return
    fi

    # Find all {{UPPERCASE_VAR}} patterns
    local template_vars=$(grep -rh "{{[A-Z_][A-Z_0-9]*}}" "$chart_dir/templates/" 2>/dev/null | \
        grep -oE '{{[A-Z_][A-Z_0-9]*}}' | \
        sed 's/[{}]//g' | \
        sort -u || echo "")

    if [ -z "$template_vars" ]; then
        return
    fi

    # Check each variable
    local missing_vars=()
    for var in $template_vars; do
        if ! echo "$current_vars" | grep -q "^${var}$"; then
            missing_vars+=("$var")
        fi
    done

    if [ ${#missing_vars[@]} -gt 0 ]; then
        echo -e "${RED}  ❌ Component: ${repo_name} / Chart: ${name}${NC}"
        echo -e "${RED}     Missing in ${config_file}:${NC}"
        printf "${YELLOW}     - %s${NC}\n" "${missing_vars[@]}"
        HAS_ERRORS=true
    fi
}

# Validate all three config files
validate_config "hack/bundle-automation/config.yaml" "bundles" "config.yaml (bundles)"
validate_config "hack/bundle-automation/charts-config.yaml" "charts" "charts-config.yaml"
validate_config "hack/bundle-automation/copy-config.yaml" "copy" "copy-config.yaml"

echo ""
if [ "$HAS_ERRORS" = true ]; then
    echo -e "${RED}❌ Validation failed!${NC}"
    echo -e "${BLUE}💡 Run sync scripts to fix:${NC}"
    echo -e "${BLUE}   - ./hack/sync-template-vars.sh regenerate-charts-from-bundles${NC}"
    echo -e "${BLUE}   - ./hack/sync-template-vars.sh regenerate-charts${NC}"
    echo -e "${BLUE}   - ./hack/sync-template-vars.sh copy-charts${NC}"
    exit 1
else
    echo -e "${GREEN}✅ All template variables are properly declared in all config files${NC}"
    exit 0
fi
