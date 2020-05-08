#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.

# 1. Check Variables Are Defined
# 2. Test Docker Login
# 3. Check for OperatorGroup
# 4. Update Namespace
# 5. Build & Install Operator
# 6. Validate Install

# 1. Check Variables Are Defined

force_flag=$1
force=false
if [[ "$force_flag" == "-f" ]] || [[ "$force_flag" == "--force" ]]; then
    force=true
fi

if [ -z ${GITHUB_USER+x} ]; then
    echo "Define variable - GITHUB_USER to avoid being prompted"
    while [[ $GITHUB_USER == '' ]] # While string is different or empty...
    do
        read -p "Enter your Github (github.com) username: " GITHUB_USER
    done
fi

if [ -z ${GITHUB_TOKEN+x} ]; then
    echo "Define variable - GITHUB_TOKEN to avoid being prompted"
    while [[ $GITHUB_TOKEN == '' ]] # While string is different or empty...
    do
        read -p "Enter your Github (github.com) password or token: " GITHUB_TOKEN
    done
fi

if [ -z ${DOCKER_USER+x} ]; then
    echo "Define variable - DOCKER_USER to avoid being prompted"
    while [[ $DOCKER_USER == '' ]] # While string is different or empty...
    do
        read -p "Enter your Docker (quay.io) username: " DOCKER_USER
    done
fi

if [ -z ${DOCKER_PASS+x} ]; then
    echo "Define variable - DOCKER_PASS to avoid being prompted"
    while [[ $DOCKER_PASS == '' ]] # While string is different or empty...
    do
        read -p "Enter your Docker (quay.io) password or token: " DOCKER_PASS
    done
fi

export GITHUB_USER=$GITHUB_USER
export GITHUB_TOKEN=$GITHUB_TOKEN
export DOCKER_USER=$DOCKER_USER
export DOCKER_PASS=$DOCKER_PASS
export NAMESPACE=open-cluster-management

# Ensure the namespace exists
oc get ns $NAMESPACE > /dev/null 2>&1
if [ $? -ne 0 ]; then
   echo "Namespace $NAMESPACE does not exist"
   exit 1
fi

# Ensure the default namespace is the one we are going to be working in
oc project $NAMESPACE

operatorSDKVersion=$(operator-sdk version | cut -d, -f 1 | tr -d '"' | cut -d ' ' -f 3)
if [[ "$operatorSDKVersion" != "v0.17.0" ]]; then
    echo "Must install operator-sdk v0.17.0."
    while [[ "$_install" != "Y" ]] && [[ "$_install" != "N" ]] # While string is different or empty...
    do
        read -p "Install operator-sdk v0.17.0? (Y/N): " _install
    done
    if [[ "$_install" == "Y" ]]; then
        echo "Installing operator-sdk v0.17.0 ..."
        make deps
    else
        echo "Must install operator-sdk v0.17.0 ... Exiting"
        # exit 1
    fi
fi

opm -h >/dev/null 2>&1
if [ $? -ne 0 ]; then
    echo "ERROR: Make sure you have opm v1.12.2 installed"
    echo "Install the binary here: https://github.com/operator-framework/operator-registry/releases/tag/v1.12.2"
    exit 1
fi

## 2. Test Docker Login

echo "Checking Docker login ..."
_output=$(docker login quay.io -u $DOCKER_USER -p $DOCKER_PASS)
if [[ "$_output" != *"Login Succeeded"* ]]; then
    echo "Incorrect Docker Credentials provided. Check your 'DOCKER_USER' and 'DOCKER_PASS' environmental variables"
    exit 1
fi
echo "- Docker login succeeded"
echo ""

## 4. Build & Install Operator

if [[ "$force" != "true" ]]; then
    echo ""
    echo "Ensure the file(s) below are correctly configured -"
    echo ""
    echo "- 'deploy/crds/operators.open-cluster-management.io_v1beta1_multiclusterhub_cr.yaml'"
    echo "-- Ensure 'spec.overrides.imageTagSuffix' is accurately set. (Ex- SNAPSHOT-YYYY-MM-DD-hh-mm-ss)."
    echo "-- Apply any changes to the CR if necessary"
    echo ""

    while [[ $_Done != 'done' ]] # While string is different or empty...
    do
        read -p "Enter 'done' when changes are completed: " _Done
    done
fi

echo "Beginning installation ..."
make cm-install
echo ""
echo "Operator online. MultiClusterHub CR applied."

## 5. Validate Install

./common/scripts/tests/validate.sh
return_code=$?

echo ""
echo "Elapsed Time: $SECONDS seconds"

exit $return_code
