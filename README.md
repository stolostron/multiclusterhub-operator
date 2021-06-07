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
kubectl annotate mch <mch-name> mch-pause=true
```

Remove or edit this annotation to resume installer operations
```bash
kubectl annotate mch <mch-name> mch-pause=false --overwrite
```

### Add Image Overrides Via Configmap  

Developer image overrides can be added by specifiying a configmap containing the overrides for the MCH resource. This configmap must be in the same namespace as the MCH resource.

This is done by creating a configmap from a new [manifest](https://github.com/open-cluster-management/pipeline/tree/2.3-integration/snapshots). A developer may use this to override any 1 or all images.


If overriding individual images, the minimum required parameters required to build the image reference are - 

- `image-name`
- `image-remote`
- `image-key`
- `image-digest` or `image-tag`, both can optionally be provided, if so the `image-digest` will be preferred.


```bash
kubectl create configmap <my-config> --from-file=docs/examples/manifest-oneimage.json # Override 1 image example
kubectl create configmap <my-config> --from-file=docs/examples/manifest-allimages.json # Overriding all images example

kubectl annotate mch <mch-name> --overwrite mch-imageOverridesCM=<my-config> # Provide the configmap as an override to the MCH
```

To remove this annotation to revert back to the original manifest
```bash
kubectl annotate mch <mch-name> mch-imageOverridesCM- --overwrite # Remove annotation
kubectl delete configmap <my-config> # Delete configmap
```

If editing the configmap directly instead of creating/deleting it each time, an operator reconcile may be necessary in order to get the changes to take effect. This can be done by cycling the MCH Operator pod

```
kubectl delete pod multiclusterhub-operator-xxxxx-xxxxx
```
### Other Development Documents

[install_guide]: /docs/installation.md
[config_guide]: /docs/configuration.md
[deploy]: https://github.com/open-cluster-management/deploy
