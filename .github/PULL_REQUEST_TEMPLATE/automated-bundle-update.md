# Automated Operator Bundle/Chart Update

_This PR was generated automatically by a scheduled workflow. Do not hand-edit
this branch directly — fix the source (the upstream component repo or the
`hack/bundle-automation` tooling) and let the workflow regenerate it instead._

## Triggered by

<!-- AUTOMATION:TRIGGER -->

## What changed

<!-- AUTOMATION:DIFFSTAT -->

## Manifests skipped during CRD scan

_Files listed here could not be parsed as plain YAML (for example, a bundle
manifest that embeds Helm/Go template syntax) and were skipped rather than
failing the whole run. This does **not** necessarily mean anything is wrong —
see the [addCRDs() docs](https://github.com/stolostron/installer-dev-tools/blob/main/scripts/bundle-generation/bundles-to-charts.py)
for why this can't be resolved by filename alone — but it does mean nobody
has confirmed the skipped file doesn't define a CRD._

<!-- AUTOMATION:WARNINGS -->

## Review checklist

- [ ] Diff matches the expected upstream changes for this branch/component
- [ ] No unexpected CRD, RBAC, or webhook changes
- [ ] If any files are listed above, confirm none of them define a `CustomResourceDefinition`
- [ ] Remove `do-not-merge/hold` only after the above are confirmed
