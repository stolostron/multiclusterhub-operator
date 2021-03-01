#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project

# Tested on Mac only
# Clones hub-crds repo and copies all crds into the crds/ directory

# Remove existing files
rm crds/*.yaml
rm -rf crd-temp
mkdir -p crd-temp

# Clone hub-crds into crd-temp
git clone https://github.com/open-cluster-management/hub-crds --branch main crd-temp

# Recursively copy yaml files
find crd-temp -name \*.yaml -exec cp {} crds  \;

# Delete clone directory
rm -rf crd-temp
