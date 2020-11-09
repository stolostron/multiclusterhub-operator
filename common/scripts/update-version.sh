#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.

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
build/configmap-install/package.yaml
build/index-install/composite/kustomization.yaml
build/index-install/non-composite/kustomization.yaml
deploy/kustomization.yaml
deploy/olm-catalog/multiclusterhub-operator/manifests/multiclusterhub-operator.clusterserviceversion.yaml
deploy/operator.yaml
deploy/subscription.yaml
docs/installation.md
pkg/controller/multiclusterhub/common_test.go
pkg/manifest/manifest_test.go
test/Makefile
test/multiclusterhub_install_test/multiclusterhub_test.go
test/multiclusterhub_update_test/multiclusterhub_test.go
test/utils/resources.go
test/utils/utils.go
version/version.go"

# SPECIAL CASE

mv ./image-manifests/$OLD_VERSION.json ./image-manifests/$NEW_VERSION.json

# BULK CASE

OLD_VERSION_NO_Z_CLEANED=$(echo "${OLD_VERSION_NO_Z//\./\\.}")
NEW_VERSION_NO_Z_CLEANED=$(echo "${NEW_VERSION_NO_Z//\./\\.}")

for FILE in $FILES; do
    sed -i '' "s/$OLD_VERSION/$NEW_VERSION/g" $FILE
    sed -i '' "s/$OLD_VERSION_NO_Z_CLEANED/$NEW_VERSION_NO_Z_CLEANED/g" $FILE
done

echo "Versions updated!"