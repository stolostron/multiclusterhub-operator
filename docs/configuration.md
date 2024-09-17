[comment]: # ( Copyright Contributors to the Open Cluster Management project )

# MultiClusterHub Configurations

This directory contains examples that cover various configurations for MultiClusterHub.

## Configurations

### Custom pull secret

```yaml
spec:
  imagePullSecret: "quay-secret"
```

### Minimum availability installation

```yaml
spec:
  availabilityConfig: "Basic"
```

### HA installation with node selector

```yaml
spec:
  availabilityConfig: "High"
  nodeSelector:
      diskType: ssd
```

> The instance is installed with High availability by default if not otherwise specified

### (Deprecated) Specify ingress SSL ciphers to support

```yaml
spec:
  ingress:
    sslCiphers:
    - "ECDHE-ECDSA-AES128-GCM-SHA256"
    - "ECDHE-RSA-AES128-GCM-SHA256"
```

### (Deprecated) Install Cert Manager in its own namespace

```yaml
spec:
  separateCertificateManagement: true
```

### Specific image pull policy

```yaml
spec:
  overrides: true
    imagePullPolicy: "IfNotPresent"
```

## Dev Configurations

### Custom image repository

```yaml
apiVersion: operator.open-cluster-management.io/v1
kind: MultiClusterHub
metadata:
  name: multiclusterhub
  namespace: open-cluster-management
  annotations:
    "installer.open-cluster-management.io/image-repository": "quay.io/stolostron"
```

### Disable install operator actions

```yaml
apiVersion: operator.open-cluster-management.io/v1
kind: MultiClusterHub
metadata:
  name: multiclusterhub
  namespace: open-cluster-management
  annotations:
    "installer.open-cluster-management.io/pause": "true"
```
