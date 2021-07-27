#!/bin/bash
# Copyright (c) 2021 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project

if [ -d "./bin" ]; then
    rm -rf ./bin
fi

mkdir ./bin

GOOS=linux go build ./templates/controller/foundation-controller.go
mv foundation-controller ./bin/controller

GOOS=linux go build ./templates/webhook/foundation-webhook.go
mv foundation-webhook ./bin/webhook

GOOS=linux go build ./templates/proxyserver/foundation-proxyserver.go
mv foundation-proxyserver ./bin/proxyserver

curl -o bin/kubectl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
chmod +x bin/kubectl