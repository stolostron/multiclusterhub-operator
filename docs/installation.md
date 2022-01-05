# Installation

The below guidelines will explain how to build and install the operator on a remote cluster.

### Prerequisites

- [go][go_tool] version v1.13+
- [operator SDK][osdk] v0.18.0+
- [opm][opm] v1.12.5+
- yq
- docker
- quay credentials for https://quay.io/organization/stolostron and https://quay.io/organization/stolostron

### Declare Required Variables

```bash
export DOCKER_USER=<DOCKER_USER>
export DOCKER_PASS=<DOCKER_PASS>
```

It is also recommended to set a unique version label
```bash
export VERSION=<A_UNIQUE_VERSION>
```
### Replace image manifest

Populate the json file located in `image-manifests/` with proper values. Values can be found in https://github.com/stolostron/pipeline/tree/2.2-integration/snapshots

### Install Options

There are 4 ways to install the operator:

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

```bash
make local-install
```

This will 
1. Apply necessary objects (namespace, secrets, operatorgroup, required subscriptions)
2. Apply the CRD
3. Start the operator locally pointing to the cluster specified in `$KUBECONFIG`

#### 3. Deploy with the Operator Lifecycle Manager (Configmap)
OLM will manage creation of most resources required to run the operator. This method simulates an index image with a configmap.

```bash
make cm-install
```

This will 
1. Build and push an installer image
2. Update the CSV
3. Apply necessary objects (namespace, secrets, operatorgroup)
4. Build an index configmap object
5. Apply OLM objects (catalogsource, index, subscription)


#### 4. Deploy with the Operator Lifecycle Manager (Index image)
OLM will manage creation of most resources required to run the operator. This method builds and pushes an actual index image.

```bash
make index-install
```

This will 
1. Build and push an installer image
2. Update the CSV
3. Apply necessary objects (namespace, secrets, operatorgroup)
4. Build an index image and push it 
5. Apply OLM objects (catalogsource, index, subscription)

### Deploy MultiClusterHub instance
Once the operator is installed in the cluster, initiate an installation by creating an instance of MultiClusterHub. To create a default instance of MultiClusterHub:
```bash
make cr
```
> To customize the instance, first modify the spec in `deploy/crds/operator.open-cluster-management.io_v1_multiclusterhub_cr.yaml`.

## Cleanup
Delete multiclusterhub instance if it exists
```bash
kubectl delete mch --all
```

Clean up the operator and its resources:
```bash
make uninstall
```

If not all resources are properly cleaned up, follow the uninstall instructions at https://github.com/stolostron/deploy to manually clean up remaining resources.


[go_tool]:https://golang.org/dl/
[osdk]:https://github.com/operator-framework/operator-sdk/releases
[opm]:https://github.com/operator-framework/operator-registry/releases
