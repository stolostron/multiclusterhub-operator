#!/bin/bash

set -e

indent() {
  local INDENT="      "
  local INDENT1S="    -"
  sed -e "s/^/${INDENT}/" \
      -e "1s/^${INDENT}/${INDENT1S} /"
}

listCSV() {
  for index in ${!CSVDIRS[*]}
  do
    indent apiVersion < "$(ls "${CSVDIRS[$index]}"/*version.yaml)"
  done
}

addReqCRDs() {
  echo "required:" | sed 's/^/    /' >> ${DEPLOYDIR}/req_crds.yaml.bak
  for f in ${DEPLOYDIR}/req_crds/*; do ( cat "${f}"; echo) | sed 's/^/    /' >> ${DEPLOYDIR}/req_crds.yaml.bak; done
  sed -i'' -e "/customresourcedefinitions:/r ${DEPLOYDIR}/req_crds.yaml.bak" "${CSVFILE}"
}

unindent(){

  local FILENAME=$1
  local INDENT="    "
  local INDENT1S="- "

  if [[ "$OSTYPE" == "linux-gnu" ]]; then
    sed -i -e "1 s/${INDENT1S}/  /" "${FILENAME}"
    sed -i -e "s/${INDENT}//" "${FILENAME}"
  elif [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' -e "s/^${INDENT}//" "${FILENAME}"
    sed -i '' -e "s/^${INDENT1S}/  /" "${FILENAME}"
  fi
}

removeNamespacePlaceholder(){
  local FILENAME=$1
  if [[ "$OSTYPE" == "linux-gnu" ]]; then
    sed -e '/namespace: placeholder/d' "${FILENAME}"
  elif [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' -e '/namespace: placeholder/d' "${FILENAME}"
  fi
  
}

DEPLOYDIR=${DIR:-$(cd "$(dirname "$0")"/../../deploy && pwd)}
BUNDLE_REGISTRY=$1
IMG=$2
BUNDLE_VERSION=$(cat COMPONENT_VERSION)

export CSV_CHANNEL=alpha
export CSV_VERSION=0.0.1
export BASE_DATA=iVBORw0KGgoAAAANSUhEUgAAADIAAAAyCAYAAAAeP4ixAAAABmJLR0QA/wD/AP+gvaeTAAAEDElEQVRoge2ZSWgUQRSGv0lmXKLiAm4RRTAqmCDoUUVBjYKKIMZ49C6auB3Ei548uBy8RNCTSkAwiKAgiDkIKoLoQY1L3EHNwV1jNDGZ8VCvrJ6eXqqT7nEi+aHo7nqvuv6/6/XrVzMwhCEkghSQleNgQgHfsn/BIgmkHeeDbVXy8F+uiEau6CwGhhT85yuiUervTF7keK3IYAstIFjIQUp3VZZ5dWbJXwV9nQPOASOT5xUJdcBPDMe/cAvpk+tvcrwGTCgOx1DsI/9BWwmZD7yW86fA7GIw9UE50CRceoEGIggpAyqBO3L9AVicOOVCjAYuCYefQL30hwrpxQgJulExMBX/B2ktpNzRVw4cxyzt9tgpF6KG4NDulxCNRkzoHSO5ymAF8EXmuQlM9PAJFfIbfyEAmzDp7zxQMSDKhdgC9Mj9W/BP/9ZCgsqXRcB78bsFTOoX5XykgAMOPmErHosQgCpU7OaA58DcCKTdSAMnMO/gVosxoUL0soYJAZgC3Bb/j8BSizFujAEuyz06gXWW40KFdMt1xvKGk4HPmPS8yXIcwAzgvox9ByyMMDZWIbMdRH7JsQ/YYzF2AfBWxjxAiYqC2ISsAT6J72NgHvnp+ST+4VkLfBW/VmBcFAWCUCH6yQ7zuUEZKrtowi2oONdwpuerwCyHrQLYj0kopwLmCcOAhIwDLmKyy1689yyLgA6H3z3gBqai7hNBA9nvWAsZ7hpYA7Rj6p7akIkmosJL30+36/hntwywGWhGpfZO4AcqvTejVluHa6gQHRZOIfXAd+m/C8wMEeFEBarYWw5MC/DbIIRzIe0FsDaqkDRw2OFzmvh3jCngqIPYA2AHUA2MklYN7ATaPISFCpmOyig51EdyW8wCNI7IHN0yh1+Nh9gaMB/tQCFdcv1Gjh3AkrhYu7AeI2Klh70VuODRX4sRs1Z3+gnJoTJNZSyUC5HB7De8Vjstti6f8Y1ifym+vkKa6H+Ot0Ed5p3wCqcwIWngofh4lkU/xBj3PsONszLPDh97mBCAXeLT7GXUQiqB8dKSEPVE5pnnY7cRUiM+z7yMWkixmi5vrgT49KDqOt3OyZgxYu/0Kuo+o7KIE2XA2IAnEweCVj2DigyNKpc9Gz8de+jQqnb0jXc1XZtNcfXr5KBD67HNLjAp3AXmAKtQX2xQ0eCEzqZfUDWbG6vleDt2dhGwEUW0Df+vud6zjPCwpYFHYq9LgqAtMsArIdLg4xMkZKfY2rHflieGdZis5LUtaEXtZdz7llUyJosqc0oChzBiGgn+9SaNWgldZx1MnF0EpMjfKrShvtg1qB/QR8v5bkxJkkWJKMk/c9djfvALau2UUDj5IY0qAM+gvjOdqJ3pQ+mrowRe7CHY4A8ixrhAvKZDJwAAAABJRU5ErkJggg==

cp "${DEPLOYDIR}"/operator.yaml "${DEPLOYDIR}"/operator.yaml.bak
if [ "$(uname)" = "Darwin" ]; then
  sed -i "" "s|multiclusterhub-operator:latest|${IMAGE}|g" "${DEPLOYDIR}"/operator.yaml
else
  sed -i "s|multiclusterhub-operator:latest|${IMAGE}|g" "${DEPLOYDIR}"/operator.yaml
fi

operator-sdk generate csv --csv-channel "${CSV_CHANNEL}" --csv-version "${CSV_VERSION}" --operator-name "${IMG}" >/dev/null 2>&1

cp "${DEPLOYDIR}"/operator.yaml.bak "${DEPLOYDIR}"/operator.yaml
rm -f "${DEPLOYDIR}"/operator.yaml.bak

BUILDDIR=${DIR:-$(cd "$(dirname "$0")"/../../build && pwd)}
OLMOUTPUTDIR="${BUILDDIR}"/_output/olm
mkdir -p "${OLMOUTPUTDIR}"

PKGDIR="${DEPLOYDIR}"/olm-catalog/multiclusterhub-operator
CSVDIRS[0]=${DIR:-$(cd "${PKGDIR}"/"${CSV_VERSION}" && pwd)}

CRD=$(grep -v -- "---" "$(ls "${DEPLOYDIR}"/crds/*crd.yaml)" | indent)
PKG=$(indent packageName < "$(ls "${PKGDIR}"/*multiclusterhub-operator.package.yaml)")
CSVFILE="${PKGDIR}"/"${CSV_VERSION}"/multiclusterhub-operator.v"${CSV_VERSION}".clusterserviceversion.yaml

# remove replaces field
sed -ie '/replaces:/d' "${CSVFILE}"

addReqCRDs
rm -f "${DEPLOYDIR}"/req_crds.yaml.bak
# disable all namespaces supported, see https://github.com/operator-framework/operator-sdk/issues/2173 
index=$(grep -n "type: AllNamespaces" "${CSVFILE}" | cut -d ":" -f 1)
index=$((index - 1))
if [ "$(uname)" = "Darwin" ]; then
  sed -i "" "${index}s/true/false/" "${CSVFILE}"
else
  sed -i "${index}s/true/false/" "${CSVFILE}"
fi
# # save "defaults" for some spec fields
sed -i -e "/maintainers:/,/^ *[^:]*:/s/- {}/- email: email@email.com/" "${CSVFILE}"
if [ "$(uname)" = "Darwin" ]; then
  sed -i -e "/email:/a\\
  \ \ \ \ name: install\\
  " "${CSVFILE}"
else
  sed -i -e "/email:/a\\\ \ \ \ name: install\\" "${CSVFILE}"
fi
sed -i -e "/keywords/{n;s/.*/    - operator/;}" "${CSVFILE}" 
sed -i -e "s/mediatype:/  mediatype:/" "${CSVFILE}"
sed -i -e "/  mediatype:/,/^ *[^:]*:/s|[\"]|a|g;/^ *mediatype:/,/^ *[^:]*:/s|aa|image/png|g" "${CSVFILE}"
sed -i -e "s/- base64data:/  - base64data:/" "${CSVFILE}"
sed -i -e "/  - base64data:/,/^ *[^:]*:/s|[\"]|a|g;/^   *- base64data:/,/^ *[^:]*:/s|aa|${BASE_DATA}|g" "${CSVFILE}"

