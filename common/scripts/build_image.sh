#!/bin/bash

CONTAINER_ENGINE=$1
REGISTRY=$2
IMG=$3
VERSION=$4

docker login "$REGISTRY" -u "$DOCKER_USER" -p "$DOCKER_PASS"

operator-sdk build --image-builder "$CONTAINER_ENGINE" "$REGISTRY"/"$IMG":"$VERSION"
"$CONTAINER_ENGINE" push "$REGISTRY"/"$IMG":"$VERSION"