# MultiCloudHub Operator

Build with [operator-sdk](https://github.com/operator-framework/operator-sdk) [v0.13.0+](https://github.com/operator-framework/operator-sdk/releases)

## Quick Install Instructions

### Declare Required Variables

```bash
export DOCKER_USER=<DOCKER_USER>
export DOCKER_PASS=<DOCKER_PASS>
```

### Optional

```bash
export CONTAINER_ENGINE=<CONTAINER_ENGINE>
```

### Install Dependencies and Subscribe

```bash
make deps subscribe
```

### Install Manually

Before running the command below, update the namespace in the following file [deploy/kustomization.yaml](deploy/kustomization.yaml) to reflect your targeted namespace.

```bash
make deps subscribe
```

## All Make Targets

## Download Dependancies

```bash
make deps
```

## Test

```bash
make test
```

## Build image

```bash
make image
```

## Subscribe to Operator on OperatorHub

```bash
make subscribe
```

## Manually Install Operator

```bash
make install
```

## Run Operator Locally

```bash
make local
```

## Deploy MultiCloudHub operator

```bash
kubectl create namespace multicloud-system

kubectl -k deploy
```

### Deploy MultiCloudHub operator on OCP as a custom operators

```bash
make olm-catalog

oc create namespace multicloud-system

oc apply -f build/_output/olm/multicloudhub.resources.yaml

cat <<EOF | oc -n multicloud-system apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: multicloudhub-operator
spec:
  channel: alpha
  installPlanApproval: Automatic
  name: multicloudhub-operator
  source: multicloudhub-operator-registry
  sourceNamespace: multicloud-system
  startingCSV: multicloudhub-operator.v0.0.1
EOF
```

or after the `multicloudhub.resources.yaml` is applied, deploy the operator in OCP OperatorHub

## Deploy MultiCloudHub

> Note: the etcd and mongo need to be installed in advance

```bash
kubectl -n multicloud-system apply -f deploy/crds/operators.open-cluster-management.io_v1alpha1_multicloudhub_cr.yaml
```
