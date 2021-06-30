#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project


if [ -z "$1" ]
  then
    echo "Pass in OLD_VERSION"
    return -1;
fi
OLD_VERSION=$1

if [ -z "$2" ]
  then
    echo "Pass in NEW_VERSION"
    return -1;
fi
NEW_VERSION=$2

OLD_VERSION_NO_Z=$(echo $OLD_VERSION | cut -d'.' -f 1-2)
NEW_VERSION_NO_Z=$(echo $NEW_VERSION | cut -d'.' -f 1-2)

FILES="COMPONENT_VERSION
Makefile
README.md
docs/installation.md
controllers/common_test.go
pkg/manifest/manifest_test.go
test/function_tests/Makefile
test/function_tests/multiclusterhub_install_test/multiclusterhub_test.go
test/function_tests/multiclusterhub_update_test/multiclusterhub_test.go
test/function_tests/utils/resources.go
test/function_tests/utils/utils.go
pkg/version/version.go"

# SPECIAL CASE

mv ./bin/image-manifests/$OLD_VERSION.json ./bin/image-manifests/$NEW_VERSION.json

# BULK CASE

OLD_VERSION_NO_Z_CLEANED=$(echo "${OLD_VERSION_NO_Z//\./\\.}")
NEW_VERSION_NO_Z_CLEANED=$(echo "${NEW_VERSION_NO_Z//\./\\.}")

for FILE in $FILES; do
    sed -i '' "s/$OLD_VERSION/$NEW_VERSION/g" $FILE
    sed -i '' "s/$OLD_VERSION_NO_Z_CLEANED/$NEW_VERSION_NO_Z_CLEANED/g" $FILE
done

echo "Versions updated!" 
