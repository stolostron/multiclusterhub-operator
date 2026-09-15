---
name: transition-to-release
description: Bump the multiclusterhub-operator to a new MCH release version (e.g. 5.0 -> 5.1) — updates COMPONENT_VERSION, MCE production/community channels, hack/bundle-automation configs, Dockerfile.rhtap, Tekton pipeline files, and every test fixture that encodes the old version/channel, then opens a draft PR linked to the release Jira epic. Trigger phrases -- "transition to 5.1", "bump MCH release version", "prepare next release", "release transition".
license: MIT
metadata:
  scope: stolostron/multiclusterhub-operator
  workflow: release-transition
---

# transition-to-release

Automates the version/channel bump multiclusterhub-operator needs whenever ACM/MCH
moves to a new release (e.g. `5.0.0` -> `5.1.0`). This is a **fully agentic** skill —
there are no helper scripts. You (the agent loading this skill) do the scanning,
reasoning, editing, validating, committing, and PR/Jira creation yourself using your
normal tools (Grep, Read, Edit, Bash, git, GitHub MCP, Jira MCP).

Read this whole file before touching anything. Several files require *semantic*,
not literal, replacement — read the "Files requiring semantic reasoning" section
carefully before editing tests.

## When NOT to use this

- Fixing a channel value that's already wrong for the *current* version (e.g. a
  one-off bug fix) is a separate task, not a "transition". Don't lump it into this
  workflow's commit — call it out to the user and offer to do it as its own commit
  first.
- Major-version jumps that also involve component migrations (e.g. moving a
  component from MCH to MCE, deprecating CRD fields) are NOT covered here. This
  skill only handles the mechanical version/channel bump. Flag anything you find
  that looks like it needs more than a string bump.

---

## Phase 1 — Inputs

1. Read `COMPONENT_VERSION` for the current version (e.g. `5.0.0`).
2. Ask the user for the new MCH version (e.g. `5.1.0`). Validate:
   - Semantic version `X.Y.Z`.
   - Strictly greater than current.
3. Derive from the new version, `{MAJOR}.{MINOR}`:
   - **MCE production channel:** `stable-{MAJOR}.{MINOR}` (e.g. `stable-5.1`).
   - **MCE community channel:** `community-{MAJOR-4}.{MINOR}` (e.g. `5.1` ->
     `community-1.1`, `5.0` -> `community-1.0`, `6.0` -> `community-2.0`).
     - **Important:** Do NOT assume the *current* community channel in the repo
       follows this formula — read it literally from
       `pkg/multiclusterengine/multiclusterengine.go`. As of writing it is
       `community-0.10`, a legacy value that predates this formula. Whatever the
       literal current string is, that's your OLD value to search for; the NEW
       value is always computed fresh from the formula above. If you see the repo
       still has the legacy `community-0.X` value and the version bump you're
       doing is NOT the one that corrects it, tell the user this looks stale and
       ask whether to correct it in the same PR or a separate one first.
