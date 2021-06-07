# Copyright Contributors to the Open Cluster Management project

# GITHUB_USER containing '@' char must be escaped with '%40'
GITHUB_USER := $(shell echo $(GITHUB_USER) | sed 's/@/%40/g')
GITHUB_TOKEN ?=

-include test/Makefile
-include mock-component-image/Makefile
-include Makefile.prow

BUILD_DIR ?= build

VERSION ?= 2.3.0
IMG ?= multiclusterhub-operator
SECRET_REGISTRY ?= quay.io
HUB_IMAGE_REGISTRY ?= quay.io/rhibmcollab
BUNDLE_REGISTRY ?= quay.io/open-cluster-management
GIT_VERSION ?= $(shell git describe --exact-match 2> /dev/null || \
                 git describe --match=$(git rev-parse --short=8 HEAD) --always --dirty --abbrev=8)

DOCKER_USER := $(shell echo $(DOCKER_USER))
DOCKER_PASS := $(shell echo $(DOCKER_PASS))
NAMESPACE ?= open-cluster-management
export ACM_NAMESPACE :=$(NAMESPACE)

# For OCP OLM
export IMAGE ?= $(shell echo $(HUB_IMAGE_REGISTRY)/$(IMG):$(VERSION))
export CSV_CHANNEL ?= alpha
export CSV_VERSION ?= 2.3.0


export PROJECT_DIR = $(shell 'pwd')
export GOPACKAGES = $(shell go list ./... | grep -E -v "manager|test|apis|operators|channel|controller$|version")
export COMPONENT_SCRIPTS_PATH = $(shell 'pwd')/cicd-scripts

# Use podman if available, otherwise use docker
ifeq ($(CONTAINER_ENGINE),)
	CONTAINER_ENGINE = $(shell podman version > /dev/null && echo podman || echo docker)
endif

.PHONY: lint image clean

all: clean lint test image

include common/Makefile.common.mk

lint: lint-all

## Run unit-tests
test:
	./cicd-scripts/run-unit-tests.sh

## Build the MultiClusterHub operator image
image:
	./cicd-scripts/build.sh "$(HUB_IMAGE_REGISTRY)/$(IMG):$(VERSION)"

## Push the MultiClusterHub operator image
push:
	./common/scripts/push.sh "$(HUB_IMAGE_REGISTRY)/$(IMG):$(VERSION)"

## Developer install script to automate full MCH operator and CR installation
install:
	./common/scripts/tests/install.sh

uninstall-cr:
	bash ./test/clean-up.sh

## Fully uninstall the MCH CR and operator
uninstall: uninstall-cr
	bash common/scripts/uninstall.sh

## Create secrets for pulling images
secrets: 
	@oc create secret docker-registry multiclusterhub-operator-pull-secret --docker-server=$(SECRET_REGISTRY) --docker-username=$(DOCKER_USER) --docker-password=$(DOCKER_PASS) || true
	@oc create secret docker-registry quay-secret --docker-server=$(SECRET_REGISTRY) --docker-username=$(DOCKER_USER) --docker-password=$(DOCKER_PASS) || true

## Uninstall and reinstall MCH Operator
reinstall: uninstall cm-install

## Subscribe is an alias for the configmap installation method
subscribe: cm-install

## Install required dependancies
deps:
	./cicd-scripts/install-dependencies.sh
	go mod tidy

## Get logs of MCH Operator
logs:
	@oc logs -f $(shell oc get pod -l name=multiclusterhub-operator -o jsonpath="{.items[0].metadata.name}")

## Update the MultiClusterHub Operator Image
update-image:
	operator-sdk build $(HUB_IMAGE_REGISTRY)/multiclusterhub-operator:$(VERSION) --go-build-args "-o build/_output/bin/multiclusterhub-operator"
	docker push $(HUB_IMAGE_REGISTRY)/multiclusterhub-operator:$(VERSION)

## Apply Observability CR
observability-cr:
	curl -H "Authorization: token $(shell echo $(GITHUB_TOKEN))" \
		-H 'Accept: application/vnd.github.v3.raw' \
		-L https://raw.githubusercontent.com/open-cluster-management/multicluster-monitoring-operator/master/deploy/crds/observability.open-cluster-management.io_v1beta1_multiclusterobservability_cr.yaml | oc apply -f -

## Apply Observability CRD
observability-crd:
	curl -H "Authorization: token $(shell echo $(GITHUB_TOKEN))" \
		-H 'Accept: application/vnd.github.v3.raw' \
		-L https://raw.githubusercontent.com/open-cluster-management/multicluster-observability-operator/main/bundle/manifests/observability.open-cluster-management.io_multiclusterobservabilities.yaml | oc apply -f -
		
