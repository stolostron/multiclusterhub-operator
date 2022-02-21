#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project


echo "Starting Installer Functional Tests ..."
echo ""

export GO111MODULE=off

if [[ "$TEST_MODE" == "install" ]]; then
    echo "Beginning Install Tests ..."
    echo ""
    ginkgo -tags functional -v --slowSpecThreshold=300 test/multiclusterhub_install_test
elif [[ "$TEST_MODE" == "uninstall" ]]; then
    echo "Beginning Uninstall Tests ..."
    echo ""
    ginkgo -tags functional -v --slowSpecThreshold=300 test/multiclusterhub_uninstall_test
elif [[ "$TEST_MODE" == "update" ]]; then
    echo "Beginning Update Tests ..."
    echo ""
    ginkgo -tags functional -v --slowSpecThreshold=900 test/multiclusterhub_update_test
else
    echo "TEST_MODE not exported. Must be of type 'install', 'uninstall', or 'update'"
    echo ""
    exit 1
fi
