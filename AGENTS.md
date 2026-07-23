# multiclusterhub-operator — Agent Instructions

This repository contains the multiclusterhub-operator for Red Hat Advanced Cluster Management (RHACM).

## What this operator does

The multiclusterhub-operator is the main operator for installing and managing the MultiClusterHub (MCH) resource. It orchestrates the deployment of RHACM components on an OpenShift cluster, including:

- Installing and managing ACM operator subscriptions
- Deploying ACM platform components (console, observability, policy framework, etc.)
- Managing component lifecycle (upgrades, reconciliation)
- Configuring platform-wide settings (custom CA certificates, node selectors, etc.)

## Repository layout

- `api/` - CRD definitions for MultiClusterHub
- `controllers/` - Operator reconciliation logic
- `pkg/` - Shared packages (rendering, utilities, templates)
- `templates/` - Helm charts and manifests for ACM components
- `test/` - Integration and functional tests
- `config/` - Operator deployment manifests (CRDs, RBAC, deployment)

## Development workflow

### Building locally

```bash
make build        # Build operator binary
make docker-build # Build operator image
```

### Running locally

```bash
# Run operator outside cluster (for development)
make run

# Deploy operator to cluster
make deploy
```

### Testing

```bash
make test              # Run unit tests
make functional-test   # Run functional tests
```

## Dependencies

- **OpenShift 4.x** - Target platform
- **Operator SDK** - Operator framework
- **Helm** - Template rendering
- **Kustomize** - Manifest generation

## Documentation

- [MultiClusterHub API Reference](https://access.redhat.com/documentation/en-us/red_hat_advanced_cluster_management_for_kubernetes/)
- [ACM Component Registry](https://github.com/stolostron/acm-config/blob/main/product/component-registry.yaml)
- [Operator Development Guide](docs/)

## Common tasks

### Deploy a development build

```bash
export QUAY_USER=<your-quay-username>
make docker-build docker-push deploy
```

### Test MCH resource changes

```bash
# Edit sample MCH
vi config/samples/operator_v1_multiclusterhub.yaml

# Apply to cluster
oc apply -f config/samples/operator_v1_multiclusterhub.yaml
```

### Debug operator logs

```bash
oc logs -n multicluster-engine deployment/multiclusterhub-operator -f
```
