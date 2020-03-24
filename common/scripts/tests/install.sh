#!/bin/bash

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

if [ -z ${NAMESPACE+x} ]; then
    echo "Define variable - NAMESPACE to avoid being prompted"
    while [[ $NAMESPACE == '' ]] # While string is different or empty...
    do
        read -p "Enter your namespace to install the operator and operands: " NAMESPACE
    done
fi

export GITHUB_USER=$GITHUB_USER
export GITHUB_TOKEN=$GITHUB_TOKEN
export DOCKER_USER=$DOCKER_USER
export DOCKER_PASS=$DOCKER_PASS
export NAMESPACE=$NAMESPACE

# Ensure the namespace exists
oc get ns $NAMESPACE > /dev/null 2>&1
if [ $? -ne 0 ]; then
   echo "Namespace $NAMESPACE does not exist"
   exit 1
fi

# Ensure the default namespace is the one we are going to be working in
oc project $NAMESPACE

operatorSDKVersion=$(operator-sdk version | cut -d, -f 1 | tr -d '"' | cut -d ' ' -f 3)
if [[ "$operatorSDKVersion" != "v0.15.1" ]]; then
    echo "Must install operator-sdk v0.15.1."
    while [[ "$_install" != "Y" ]] && [[ "$_install" != "N" ]] # While string is different or empty...
    do
        read -p "Install operator-sdk v0.15.1? (Y/N): " _install
    done
    if [[ "$_install" == "Y" ]]; then
        echo "Installing operator-sdk v0.15.1 ..."
        make deps
    else
        echo "Must install operator-sdk v0.15.1 ... Exiting"
        exit 1
    fi
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

## 3. Check for OperatorGroup

echo "Checking for operatorgroup ..."
_output=$(oc get operatorgroup | wc -l | awk '{$1=$1};1')
if [[ "$_output" != "2" ]]; then
    echo "No operatorgroup found. Applying default Operatorgroup."
    sed -i -e "s/- .*/- $NAMESPACE/g" common/scripts/tests/resources/operatorgroup.yaml
    rm -rf common/scripts/tests/resources/operatorgroup.yaml-e
    oc apply -f common/scripts/tests/resources/operatorgroup.yaml
    echo "Default operator group applied"
fi
echo "- Operator group exists"
echo ""

## 4. Update Namespace

sed -i -e "s/namespace:.*/namespace: $NAMESPACE/g" deploy/crds/operators.open-cluster-management.io_v1alpha1_multiclusterhub_cr.yaml
sed -i -e "s/endpoints: mongo-0.mongo.*/endpoints: mongo-0.mongo.$NAMESPACE/g" deploy/crds/operators.open-cluster-management.io_v1alpha1_multiclusterhub_cr.yaml
sed -i -e "s/namespace:.*/namespace: $NAMESPACE/g" deploy/kustomization.yaml
sed -i -e "s/sourceNamespace:.*/sourceNamespace: $NAMESPACE/g" deploy/subscription.yaml
rm -rf deploy/crds/operators.open-cluster-management.io_v1alpha1_multiclusterhub_cr.yaml-e
rm -rf deploy/kustomization.yaml-e
rm -rf deploy/subscription.yaml-e

## 5. Build & Install Operator

if [[ "$force" != "true" ]]; then
    echo ""
    echo "Ensure the file(s) below are correctly configured -"
    echo ""
    echo "- 'deploy/crds/operators.open-cluster-management.io_v1alpha1_multiclusterhub_cr.yaml'"
    echo "-- Ensure 'spec.imageTagSuffix' is accurately set. (Ex- SNAPSHOT-YYYY-MM-DD-hh-mm-ss)."
    echo "-- Apply any changes to the CR if necessary"
    echo ""

    while [[ $_Done != 'done' ]] # While string is different or empty...
    do
        read -p "Enter 'done' when changes are completed: " _Done
    done
fi

echo "Creating hive namespace if it does not exist"
_output=$(oc create ns hive 2>/dev/null)
echo "- hive namespace created"
echo ""

echo "Beginning installation ..."
_output=$(make install 2>/dev/null)
echo ""

while [[ $_output != "multiclusterhub.operators.open-cluster-management.io/example-multiclusterhub created" ]] # While string is different or empty...
do
    echo "Waiting for Operator to come online ..."
    _output=$(oc apply -f deploy/crds/operators.open-cluster-management.io_v1alpha1_multiclusterhub_cr.yaml 2>/dev/null)
    sleep 10
done

echo ""
echo "Operator online. MultiClusterHub CR applied."

## 6. Validate Install

./common/scripts/tests/validate.sh
return_code=$?

echo ""
echo "Elapsed Time: $SECONDS seconds"

exit $return_code