4. Confirm the calculated channels back to the user before scanning ("MCE prod:
   `stable-5.1`, community: `community-1.1` — correct?").
5. Search Jira for an existing release epic (`project = ACM AND type = Epic AND
   summary ~ "{NEW_MAJOR}.{NEW_MINOR}"` or similar). If found, confirm with the
   user you'll link the PR to it. If not found, ask whether to create one
   (`MCH {NEW_VERSION} Release`) or proceed without.

---

## Phase 2 — Scan

Don't trust a static file list from a previous release — the codebase drifts.
Re-derive the file list every run with fresh greps against `{OLD_MAJOR}.{OLD_MINOR}`,
`stable-{OLD_MAJOR}.{OLD_MINOR}`, the literal old community-channel string, and
`release-{OLD_MAJOR}.{OLD_MINOR}`:

```
grep -rn "stable-{OLD_MAJOR}\.{OLD_MINOR}"        --include="*.go" --include="*.yaml" .
grep -rn "{OLD_COMMUNITY_CHANNEL_LITERAL}"         --include="*.go" --include="*.yaml" .
grep -rln "release-{OLD_MAJOR}\.{OLD_MINOR}"       --include="*.yaml" .
grep -rn  "acm-release-version"                    --include="*.yaml" .
grep -n   "PIPELINE_BRANCH"                        Makefile.dev
grep -n   "RequiredMCEVersion"                     pkg/version/version.go pkg/version/version_test.go
grep -n   "cpe=\"cpe:/a:redhat:acm:"                build/Dockerfile.rhtap
ls .tekton/ | grep "acm-{OLD_MAJOR}{OLD_MINOR}\|acm-{NEW_MAJOR}{NEW_MINOR}"
```

As of this writing, that produces the following file set. Treat it as a starting
point, not gospel — always confirm against a fresh scan.

### Files safe for direct literal replacement

| # | File | Change |
|---|------|--------|
| 1 | `COMPONENT_VERSION` | `{OLD_VERSION}` -> `{NEW_VERSION}` |
| 2 | `Makefile.dev` | `PIPELINE_BRANCH ?= {OLD_MAJOR}.{OLD_MINOR}-integration` -> `{NEW_MAJOR}.{NEW_MINOR}-integration` |
| 3 | `pkg/multiclusterengine/multiclusterengine.go` | `MCEProdChannel = "stable-{OLD}"` -> `"stable-{NEW}"`; `MCECommunityChannel = "{OLD_COMMUNITY_LITERAL}"` -> `"community-{NEW_COMMUNITY}"` |
| 4 | `pkg/version/version.go` | `var RequiredMCEVersion = "{OLD_VERSION}"` -> `"{NEW_VERSION}"` |
| 5 | `hack/bundle-automation/charts-config.yaml` | `acm-release-version: '{OLD_MAJOR}.{OLD_MINOR}'` -> `'{NEW_MAJOR}.{NEW_MINOR}'`; `branch: "release-{OLD}"` -> `"release-{NEW}"` (1 occurrence) |
| 6 | `hack/bundle-automation/config.yaml` | Same `acm-release-version` (1x) and `branch: "release-{OLD}"` — replace **every** occurrence (currently ~6, one per component block) |
| 7 | `hack/bundle-automation/copy-config.yaml` | `branch: "release-{OLD}"` -> `"release-{NEW}"` (1 occurrence) |
| 8 | `build/Dockerfile.rhtap` | `cpe="cpe:/a:redhat:acm:{OLD_MAJOR}.{OLD_MINOR}::el9"` -> `{NEW_MAJOR}.{NEW_MINOR}` |
| 9 | `pkg/multiclusterengine/olm/v0/catalog_test.go` | Two `Name: "stable-{OLD}"` / `Name: "{OLD_COMMUNITY_LITERAL}"` fixture literals tied 1:1 to the constants above — same replacement |

For these, replace **all** matching instances in the file. There's no ambiguity —
every occurrence of the old literal string in these files means "the current
version/channel", full stop.

### Files requiring semantic reasoning (do NOT blind-replace)

These test files encode *relative* version relationships (ahead/behind, higher
minor/major, compliant/non-compliant) using numbers that are NOT all "the old
version" — some are deliberately offset from it to test comparison logic. A naive
"replace {OLD} with {NEW}" will corrupt or silently no-op these. Read each test's
`name` field to understand intent, then shift *only* the values that are meant to
track the baseline, preserving each test's original relative offset.

**10. `controllers/status_test.go`, function `TestCalculateMCEVersionCompliance`**

This block tests community-channel compliance. All `RequiredChannel` fixtures use
the literal community channel string — update every one to the new community
channel. For `CurrentVersion` / `Message`, use the test `name` to decide the new
value, using `NEW_COMMUNITY_MAJOR.NEW_COMMUNITY_MINOR` as the baseline:

| test `name` | old pattern | new pattern |
|---|---|---|
| "MCE not found" / "MCE without current version" | no version set | no change |
| "MCE with exact required version" | `{OLD_BASE}.0` | `{NEW_BASE}.0` |
| "MCE with higher patch version is compliant" | `{OLD_BASE}.3` (any nonzero patch) | `{NEW_BASE}.{same patch}` |
| "MCE with higher minor version is not compliant" | `{OLD_MAJ}.{OLD_MIN+1}.0` | `{NEW_MAJ}.{NEW_MIN+1}.0` |
| "MCE with higher major version is not compliant" | `{OLD_MAJ+1}.0.0` | `{NEW_MAJ+1}.0.0` (only bump if it needs to stay "one major above"; leaving it as `{OLD_MAJ+1}.0.0` is also still valid/non-compliant, but prefer keeping it consistent with the new baseline) |
| "MCE with lower version is not compliant" | some version below baseline | **leave unchanged** — a value already below the old baseline is still below the new (higher) baseline |

Update the `Message:` string in each case to match
(`"MCE version {ver} meets/does not meet channel {channel} requirements"`) and
regenerate it consistently with the new channel/version values.

**11. `controllers/console_notification_test.go`**

Uses `stable-{OLD}` as the baseline channel throughout, plus deliberately-offset
comparison versions:

- `TestMCEComplianceBannerText_Ahead`: version is baseline **+1 minor**
  (`{OLD_MAJ}.{OLD_MIN+1}.0`) -> shift to `{NEW_MAJ}.{NEW_MIN+1}.0`. Channel
  literal `stable-{OLD}` -> `stable-{NEW}`.
- `TestMCEComplianceBannerText_Behind`: version `2.17.0` represents an
  intentionally old/unrelated ACM version demonstrating "behind" — **leave this
  value unchanged**, it's still behind any current baseline. Only the channel
  literal (`stable-{OLD}` -> `stable-{NEW}`) needs updating.
- `TestEnsureMCEComplianceBanner_NonCompliant`: `CurrentVersion`/message use
  baseline **+1 minor**, same as the "Ahead" case above — shift identically.
  `RequiredChannel` -> new channel.
- `TestEnsureMCEComplianceBanner_Compliant_RemovesBanner`: `CurrentVersion` is
  exactly the baseline (`{OLD_MAJ}.{OLD_MIN}.0`) -> shift to exactly the new
  baseline (`{NEW_MAJ}.{NEW_MIN}.0`). `RequiredChannel` -> new channel.
- `TestEnsureMCEComplianceBanner_NoVersion_NoBanner`: only `RequiredChannel`
  needs updating; `CurrentVersion` is empty string, leave as-is.
- `TestEnsureMCEComplianceBanner_UpdatesExistingBanner`: uses baseline **+2
  minor** (`{OLD_MAJ}.{OLD_MIN+2}.0`) in two places (existing banner text and the
  new compliance status) — shift both to `{NEW_MAJ}.{NEW_MIN+2}.0`, preserving
  the +2 offset. `RequiredChannel` -> new channel in both places.

The rule of thumb: every `RequiredChannel`/channel-literal reference becomes the
new channel unconditionally. Every version number gets shifted by the same delta
(`NEW_MINOR - OLD_MINOR`) *if and only if* it was at or above the old baseline
minor; numbers already below the old baseline (like `2.17.0`) are left alone.

**12. `pkg/version/version_test.go`, function `Test_ValidMCEVersion`**

| test `name` | old value | new value |
|---|---|---|
| "higher patch version" | `{OLD_MAJ}.{OLD_MIN}.5` | `{NEW_MAJ}.{NEW_MIN}.5` |
| "higher minor version rejected" | `{OLD_MAJ}.{OLD_MIN+1}.0` | `{NEW_MAJ}.{NEW_MIN+1}.0` |
| "higher major version rejected" | `{OLD_MAJ+1}.0.0` | leave unchanged (still a higher major than the new baseline too) |
| "below min" / "below min ignored" | low version (e.g. `2.1.11`) | leave unchanged |
| "no version found" | empty string | leave unchanged |
| "prerelease tag compliant (\*)" / "exact version" | built from `RequiredMCEVersion` constant via `fmt.Sprintf` | **no change needed** — these reference the constant directly and will pick up the new value automatically once you update `pkg/version/version.go` |

Do NOT touch `Test_ValidOCPVersion` in this same file (uses `ocpVersion` field,
OCP versions, unrelated to MCE — e.g. its own "above min" case with `4.99.99` is
about OCP compatibility, not MCH/MCE version).

---

## Phase 3 — Tekton pipeline files

```
OLD_TEKTON_SUFFIX = {OLD_MAJOR}{OLD_MINOR}   # e.g. "50" for 5.0
NEW_TEKTON_SUFFIX = {NEW_MAJOR}{NEW_MINOR}   # e.g. "51" for 5.1
```

1. Delete `.tekton/multiclusterhub-operator-acm-{OLD_TEKTON_SUFFIX}-pull-request.yaml`
   and `.tekton/multiclusterhub-operator-acm-{OLD_TEKTON_SUFFIX}-push.yaml`.
2. Check whether
   `.tekton/multiclusterhub-operator-acm-{NEW_TEKTON_SUFFIX}-{pull-request,push}.yaml`
   already exist.
   - If yes: leave them alone, nothing to do.
   - If no: do NOT fabricate them yourself. Tell the user these files don't exist
     yet and need to be created by the Konflux/release tooling (or copied from
     another branch) — this is out of scope for a mechanical string bump.

---

## Phase 4 — Preview & approval

Before editing anything, show the user a concise table of every file and the
exact before/after for each change (including the semantic test-file shifts
worked out above). Ask for explicit approval ("yes" / "no" / "let me review
details first") before touching any file.

## Phase 5 — Apply

Use the Edit tool per file (not sed) so you can verify each replacement lands
correctly and handle the semantic files case-by-case as designed above. After
editing:

1. Sanity-check YAML files parse (e.g. `python3 -c "import yaml,sys; yaml.safe_load(open(sys.argv[1]))" file.yaml` or `yq`).
2. Run `gofmt -l` on changed `.go` files — should print nothing.
3. Run the affected unit tests to make sure your semantic edits are actually
   self-consistent:
   ```
   go test ./controllers/... ./pkg/version/... ./pkg/multiclusterengine/... ./pkg/multiclusterengine/olm/...
   ```
   Fix any failures before proceeding — a red test here means a shift was
   computed incorrectly.

## Phase 6 — Commit

```
git checkout -b update-to-{NEW_VERSION}   # if not already on a feature branch
git add -A
git commit -m "chore: Update MCH operator version to {NEW_VERSION}"
```

One commit is fine; split into more if it reads better (e.g. one for the version
bump, one for Tekton cleanup) — either is acceptable.

## Phase 7 — Jira + Draft PR

1. If an epic was found/created in Phase 1, note its key.
2. Push the branch and open a **draft** PR via the GitHub MCP tools:
   - Title: `chore: Update MCH operator version to {NEW_VERSION}`
   - Base: `main`
   - Draft: true
   - Body: summarize the version/channel bump, list files touched, link the Jira
     epic if any, and include next-step instructions (`make test`,
     `make generate-bundles`, mark ready when satisfied).
3. Report the PR URL and Jira link back to the user.

## Phase 8 — Summary

Print a short summary: old -> new version, old -> new MCE/community channels,
files changed count, Tekton files deleted/skipped, PR URL, Jira link, and
explicit next steps for the user (review, `make test`, `make generate-bundles`,
mark PR ready, merge after CI).

---

## Error handling

- Missing/renamed file the scan expected: skip it, tell the user, keep going.
- YAML/Go syntax break after edit: stop, show the diff, ask the user to fix
  manually rather than guessing further.
- `go test` failures after semantic edits: this means your relative-offset math
  was wrong somewhere — re-derive the shift for that specific test case rather
  than forcing the test to pass by editing assertions blindly.
- PR creation fails: report the error and give the user the exact `gh pr create`
  equivalent to run manually.
- No Jira epic found and user declines to create one: proceed without linking,
  note it in the PR body as "no linked epic".
