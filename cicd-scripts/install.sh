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

podmanAvailable=$(podman -v 2>/dev/null)
dockerAvailable=$(docker -v 2>/dev/null)

if [ ! -z "$podmanAvailable" ]
then
  containerCli=$(which podman)
elif [ ! -z "$dockerAvailable" ]
then
  containerCli=$(which docker) 
else
  echo "Must install docker or podman ... Exiting"
  exit 1
fi

_output=$($containerCli login quay.io -u $DOCKER_USER -p $DOCKER_PASS)
if [[ "$_output" != *"Login Succeeded"* ]]; then
    echo "Incorrect Docker Credentials provided. Check your 'DOCKER_USER' and 'DOCKER_PASS' environmental variables"
    exit 1
fi

## 3. Check for OperatorGroup

_output=$(oc get operatorgroup | wc -l | awk '{$1=$1};1')
if [[ "$_output" != "2" ]]; then
    echo "No operatorgroup found. Applying default Operatorgroup."
    sed -i -e "s/- .*/- $NAMESPACE/g" cicd-scripts/resources/operatorgroup.yaml
    rm -rf cicd-scripts/resources/operatorgroup.yaml-e
    oc apply -f cicd-scripts/resources/operatorgroup.yaml
    echo "Default operator group applied"
fi


## 4. Update Namespace

sed -i -e "s/namespace:.*/namespace: $NAMESPACE/g" deploy/crds/operators.multicloud.ibm.com_v1alpha1_multicloudhub_cr.yaml
sed -i -e "s/endpoints: mongo-0.mongo.*/endpoints: mongo-0.mongo.$NAMESPACE/g" deploy/crds/operators.multicloud.ibm.com_v1alpha1_multicloudhub_cr.yaml
sed -i -e "s/endpoints: http:\/\/etcd-cluster.*/endpoints: http:\/\/etcd-cluster.$NAMESPACE.svc.cluster.local:2379/g" deploy/crds/operators.multicloud.ibm.com_v1alpha1_multicloudhub_cr.yaml
sed -i -e "s/namespace:.*/namespace: $NAMESPACE/g" deploy/kustomization.yaml
sed -i -e "s/sourceNamespace:.*/sourceNamespace: $NAMESPACE/g" deploy/subscription.yaml
rm -rf deploy/crds/operators.multicloud.ibm.com_v1alpha1_multicloudhub_cr.yaml-e
rm -rf deploy/kustomization.yaml-e
rm -rf deploy/subscription.yaml-e

## 5. Build & Install Operator

if [[ "$force" != "true" ]]; then
    echo ""
    echo "Ensure the file(s) below are correctly configured -"
    echo ""
    echo "- 'deploy/crds/operators.multicloud.ibm.com_v1alpha1_multicloudhub_cr.yaml'"
    echo "-- Ensure 'spec.imageTagPostfix' is accurately set. (Maybe- 202002170800)."
    echo "-- Apply any changes to the CR if necessary"
    echo ""

    while [[ $_Done != 'done' ]] # While string is different or empty...
    do
        read -p "Enter 'done' when changes are completed: " _Done
    done
fi

_output=$(oc create ns hive)

make install

## 6. Validate Install

_totalAttempts=0
_maxAttempts=30
_totalPods=0
_podsReady=0
_hivePodsReady=0
_hivePodsTotal=0

echo ""
echo ""

while true
do
    _totalPods=0
    _podsReady=0
    _hivePodsReady=0
    _hivePodsTotal=0
    _totalAttempts=$((_totalAttempts + 1))
    _output=$(oc get deploy -o name)
    _outputHive=$(oc get deploy -n hive -o name)
    while IFS= read -r line; do
        if [[ "$line" == "" ]]; then
            continue
        fi
        _deployTotals=$(oc get $line  | tail -n +2 | awk '{ print $2 }')
        _podsReady=$((_podsReady + ${_deployTotals:0:1}))
        _totalPods=$((_totalPods + ${_deployTotals:2:3}))
    done <<< "$_output"

    while IFS= read -r line; do
        if [[ "$line" == "" ]]; then
            continue
        fi
        _deployTotalsHive=$(oc get $line -n hive | tail -n +2 | awk '{ print $2 }')

        _hivePodsReady=$((_hivePodsReady + ${_deployTotalsHive:0:1}))
        _hivePodsTotal=$((_hivePodsTotal + ${_deployTotalsHive:2:3}))
    done <<< "$_outputHive"

    if [[ ( "$_podsReady" != "$_totalPods" || "$_hivePodsReady" != "$_hivePodsTotal" || "$_hivePodsTotal" < 1 || "$_totalPods" < 1 ) ]]; then
        echo -ne "---    Attempt $_totalAttempts/$_maxAttempts: Namespace: $NAMESPACE - $_podsReady/$_totalPods | Namespace: Hive - $_hivePodsReady/$_hivePodsTotal    ---\r"
        sleep 10
    else
        echo ""
        echo ""
        echo "Install successfully completed. Exiting 0"
        exit 0
    fi

    if [[ "$_totalAttempts" == "$_maxAttempts" ]]; then
        echo ""
        echo ""
        echo "Failed. Too many attempts. Exiting 1"
        exit 1
    fi
done 

echo "Install Failed. Exiting 1"
exit 1