## Operator-sdk generate CRD(s)
crd:
	operator-sdk generate crds --crd-version=v1

## Operator-sdk regenerate CSV
csv:
	operator-sdk generate csv --operator-name=multiclusterhub-operator

## Apply the MultiClusterHub CR
cr:
	cat deploy/crds/operator.open-cluster-management.io_v1_multiclusterhub_cr.yaml | yq w - "spec.imagePullSecret" "quay-secret" | oc apply -f -

## Apply the default OperatorGroup
og:
	oc apply -f build/operatorgroup.yaml

## Apply and switch to the open-cluster-management namesapce
ns:
	oc apply -f build/namespace.yaml
	oc project open-cluster-management

## Apply subscriptions normally created by OLM
subscriptions:
	oc apply -k build/subscriptions

## Run operator locally outside the cluster
local-install: ns secrets og subscriptions observability-crd
	oc apply -f deploy/crds/operator.open-cluster-management.io_multiclusterhubs_crd.yaml
	OPERATOR_NAME=multiclusterhub-operator \
	TEMPLATES_PATH="$(shell pwd)/templates" \
	MANIFESTS_PATH="$(shell pwd)/image-manifests" \
	CRDS_PATH="$(shell pwd)/crds" \
	POD_NAMESPACE="open-cluster-management" \
	operator-sdk run local --watch-namespace=open-cluster-management --kubeconfig=$(KUBECONFIG)

## Run as a Deployment inside the cluster
in-cluster-install: ns secrets og update-image subscriptions observability-crd
	oc apply -f deploy/crds/operator.open-cluster-management.io_multiclusterhubs_crd.yaml
	VERSION="${VERSION}" yq eval '.images.[0].newTag = env(VERSION)' -i deploy/kustomization.yaml
	oc apply -k deploy
	
## Creates a configmap index and catalogsource that it subscribes to
cm-install: ns secrets og csv update-image subscriptions observability-crd
	bash common/scripts/generate-cm-index.sh ${VERSION} ${HUB_IMAGE_REGISTRY}
	oc apply -k build/configmap-install

## Generates an index image and catalogsource that serves it
index-install: ns secrets og csv update-image subscriptions observability-crd
	oc patch serviceaccount default -n open-cluster-management -p '{"imagePullSecrets": [{"name": "quay-secret"}]}'
	bash common/scripts/generate-index.sh ${VERSION} ${HUB_IMAGE_REGISTRY}
	oc apply -k build/index-install/non-composite


## Apply BMA CR
bma-cr:
	curl -H "Authorization: token $(shell echo $(GITHUB_TOKEN))" \
		-H 'Accept: application/vnd.github.v3.raw' \
		-L https://raw.githubusercontent.com/open-cluster-management/demo-subscription-gitops/master/bma/BareMetalAssets/dc01r3c3b2-powerflex390.yaml | oc apply -f -

time:
	bash common/scripts/timer.sh

update-version:
	./common/scripts/update-version.sh $(OLD_VERSION) $(NEW_VERSION)

update-crds:
	bash common/scripts/gather-crds.sh

update-manifest:
	bash common/scripts/update-image-manifest.sh

set-copyright:
	@bash ./cicd-scripts/set-copyright.sh

cleanup-mock-image:
	make mock-cleanup

prep-mock-install:
	export PRODUCT_VERSION=$(shell cat COMPONENT_VERSION); \
	make mock-build-image
	cp mock-component-image/results/* ./image-manifests
	echo "mock install prepped!"

# different from `in-cluster-install` (no secrets, no observability-crd)
mock-install: ns og subscriptions update-crds update-image
	oc apply -f deploy/crds/operator.open-cluster-management.io_multiclusterhubs_crd.yaml
	VERSION="${VERSION}" yq eval '.images.[0].newTag = env(VERSION)' -i deploy/kustomization.yaml
	MOCK_REGISTRY="${HUB_IMAGE_REGISTRY}/${IMG}" yq eval '.images.[0].newName = env(MOCK_REGISTRY)' -i deploy/kustomization.yaml
	oc apply -k deploy


## Apply the MultiClusterHub CR (with no self management and no secrets)
mock-cr:
	cat deploy/crds/operator.open-cluster-management.io_v1_multiclusterhub_cr.yaml | yq eval '.spec.disableHubSelfManagement = true' - |  oc apply -f -