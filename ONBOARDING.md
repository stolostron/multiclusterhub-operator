# Onboarding New Components to Backplane Operator

This guide explains how to onboard new OLM or Helm components to the backplane operator.

## Overview

**Important:** Component onboarding is done **locally** by contributors, not via automated CI/CD. This approach:
- ✅ Allows you to validate changes before submitting
- ✅ Eliminates security risks from automated workflows
- ✅ Gives you full control over generated files
- ✅ Follows standard PR review process

## Prerequisites

### 1. Install Python Requirements

```bash
pip3 install -r hack/bundle-automation/requirements.txt
```

### 2. Install yq (YAML processor)

**macOS:**
```bash
brew install yq
```

**Linux:**
```bash
sudo wget https://github.com/mikefarah/yq/releases/download/v4.40.5/yq_linux_amd64 -O /usr/bin/yq
sudo chmod +x /usr/bin/yq
```

## Onboarding Workflow

### Step 1: Create Your Onboard Request

Create or update `onboard-request.yaml` in the repository root with your component details:

**For OLM Components:**
```yaml
onboard-type: olm
# ... your OLM component configuration
# See existing config in hack/bundle-automation/config.yaml for examples
```

**For Helm Components:**
```yaml
onboard-type: helm
# ... your Helm component configuration
# See existing config in hack/bundle-automation/charts-config.yaml for examples
```

### Step 2: Generate Charts Locally

Run the appropriate make target based on your component type:

**For OLM components:**
```bash
make regenerate-charts-from-bundles CONFIG=onboard-request.yaml
```

**For Helm components:**
```bash
make regenerate-charts CONFIG=onboard-request.yaml
```

This will:
- Read your `onboard-request.yaml`
- Generate necessary charts and manifests
- Update configuration files in `hack/bundle-automation/`
- Create/update chart files in the appropriate directories

### Step 3: Review Generated Files

Check what files were generated/modified:

```bash
git status
git diff
```

**Verify:**
- Generated charts look correct
- Configuration files were updated properly
- No unexpected files were created
- No sensitive information was accidentally included

### Step 4: Submit Pull Request

```bash
# Add all generated files
git add .

# Commit with a descriptive message
git commit -sm "Add <component-name> to backplane operator

Onboards <component-name> as <olm/helm> component.
Generated charts and updated configuration using local automation.

Signed-off-by: Your Name <your.email@example.com>"

# Push to your fork
git push origin your-branch-name
```

Create a pull request that includes:
- `onboard-request.yaml` (your component request)
- Generated charts/manifests
- Updated configuration files

### Step 5: PR Review

Maintainers will review:
- Your onboard request configuration
- Generated charts for correctness
- Compliance with repository standards
- Any security concerns

You may be asked to make adjustments. If so, update `onboard-request.yaml` locally, re-run the make command, and push the updates.

## Advanced: Using onboard-new-component

For initial component creation, you can use:

```bash
make onboard-new-component COMPONENT=<your-component-name>
```

This helps scaffold a new component configuration. You'll still need to:
1. Edit the generated configuration
2. Run the appropriate regenerate command
3. Submit a PR with the results

## Troubleshooting

### Python Dependencies Missing

```bash
# Reinstall requirements
pip3 install --upgrade -r hack/bundle-automation/requirements.txt
```

### Invalid YAML Syntax

Validate your `onboard-request.yaml`:

```bash
yq eval '.' onboard-request.yaml
```

If this fails, you have a YAML syntax error.

### Make Command Fails

Check the error message carefully:
- Missing required fields in `onboard-request.yaml`?
- Invalid `onboard-type` (must be `olm` or `helm`)?
- Network issues downloading bundle data?
- Permissions issues writing files?

### Generated Files Look Wrong

Compare with existing components in:
- `hack/bundle-automation/config.yaml` (OLM components)
- `hack/bundle-automation/charts-config.yaml` (Helm components)

Ensure your `onboard-request.yaml` follows the same structure.

## Available Make Targets

| Target | Description | Usage |
|--------|-------------|-------|
| `regenerate-charts-from-bundles` | Generate charts from OLM bundles | `make regenerate-charts-from-bundles CONFIG=onboard-request.yaml` |
| `regenerate-charts` | Generate Helm charts | `make regenerate-charts CONFIG=onboard-request.yaml` |
| `onboard-new-component` | Scaffold new component | `make onboard-new-component COMPONENT=mycomponent` |
| `install-requirements` | Install Python dependencies | `make install-requirements` |

## Additional Parameters

All targets support additional parameters for customization:

```bash
# Specify organization and repository
make regenerate-charts \
  CONFIG=onboard-request.yaml \
  ORG=myorg \
  REPO=myrepo \
  BRANCH=main
```

**Available parameters:**
- `ORG` - GitHub organization (default: `stolostron`)
- `REPO` - Repository name (default: `installer-dev-tools`)
- `BRANCH` - Branch name (default: `main`)
- `COMPONENT` - Component name (for onboard-new-component)
- `CONFIG` - Path to onboard request file

## Examples

### Example 1: Onboarding an OLM Component

```bash
# 1. Create onboard-request.yaml
cat > onboard-request.yaml <<EOF
onboard-type: olm
example-operator:
  bundle_repo: quay.io/example/example-operator-bundle
  bundle_version: v1.0.0
EOF

# 2. Generate charts
make regenerate-charts-from-bundles CONFIG=onboard-request.yaml

# 3. Review
git status
git diff

# 4. Commit and push
git add .
git commit -sm "Add example-operator to backplane

Signed-off-by: Your Name <your@email.com>"
git push origin add-example-operator
```

### Example 2: Onboarding a Helm Component

```bash
# 1. Create onboard-request.yaml
cat > onboard-request.yaml <<EOF
onboard-type: helm
example-chart:
  chart_repo: https://charts.example.com
  chart_name: example
  chart_version: 2.0.0
EOF

# 2. Generate charts
make regenerate-charts CONFIG=onboard-request.yaml

# 3. Review, commit, and push (same as above)
```

## Why Local Instead of CI/CD?

**Security:** Running automated workflows with repository secrets creates attack vectors:
- Arbitrary code execution vulnerabilities
- Secret exfiltration risks
- Supply chain compromise potential

**Simplicity:** Local generation is straightforward:
- No complex CI/CD configuration
- No environment setup in GitHub
- No approval gates

**Control:** You validate before submitting:
- See exactly what's generated
- Iterate quickly on failures
- Ensure correctness before review

## Getting Help

If you encounter issues:

1. **Check existing components** in `hack/bundle-automation/config.yaml` or `charts-config.yaml`
2. **Review the script** at `hack/bundle-automation/generate-shell.py`
3. **Ask in your PR** - maintainers will help
4. **Open an issue** for documentation improvements

## Related Documentation

- [CONTRIBUTING.md](CONTRIBUTING.md) - General contribution guidelines
- [README.md](README.md) - Repository overview
- `hack/bundle-automation/` - Automation scripts and configurations
