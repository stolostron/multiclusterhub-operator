#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project

function delete_cluster() { 
    echo "Deleting clusterclaim ..."
    oc login --token="${COLLECTIVE_TOKEN}" --server="${COLLECTIVE_SERVER}"  --insecure-skip-tls-verify

    cd ./lifeguard/clusterclaims/
    echo "Y" | ./delete.sh
}

set -e

export CLUSTERCLAIM_LIFETIME=4h
export CLUSTERPOOL_TARGET_NAMESPACE=install
export CLUSTERPOOL_NAME=installer-function-test
export CLUSTERCLAIM_GROUP_NAME=Installer
export CLUSTERCLAIM_NAME=install-function-test

export COLLECTIVE_SERVER=https://api.collective.aws.red-chesterfield.com:6443

if [[ -z "${COLLECTIVE_TOKEN}" ]]; then
    echo "environment variable 'COLLECTIVE_TOKEN' must be set"
    exit 1
fi

if ! command -v yq &> /dev/null
then
    echo "Installing yq ..."
    wget https://github.com/mikefarah/yq/releases/download/v4.9.3/yq_linux_amd64.tar.gz -O - |\
  tar xz && sudo mv yq_linux_amd64 /usr/bin/yq >/dev/null
fi


oc login --token="${COLLECTIVE_TOKEN}" --server="${COLLECTIVE_SERVER}"  --insecure-skip-tls-verify

git clone https://github.com/open-cluster-management/lifeguard.git

cd lifeguard/clusterclaims/

READY_CLUSTERS=$(oc get clusterpool installer-function-test -o yaml | yq eval '.status.ready' -)
if [ "$READY_CLUSTERS" -eq "0" ]; then
   echo "No clusterpool clusters available currently. Please try again later ..."
   exit 1
fi

echo "Applying clusterclaim ..."
./apply.sh
set +e
trap 'delete_cluster' ERR

cd ../..

oc login $(jq -r '.api_url' ./lifeguard/clusterclaims/${CLUSTERCLAIM_NAME}/${CLUSTERCLAIM_NAME}.creds.json) -u kubeadmin -p $(jq -r '.password' ./lifeguard/clusterclaims/${CLUSTERCLAIM_NAME}/${CLUSTERCLAIM_NAME}.creds.json) --insecure-skip-tls-verify=true
oc project

make prep-mock-install MOCK_IMAGE_REGISTRY='quay.io/rhibmcollab' MOCK_IMAGE_NAME='multiclusterhub-operator' MOCK_IMAGE_TAG='mock'
make mock-install MOCK_IMAGE_REGISTRY='quay.io/rhibmcollab' MOCK_IMAGE_NAME='multiclusterhub-operator' MOCK_IMAGE_TAG='mock'
make ft-install MOCK=true

delete_cluster()
echo "Pull request function tests completed successfully!"
