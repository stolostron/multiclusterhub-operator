[comment]: # ( Copyright Contributors to the Open Cluster Management project )

# Contributing guidelines

- [Contributing guidelines](#contributing-guidelines)
  - [Contributions](#contributions)
  - [Certificate of Origin](#certificate-of-origin)
  - [DCO Sign Off](#dco-sign-off)
  - [Contributing A Patch](#contributing-a-patch)
  - [Issues and Pull Request Management](#issues-and-pull-request-management)
  - [Pull Request Etiquette](#pull-request-etiquette)
  - [Pre-check before submitting a PR](#pre-check-before-submitting-a-pr)
  - [Build images](#build-images)
  - [Development and Testing](#development-and-testing)
    - [Installing MultiClusterHub Operator with local code changes](#installing-multiclusterhub-operator-with-local-code-changes)
    - [Automated testing of local development code](#automated-testing-of-local-development-code)

## Contributions

All contributions to the repository must be submitted under the terms of the [Apache Public License 2.0](https://www.apache.org/licenses/LICENSE-2.0).

## Certificate of Origin

By contributing to this project you agree to the Developer Certificate of
Origin (DCO). This document was created by the Linux Kernel community and is a
simple statement that you, as a contributor, have the legal right to make the
contribution. See the [DCO](DCO) file for details.

## DCO Sign Off

You must sign off your commit to state that you certify the
[DCO](https://github.com/open-cluster-management-io/community/blob/main/DCO).
To certify your commit for DCO, add a line like the following at the end of your commit message:

```bash
Signed-off-by: John Smith <john@example.com>
```

This can be done with the `--signoff` option to `git commit`. See the [Git documentation](https://git-scm.com/docs/git-commit#Documentation/git-commit.txt--s) for details.

## Contributing A Patch

1. Submit an issue describing your proposed change to the repo in question.
2. The [repo owners](OWNERS) will respond to your issue promptly.
3. Fork the desired repo, develop and test your changes (see [Development and Testing](#development-and-testing))
4. Submit a pull request.

## Issues and Pull Request Management

Anyone may comment on issues and submit reviews for pull requests. However, in order to be assigned an issue or pull
request, you must be a member of the [stolostron](https://github.com/stolostron) GitHub organization.

Repo maintainers can assign you an issue or pull request by leaving a `/assign <your Github ID>` comment on the issue
or pull request.

## Pull Request Etiquette

Anyone may submit a pull request, although in order to ensure that the pull request can be responded to properly,
link the associated issue in the PR. To have a pull request merged, it will require approval from a
[repository owner](OWNERS).

We request that all PRs which contribute new code into the repo also contain their associated unit and function tests.

## Pre-check before submitting a PR

After your PR is ready to commit, please run following commands to check your code.

```bash
make test                   # Run unit tests
```

## Build images

Make sure your code build passed.

```bash
make manifests              # Regenerate manifests if necessary

make docker-build           # Ensure build succeeds
# or
make podman-build
```

## Development and Testing

### Installing MultiClusterHub Operator with local code changes

This approach _only_ tests MultiClusterHub Operator functionality (not the functionality of Open Cluster Management
sub-components).

1. Confirm the following are installed and configured on your local machine:

   - `docker` or `podman`
   - `go` (version 1.22.4 minimum)
   - `python3`
   - `make`

2. `oc login` into an OCP cluster. See [Requirements and recommendations](https://access.redhat.com/documentation/en-us/red_hat_advanced_cluster_management_for_kubernetes/2.2/html/install/installing#requirements-and-recommendations) for supported cluster sizes.

3. Fork the `multiclusterhub-operator` GitHub repository

4. Export the following environment variables:

```bash
export MOCK_IMAGE_REGISTRY=<some-public-image-repository>
export HUB_IMAGE_REGISTRY=<some-public-image-repository>
```

This environment variable correlate with the image you'll need to build and push to public image repositories:

- `HUB_IMAGE_REGISTRY` correlates to the `multiclusterhub-operator` image, which is built from the `multiclusterhub-operator` codebase. The resulting image will be pushed to `$HUB_IMAGE_REGISTRY/multiclusterhub-operator`

These images are assumed to be public to eliminate any requirement for pull secrets.

5. Run the following `make` commands to install the `multiclusterhub-operator` with a "mock" component image.

```bash
make prep-mock-install
```

... builds and pushes the mock image and preps the image manifest to use the freshly-built mock image

```bash
make mock-install
```

... builds and pushes the hub image, installs the `multiclusterhub-operator` as a deployment, and installs other required community-available operators

```bash
make mock-cr
```

... creates a legitimate `MultiClusterHub` Custom Resource. *Note: In this Custom Resource, hub self-management is disabled (`disableHubSelfManagement: true`)*

6. Confirm "mock" installation successful by checking the `mch` Custom Resource for a `Running` status

```bash
oc get mch
```

7. To uninstall all Hub components, run the following:

```bash
make uninstall
```

8. To reinstall, run through these steps again.

### Automated testing of local development code

1. Run unit tests

```bash
make unit-tests
```
