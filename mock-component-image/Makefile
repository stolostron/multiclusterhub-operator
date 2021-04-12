SHELL := /bin/bash
CURR_DIR := "$(PWD)/mock-component-image"

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
	make check-env-vars
	make mock-cleanup 
	make gen-mock-charts 
	make gen-helm-repo-index 
	make gen-foundation-binaries
	make gen-and-push-image
	make get-image-sha
	make gen-mock-image-manifest