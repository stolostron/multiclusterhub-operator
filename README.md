[comment]: # ( Copyright Contributors to the Open Cluster Management project )

# WORK IN PROGRESS

We are in the process of enabling this repo for community contribution. See wiki [here](https://open-cluster-management.io/concepts/architecture/).

# MultiClusterHub Operator

The MultiCusterHub operator manages the install of Open Cluster Management (OCM) on RedHat Openshift Container Platform

## Contributing

For steps on how to contribute and test the MultiClusterHub Operator component, see [CONTRIBUTING](./CONTRIBUTING.md) guidelines.

## Development Tools

### Disabling MultiClusterHub Operator

Once installed, the hub operator will monitor changes in the cluster that affect an instance of the multiclusterhub (mch) and reconcile deviations to maintain desired state. To stop the installer from making these changes you can apply an annotation to the mch instance.

```bash
kubectl annotate mch <mch-name> installer.open-cluster-management.io/pause=true
```

Remove or edit this annotation to resume installer operations

```bash
kubectl annotate mch <mch-name> installer.open-cluster-management.io/pause=false --overwrite
```

### Add Image Overrides Via Configmap  

Developer image overrides can be added by specifiying a configmap containing the overrides for the MCH resource. This configmap must be in the same namespace as the MCH resource.

This is done by creating a configmap from a new [manifest](https://github.com/stolostron/pipeline/tree/2.7-integration/snapshots). A developer may use this to override any 1 or all images.

If overriding individual images, the minimum required parameters required to build the image reference are -

- `image-name`
- `image-remote`
- `image-key`
- `image-digest` or `image-tag`, both can optionally be provided, if so the `image-digest` will be preferred.

```bash
kubectl create configmap <my-config> --from-file=docs/examples/manifest-oneimage.json # Override 1 image example
kubectl create configmap <my-config> --from-file=docs/examples/manifest-allimages.json # Overriding all images example

kubectl annotate mch <mch-name> --overwrite installer.open-cluster-management.io/image-overrides-configmap=<my-config> # Provide the configmap as an override to the MCH
```

To remove this annotation to revert back to the original manifest

```bash
kubectl annotate mch <mch-name> installer.open-cluster-management.io/image-overrides-configmap --overwrite # Remove annotation
kubectl delete configmap <my-config> # Delete configmap
```

If editing the configmap directly instead of creating/deleting it each time, an operator reconcile may be necessary in order to get the changes to take effect. This can be done by cycling the MCH Operator pod

```bash
kubectl delete pod multiclusterhub-operator-xxxxx-xxxxx
```

### Overriding MultiCluster Engine Subscription

The multicluster engine subscription is stood up by default as part of a standard MCH installation. The spec of the subscription can be overriden by providing the following annotation to the MCH resource. One or many parameters can be provided from the ones listed in the `installer.open-cluster-management.io/mce-subscription-spec` annotation below

```yaml
apiVersion: operator.open-cluster-management.io/v1
kind: MultiClusterHub
metadata:
  annotations:
    installer.open-cluster-management.io/mce-subscription-spec: '{"channel": "stable-2.0","installPlanApproval": "Manual","name":
      "multicluster-engine","source": "multiclusterengine-catalog","sourceNamespace": "catalogsourcenamespace","startingCSV":
      "csv-1.0"}'
  name: multiclusterhub
spec: {}
```

### Overriding OADP Operator Subscription

The OADP operator is installed from redhat-operators by the cluster-backup chart. The spec of the subscription can be overriden by providing the following annotation to the MCH resource. One or many parameters can be provided from the ones listed in the `installer.open-cluster-management.io/oadp-subscription-spec` annotation below

```yaml
apiVersion: operator.open-cluster-management.io/v1
kind: MultiClusterHub
metadata:
  annotations:
    installer.open-cluster-management.io/oadp-subscription-spec: '{"channel": "stable-1.0","installPlanApproval": "Automatic","name":
      "redhat-oadp-operator","source": "redhat-operators","sourceNamespace": "openshift-marketplace","startingCSV":
      "oadp-operator.v1.0.2"}'
  name: multiclusterhub
spec: {}
```

Setting OADP annotation via CLI

```bash
oc annotate mch multiclusterhub installer.open-cluster-management.io/oadp-subscription-spec='{"channel":"stable-1.0","installPlanApproval":"Automatic","name":"redhat-oadp-operator","source":"redhat-operators","sourceNamespace":"openshift-marketplace","startingCSV": "oadp-operator.v1.0.2"}'
```

### Ignore OCP Version Requirement

The operator defines a minimum version of OCP it can run in to avoid unexpected behavior. If the OCP environment is below this threshold then the MCH instance will report failure early on. This requirement can be ignored in the following two ways

1. Set `DISABLE_OCP_MIN_VERSION` as an environment variable. The presence of this variable in the container the operator runs will skip the check.

2. Set `installer.open-cluster-management.io/ignore-ocp-version` annotation in the MCH instance.

```bash
kubectl annotate mch <mch-name> installer.open-cluster-management.io/ignore-ocp-version=true
```

### Ignore MCE Version Requirement

After deploying MCE the operator waits for MCE install to complete and verifies it is running at a minimum version. To ignore this version check:

1. Set `DISABLE_MCE_MIN_VERSION` as an environment variable. With this set the operator will only check that MCE has set its currentVersion status.

### Other Development Documents

- [Installation Guide](/docs/installation.md)
- [Configuration Guide](/docs/configuration.md)
- [Deploy automation](https://github.com/stolostron/deploy)
