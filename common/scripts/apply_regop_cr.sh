#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.

_registrationOpDir=build/registration-operator

if [ ! -d "$_registrationOpDir" ]; then
    echo ""
    git clone git@github.com:open-cluster-management/registration-operator.git $_registrationOpDir
fi

cd  $_registrationOpDir
make apply-hub-cr