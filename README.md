# MultiClusterHub Operator

The MultiCusterHub operator manages the install of Open Cluster Management (OCM) on RedHat Openshift Container Platform

## Quick Install

For a standard installation of Open Cluster Management, follow the instructions at https://github.com/open-cluster-management/deploy. For more details on how to do a custom installation with code from this repository, see the [installation guide][install_guide].

## Cleanup
Delete multiclusterhub instance if it exists
```bash
kubectl delete mch --all
```

Clean up the operator and its resources:
```bash
make uninstall
```

If not all resources are properly cleaned up, follow the uninstall instructions at https://github.com/open-cluster-management/deploy to manually clean up remaining resources.


## Useful Make Targets

- `make image`: Build the image
- `make push`: Push built image to Quay
- `make secrets`: Generate secrets needed for install
- `make cr`: Apply basic multiclusterhub instance
- `make deps`: Installs operator sdk and opm

## Disabling MultiClusterHub Operator

Once installed, the hub operator will monitor changes in the cluster that affect an instance of the multiclusterhub (mch) and reconcile deviations to maintain a desired state. To stop the installer from making these changes you can apply an annotation to the mch instance.
```bash
kubectl annotate mch <mch-name> mch-pause=true
```

Remove or edit this annotation to resume installer operations
```bash
kubectl annotate mch <mch-name> mch-pause=false
```

[install_guide]: /docs/installation.md