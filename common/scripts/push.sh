# Copyright (c) 2020 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project


FULL_IMAGE_NAME=$1

docker login "$FULL_IMAGE_NAME" -u "$DOCKER_USER" -p "$DOCKER_PASS"

docker push  "$FULL_IMAGE_NAME"
