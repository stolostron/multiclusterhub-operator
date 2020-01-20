#!/bin/bash

# licensed Materials - Property of IBM
# 5737-E67
# (C) Copyright IBM Corporation 2016, 2019 All Rights Reserved
# US Government Users Restricted Rights - Use, duplication or disclosure restricted by GSA ADP Schedule Contract with IBM Corp.

set -e

_script_dir=$(dirname "$0")
echo 'mode: atomic' > cover.out
echo '' > cover.tmp

_pkgs=$(go list ./... | grep -v /build | grep -v /vendor | grep -E "deploying|rendering|utils")
echo "$_pkgs" | xargs -n1 -I{} "$_script_dir"/test_package.sh {}
