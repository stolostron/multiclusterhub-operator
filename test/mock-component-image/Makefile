SHELL := /bin/bash
CURR_DIR := "$(PWD)/test/mock-component-image"

check-env-vars:
	cd ${CURR_DIR}; \
	python3 ./scripts/check_env_vars.py

gen-mock-charts:
	cd ${CURR_DIR}; \
	python3 ./scripts/generate-mock-charts.py

gen-helm-repo-index:
	cd ${CURR_DIR}; \
	python3 ./scripts/generate-helm-index.py

gen-foundation-binaries:
	cd ${CURR_DIR}; \
	bash ./scripts/generate-mock-foundation-binaries.sh

gen-and-push-image:
	cd ${CURR_DIR}; \
	python3 ./scripts/generate-mock-images.py

gen-mock-image-manifest:
	cd ${CURR_DIR}; \
	. ./scripts/get-mock-image-sha.env && \
	python3 ./scripts/check_sha_env_var.py && \
	python3 ./scripts/generate-mock-image-manifest.py

get-image-sha:
	cd ${CURR_DIR}; \
	python3 ./scripts/check_env_vars.py && \
	. ./scripts/get-mock-image-sha.env && \
	python3 ./scripts/check_sha_env_var.py

del-mock-chart-repo:
	cd ${CURR_DIR}; \
	rm -rf ./multiclusterhub

del-mock-image-manifests:
	cd ${CURR_DIR}; \
	rm -rf ./results

del-mock-bins:
	cd ${CURR_DIR}; \
	rm -rf ./bin

mock-cleanup: 
	make del-mock-chart-repo 
	make del-mock-image-manifests 
	make del-mock-bins

mock-build-image:
	make check-env-vars 			PRODUCT_VERSION=$(VERSION) MOCK_IMAGE_REGISTRY=$(REGISTRY) MOCK_IMAGE_SHA="sha256:test"
	make mock-cleanup 				PRODUCT_VERSION=$(VERSION) MOCK_IMAGE_REGISTRY=$(REGISTRY) MOCK_IMAGE_SHA="sha256:test"
	make gen-mock-charts 			PRODUCT_VERSION=$(VERSION) MOCK_IMAGE_REGISTRY=$(REGISTRY) MOCK_IMAGE_SHA="sha256:test"
	make gen-helm-repo-index 		PRODUCT_VERSION=$(VERSION) MOCK_IMAGE_REGISTRY=$(REGISTRY) MOCK_IMAGE_SHA="sha256:test"
	make gen-foundation-binaries 	PRODUCT_VERSION=$(VERSION) MOCK_IMAGE_REGISTRY=$(REGISTRY) MOCK_IMAGE_SHA="sha256:test"
	make gen-and-push-image 		PRODUCT_VERSION=$(VERSION) MOCK_IMAGE_REGISTRY=$(REGISTRY) MOCK_IMAGE_SHA="sha256:test"
	make get-image-sha 				PRODUCT_VERSION=$(VERSION) MOCK_IMAGE_REGISTRY=$(REGISTRY) MOCK_IMAGE_SHA="sha256:test"
	make gen-mock-image-manifest 	PRODUCT_VERSION=$(VERSION) MOCK_IMAGE_REGISTRY=$(REGISTRY) MOCK_IMAGE_SHA="sha256:test"