# MultiClusterHub Operator

The MultiCusterHub operator manages the install of Open Cluster Management (OCM) on RedHat Openshift Container Platform

## Quick Install Instructions

For a standard installation of Open Cluster Management, follow the instructions at https://github.com/open-cluster-management/deploy

## Install

The below guidelines will explain how to build and run the code in this repo for manually installing the MultiCusterHub.

### Prerequisites

- [Operator SDK >= v0.18.0](https://github.com/operator-framework/operator-sdk/releases)
- [OPM >= v1.12.5](https://github.com/operator-framework/operator-registry/releases)
- yq
- Docker
- Credentials to access https://quay.io/organization/rhibmcollab and https://quay.io/organization/open-cluster-management

### Declare Required Variables

```bash
export DOCKER_USER=<DOCKER_USER>
export DOCKER_PASS=<DOCKER_PASS>
```

*Recommended* Set custom version
```bash
export VERSION=<A_UNIQUE_VERSION>
```

### Install Dependencies

```bash
make deps subscribe
```
*This will not install all required dependencies* Set custom version

### Replace image manifest

The json file located in `image-manifests/` contains a template for images versions. Before installing this file needs to be populated with proper values, which can be found in https://github.com/open-cluster-management/pipeline/tree/2.0-integration/snapshots

### Install Manually

#### 1. Run as a Deployment inside the cluster
```bash
make in-cluster-install
```

This will 
1. Build and push an installer image
2. Apply necessary objects (namespace, secrets, operatorgroup, required subscriptions)
3. Apply the CRD
4. Apply resources from the `deploy` directory

#### 2. Run locally outside the cluster
This method is preferred during development cycle to deploy and test faster.

This will 
1. Apply necessary objects (namespace, secrets, operatorgroup, required subscriptions)
2. Apply the CRD
3. Start the operator locally pointing to the cluster specified in `$KUBECONFIG`

#### 3. Deploy with the Operator Lifecycle Manager (Configmap)
OLM will manage creation of most resources required to run the operator. This method simulates an index image with a configmap.

This will 
1. Build and push an installer image
2. Update the CSV
3. Apply necessary objects (namespace, secrets, operatorgroup)
4. Build an index configmap object
5. Apply OLM objects (catalogsource, index, subscription)


#### 3. Deploy with the Operator Lifecycle Manager (Index image)
OLM will manage creation of most resources required to run the operator. This method builds and pushes an actual index image.

This will 
1. Build and push an installer image
2. Update the CSV
3. Apply necessary objects (namespace, secrets, operatorgroup)
4. Build an index image and push it 
5. Apply OLM objects (catalogsource, index, subscription)

### Deploy MultiClusterHub instance
To create a default instance of MultiClusterHub:
```bash
make cr
```
To customize the instance, first modify the `deploy/crds/operators.open-cluster-management.io_v1beta1_multiclusterhub_cr.yaml` file in advance.

## Cleanup
Delete multiclusterhub instance if it exists
```bash
kubectl delete mch --all
```

Clean up operator resources:
```bash
make uninstall
```

If not all resources are properly cleaned up, follow the uninstall instructions at https://github.com/open-cluster-management/deploy to manually clean up remaining resources.


## Useful Make Targets

`make image`: Build the image
`make push`: Push built image to Quay
`make secrets`: Generate secrets needed for install
`make cr`: Apply basic multiclusterhub instance


## Disabling MultiClusterHub Operator
Once installed, the hub operator will monitor changes in the cluster that affect an instance of the multiclusterhub (mch) and reconcile deviations to maintain a desired state. To stop the installer from making these changes you can apply an annotation to the mch instance.
```bash
kubectl annotate mch <mch-name> mch-pause=true
```

Remove or edit this annotation to resume installer operations
```bash
kubectl annotate mch <mch-name> mch-pause=false
```
