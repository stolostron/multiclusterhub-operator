BUILD_DIR ?= build

VERSION ?= latest
IMG ?= multicloudhub-operator
REGISTRY ?= quay.io/rhibmcollab
GIT_VERSION ?= $(shell git describe --exact-match 2> /dev/null || \
                 git describe --match=$(git rev-parse --short=8 HEAD) --always --dirty --abbrev=8)

QUAY_USER := $(shell echo $(QUAY_USER))
QUAY_TOKEN := $(shell echo $(QUAY_TOKEN))
QUAY_EMAIL := $(shell echo $(QUAY_EMAIL))
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

install: image
	@kubectl create namespace multicloud-system | true
	@kubectl create secret docker-registry quay-secret --docker-server=$(REGISTRY) --docker-username=$(QUAY_USER) --docker-password=$(QUAY_TOKEN) --docker-email=$(QUAY_EMAIL) | true
	@kubectl apply -k deploy | true
	@kubectl apply -f deploy/crds/operators.multicloud.ibm.com_v1alpha1_multicloudhub_cr.yaml | true

uninstall:
	@kubectl delete -f deploy/crds/operators.multicloud.ibm.com_v1alpha1_multicloudhub_cr.yaml | true
	@kubectl delete -k deploy | true
	@kubectl delete deploy etcd-operator | true
	@kubectl delete  namespace multicloud-system | true

reinstall: uninstall install

local: 
	@operator-sdk up local --namespace="" --operator-flags="--zap-devel=true"

subscribe: image olm-catalog
	@kubectl create namespace | true
	@kubectl create secret docker-registry quay-secret --docker-server=$(REGISTRY) --docker-username=$(QUAY_USER) --docker-password=$(QUAY_TOKEN) --docker-email=$(QUAY_EMAIL) | true
	@oc apply -f build/_output/olm/multicloudhub.resources.yaml

unsubscribe:
	@oc delete MultiCloudHub example-multicloudhub | true
	@oc delete csv multicloudhub-operator.v0.0.1 | true
	@oc delete csv etcdoperator.v0.9.4 | true
	@oc delete csv multicloud-operators-subscription.v0.1.1 | true
	@oc delete crd multicloudhubs.operators.multicloud.ibm.com | true
	@oc delete crd channels.app.ibm.com | true
	@oc delete crd deployables.app.ibm.com | true
	@oc delete crd helmreleases.app.ibm.com | true
	@oc delete crd subscriptions.app.ibm.com | true
	@oc delete crd etcdbackups.etcd.database.coreos.com | true
	@oc delete crd etcdclusters.etcd.database.coreos.com | true
	@oc delete crd etcdrestores.etcd.database.coreos.com | true
	@oc delete subscription multicloudhub-operator | true
	@oc delete catalogsource multicloudhub-operator-registry| true

resubscribe: unsubscribe subscribe