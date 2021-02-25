# Copyright (c) 2020 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project

#!/bin/bash

echo "BUILD GOES HERE!"

echo "<repo>/<component>:<tag> : $1"

operator-sdk build $1  --go-build-args "-o build/_output/bin/multiclusterhub-operator"
