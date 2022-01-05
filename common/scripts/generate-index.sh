#!/bin/bash

# Copyright (c) 2020 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project

indent() {
  local INDENT="      "
  local INDENT1S="-"
  sed -e "s/^/${INDENT}/" \
      -e "1s/^${INDENT}/${INDENT1S} /"
}

channel=latest
version=$1
registry=quay.io/stolostron

# Generate bundle files with SDK
operator-sdk generate bundle \
--operator-name=multiclusterhub-operator \
--manifests --metadata \
--channels=$channel \
--default-channel=$channel \
--output-dir=./bundles/$version \
--overwrite

# Update operator image
yq w -i bundles/$version/manifests/multiclusterhub-operator.clusterserviceversion.yaml 'spec.install.spec.deployments(name==multiclusterhub-operator).spec.template.spec.containers.(name==multiclusterhub-operator).image' "$registry/multiclusterhub-operator:$version"

# Build bundle image with opm
opm alpha bundle build --directory ./bundles/$version/manifests \
--package=multiclusterhub-operator \
--channels=$channel \
--default=$channel \
--image-builder=docker \
--tag $registry/multiclusterhub-operator:$version-bundle

# Push bundle image to quay
docker push $registry/multiclusterhub-operator:$version-bundle

mkdir "database"

opm registry add -b $registry/multiclusterhub-operator:$version-bundle -d "database/index.db"

# Generate index from bundle
# opm index add \
# --bundles $registry/multiclusterhub-operator:$version-bundle \
# --tag $registry/multiclusterhub-operator:$version-index \
# -c docker

cp build/dockerfile.index .
mkdir "etc"
touch "etc/nsswitch.conf"
chmod a+r "etc/nsswitch.conf"
docker build -f dockerfile.index -t $registry/multiclusterhub-operator:$version-index .

rm -rf database
rm dockerfile.index
rm -rf etc/

# Push index image to quay
docker push $registry/multiclusterhub-operator:$version-index 

# Update catalogsource image
yq w -i build/index-install/non-composite/kustomization.yaml 'images[0].newTag' "$version-index"
