CLUSTERPOOL_GIT_REPO ?=
CLUSTERPOOL_STATE_PROJECT ?= "https://$(GITHUB_USER):$(GITHUB_TOKEN)@$(CLUSTERPOOL_GIT_REPO)"
CLUSTERPOOL_GIT_BRANCH ?= master
CLUSTERPOOL_GIT_DIR ?= $(PWD)/clusterpool
CLUSTERPOOL_DIR := $(BUILD_HARNESS_PATH)/modules/clusterpool
CLUSTERPOOL_RUNNER_DIR ?= $(CLUSTERPOOL_DIR)
CLUSTERPOOL_VENV_DIR ?= $(CLUSTERPOOL_DIR)/clusterpool-pyvenv
CLUSTERPOOL_VENV_ACTIVATE = $(CLUSTERPOOL_VENV_DIR)/bin/activate

# for dev environments, we do not want to run within a virtualenv
# instead, developers are required to have python3.7 installed and we will create a venv for them
# Change the activate command if we are running in Travis
ifdef HAS_JOSH_K_SEAL_OF_APPROVAL
	CLUSTERPOOL_VENV_ACTIVATE = ~/virtualenv/python3.7/bin/activate
endif

.PHONY: clusterpool/init
clusterpool/init: %init:
ifneq ($(shell test -d $(CLUSTERPOOL_GIT_DIR) && echo -n yes), yes)
	@$(GIT) clone -b $(CLUSTERPOOL_GIT_BRANCH) $(CLUSTERPOOL_STATE_PROJECT) $(CLUSTERPOOL_GIT_DIR)/ 
endif
ifneq ($(shell test -d ./state && echo -n yes), yes)
	@mkdir state 
endif
ifndef HAS_JOSH_K_SEAL_OF_APPROVAL
	python3 -m venv $(CLUSTERPOOL_VENV_DIR)
endif
	@source $(CLUSTERPOOL_VENV_ACTIVATE); \
	pip3 install --upgrade pip > /dev/null; \
	python3 -V; \
	pip3 -V; \
	pip3 install -r $(CLUSTERPOOL_RUNNER_DIR)/requirements.txt > /dev/null; \
	python3 $(CLUSTERPOOL_RUNNER_DIR)/clusterpool_runner.py --version
ifndef CLUSTERPOOL_TARGET_PLATFORM
	$(error You have to either define the CLUSTERPOOL_TARGET_PLATFORM environment variable or pass it to the make target)
endif

.PHONY: clusterpool/checkout
## Checkout a cluster from the cluster pool of choice and configure the system to access it (from Travis)
clusterpool/checkout: %checkout: %init
	@source $(CLUSTERPOOL_VENV_ACTIVATE); \
	python3 $(CLUSTERPOOL_RUNNER_DIR)/clusterpool_runner.py checkout --repo $(CLUSTERPOOL_GIT_DIR) --platform $(CLUSTERPOOL_TARGET_PLATFORM)

.PHONY: clusterpool/configure
## Configure the system with the currently checked out cluster (from Travis)
clusterpool/configure: %configure: %init
	@source $(CLUSTERPOOL_VENV_ACTIVATE); \
	python3 $(CLUSTERPOOL_RUNNER_DIR)/clusterpool_runner.py configure --repo $(CLUSTERPOOL_GIT_DIR) --platform $(CLUSTERPOOL_TARGET_PLATFORM)

.PHONY: clusterpool/return
## Return the checked out cluster to the cluster pool (from Travis)
clusterpool/return: %return: %init
	@source $(CLUSTERPOOL_VENV_ACTIVATE); \
	python3 $(CLUSTERPOOL_RUNNER_DIR)/clusterpool_runner.py return --repo $(CLUSTERPOOL_GIT_DIR) --platform $(CLUSTERPOOL_TARGET_PLATFORM)

