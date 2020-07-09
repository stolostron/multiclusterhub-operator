#!/bin/bash

# Copyright (c) 2020 Red Hat, Inc.

echo "Starting Installer Functional Tests ..."
echo ""

if [ -z "$TEST_MODE" ]; then
    echo "TEST_MODE not exported. Must be of type 'install' or 'uninstall'"
    exit 1
fi

echo ""

if [[ "$TEST_MODE" == "install" ]]; then
    echo "Beginning Installation ..."
    echo ""
    ginkgo -tags functional -v --slowSpecThreshold=10 test/multiclusterhub_install_test
elif [[ "$TEST_MODE" == "uninstall" ]]; then
    echo "Beginning Uninstallation ..."
    echo ""
    ginkgo -tags functional -v --slowSpecThreshold=10 test/multiclusterhub_uninstall_test
fi