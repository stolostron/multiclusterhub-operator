# MultiClusterHub Operator

The MultiCusterHub operator manages the install of Open Cluster Management (OCM) on RedHat Openshift Container Platform

## Quick Install

For a standard installation of Open Cluster Management, follow the instructions in the [deploy repo][deploy]. To install directly from this repository, see the [installation guide][install_guide].

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
kubectl annotate mch <mch-name> mch-pause=false --overwrite
```

[install_guide]: /docs/installation.md
[deploy]: https://github.com/open-cluster-management/deploy