#!/bin/bash

# Copyright (c) 2020 Red Hat, Inc.

echo "Starting Installer Functional Tests ..."
echo ""

if [ -z "$TEST_MODE" ]; then
    echo "TEST_MODE not exported. Must be of type 'install' or 'uninstall'"
    exit 1
fi

# check and load options.yaml
OPTIONS_FILE=/resources/options.yaml
if [ -f $OPTIONS_FILE ]; then
    echo "Processing options file..."
    export clusterAPIServer=`yq r $OPTIONS_FILE 'options.hub.apiserver'`
    export clusterToken=`yq r $OPTIONS_FILE 'options.hub.token'`
    export clusterUser=`yq r $OPTIONS_FILE 'options.hub.user'`
    export clusterPass=`yq r $OPTIONS_FILE 'options.hub.pass'`
    echo "Options file processed."
else 
    echo "No Options file found... exiting"
    exit 1
fi

echo ""

echo "Logging into OpenShift API server..."
echo ""
if [ -z "$clusterToken" ]; then
    oc login $clusterAPIServer --username $clusterUser --password $clusterPass --insecure-skip-tls-verify
else
     oc login $clusterAPIServer --token $clusterToken --insecure-skip-tls-verify
fi


if [[ "$TEST_MODE" == "install" ]]; then
    ginkgo -tags functional -v --slowSpecThreshold=10 test/multiclusterhub_install_test
elif [[ "$TEST_MODE" == "uninstall" ]]; then
    ginkgo -tags functional -v --slowSpecThreshold=10 test/multiclusterhub_uninstall_test
fi