NAME=${NAME:-multiclusterhub-operator-registry}
NAMESPACE=${NAMESPACE:-multicluster-system}
DISPLAYNAME=${DISPLAYNAME:-multiclusterhub-operator}

cat <<< "$CRD" > "${OLMOUTPUTDIR}"/multiclusterhub.crd.yaml
cat <<< "$(listCSV)" > "${OLMOUTPUTDIR}"/multiclusterhub.csv.yaml

cat > "${OLMOUTPUTDIR}"/multiclusterhub.resources.yaml <<EOF | sed 's/^  *$//'
# This file was autogenerated by 'common/scripts/olm_catalog.sh'
# Do not edit it manually!
---
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: $NAME
spec:
  configMap: $NAME
  displayName: $DISPLAYNAME
  publisher: Red Hat
  sourceType: configmap
---
kind: ConfigMap
apiVersion: v1
metadata:
  name: $NAME
data:
  customResourceDefinitions: |-
$(cat "${OLMOUTPUTDIR}"/multiclusterhub.crd.yaml)
  clusterServiceVersions: |-
$(cat "${OLMOUTPUTDIR}"/multiclusterhub.csv.yaml)
  packages: |-
$PKG
EOF

unindent "${OLMOUTPUTDIR}"/multiclusterhub.crd.yaml
unindent "${OLMOUTPUTDIR}"/multiclusterhub.csv.yaml
removeNamespacePlaceholder "${OLMOUTPUTDIR}"/multiclusterhub.csv.yaml

