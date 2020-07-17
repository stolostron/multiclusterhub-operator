
## Run the installer functional tests
functional-test-install:
	docker run --network host \
		--env pullSecret=multiclusterhub-operator-pull-secret \
		--env source="acm-custom-registry" \
		--env channel="release-2.1" \
		--env sourceNamespace=open-cluster-management \
		--env name="advanced-cluster-management" \
		--env TEST_MODE="install" \
		--env full_test_suite="true" \
		--volume ~/.kube/config:/opt/.kube/config \
		$(REGISTRY)/$(IMG)-tests:$(VERSION)
	# ginkgo -tags functional -v --slowSpecThreshold=10 test/multiclusterhub_install_test

## Run the uninstall functional tests
functional-test-uninstall:
	docker run --network host \
		--env pullSecret=multiclusterhub-operator-pull-secret \
		--env source="acm-custom-registry" \
		--env channel="release-2.1" \
		--env sourceNamespace=open-cluster-management \
		--env name="advanced-cluster-management" \
		--env TEST_MODE="uninstall" \
		--volume ~/.kube/config:/opt/.kube/config \
		$(REGISTRY)/$(IMG)-tests:$(VERSION)
	# ginkgo -tags functional -v --slowSpecThreshold=10 test/multiclusterhub_uninstall_test

## Run the uninstall functional tests
functional-test-update:
	docker run --network host \
		--env pullSecret=multiclusterhub-operator-pull-secret \
		--env source="acm-custom-registry" \
		--env channel="release-2.1" \
		--env sourceNamespace=open-cluster-management \
		--env name="advanced-cluster-management" \
		--env TEST_MODE="update" \
		--env startVersion="2.0.0" \
		--volume ~/.kube/config:/opt/.kube/config \
		$(REGISTRY)/$(IMG)-tests:$(VERSION)

## Build the MCH functional test image
test-image:
	@echo "Building $(REGISTRY)/$(IMG)-tests:$(VERSION)"
	docker build . -f build/Dockerfile.test -t $(REGISTRY)/$(IMG)-tests:$(VERSION)

# Pass downstream bundle image snapshots for beginning version and the desired version to upgrade to 
# https://quay.io/repository/open-cluster-management/acm-operator-bundle?tab=tags
# Usage: common/scripts/bundle-acm.sh "<Starting Snapshot>" <Update Snapshot> <Start Version> <Update Version>
## Creates an index image to test updates
test-update-image:
	bash common/scripts/bundle-acm.sh "2.0.0-SNAPSHOT-2020-07-16-16-16-06" "2.1.0-SNAPSHOT-2020-07-16-16-44-10" "2.0.0" "2.1.0"