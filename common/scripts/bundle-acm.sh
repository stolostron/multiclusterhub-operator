#!/bin/bash

registry="quay.io/rhibmcollab"
acmBundleV200="quay.io/open-cluster-management/acm-operator-bundle:2.0.0-SNAPSHOT-2020-07-14-17-05-13"
acmBundleV201="quay.io/open-cluster-management/acm-operator-bundle:2.1.0-SNAPSHOT-2020-07-14-18-42-16"
indexImage="quay.io/rhibmcollab/multiclusterhub-operator:2.1.0-index"
Version="2.1.0"

docker pull $acmBundleV200
docker pull $acmBundleV201
docker tag $acmBundleV200 $registry/acm-operator-bundle:2.0.0
docker tag $acmBundleV201 $registry/acm-operator-bundle:2.1.0

acmBundleV200="$registry/acm-operator-bundle:2.0.0"
acmBundleV201="$registry/acm-operator-bundle:2.1.0"


id=$(docker create $acmBundleV201 sh)
echo "ID: $acmBundleV201"
mkdir build/temp-bundle
docker cp $id:"/"  - > build/temp-bundle/local-tar-file.tgz
tar -xzvf build/temp-bundle/local-tar-file.tgz -C build/temp-bundle
docker rm $id

rm -rf bundles/$Version
mkdir bundles/$Version
cp -r build/temp-bundle/manifests bundles/$Version/
cp -r build/temp-bundle/metadata bundles/$Version/


yq w -i bundles/$Version/manifests/advanced-cluster-management.v$Version.clusterserviceversion.yaml "spec.replaces" "advanced-cluster-management.v2.0.0"

opm alpha bundle build --directory ./bundles/$Version/manifests \
--package=advanced-cluster-management \
--channels=release-2.1 \
--default=release-2.1 \
--image-builder=docker \
--tag $acmBundleV201

docker push $acmBundleV200
docker push $acmBundleV201

opm index add \
--bundles $acmBundleV200,$acmBundleV201 \
--tag $indexImage \
-c docker

docker push $indexImage

yq w -i build/index-install/kustomization.yaml 'images[0].newTag' "$Version-index"
