#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.

echo "Installing MultiClusterHub Operator Dependancies ..."
echo ""

_OPERATOR_SDK_VERSION=v0.18.0

if ! [ -x "$(command -v operator-sdk)" ]; then
    if [[ "$OSTYPE" == "linux-gnu" ]]; then
            curl -L https://github.com/operator-framework/operator-sdk/releases/download/${_OPERATOR_SDK_VERSION}/operator-sdk-${_OPERATOR_SDK_VERSION}-x86_64-linux-gnu -o operator-sdk
    elif [[ "$OSTYPE" == "darwin"* ]]; then
            curl -L https://github.com/operator-framework/operator-sdk/releases/download/${_OPERATOR_SDK_VERSION}/operator-sdk-${_OPERATOR_SDK_VERSION}-x86_64-apple-darwin -o operator-sdk
    fi
    chmod +x operator-sdk
    sudo mv operator-sdk /usr/local/bin/operator-sdk
fi

_OPM_VERSION=v1.12.5

if ! [ -x "$(command -v opm)" ]; then
    if [[ "$TRAVIS" ]]; then
        echo "Skipping OPM install on Travis builds ..."
    elif [[ "$OSTYPE" == "linux-gnu" ]]; then
        echo "Build opm from source from here: https://github.com/operator-framework/operator-registry/releases/tag/${_OPM_VERSION}"
        exit 1
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        curl -L https://github.com/operator-framework/operator-registry/releases/download/${_OPM_VERSION}/darwin-amd64-opm -o opm
        chmod +x opm
        sudo mv opm /usr/local/bin/opm
    fi
fi

# Go tools

if ! which patter > /dev/null; then      echo "Installing patter ..."; go get -u github.com/apg/patter; fi
if ! which gocovmerge > /dev/null; then  echo "Installing gocovmerge..."; go get -u github.com/wadey/gocovmerge; fi
if ! which golangci-lint > /dev/null; then
   curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.23.6
fi