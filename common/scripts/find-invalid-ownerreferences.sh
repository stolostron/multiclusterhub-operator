#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project


namespaces=$(oc get ns | grep "open-cluster-management\|local-cluster\|kube-system" | awk '{print $1}')

ownerReferenceIsValid() {
    local APIgroup=$1
    local Kind=$2
    local resource=$3
    local namespaceScoped=$4
    local namespace=$5

    # If resource is clusterwide, owner must be clusterwide as well
    local apiresource=""
    if [[ "$namespaceScoped" == "true" ]]; then
        apiresource=$(oc api-resources --api-group=$APIgroup | grep $Kind)
    else
        apiresource=$(oc api-resources --api-group=$APIgroup | grep $namespaceScoped | grep $Kind)
    fi
    
    if [ -z "${apiresource}" ]; then
        return 1 # OwnerReference is invalid
    else
        if [[ "$namespaceScoped" == "true" ]]; then
            output=$(oc get $resource -n $namespace)
        elif [[ "$namespaceScoped" == "false" ]]; then
            output=$(oc get $resource)
        else
            return 1
        fi

        if [ $? -eq 0 ]; then
                return 0
        else
            return 1
        fi
    fi
}

# Check if ownerref exists. If exists, then validate it
resourceIsValid() {
    local resource=$1
    local namespaceScoped=$2
    local namespace=$3

    if [[ "$namespaceScoped" == "true" ]]; then
        result=$(oc get $resource -n $namespace -o yaml | yq r - metadata.ownerReferences)
        if [ -z "${result}" ]; then 
            return 0         # OwnerReference does not exist
        else
            local APIgroup=$(oc get -n $namespace $resource -o yaml | yq r - "metadata.ownerReferences.[0].apiVersion" | cut -f1 -d"/")
            if [[ "$APIgroup" == "v1" ]]; then
                APIgroup=""
            fi
            Kind=$(oc get -n $namespace $resource -o yaml | yq r - "metadata.ownerReferences.[0].kind")
            Name=$(oc get -n $namespace $resource -o yaml | yq r - "metadata.ownerReferences.[0].name")
            if ownerReferenceIsValid "$APIgroup" "$Kind" "$resource" "$namespaceScoped" "$namespace"; then
                return 0
            fi
            return 1        # OwnerReference exists
        fi
    else
        local result=$(oc get $resource -o yaml | yq r - metadata.ownerReferences)
        if [ -z "${result}" ]; then 
            return 0         # OwnerReference does not exist
        else
            local APIgroup=$(oc get $resource -o yaml | yq r - "metadata.ownerReferences.[0].apiVersion" | cut -f1 -d"/")
            if [[ "$APIgroup" == "v1" ]]; then
                APIgroup=""
            fi
            local Kind=$(oc get $resource -o yaml | yq r - "metadata.ownerReferences.[0].kind")
            local Name=$(oc get $resource -o yaml | yq r - "metadata.ownerReferences.[0].name")
            if ownerReferenceIsValid "$APIgroup" "$Kind" "$resource" $namespaceScoped; then
                return 0
            fi
            return 1        # OwnerReference exists
        fi
    fi
    
}

# When given a resource definition, loop through all of its kind
validateResources() {
    local resourceType=$1
    local namespaceScoped=$2

    if [[ "$resourceType" == "events" ]]; then
        return
    fi

    if [[ "$namespaceScoped" == "true" ]]; then
        while IFS= read -r namespace; do
            oc get $resourceType -n $namespace > /dev/null 2>&1
            if [ ! $? -eq 0 ]; then
                continue
            fi
            local resources=$(oc get $resourceType -n $namespace | tail -n +2 | awk '{print $1}')
            if [ -z "${resources}" ]; then
                continue
            fi

            while IFS= read -r resourceName; do
                echo "- Validating - $resourceType/$resourceName --namespace $namespace..."
                if ! resourceIsValid "$resourceType/$resourceName" "$namespaceScoped" "$namespace"; then
                    echo ">>> $resourceType/$resourceName contains an improper ownerReference"
                    echo "$resourceType/$resourceName --namespace $namespace" >> improper-owner-references.txt
                fi
            done <<< "$resources"
        done <<< "$namespaces"
    else
        oc get $resourceType > /dev/null 2>&1
        if [ ! $? -eq 0 ]; then
            return
        fi

        local resources=$(oc get $resourceType | tail -n +2 | awk '{print $1}')
        if [ -z "${resources}" ]; then
            return
        fi

        while IFS= read -r resourceName; do
            echo "- Validating $resourceType/$resourceName ..."
            if ! resourceIsValid "$resourceType/$resourceName" $namespaceScoped; then
                echo ">>> $resourceType/$resourceName contains an improper ownerReference"
                echo "$resourceType/$resourceName" >> improper-owner-references.txt
            fi
        done <<< "$resources"

    fi
}

# Clear file contents/create if it does not exist
> improper-owner-references.txt

# Loop through all APIgroups, checking all clusterwide and namespaced scoped resources(within certain namespaces)
APIVersions=$(oc api-versions | cut -f1 -d"/" | xargs -n1 | sort -u)
# Insert blank line to get core objects
APIVersions="
\"$APIVersions\""

while IFS= read -r APIgroup; do

    echo "== Checking APIGroup: $APIgroup =="
    echo ""
    echo "Checking Cluster Scoped Resources ..."
    echo ""
    clusterScopedResources=$(oc api-resources --namespaced=false --api-group=$APIgroup | tail -n +2 | awk '{print $1}') # Get all resources
    if [ ! -z "${clusterScopedResources}" ]; then #If empty, continue
        while IFS= read -r resourceType; do
            echo "-- Validating $resourceType --"
            if [[ "$APIgroup" == "v1" ]]; then
                APIgroup=""
            fi
            if [[ "$APIgroup" == "" ]]; then
                validateResources "$resourceType" "false"
            else
                validateResources "$resourceType.$APIgroup" "false"
            fi
            
            echo "- $resourceType.$APIgroup are valid"
        done <<< "$clusterScopedResources"
    fi

    echo ""
    echo "Checking Namespace Scoped Resources ..."
    echo ""
    namespacedScopedResources=$(oc api-resources --namespaced=true --api-group=$APIgroup | tail -n +2 | awk '{print $1}')
    if [ ! -z "${namespacedScopedResources}" ]; then
        while IFS= read -r resourceType; do
            echo "-- Validating $resourceType --"
            if [[ "$APIgroup" == "v1" ]]; then
                APIgroup=""
            fi
            if [[ "$APIgroup" == "" ]]; then
                validateResources "$resourceType" "true"
            else
                validateResources "$resourceType.$APIgroup" "true"
            fi
            echo "- $resourceType.$APIgroup are valid"
        done <<< "$namespacedScopedResources"
    fi
    echo ""
done <<< "$APIVersions"
