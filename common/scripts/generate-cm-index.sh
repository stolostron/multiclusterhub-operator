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
--operator-name=multiclusterhub-operator \
--manifests --metadata \
--channels=$channel \
--default-channel=$channel \
--output-dir=./bundles/$version \
--overwrite

# Update operator image
yq w -i bundles/$version/manifests/multiclusterhub-operator.clusterserviceversion.yaml 'spec.install.spec.deployments(name==multiclusterhub-operator).spec.template.spec.containers.(name==multiclusterhub-operator).image' "$registry/multiclusterhub-operator:$version"

# Compile bundle info into configmap
csv=$(yq r bundles/$version/manifests/multiclusterhub-operator.clusterserviceversion.yaml | indent)
crd=$(yq r bundles/$version/manifests/operators.open-cluster-management.io_multiclusterhubs_crd.yaml | indent)
pkg=$(yq r build/configmap-install/package.yaml | indent)

# Contruct composite Configmap
cat > build/configmap-install/index-configmap.yaml <<-EOF
kind: ConfigMap
apiVersion: v1
metadata:
  name: mch-index
data:
  customResourceDefinitions: |-
    $crd
  clusterServiceVersions: |-
    $csv
  packages: |-
    $pkg
EOF
