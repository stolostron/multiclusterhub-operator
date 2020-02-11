BUILD_DIR ?= build

VERSION ?= latest
IMG ?= multicloudhub-operator
REGISTRY ?= quay.io/rhibmcollab
GIT_VERSION ?= $(shell git describe --exact-match 2> /dev/null || \
                 git describe --match=$(git rev-parse --short=8 HEAD) --always --dirty --abbrev=8)

DOCKER_USER := $(shell echo $(DOCKER_USER))
DOCKER_PASS := $(shell echo $(DOCKER_PASS))
NAMESPACE ?= default

# For OCP OLM
export IMAGE ?= $(shell echo $(REGISTRY)/$(IMG):$(VERSION))
export CSV_CHANNEL ?= alpha
export CSV_VERSION ?= 0.0.1

# Use podman if available, otherwise use docker
ifeq ($(CONTAINER_ENGINE),)
	CONTAINER_ENGINE = $(shell podman version > /dev/null && echo podman || echo docker)
endif

.PHONY: lint image olm-catalog clean

all: clean lint test image

include common/Makefile.common.mk

lint: lint-all

image:
	./common/scripts/build_image.sh "$(CONTAINER_ENGINE)" "$(REGISTRY)" "$(IMG)" "$(VERSION)"

	@$(CONTAINER_ENGINE) tag $(REGISTRY)/$(IMG):latest $(REGISTRY)/$(IMG):nweathe
	@$(CONTAINER_ENGINE) push $(REGISTRY)/$(IMG):nweathe

olm-catalog: clean
	@common/scripts/olm_catalog.sh

clean::
	rm -rf $(BUILD_DIR)/_output
	rm -f cover.out

install: image
	@kubectl create secret docker-registry quay-secret --docker-server=$(REGISTRY) --docker-username=$(DOCKER_USER) --docker-password=$(DOCKER_PASS) || true
	@kubectl apply -k deploy || true
	@kubectl apply -f deploy/crds/operators.multicloud.ibm.com_v1alpha1_multicloudhub_cr.yaml || true

uninstall:
	@kubectl delete -f deploy/crds/operators.multicloud.ibm.com_v1alpha1_multicloudhub_cr.yaml || true
	@kubectl delete -k deploy || true
	@kubectl delete deploy etcd-operator || true

reinstall: uninstall install

local: 
	@operator-sdk up local --namespace="" --operator-flags="--zap-devel=true"

subscribe: image olm-catalog
	@kubectl create secret docker-registry quay-secret --docker-server=$(REGISTRY) --docker-username=$(DOCKER_USER) --docker-password=$(DOCKER_PASS) || true
	@oc apply -f build/_output/olm/multicloudhub.resources.yaml

unsubscribe:
	@oc delete MultiCloudHub example-multicloudhub | true
	@oc delete csv multicloudhub-operator.v0.0.1 | true
	@oc delete csv etcdoperator.v0.9.4 | true
	@oc delete csv multicloud-operators-subscription.v0.1.1 | true
	@oc delete subscription multicloudhub-operator | true
	@oc delete catalogsource multicloudhub-operator-registry| true

resubscribe: unsubscribe subscribe


deps:
	./common/scripts/install_dependancies.sh
	go mod tidy