[comment]: # ( Copyright Contributors to the Open Cluster Management project )

# MultiClusterHub Operator Functional Testss

## Background

Our functional tests leverage ginkgo and gomega to test install, uninstall, and upgrade of the MultiClusterHub. Our tests are packaged as an image and can be pulled from [Quay](https://quay.io/repository/stolostron/multiclusterhub-operator-tests?tab=tags).
Currently, our functional tests have 2 types of targets, one intended for the development flow, and the other intended for the fully containerized test approach.

## Running development functional tests

Running a development functional test assumes that the MCH Operator is already running, as this will not attempt to install a subscription due to the different development approaches of installing the Operator. The development approach also runs the functional tests directly on the local machine.

### Run the install functional tests directly

This will run the test suite of the MCH CR installation. After it is completed, the MCH will remain on the cluster.

```bash
make ft-install
```

### Run the uninstall functional tests directly

This will run the test suite of the MCH CR uninstallation. After it is completed, the MCH CR will have been removed from the cluster, but the operator will remain on the cluster.

```bash
make ft-uninstall
```

### Run the update functional tests directly

When running this test suite, ensure that an index containing the proper bundles is provided. For help generating a test development bundle, see - `make test-update-image`.

```bash
make ft-update
```

## Running Composite Functional Tests

Running the installer composite functional tests requires a CatalogSource to be stood up on this cluster to which the MultiClusterHub subscription can be subscribed too. This must be the composite bundle of ACM.

### Run the installer composite functional tests

Running the install composite functional test will first attempt to install a subscription of ACM. After the subscription has been validated, the MCH will be installed.

```bash
make ft-downstream-install
```

### Run the uninstall composite functional tests

Running the uninstall composite functional test will first attempt to remove the MCH CR. After the CR has been validated as removed, the subscriptions will and related resources will be removed.

```bash
# If you want all install tests, export full_test_suite
export full_test_suite=true

make ft-downstream-uninstall
```

### Run the update composite functional tests

Running the update composite functional test will first attempt to install a subscription of ACM. This subscription will need manual approval which is taken care of by the functional tests. Once the MCH has been validated at the first provided version, it will begin updating and validating the MCH at the ugprade version.

```bash
make ft-downstream-update
```

## Helpful Commands

### Building a test image

To build the MCH functional test image, run the following command below. This can be used to test the composite functional tests.

```bash
make test-image
```

### Build a test development update image

It can be difficult to test updates if a proper bundle isn't available or provided. If you would like to create a development index image, run the command below. See instructions above Make command for configuration options. -

```bash
make test-update-image
```

### Install the test development catalog source

This will create the namespace, secrets, operatorgroup, create the test development update image. After this has been created, a CatalogSource along with its related resources will be created on the targeted cluster, from which the composite functional tests can be ran.

```bash
make acm-index-install
```