.PHONY: clusterpool/check-queue
clusterpool/check-queue: %check-queue: %init
	@source $(CLUSTERPOOL_VENV_ACTIVATE); \
	python3 $(CLUSTERPOOL_RUNNER_DIR)/clusterpool_runner.py check-queue --repo $(CLUSTERPOOL_GIT_DIR) --platform $(CLUSTERPOOL_TARGET_PLATFORM) --count $(PLATFORM_DESIRED_COUNT) --should-clean True --commit-message "$(TRAVIS_COMMIT_MESSAGE)"

.PHONY: clusterpool/clean-queue
clusterpool/clean-queue: %clean-queue: %init
	@source $(CLUSTERPOOL_VENV_ACTIVATE); \
	python3 $(CLUSTERPOOL_RUNNER_DIR)/clusterpool_runner.py clean-queue --repo $(CLUSTERPOOL_GIT_DIR) --platform $(CLUSTERPOOL_TARGET_PLATFORM) --count $(PLATFORM_DESIRED_COUNT)

.PHONY: clusterpool/get-config
## Get the complete terraform output for the configured node
clusterpool/get-config: %get-config:
	@source $(CLUSTERPOOL_VENV_ACTIVATE); \
	python3 $(CLUSTERPOOL_RUNNER_DIR)/clusterpool_runner.py get-reserved-info --repo $(CLUSTERPOOL_GIT_DIR) --platform $(CLUSTERPOOL_TARGET_PLATFORM) --output '' --length 0

.PHONY: clusterpool/status
## Get the status of all clusters in the pool
clusterpool/status: %status:
ifneq ($(shell test -d $(CLUSTERPOOL_GIT_DIR) && echo -n yes), yes)
	@$(GIT) clone -b $(CLUSTERPOOL_GIT_BRANCH) $(CLUSTERPOOL_STATE_PROJECT) $(CLUSTERPOOL_GIT_DIR)/ 
endif
	@source $(CLUSTERPOOL_VENV_ACTIVATE); \
	python3 $(CLUSTERPOOL_RUNNER_DIR)/clusterpool_runner.py check-queue --repo $(CLUSTERPOOL_GIT_DIR) --platform $(CLUSTERPOOL_TARGET_PLATFORM) --count 0 --should-clean False

.PHONY: clusterpool/get-master-ip
## Get the master ip address for the configured node
clusterpool/get-master-ip: %get-master-ip:
	@source $(CLUSTERPOOL_VENV_ACTIVATE); \
	python3 $(CLUSTERPOOL_RUNNER_DIR)/clusterpool_runner.py get-reserved-info --repo $(CLUSTERPOOL_GIT_DIR) --platform $(CLUSTERPOOL_TARGET_PLATFORM) --output master-node

.PHONY: clusterpool/destroy
## Destroy the cluster named by CLUSTERPOOL_DESTROY_NAME
clusterpool/destroy: %destroy: %init
	@source $(CLUSTERPOOL_VENV_ACTIVATE); \
	python3 $(CLUSTERPOOL_RUNNER_DIR)/clusterpool_runner.py destroy --repo $(CLUSTERPOOL_GIT_DIR) --platform $(CLUSTERPOOL_TARGET_PLATFORM) --cluster $(CLUSTERPOOL_DESTROY_NAME)

.PHONY: clusterpool/upgrade-available-pool
## Redeploy all non-reserved clusters
clusterpool/upgrade-available-pool: %upgrade-available-pool: %init
	@source $(CLUSTERPOOL_VENV_ACTIVATE); \
	python3 $(CLUSTERPOOL_RUNNER_DIR)/clusterpool_runner.py upgrade --repo $(CLUSTERPOOL_GIT_DIR) --platform $(CLUSTERPOOL_TARGET_PLATFORM) --count $(PLATFORM_DESIRED_COUNT)

.PHONY: clusterpool/execute-ssh
## Execute commands on the configured node (i.e. make clusterpool:execute-ssh < test.sh)
clusterpool/execute-ssh: %execute-ssh:
	@source $(CLUSTERPOOL_VENV_ACTIVATE); \
	python3 $(CLUSTERPOOL_RUNNER_DIR)/clusterpool_runner.py run-command --repo $(CLUSTERPOOL_GIT_DIR) --platform $(CLUSTERPOOL_TARGET_PLATFORM)
