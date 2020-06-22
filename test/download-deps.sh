#!/bin/bash

# Copyright (c) 2020 Red Hat, Inc.

# Install OpenShift CLI.
echo "Installing oc CLI..."
curl -kLo oc.tar.gz https://mirror.openshift.com/pub/openshift-v4/clients/oc/4.4/linux/oc.tar.gz
mkdir oc-unpacked
tar -xzf oc.tar.gz -C oc-unpacked
chmod 755 ./oc-unpacked/oc
mv ./oc-unpacked/oc /usr/local/bin/oc
rm -rf ./oc-unpacked ./oc.tar.gz

echo 'set up complete'