#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.

_nucleusDir=build/nucleus
_olmNamespace=openshift-operator-lifecycle-manager

echo ""
echo "Beginning Installation of Nucleus Operator ..."

if [ -d "$_nucleusDir" ]; then
    echo "- Removing Existing Nucleus Directory ..."
    rm -rf "$_nucleusDir"
fi

echo ""
git clone git@github.com:open-cluster-management/nucleus.git $_nucleusDir

cd  $_nucleusDir

if ! [ -x "$(command -v gsed)" ]; then
    if [[ "$OSTYPE" == "darwin"* ]]; then
        brew install gnu-sed
    fi
fi

if [[ "$OSTYPE" == "darwin"* ]]; then
        gsed -i 's/sed/gsed/g' Makefile
fi

echo ""
echo "Deleting OperatorGroups if they exist. (Nucleus always creates OperatorGroup in a given NS.)"
oc delete og --all

echo ""
echo "Attempting deploy of Nucleus ..."

echo ""
make update-all
make deploy-hub OLM_NAMESPACE=$_olmNamespace