#!/bin/bash

export CONFIG_FILE=onboard-request.yaml
yq e '.components | keys | .[]' "$CONFIG_FILE" | while read -r c_index; do
    echo "Component: $c_index"
    component_yaml=$(yq e ".components[$c_index]" $CONFIG_FILE)
    
    echo "$component_yaml" | yq e '.operators | keys | .[]' - | while read -r o_index; do
        OPERATOR_NAME=$(echo "$component_yaml" | yq e ".operators[$o_index].name" -)
        OPERATOR_DESCRIPTION=$(echo "$component_yaml" | yq e ".operators[$o_index].description" -)
        OPERATOR_ENABLED=$(echo "$component_yaml" | yq e ".operators[$o_index].enabled-by-default" -)
        
        echo "OPERATOR_NAME=$OPERATOR_NAME"
        echo "OPERATOR_DESCRIPTION=$OPERATOR_DESCRIPTION"
        echo "OPERATOR_ENABLED=$OPERATOR_ENABLED"
    done
done