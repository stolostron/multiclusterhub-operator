# MultiCloudHub Operator

Build with [operator-sdk](https://github.com/operator-framework/operator-sdk) [v0.13.0+](https://github.com/operator-framework/operator-sdk/releases)

## Test

```bash
make test
```

## Build image

```bash
make image
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
kubectl -n multicloud-system apply -f deploy/crds/operators.multicloud.ibm.com_v1alpha1_multicloudhub_cr.yaml
```