#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project

_totalAttempts=0
_maxAttempts=50
_totalPods=0
_podsReady=0
_hivePodsReady=0
_hivePodsTotal=0

echo ""

while true
do
    _totalPods=45
    _podsReady=0
    _totalAttempts=$((_totalAttempts + 1))
    _output=$(oc get pods | grep Running | awk '{ print $2 }')
    _outputHive=$(oc get pods -n hive 2>/dev/null | grep Running | awk '{ print $2 }' )
    _outputCertManager=$(oc get pods -n cert-manager 2>/dev/null | grep Running | awk '{ print $2 }' )
    while IFS= read -r line; do
        if [[ "$line" == "" ]]; then
            continue
        fi
        _podsReady=$((_podsReady + ${line:0:1}))
    done <<< "$_output"

    while IFS= read -r line; do
        if [[ "$line" == "" ]]; then
            continue
        fi
        _podsReady=$((_podsReady + ${line:0:1}))
    done <<< "$_outputHive"

    while IFS= read -r line; do
        if [[ "$line" == "" ]]; then
            continue
        fi
        _podsReady=$((_podsReady + ${line:0:1}))
    done <<< "$_outputCertManager"


    if [[ ( "$_podsReady" != "$_totalPods" ) ]]; then
        END_SECONDS=$((SECONDS+10))
        while [ $SECONDS -lt $END_SECONDS ]; do
            _seconds_left=$((END_SECONDS - SECONDS))
            echo -ne "---    Iteration $_totalAttempts of $_maxAttempts | Pods Ready - $_podsReady/$_totalPods. | Checking namespaces - $NAMESPACE, hive, and cert-manager   --- Retrying in ${_seconds_left:0:1}\r"
            sleep 1
        done
    else
        echo ""
        echo "Install successfully completed. Exiting 0"
        exit 0
    fi

    if [[ "$_totalAttempts" == "$_maxAttempts" ]]; then
        echo ""
        echo "Failed. Too many attempts. Exiting 1"
        exit 1
    fi
done 

echo "Install Failed. Exiting 1"
exit 1
