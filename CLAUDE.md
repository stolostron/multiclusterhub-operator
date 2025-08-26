# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is the MultiClusterHub Operator, a Kubernetes operator that manages the installation of Open Cluster Management (OCM) on Red Hat OpenShift Container Platform. The operator uses Helm charts and controller-runtime to deploy and manage various OCM components.

## Development Commands

### Build & Test
```bash
make build                    # Build manager binary
make test                     # Run unit tests with coverage (NOTE: Use 10-minute timeout - this command takes a while)
make test-prep                # Prepare environment for running tests
make docker-build             # Build Docker image
make podman-build             # Build Podman image
make podman-push              # Push Podman image to {IMG} (defaults to quay.io/stolostron)
make fmt                      # Run go fmt
make vet                      # Run go vet
```

### Code Generation
```bash
make manifests                # Generate CRDs, ClusterRoles, and WebhookConfigurations
make generate                 # Generate DeepCopy methods
```

### Development Testing
```bash
make unit-tests               # Run unit tests (alias for test)
make prep-mock-install        # Build and push mock image for testing
make mock-install             # Install operator with mock components
make mock-cr                  # Create MultiClusterHub Custom Resource
make uninstall                # Remove all Hub components
```

### Deployment
```bash
make install                  # Install CRDs into cluster
make deploy                   # Deploy controller to cluster
make undeploy                 # Remove controller from cluster
```

## Architecture

### Core Components
- **controllers/**: Main reconciliation logic for MultiClusterHub resources
- **api/v1/**: CRD definitions and webhook validation for MultiClusterHub
- **pkg/**: Core business logic packages:
  - `deploying/`: Deployment orchestration
  - `rendering/`: Helm template rendering
  - `manifest/`: Image manifest management
  - `multiclusterengine/`: MCE operator integration
  - `overrides/`: Image and configuration overrides
  - `templates/`: Helm chart templates for all OCM components

### Template Structure
The `pkg/templates/charts/toggle/` directory contains Helm charts for individual OCM components:
- `console/`: OCM web console
- `grc/`: Governance, Risk, and Compliance
- `search-v2-operator/`: Cluster search functionality
- `multicluster-observability-operator/`: Observability stack
- `cluster-backup/`: Backup and restore capabilities
- And many others...

### Key Patterns
- Uses controller-runtime for Kubernetes controller implementation
- Helm-based templating for component deployment
- Image override system via ConfigMaps for development
- Webhook validation for MultiClusterHub resources
- Status reporting through custom resource conditions

## Environment Variables

Required for development:
- `MOCK_IMAGE_REGISTRY`: Public registry for mock images
- `HUB_IMAGE_REGISTRY`: Public registry for operator image
- `OPERATOR_VERSION`: Version for testing (default: 9.9.9)
- `CRDS_PATH`: Path to CRDs (default: "bin/crds")
- `POD_NAMESPACE`: Operator namespace (default: "open-cluster-management")

Used for smoketesting an image:
- `IMG`: Set this in front of `make podman-build` or `make podman-push` to build/push the image to a custom location for smoketesting

## Pre-approved Commands

The following commands are safe to run without prompting:

### Read-only Operations
```bash
kubectl get [resource] [flags]          # Safe to get any Kubernetes resources
kubectl logs [resource] [flags]         # Safe to view logs from any resource  
kubectl describe [resource] [flags]     # Safe to describe any Kubernetes resources
kubectl top [resource] [flags]          # Safe to view resource usage metrics
grep [pattern] [files]                  # Safe to search within files
```

### Git Operations
```bash
git status
git log --oneline -10
git branch
git diff
git config user.name                    # Safe to read git user configuration
git config user.email                   # Safe to read git user configuration
```

### Safe Analysis Commands
```bash
make --dry-run [target]
which [command]
ls -la [directory]
```

## Development Notes

- Go version 1.24.4 minimum required
- Uses Ginkgo/Gomega for testing framework
- Supports both Docker and Podman for container builds
- Development testing uses mock components to avoid full OCM deployment
- All commits must be signed off with DCO
- **Important: `make test` often takes 5-10 minutes - use extended timeout (600000ms) when running**

### Known Log Messages (Actual Issues)
- **"map has no entry for key"** messages in MCE and MCH operator logs indicate actual problems and should be investigated
- These messages are NOT benign and represent real issues that need attention
- **Critical**: Even when components are disabled, template rendering errors can still prevent ACM from fully spinning up properly
- These errors cause reconciliation loops that can block proper deployment despite "Available" status
- When analyzing logs, treat these as error conditions requiring investigation

### Checking MCH/MCE Operator Health

To verify MCH and MCE operators are running properly:

1. **Find active pods** (leaders consume more memory):
   ```bash
   kubectl top pods -n open-cluster-management | grep multiclusterhub-operator
   kubectl top pods -n multicluster-engine | grep multicluster-engine-operator
   ```

2. **Check logs of high-memory pods** for proper operation:
   ```bash
   kubectl logs -n open-cluster-management <active-mch-pod> --tail=10
   kubectl logs -n multicluster-engine <active-mce-pod> --tail=10
   ```

3. **Look for healthy indicators**:
   - "Reconcile completed. Requeuing after 5m0s" near the end of logs
   - Normal operations like "using trust bundle configmap" and "ManagedCluster" activities
   - No repeated error messages or reconciliation failures

**Note**: Operators run in leader/follower pairs - always check the high-memory pod as it's the active leader.

## Behavioral Guidelines

### Explanatory vs Action Requests
When the user asks explanatory questions, they are requesting **dry-run explanations only**, not actions. Explanatory questions include patterns like:
- "how would you..." / "how do you..." / "how does..."
- "what would you..." / "what do you..." / "what happens when..."
- "how would I..." / "how do I..." / "how can I..."
- "what's the best way to..." / "how should I..."
- Questions about processes, workflows, or "what if" scenarios

For these questions, do not perform actual actions even if you have permission. Instead:
- Explain the approach you would take
- Show the specific changes you would make (e.g., code snippets, file modifications)
- Wait for explicit confirmation before making actual changes

Only take direct action when given imperative commands like "create", "update", "run", "build", etc.

### Long-Running Commands
When the user asks you to run `make test`, automatically spawn a parallel agent to handle this task since it takes 5-10 minutes. Use the Task tool with:
- description: "Run make test"
- prompt: "Run 'make test' command in the multiclusterhub-operator repository. This command takes 5-10 minutes to complete. Report back the full output including any test failures, coverage results, and final status."
- subagent_type: "general-purpose"

This allows the main session to continue working while tests run in parallel.

## When writing commit messages
- Include a signoff message for the developer in the format "Signed-off-by: {user.name} <{user.email}>