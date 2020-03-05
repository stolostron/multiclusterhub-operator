# GITHUB_USER containing '@' char must be escaped with '%40'
GITHUB_USER := $(shell echo $(GITHUB_USER) | sed 's/@/%40/g')
GITHUB_TOKEN ?=

-include $(shell curl -H 'Authorization: token ${GITHUB_TOKEN}' -H 'Accept: application/vnd.github.v4.raw' -L https://api.github.com/repos/open-cluster-management/build-harness-extensions/contents/templates/Makefile.build-harness-bootstrap -o .build-harness-bootstrap; echo .build-harness-bootstrap)


BUILD_DIR ?= build

VERSION ?= latest
IMG ?= multicloudhub-operator
SECRET_REGISTRY ?= quay.io 
REGISTRY ?= quay.io/rhibmcollab
BUNDLE_REGISTRY ?= quay.io/open-cluster-management
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
	./cicd-scripts/build.sh "$(REGISTRY)/$(IMG):$(VERSION)"

push:
	./common/scripts/push.sh "$(REGISTRY)/$(IMG):$(VERSION)"

olm-catalog: clean
	@common/scripts/olm_catalog.sh "$(BUNDLE_REGISTRY)" "$(IMG)" "$(VERSION)" "$(REGISTRY)"

clean::
	rm -rf $(BUILD_DIR)/_output
	rm -f cover.out

install: image push olm-catalog 
	# need to check for operator group
	@oc create secret docker-registry quay-secret --docker-server=$(SECRET_REGISTRY) --docker-username=$(DOCKER_USER) --docker-password=$(DOCKER_PASS) || true
	# @oc apply -k ./build/_output/olm || true

directuninstall:
	@ oc delete -k ./build/_output/olm || true

uninstall: directuninstall unsubscribe

reinstall: uninstall install

local: 
	@operator-sdk run --local --namespace="" --operator-flags="--zap-devel=true"

subscribe: image olm-catalog
	# @kubectl create secret docker-registry quay-secret --docker-server=$(REGISTRY) --docker-username=$(DOCKER_USER) --docker-password=$(DOCKER_PASS) || true
	@oc apply -f build/_output/olm/multicloudhub.resources.yaml

unsubscribe:
	@oc delete MultiCloudHub example-multicloudhub || true
	@oc delete csv multicloudhub-operator.v0.0.1 || true
	@oc delete csv etcdoperator.v0.9.4 || true
	@oc delete csv multicloud-operators-subscription.v0.1.2 || true
	@oc delete csv multicloud-operators-subscription.v0.1.1 || true
	@oc delete crd multicloudhubs.operators.multicloud.ibm.com || true
	@oc delete crd channels.app.ibm.com || true
	@oc delete crd deployables.app.ibm.com || true
	@oc delete crd helmreleases.app.ibm.com || true
	@oc delete crd subscriptions.app.ibm.com || true
	@oc delete crd etcdbackups.etcd.database.coreos.com || true
	@oc delete crd etcdclusters.etcd.database.coreos.com || true
	@oc delete crd etcdrestores.etcd.database.coreos.com || true
	@oc delete crd multicloudhubs.operators.multicloud.ibm.com || true
	@oc delete subscription multicloudhub-operator || true
	@oc delete subscription etcdoperator.v0.9.4 || true
	@oc delete subscription multicloud-operators-subscription.v0.1.2 || true
	@oc delete catalogsource multicloudhub-operator-registry || true
	@oc delete deploy -n hive hive-controllers || true
	@oc delete deploy -n hive hiveadmission || true
	@oc delete apiservice v1.admission.hive.openshift.io || true
	@oc delete ns hive || true
	@oc delete scc multicloud-scc || true

resubscribe: unsubscribe subscribe


deps:
	./cicd-scripts/install-dependencies.sh
	go mod tidy