if [ ! -z ${TRAVIS_BRANCH+x} ]; then
cat <<EOF > "${OLMOUTPUTDIR}"/annotations.yaml
annotations:
  operators.operatorframework.io.bundle.mediatype.v1: "registry+v1"
  operators.operatorframework.io.bundle.manifests.v1: "manifests/"
  operators.operatorframework.io.bundle.metadata.v1: "metadata/"
  operators.operatorframework.io.bundle.package.v1: "$IMG-bundle"
  operators.operatorframework.io.bundle.channels.v1: "$CSV_CHANNEL"
  operators.operatorframework.io.bundle.channel.default.v1: "$CSV_CHANNEL"
EOF

  _IMAGE_REFERENCE=$BUNDLE_REGISTRY/$IMG:$BUNDLE_VERSION$COMPONENT_TAG_EXTENSION
  docker login $BUNDLE_REGISTRY -u $DOCKER_USER -p $DOCKER_PASS
  docker push $_IMAGE_REFERENCE
  _SHA_IMAGE_REFERENCE=$(docker inspect --format='{{index .RepoDigests 0}}' $_IMAGE_REFERENCE)
  if [ "$(uname)" = "Darwin" ]; then
    sed -i "" "s|quay.io/rhibmcollab/multiclusterhub-operator:latest|${_SHA_IMAGE_REFERENCE}|g" "${OLMOUTPUTDIR}"/multiclusterhub.csv.yaml
  else
    sed -i "s|quay.io/rhibmcollab/multiclusterhub-operator:latest|${_SHA_IMAGE_REFERENCE}|g" "${OLMOUTPUTDIR}"/multiclusterhub.csv.yaml
  fi

  cat "${OLMOUTPUTDIR}"/multiclusterhub.csv.yaml

  docker build --file "${BUILDDIR}"/Dockerfile.bundle \
    --build-arg OPERATOR_NAME=$IMG \
    --build-arg OPERATOR_CHANNELS=$CSV_CHANNEL \
    --build-arg OPERATOR_DEFAULT_CHANNEL=$CSV_CHANNEL \
    --build-arg OPERATOR_CRD="_output/olm/multiclusterhub.crd.yaml" \
    --build-arg OPERATOR_CSV="_output/olm/multiclusterhub.csv.yaml" \
    --build-arg ANNOTATION_YML="_output/olm/annotations.yaml" "${BUILDDIR}" -t $BUNDLE_REGISTRY/$IMG-bundle:$BUNDLE_VERSION$COMPONENT_TAG_EXTENSION

fi

\cp -r "${PKGDIR}" "${OLMOUTPUTDIR}"
rm -rf "${DEPLOYDIR}"/olm-catalog

rm -f ${OLMOUTPUTDIR}/*/*/*.yamle
rm -f ${OLMOUTPUTDIR}/*/*/*.yaml-e 

cp "${DEPLOYDIR}"/subscription.yaml "${OLMOUTPUTDIR}"
cp "${DEPLOYDIR}"/operator.yaml "${OLMOUTPUTDIR}"
cp "${DEPLOYDIR}"/crds/*_cr.yaml "${OLMOUTPUTDIR}"
cp "${DEPLOYDIR}"/kustomization.yaml "${OLMOUTPUTDIR}"

echo "Created ${OLMOUTPUTDIR}/annotations.yaml"
echo "Created ${OLMOUTPUTDIR}/multiclusterhub-operator"
echo "Created ${OLMOUTPUTDIR}/multiclusterhub.resources.yaml"
echo "Created ${OLMOUTPUTDIR}/multiclusterhub.crd.yaml"
echo "Created ${OLMOUTPUTDIR}/multiclusterhub.csv.yaml"
echo "Created ${OLMOUTPUTDIR}/operator.yaml"
echo "Created ${OLMOUTPUTDIR}/subscription.yaml"
echo "Created ${OLMOUTPUTDIR}/kustomization.yaml"
