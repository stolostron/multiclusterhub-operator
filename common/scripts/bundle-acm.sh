#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project

# Tested on Mac only


function extractBundleFromImage {
    id=$(docker create $1 sh)
    mkdir -p build/temp-bundle-$2
    docker cp $id:"/"  - > build/temp-bundle-$2/local-tar-file.tgz
    tar -xzvf build/temp-bundle-$2/local-tar-file.tgz -C build/temp-bundle-$2
    docker rm $id
    
    rm -rf bundles/$2
    mkdir -p bundles/$2
    cp -R build/temp-bundle-$2/manifests bundles/$2
    cp -R build/temp-bundle-$2/metadata bundles/$2
    
    rm -rf build/temp-bundle-$2
}

if [ "$#" -ne 4 ]; then
    echo "Incorrect Usage"
    echo "Usage: common/scripts/bundle-acm.sh <Starting Snapshot> <Update Snapshot> <Start Version> <Update Version>"
    exit 1
fi


startBundle="quay.io/open-cluster-management/acm-operator-bundle:$1"
updateBundle="quay.io/open-cluster-management/acm-operator-bundle:$2"
startVersion=$3
updateVersion=$4

registry="quay.io/zkayyali812"
indexImage="quay.io/zkayyali812/multiclusterhub-operator:$updateVersion-index"

# Pull and tag images
docker pull $startBundle
docker pull $updateBundle
docker tag $startBundle $registry/acm-operator-bundle:$startVersion
docker tag $updateBundle $registry/acm-operator-bundle:$updateVersion
startBundle="$registry/acm-operator-bundle:$startVersion"
updateBundle="$registry/acm-operator-bundle:$updateVersion"

# # Extract Contents of Images
# extractBundleFromImage $startBundle $startVersion
# extractBundleFromImage $updateBundle $updateVersion

# # Add 'Replaces' to Update CSV

# REPLACES="advanced-cluster-management.v$startVersion" yq eval -i '.spec.replaces = env(REPLACES)' bundles/$updateVersion/manifests/advanced-cluster-management.v$updateVersion.clusterserviceversion.yaml

# Generate and Build Bundles
opm alpha bundle generate \
--directory bundles/$startVersion/manifests \
--package advanced-cluster-management \
--channels release-2.2 \
--default release-2.2 \

docker build -f bundle.Dockerfile -t $startBundle .

opm alpha bundle generate \
--directory bundles/$updateVersion/manifests \
--package advanced-cluster-management \
--channels release-2.3 \
--default release-2.3

docker build -f bundle.Dockerfile -t $updateBundle .

rm bundle.Dockerfile

# Push new bundles
docker push $startBundle
docker push $updateBundle

mkdir "database"

opm registry add -b $startBundle -d "database/index.db"
opm registry add -b $updateBundle -d "database/index.db"


cp build/dockerfile.index .
mkdir "etc"
touch "etc/nsswitch.conf"
chmod a+r "etc/nsswitch.conf"
docker build -f dockerfile.index -t $indexImage .

rm -rf database
rm dockerfile.index
rm -rf etc/

# Generate and push index image of bundles
# opm index add \
# --bundles $startBundle,$updateBundle \
# --tag $indexImage \
# -c docker

docker push $indexImage

# # Update Kustomize with new tag if necessary
# yq w -i build/index-install/composite/kustomization.yaml 'images[0].newTag' "$updateVersion-index"
