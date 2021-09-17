[comment]: # ( Copyright Contributors to the Open Cluster Management project )

# Installation

The below guidelines will explain how to build and install the operator on a remote cluster.

### Prerequisites

- [go][go_tool] version v1.17+
- [operator SDK][osdk] v1.9.0+
- [opm][opm] v1.12.5+
- yq
- docker
- quay credentials
- Add your valid quay pull-secret.yaml file in `hack/prereqs/pull-secret.yaml`

### Declare Required Variables

```bash
export DOCKER_USER=<DOCKER_USER>
export DOCKER_PASS=<DOCKER_PASS>
```

It is also recommended to set a unique version label
```bash
export VERSION=<A_UNIQUE_VERSION>
```

### Install Options

There are 4 ways to install the operator:

#### 1. Run as a Deployment inside the cluster
```bash
make prereqs manifests update-manifest update-crds subscriptions docker-build docker-push deploy

OR 

make full-dev-install
```

This will 
1. Apply necessary prereqs. (Namespace, operatorgroup)
2. Update manifests via kubebuilder
3. Updates CRDs and retrieves latest image manifests
4. Applies community operator subscriptions for Hive, AppSub, and ClusterManager
5. Builds and pushes MCH Operator dev image
6. Deploys operator via kustomize from `config/manager`

#### 2. Run locally outside the cluster
This method is preferred during development cycle to deploy and test faster. LeaderElectionNamespace line in main.go must be uncommented. POD_NAMESPACE must be set.

```bash
make run
```

This will 
1. Run the go application from main.go directly against targetting cluster

#### 3. Build and deploy catalog image to deploy operator via subscription
OLM will manage creation of most resources required to run the operator. This method builds and pushes an actual index image.

```bash
make manifests generate bundle bundle-build bundle-push catalog-build catalog-push prereqs subscriptions catalog

OR

make full-catalog-install
```

This will 
1. Update manifests via kubebuilder
2. Bundle the operator
3. Build and push the bundle image
4. Build and push the catalog image
5. Apply prereqs (Namespace, operatorgroup)
6. Applies community operator subscriptions for Hive, AppSub, and ClusterManager
7. Deploys Operator by deploying Catalogsource and subscription

### Deploy MultiClusterHub instance
Once the operator is installed in the cluster, initiate an installation by creating an instance of MultiClusterHub. To create a default instance of MultiClusterHub:
```bash
make cr
```
> To customize the instance, first modify the spec in `config/samples/operator_v1_multiclusterhub.yaml`.

## Cleanup
Delete multiclusterhub instance if it exists
```bash
kubectl delete mch --all
```

Clean up the operator and its resources:
```bash
make undeploy
```

If not all resources are properly cleaned up, follow the uninstall instructions at https://github.com/open-cluster-management/deploy to manually clean up remaining resources.


[go_tool]:https://golang.org/dl/
[osdk]:https://github.com/operator-framework/operator-sdk/releases
[opm]:https://github.com/operator-framework/operator-registry/releases
