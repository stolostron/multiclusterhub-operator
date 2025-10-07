# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## IMPORTANT: Pre-approved Commands

**NEVER PROMPT FOR THESE COMMANDS - RUN IMMEDIATELY:**

```bash
git status                              # Check git working directory status
git log --oneline -10                   # View recent commit history
git branch                              # List git branches
git diff                                # Show git changes
git config user.name                    # Get git user name for commits
git config user.email                   # Get git user email for commits
kubectl get [resource] [flags]          # Get Kubernetes resources
kubectl logs [resource] [flags]         # View Kubernetes pod/container logs
kubectl describe [resource] [flags]     # Describe Kubernetes resources
kubectl top [resource] [flags]          # View Kubernetes resource usage
grep [pattern] [files]                  # Search within files
make --dry-run [target]                 # Show what make would do without executing
which [command]                         # Show location of command
ls -la [directory]                      # List directory contents with details
```

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

## Git Repository Workflow
This repository typically involves working with forks. When performing git operations:

1. **Identify Repository Type**: Always check which remote is the fork vs. upstream by examining remote URLs:
   ```bash
   git remote -v
   ```
   Look for URLs containing personal usernames (forks) vs. organization names (upstream).
   
2. **Find Fork Remote**: Check for common fork remote names in this order:
   - `fork` (preferred for this user)
   - `origin` (if it points to a personal fork)
   - Any remote with a personal username in the URL
   
3. **Push to Fork**: When pushing branches for PRs, prefer pushing to the fork remote:
   ```bash
   git push -u [fork-remote-name] [branch-name]
   ```
   
4. **Upstream Pushes**: Generally prefer pushing to fork remotes over upstream (organization) remotes, though direct upstream pushes may sometimes be necessary

## Development Notes

- Go version 1.24.6 minimum required
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

## Live Cluster Deployment Updates

### Updating Running Operator via CSV
When deploying custom operator images to test code changes, update the CSV (not the deployment directly):

**Important**: The CSV contains the image reference for the MCH operator deployment. MCE likely has a similar CSV structure. ACM CSV can be in various namespaces.

```bash
# Find the ACM CSV and namespace
kubectl get csv -A | grep advanced-cluster-management

# Look for the custom image line to identify the correct path
kubectl get csv <csv-name> -n <namespace> -o yaml | grep -n "<custom-registry-pattern>"

# Update image in CSV 
kubectl patch csv <csv-name> -n <namespace> --type='json' \
  -p='[{"op": "replace", "path": "/spec/install/spec/deployments/0/spec/template/spec/containers/0/image", "value": "<desired-image>"}]'
```

**Note**: Always ask the user for the desired image registry/tag when smoketesting, as custom images may be in various registries.

### Smoketest Verification Commands
Quick health checks after deployment changes:

```bash
# Check MCH/MCE status - MCH should show "Running", MCE should show "Available" when ready
kubectl get multiclusterhub -A && kubectl get multiclusterengine -A

# Find any crashlooping pods across all namespaces
kubectl get pods -A | grep -E "(CrashLoopBackOff|Error|Pending)"
```

## Pull Request Guidelines

When creating pull requests, follow these reviewer assignment guidelines:
- Among `cameronmwall`, `dislbenn`, and `ngraham20`, the submitter should `/cc` the other two
- If the submitter is not one of these three, `/cc` all three of them

## Custom Bundle and Catalog Development

### Quick Bundle Development Workflow

For testing operator changes with custom bundles:

1. **Make code changes** in multiclusterhub-operator
2. **Build operator image**:
   ```bash
   IMG=quay.io/YOUR_USERNAME/multiclusterhub-operator:2.15.0-custom-v1 make podman-build
   podman push quay.io/YOUR_USERNAME/multiclusterhub-operator:2.15.0-custom-v1
   ```

3. **Clone and modify bundle**:
   ```bash
   git clone https://github.com/stolostron/acm-operator-bundle.git
   cd acm-operator-bundle
   # Update CSV with your operator image and OPERATOR_VERSION
   ```

4. **Build and push bundle**:
   ```bash
   podman build -t quay.io/YOUR_USERNAME/acm-custom-bundle:2.15.0-custom-v1 .
   podman push quay.io/YOUR_USERNAME/acm-custom-bundle:2.15.0-custom-v1
   ```

5. **Create catalog structure** in main repo:
   ```bash
   mkdir -p custom-catalog/advanced-cluster-management
   # Create bundles.yaml, channel.yaml, package.yaml, Dockerfile
   ```

6. **Build and push catalog**:
   ```bash
   cd custom-catalog
   podman build -t quay.io/YOUR_USERNAME/acm-custom-catalog:v1.0.0 .
   podman push quay.io/YOUR_USERNAME/acm-custom-catalog:v1.0.0
   ```

7. **Deploy to cluster**:
   ```bash
   kubectl apply -f catalogsource.yaml
   kubectl apply -f subscription.yaml
   ```

### Critical Requirements

**Version Synchronization**: These MUST match exactly:
- Operator image tag: `2.15.0-custom-v1` 
- CSV OPERATOR_VERSION env var: `2.15.0-custom-v1`
- Bundle image tag: `2.15.0-custom-v1`

**Naming Consistency**: These MUST match exactly:
- CSV metadata.name: `advanced-cluster-management.v2.15.0-custom`
- Bundle name in bundles.yaml: `advanced-cluster-management.v2.15.0-custom`
- Channel entries.name: `advanced-cluster-management.v2.15.0-custom`
- Subscription startingCSV: `advanced-cluster-management.v2.15.0-custom`

### Troubleshooting

**Image-Code Mismatch**: If changes don't work, verify operator image contains your code:
- Always increment image tag after code changes
- Update both image reference AND OPERATOR_VERSION in CSV

**Subscription Issues**: If CSV won't install:
- Delete and recreate subscription to get fresh install plan
- Check catalog source is healthy: `kubectl get catalogsource -n openshift-marketplace`

**Missing Status Fields**: If new CRD fields don't appear:
- Copy updated CRDs to bundle manifests
- Apply CRDs to cluster: `kubectl apply -f config/crd/bases/operator.open-cluster-management.io_multiclusterhubs.yaml`

### Documentation References

See `CLONED_BUNDLE_GUIDE.md` and `CUSTOM_BUNDLE_GUIDE.md` for complete step-by-step instructions.
## When writing commit messages
- Include a signoff message for the developer in the format "Signed-off-by: {user.name} <{user.email}>