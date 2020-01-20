BUILD_DIR ?= build

VERSION ?= latest
IMG ?= multicloudhub-operator
REGISTRY ?= quay.io/rhibmcollab
GIT_VERSION ?= $(shell git describe --exact-match 2> /dev/null || \
                 git describe --match=$(git rev-parse --short=8 HEAD) --always --dirty --abbrev=8)

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
	@operator-sdk build --image-builder $(CONTAINER_ENGINE) $(REGISTRY)/$(IMG):latest
	@$(CONTAINER_ENGINE) tag $(REGISTRY)/$(IMG):latest $(REGISTRY)/$(IMG):$(GIT_VERSION)
	@$(CONTAINER_ENGINE) tag $(REGISTRY)/$(IMG):latest $(REGISTRY)/$(IMG):$(VERSION)
	@$(CONTAINER_ENGINE) push $(REGISTRY)/$(IMG):latest
	@$(CONTAINER_ENGINE) push $(REGISTRY)/$(IMG):$(GIT_VERSION)
	@$(CONTAINER_ENGINE) push $(REGISTRY)/$(IMG):$(VERSION)

olm-catalog: clean
	@common/scripts/olm_catalog.sh

clean::
	rm -rf $(BUILD_DIR)/_output
	rm -f cover.out