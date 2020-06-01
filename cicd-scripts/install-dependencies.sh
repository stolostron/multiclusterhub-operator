#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.

echo "INSTALL DEPENDENCIES GOES HERE!"

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

_OPM_VERSION=v1.12.3

if ! [ -x "$(command -v opm)" ]; then
    if [[ "$OSTYPE" == "linux-gnu" ]]; then
        echo "Build opm from source from here: https://github.com/operator-framework/operator-registry/releases/tag/${_OPM_VERSION}"
        exit 1
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        curl -L https://github.com/operator-framework/operator-registry/releases/download/${_OPM_VERSION}/darwin-amd64-opm -o opm
    fi
    chmod +x opm
    sudo mv opm /usr/local/bin/opm
fi
