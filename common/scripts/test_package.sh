#!/bin/bash

# licensed Materials - Property of IBM
# 5737-E67
# (C) Copyright IBM Corporation 2016, 2019 All Rights Reserved
# US Government Users Restricted Rights - Use, duplication or disclosure restricted by GSA ADP Schedule Contract with IBM Corp.

# NOTE: This script should not be called directly. Please run `make test`.

set -e

_package=$1
echo "Testing package $_package"

# Make sure temporary files do not exist
rm -f cover.tmp

# Run tests
# -coverpkg=./... produces warnings to stderr that we filter out
go test -cover -covermode=atomic -coverprofile=cover.tmp "$_package"

# Merge coverage files
if [ -a cover.tmp ]; then
    go get -v github.com/wadey/gocovmerge
    gocovmerge cover.tmp cover.out > cover.all
    mv cover.all cover.out
fi

# Clean up temporary files
rm -f cover.tmp
