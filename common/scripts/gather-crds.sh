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

curl  -o './crds/0000_01_addon.open-cluster-management.io_managedclusteraddons.crd.yaml' -LJO https://raw.githubusercontent.com/open-cluster-management/api/main/addon/v1alpha1/0000_01_addon.open-cluster-management.io_managedclusteraddons.crd.yaml

curl  -o './crds/0000_00_addon.open-cluster-management.io_clustermanagementaddons.crd.yaml' -LJO https://raw.githubusercontent.com/open-cluster-management/api/main/addon/v1alpha1/0000_00_addon.open-cluster-management.io_clustermanagementaddons.crd.yaml
mv "./crds/0000_00_addon.open-cluster-management.io_clustermanagementaddons.crd.yaml -LJO" ./crds/0000_00_addon.open-cluster-management.io_clustermanagementaddons.crd.yaml
ls ./crds