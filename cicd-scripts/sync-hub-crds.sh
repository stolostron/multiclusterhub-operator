#!/bin/bash
# Copyright (c) 2021 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project

DESIRED=$(cat cicd-scripts/targetCRDs.txt)
CURRENT=$(cat cicd-scripts/currentCRDs.txt)

if [[ $CURRENT == $DESIRED ]]; then
    echo "Current sha matches desired sha"
    exit 0
fi

# Determine whether we are pulling a branch or a specific sha
if [[ $DESIRED == main ]] || [[ $DESIRED == master ]] || [[ $DESIRED == release* ]]; then
    # This is a branch
    LATEST_SHA=$(curl -s -H "Accept: application/vnd.github.v3+json" https://api.github.com/repos/open-cluster-management/hub-crds/git/refs/heads/${DESIRED} | jq -r '.object.sha')

    if [[ -z "$LATEST_SHA" ]] || [[ $LATEST_SHA == null ]]; then 
        echo "Error getting most recent sha in branch ${DESIRED}. LATEST_SHA is ${LATEST_SHA}."
        exit 1
    fi

    if [[ $CURRENT == $LATEST_SHA ]]; then
        echo "Current sha matches latest sha in branch ${DESIRED}"
        exit 0
    fi

    echo -n $LATEST_SHA > cicd-scripts/currentCRDs.txt
else
    # Must be a sha
    echo "Setting current SHA to desired SHA"
    echo -n $DESIRED > cicd-scripts/currentCRDs.txt
fi
