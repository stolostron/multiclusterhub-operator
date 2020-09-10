#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.

echo "Building test-image"

make deps
go mod vendor

docker build . -f build/Dockerfile.test -t $1