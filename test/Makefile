SUBSCRIPTION_NAME := "advanced-cluster-management"
PULL_SECRET_NAME := "multiclusterhub-operator-pull-secret"
CHANNEL := "release-2.1"
SOURCE_NAMESPACE := "open-cluster-management"
SOURCE_NAME := "acm-custom-registry"


## Run the installer downstream functional tests
ft-downstream-install:
	docker run --network host \
		--env pullSecret=$(PULL_SECRET_NAME) \
		--env source=$(SOURCE_NAME) \
		--env channel=$(CHANNEL) \
		--env sourceNamespace=$(SOURCE_NAMESPACE) \
		--env name=$(SUBSCRIPTION_NAME) \
		--env TEST_MODE="install" \
		--env full_test_suite="false" \
		--volume ~/.kube/config:/opt/.kube/config \
		$(REGISTRY)/$(IMG)-tests:$(VERSION)

## Run the uninstall functional tests
ft-downstream-uninstall:
	docker run --network host \
		--env pullSecret=$(PULL_SECRET_NAME) \
		--env source=$(SOURCE_NAME) \
		--env channel=$(CHANNEL) \
		--env sourceNamespace=$(SOURCE_NAMESPACE) \
		--env name=$(SUBSCRIPTION_NAME) \
		--env TEST_MODE="uninstall" \
		--volume ~/.kube/config:/opt/.kube/config \
		$(REGISTRY)/$(IMG)-tests:$(VERSION)

## Run the uninstall functional tests
ft-downstream-update:
	docker run --network host \
		--env pullSecret=$(PULL_SECRET_NAME) \
		--env source=$(SOURCE_NAME) \
		--env channel=$(CHANNEL) \
		--env sourceNamespace=$(SOURCE_NAMESPACE) \
		--env name=$(SUBSCRIPTION_NAME) \
		--env TEST_MODE="update" \
		--env startVersion="2.0.0" \
		--env updateVersion="2.1.0" \
		--volume ~/.kube/config:/opt/.kube/config \
		$(REGISTRY)/$(IMG)-tests:$(VERSION)

## Run the install functional tests directly
ft-install: 
	export skipSubscription="true";	ginkgo -tags functional -v --slowSpecThreshold=150 test/multiclusterhub_install_test

## Run the uninstall functional tests directly
ft-uninstall:
	export skipSubscription="true";	ginkgo -tags functional -v --slowSpecThreshold=100 test/multiclusterhub_uninstall_test

## Run the update functional tests directly
ft-update:
	export pullSecret=$(PULL_SECRET_NAME) \
		source=$(SOURCE_NAME) \
		channel="latest" \
		sourceNamespace=$(SOURCE_NAMESPACE) \
		name=$(SUBSCRIPTION_NAME) \
		TEST_MODE="update" \
		startVersion="2.0.1" \
		updateVersion="2.1.0";	ginkgo -tags functional -v --slowSpecThreshold=300 test/multiclusterhub_update_test

## Build the MCH functional test image
test-image:
	@echo "Building $(REGISTRY)/$(IMG)-tests:$(VERSION)"
	docker build . -f build/Dockerfile.test -t $(REGISTRY)/$(IMG)-tests:$(VERSION)

# Pass downstream bundle image snapshots for beginning version and the desired version to upgrade to 
# https://quay.io/repository/open-cluster-management/acm-operator-bundle?tab=tags
# Usage: common/scripts/bundle-acm.sh "<Starting Snapshot>" <Update Snapshot> <Start Version> <Update Version>
## Creates an index image to test updates
test-update-image:
	bash common/scripts/bundle-acm.sh "2.0.1-SNAPSHOT-2020-08-20-19-07-00" "2.1.0-SNAPSHOT-2020-08-21-16-48-23" "2.0.1" "2.1.0"

acm-index-install: ns secrets og test-update-image
	# Run `make  test-update-image` to generate a new index if necessary
	oc apply -k build/index-install/composite