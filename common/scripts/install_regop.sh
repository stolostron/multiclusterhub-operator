#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.

_registrationOpDir=build/registration-operator
_olmNamespace=openshift-operator-lifecycle-manager

echo ""
echo "Beginning Installation of Registration Operator ..."

if [ -d "$_registrationOpDir" ]; then
    echo "- Removing Existing Registration-Operator Directory ..."
    rm -rf "$_registrationOpDir"
fi

echo ""
git clone git@github.com:open-cluster-management/registration-operator.git $_registrationOpDir

cd  $_registrationOpDir

if ! [ -x "$(command -v gsed)" ]; then
    if [[ "$OSTYPE" == "darwin"* ]]; then
        brew install gnu-sed
    fi
fi

echo ""
echo "Deleting OperatorGroups if they exist. (Registration Operator always creates OperatorGroup in a given NS.)"
oc delete og --all

echo ""
echo "Attempting deploy of Registration Operator ..."

echo ""
make update-all
make deploy-hub-operator OLM_NAMESPACE=$_olmNamespace