# GITHUB_USER containing '@' char must be escaped with '%40'
GITHUB_USER := $(shell echo $(GITHUB_USER) | sed 's/@/%40/g')
GITHUB_TOKEN ?=

USE_VENDORIZED_BUILD_HARNESS ?=

ifndef USE_VENDORIZED_BUILD_HARNESS
-include $(shell curl -s -H 'Authorization: token ${GITHUB_TOKEN}' -H 'Accept: application/vnd.github.v4.raw' -L https://api.github.com/repos/open-cluster-management/build-harness-extensions/contents/templates/Makefile.build-harness-bootstrap -o .build-harness-bootstrap; echo .build-harness-bootstrap)
else
-include vbh/.build-harness-vendorized
endif

BUILD_DIR ?= build

VERSION ?= 2.0.0
IMG ?= multiclusterhub-operator
SECRET_REGISTRY ?= quay.io
REGISTRY ?= quay.io/rhibmcollab
BUNDLE_REGISTRY ?= quay.io/open-cluster-management
GIT_VERSION ?= $(shell git describe --exact-match 2> /dev/null || \
                 git describe --match=$(git rev-parse --short=8 HEAD) --always --dirty --abbrev=8)

DOCKER_USER := $(shell echo $(DOCKER_USER))
DOCKER_PASS := $(shell echo $(DOCKER_PASS))
NAMESPACE ?= open-cluster-management

# For OCP OLM
export IMAGE ?= $(shell echo $(REGISTRY)/$(IMG):$(VERSION))
export CSV_CHANNEL ?= alpha
export CSV_VERSION ?= 2.0.0

# Use podman if available, otherwise use docker
ifeq ($(CONTAINER_ENGINE),)
	CONTAINER_ENGINE = $(shell podman version > /dev/null && echo podman || echo docker)
endif

.PHONY: lint image olm-catalog clean

all: clean lint test image

include common/Makefile.common.mk

lint: lint-all

image:
	./cicd-scripts/build.sh "$(REGISTRY)/$(IMG):$(VERSION)"

push:
	./common/scripts/push.sh "$(REGISTRY)/$(IMG):$(VERSION)"

# configmap subscription install with additional logic
install:
	./common/scripts/tests/install.sh

uninstall:
	bash common/scripts/uninstall.sh

## Install Registration-Operator Hub
regop:
	@bash ./common/scripts/install_regop.sh

# create secrets for pulling images
secrets: 
	@oc create secret docker-registry multiclusterhub-operator-pull-secret --docker-server=$(SECRET_REGISTRY) --docker-username=$(DOCKER_USER) --docker-password=$(DOCKER_PASS) || true
	@oc create secret docker-registry quay-secret --docker-server=$(SECRET_REGISTRY) --docker-username=$(DOCKER_USER) --docker-password=$(DOCKER_PASS) || true

reinstall: uninstall cm-install

# subscribe is an alias for the configmap installation method
subscribe: cm-install

deps:
	./common/scripts/install-dependencies.sh
	go mod tidy

update-image:
	operator-sdk build quay.io/rhibmcollab/multiclusterhub-operator:$(VERSION) --go-build-args "-o build/_output/bin/multiclusterhub-operator"
	docker push quay.io/rhibmcollab/multiclusterhub-operator:$(VERSION)

crd:
	operator-sdk generate crds --crd-version=v1beta1 

# regenerate CSV
csv:
	operator-sdk generate csv --operator-name=multiclusterhub-operator

# apply CR
cr:
	yq w  deploy/crds/operators.open-cluster-management.io_v1beta1_multiclusterhub_cr.yaml 'spec.imagePullSecret' "quay-secret" | oc apply -f -

og:
	oc apply -f build/operatorgroup.yaml

ns:
	oc apply -f build/namespace.yaml
	oc project open-cluster-management

# apply subscriptions normally created by OLM
subscriptions:
	oc apply -k build/subscriptions

# run operator locally outside the cluster
local-install: ns secrets og subscriptions regop
	oc apply -f deploy/crds/operators.open-cluster-management.io_multiclusterhubs_crd.yaml
	OPERATOR_NAME=multiclusterhub-operator \
	TEMPLATES_PATH="$(shell pwd)/templates" \
	MANIFESTS_PATH="$(shell pwd)/image-manifests" \
	operator-sdk18 run local --watch-namespace=open-cluster-management --kubeconfig=$(KUBECONFIG)

# run as a Deployment inside the cluster
in-cluster-install: ns secrets og update-image subscriptions regop
	oc apply -f deploy/crds/operators.open-cluster-management.io_multiclusterhubs_crd.yaml
	yq w -i deploy/kustomization.yaml 'images(name==multiclusterhub-operator).newTag' "${VERSION}"
	oc apply -k deploy
	# oc apply -f deploy/crds/operators.open-cluster-management.io_v1beta1_multiclusterhub_cr.yaml

# creates a configmap index and catalogsource that it subscribes to
cm-install: ns secrets og csv update-image regop
	bash common/scripts/generate-cm-index.sh ${VERSION} ${REGISTRY}
	oc apply -k build/configmap-install

# generates an index image and catalogsource that serves it
index-install: ns secrets og csv update-image regop
	oc patch serviceaccount default -n open-cluster-management -p '{"imagePullSecrets": [{"name": "quay-secret"}]}'
	bash common/scripts/generate-index.sh ${VERSION} ${REGISTRY}
	oc apply -k build/index-install
