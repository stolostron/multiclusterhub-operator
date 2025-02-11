CONFIG_FILE="onboard-request.yaml"

# Iterate over each component
yq e '.components | keys | .[]' "$CONFIG_FILE" | while read -r c_index; do
    echo "Component: $c_index"

    # Extract the full component YAML once
    component_yaml=$(yq e ".components[$c_index]" "$CONFIG_FILE")

    # Extract component-level details
    REPO_NAME=$(echo "$component_yaml" | yq e '.repo_name' -)
    GITHUB_REF=$(echo "$component_yaml" | yq e '.github_ref' -)
    BRANCH=$(echo "$component_yaml" | yq e '.branch' -)

    echo "  Repo Name: $REPO_NAME"
    echo "  GitHub Ref: $GITHUB_REF"
    echo "  Branch: $BRANCH"

    # Extract operator keys safely
    echo "$component_yaml" | yq e '.operators | keys | .[]' - | while read -r o_index; do
        # Extract operator YAML
        operator_yaml=$(echo "$component_yaml" | yq e ".operators[$o_index]" -)

        OPERATOR_NAME=$(echo "$operator_yaml" | yq e '.name' -)
        OPERATOR_DESCRIPTION=$(echo "$operator_yaml" | yq e '.description // "No description available"' -)
        BUNDLE_PATH=$(echo "$operator_yaml" | yq e '.bundlePath' -)
        ENABLED_BY_DEFAULT=$(echo "$operator_yaml" | yq e '.enabled-by-default // false' -)

        echo "  Operator: $OPERATOR_NAME"
        echo "    Description: $OPERATOR_DESCRIPTION"
        echo "    Bundle Path: $BUNDLE_PATH"
        echo "    Enabled by Default: $ENABLED_BY_DEFAULT"

        # Handle imageMappings safely
        echo "    Image Mappings:"
        echo "$operator_yaml" | yq e '.imageMappings | to_entries | .[] | "      \(.key): \(.value)"' - || echo "      None"

        # Handle escape-template-variables safely
        echo "    Escape Template Variables:"
        echo "$operator_yaml" | yq e '.escape-template-variables[]' - 2>/dev/null | sed 's/^/      - /' || echo "      None"

        # Handle exclusions safely
        echo "    Exclusions:"
        echo "$operator_yaml" | yq e '.exclusions[]' - 2>/dev/null | sed 's/^/      - /' || echo "      None"
    done
done
