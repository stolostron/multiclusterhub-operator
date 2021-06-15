#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project

# Tested on Mac only
# Replaces the image-manifest contents with the latest integration snapshot from the pipeline repo

# Where manifests are kept in the pipeline repo
MANIFEST_FOLDER="snapshots"

# Full version
VERSION=$(cat COMPONENT_VERSION 2> /dev/null)
if [ -z "${VERSION}" ]; then
  echo "VERSION is unset or set to the empty string"
  exit 1
fi

# Branch excludes patch version
BRANCH_NAME="${VERSION%.*}-edge"

# Remove existing files
rm -rf pipeline-temp
mkdir -p pipeline-temp
# Clone cicd pipeline repo
if [ -z "${GH_USER}" ] || [ -z "${GH_TOKEN}" ]; then
  git clone https://github.com/open-cluster-management/pipeline --branch ${BRANCH_NAME} pipeline-temp
else
  git clone https://${GH_USER}:${GH_TOKEN}@github.com/open-cluster-management/pipeline --branch ${BRANCH_NAME} pipeline-temp
fi

# Find manifest from the latest snapshot
LATEST_SNAPSHOT=$(find pipeline-temp/snapshots -name 'manifest-*' | sort | tail -n 1)
if [ -z "${LATEST_SNAPSHOT}" ]; then
  echo "LATEST_SNAPSHOT is unset or set to the empty string"
  rm -rf pipeline-temp
  exit 1
fi

echo "Using manifest file ${LATEST_SNAPSHOT}"

# Verify the snapshot file exists
if [ ! -f ${LATEST_SNAPSHOT} ]; then
    echo "File ${LATEST_SNAPSHOT} not found!"
    exit 1
fi

# Verify the current manifest file exists
CURRENT_MANIFEST="image-manifests/${VERSION}.json"
if [ ! -f ${CURRENT_MANIFEST} ]; then
    echo "File ${CURRENT_MANIFEST} not found!"
    exit 1
fi

# Copy snapshot into current manifest file
cp -f ${LATEST_SNAPSHOT} ${CURRENT_MANIFEST}

# Delete pipeline directory
rm -rf pipeline-temp
