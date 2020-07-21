#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.

echo "Building test-image"

docker build . -f build/Dockerfile.test -t $1