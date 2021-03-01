#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project

# Tested on Mac only

### This is a temp developer script to test enablement of enviornmental variables to replace the image manifest
## To define env vars in your terminal, make sure to source this script

jq -c '.[]' image-manifests/2.3.0.json | while read imageRef; do
    imageKey=$(echo $imageRef | jq -r '."image-key"' | awk '{print toupper($0)}')
    imageKey="OPERAND_IMAGE_$imageKey"  

    imageRemote=$(echo $imageRef | jq -r '."image-remote"')
    imageName=$(echo $imageRef | jq -r '."image-name"')
    imageDigest=$(echo $imageRef | jq -r '."image-digest"')
    fullImage="${imageRemote}/${imageName}@${imageDigest}"
    echo "export $imageKey=$fullImage"
    export $imageKey=$fullImage
done
