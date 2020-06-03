#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.

indent() {
  local INDENT="      "
  local INDENT1S="-"
  sed -e "s/^/${INDENT}/" \
      -e "1s/^${INDENT}/${INDENT1S} /"
}

channel=latest
version=$1
registry=quay.io/rhibmcollab

# Generate bundle files with SDK
operator-sdk generate bundle \
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

# Generate index from bundle
opm index add \
--bundles $registry/multiclusterhub-operator:$version-bundle \
--tag $registry/multiclusterhub-operator:$version-index \
-c docker

# Push index image to quay
docker push $registry/multiclusterhub-operator:$version-index 

# Update catalogsource image
yq w -i build/index-install/catalogsource.yaml 'spec.image' "$registry/multiclusterhub-operator:$version-index"
