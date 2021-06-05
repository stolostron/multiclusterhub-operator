#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project

function delete_cluster() {
    oc login --token="${COLLECTIVE_TOKEN}" --server="${COLLECTIVE_SERVER}"

    cd ./lifeguard/clusterclaims/
    echo "Y" | ./delete.sh
}

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

if ! command -v jq &> /dev/null
then
    echo "jq could not be found"
    exit
fi

if ! command -v yq &> /dev/null
then
    echo "yq could not be found"
    exit
fi


oc login --token="${COLLECTIVE_TOKEN}" --server="${COLLECTIVE_SERVER}"

git clone https://github.com/open-cluster-management/lifeguard.git

cd lifeguard/clusterclaims/
./apply.sh
trap 'delete_cluster' ERR

cd ../..

oc login $(jq -r '.api_url' ./lifeguard/clusterclaims/${CLUSTERCLAIM_NAME}/${CLUSTERCLAIM_NAME}.creds.json) -u kubeadmin -p $(jq -r '.password' ./lifeguard/clusterclaims/${CLUSTERCLAIM_NAME}/${CLUSTERCLAIM_NAME}.creds.json) --insecure-skip-tls-verify=true
oc project

make prep-mock-install MOCK_IMAGE_REGISTRY='quay.io/rhibmcollab' MOCK_IMAGE_NAME='multiclusterhub-operator' MOCK_IMAGE_TAG='mock'
make mock-install MOCK_IMAGE_REGISTRY='quay.io/rhibmcollab' MOCK_IMAGE_NAME='multiclusterhub-operator' MOCK_IMAGE_TAG='mock'
make ft-install MOCK=true

delete_cluster()
echo "Pull request function tests completed successfully!"
