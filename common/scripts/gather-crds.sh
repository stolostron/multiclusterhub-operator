#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.
# Tested on Mac only
# Clones hub-crds repo and copies all crds into the crds/ directory

# Remove existing files
rm -rf crd-temp
rm -rf crds/
mkdir -p crds/
mkdir -p crd-temp

# Clone hub-crds into crd-temp
git clone https://github.com/stolostron/hub-crds --branch main crd-temp

# Recursively copy yaml files
find crd-temp -name \*.yaml -exec cp {} crds  \;

# Delete clone directory
rm -rf crd-temp
