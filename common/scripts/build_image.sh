#!/bin/bash

set -x

CONTAINER_ENGINE=$1
REGISTRY=$2
IMG=$3
VERSION=$4

echo $DOCKER_USER

docker login "REGISTRY" -u "$DOCKER_USER" -p "$DOCKER_PASS"

operator-sdk build --image-builder "$CONTAINER_ENGINE" "$REGISTRY"/"$IMG":"$VERSION" --verbose 
"$CONTAINER_ENGINE" push "$REGISTRY"/"$IMG":"$VERSION"