#!/usr/bin/env python3
"""
ART Builds Monitor

Polls the Quay.io REST API for ART File-Based Catalog (FBC) images and
notifies a Slack channel when a new build lands or an existing build goes stale.

For each release train, ALL active OCP variant tags are discovered and checked.
Results are grouped into one Slack message per product so the full variant
picture is visible without spamming the channel with one message per train.

Digest / timestamp come from the Quay tag API (no auth). When a digest change
is detected, `skopeo inspect` (linux/amd64) is invoked once for that tag to
recover the human-readable ART NVR label, so the notification can show the
"Previous -> New" NVR transition like the legacy Konflux monitor.
"""

import argparse
import json
import logging
import os
import subprocess
import sys
import urllib.request
import urllib.parse
from datetime import datetime, timezone, timedelta
from pathlib import Path

import yaml

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

QUAY_API_BASE = "https://quay.io/api/v1"

# Business-day staleness thresholds → Slack color / warning level
# Green (Healthy):    ≤ 1 business day — normal cyclical builds
# Yellow (Inactive):  > 1 to ≤ 3 business days — stable but idle
# Orange (Degraded):  > 3 to ≤ 7 business days — stale, missed sprint windows
# Red (Critical):     > 7 business days — hard ceiling, more than 1 business week without updates
STALE_YELLOW_DAYS = 1
STALE_ORANGE_DAYS = 3
STALE_RED_DAYS    = 7

log = logging.getLogger("art-monitor")


# ---------------------------------------------------------------------------
# Quay API helpers
# ---------------------------------------------------------------------------

def quay_get(path: str, params: dict | None = None) -> dict:
    """GET a Quay API endpoint, return parsed JSON."""
    url = f"{QUAY_API_BASE}/{path}"
    if params:
        url += "?" + urllib.parse.urlencode(params)
    req = urllib.request.Request(url, headers={"Accept": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return json.load(resp)
    except Exception as exc:
        raise RuntimeError(f"Quay API request failed ({url}): {exc}") from exc


def _repo_path(registry: str) -> str:
    """
    Strip the domain from a registry string.
    'quay.io/redhat-user-workloads/ocp-art-tenant/art-fbc'
    → 'redhat-user-workloads/ocp-art-tenant/art-fbc'
    """
    parts = registry.split("/", 1)
    if len(parts) < 2 or not parts[1]:
        raise ValueError(f"Cannot parse registry path: {registry!r}")
    return parts[1]


def _quay_tag_url(registry: str, tag: str) -> str:
    """Build clickable Quay URL for a specific tag."""
    repo = _repo_path(registry)
    return f"https://quay.io/repository/{repo}?tab=tags&tag={tag}"


def _quay_repo_url(registry: str) -> str:
    """Build clickable Quay URL for the repo root tags page."""
    repo = _repo_path(registry)
    return f"https://quay.io/repository/{repo}?tab=tags"


def discover_ocp_variants(registry: str, product: str, version: str, image_suffix: str) -> list[str]:
    """
    Return sorted list of OCP version strings (e.g. ['4.18','4.19','4.20'])
    for which active rolling tags currently exist for the given product version.

    Scans tags matching '<product>-<version>__*__<image_suffix>' and excludes
    immutable per-commit '__g<sha>' tags — those are not rolling references.
    """
    repo = _repo_path(registry)
    prefix = f"{product}-{version}__"
    suffix = f"__{image_suffix}"

    # Paginate — immutable __g<sha> tags can push the count above 100.
    page = 1
    all_tags: list[dict] = []
    while True:
        data = quay_get(
            f"repository/{repo}/tag/",
            {"filter_tag_name": f"like:{prefix}", "limit": "100",
             "onlyActiveTags": "true", "page": str(page)},
        )
        all_tags.extend(data.get("tags", []))
        if not data.get("has_additional"):
            break
        page += 1

    ocp_versions = set()
    for t in all_tags:
        name = t["name"]
        if "end_ts" in t:
            continue
        if not name.endswith(suffix):
            continue
        # Strip prefix and suffix, leaving just the OCP part: 'v4.20'
        inner = name[len(prefix):-len(suffix)]
        # Plain rolling tag: inner == 'v4.20'
        # Immutable tag:     inner == 'v4.20__g7bf6b36'  (skip)
        if "__" in inner:
            continue
        ocp = inner.lstrip("v")
        ocp_versions.add(ocp)
    return sorted(ocp_versions, key=lambda v: [int(x) for x in v.split(".")])


def query_tag(registry: str, tag: str) -> dict | None:
    """
    Return the active tag entry from Quay for the given tag, or None if
    the tag does not exist / has been pruned.
    Active = no 'end_ts' field (Quay sets end_ts on superseded entries).
    """
    repo = _repo_path(registry)
    data = quay_get(f"repository/{repo}/tag/", {"specificTag": tag})
    tags = [t for t in data.get("tags", []) if "end_ts" not in t]
    if not tags:
        return None
    return max(tags, key=lambda t: t.get("start_ts", 0))


def skopeo_inspect(registry: str, tag: str) -> dict | None:
    """
    Run `skopeo inspect` anonymously against a tag and return the parsed JSON.

    ART FBC images are multi-arch indexes; the runner (and default CI env) is
    linux/amd64, but local dev on arm64 would otherwise fail to pick an
    instance — so we pin the platform explicitly. Returns None on any failure
    (skopeo missing, network error, tag gone); the caller degrades gracefully.
    """
    ref = f"docker://{registry}:{tag}"
    cmd = [
        "skopeo", "inspect", "--no-tags",
        "--override-os", "linux", "--override-arch", "amd64",
        ref,
    ]
    try:
        proc = subprocess.run(cmd, capture_output=True, text=True, timeout=90, check=True)
        return json.loads(proc.stdout)
    except FileNotFoundError:
        log.warning("skopeo not found on PATH — NVR lookup skipped for %s", tag)
    except subprocess.CalledProcessError as exc:
        log.warning("skopeo inspect failed for %s: %s", tag, (exc.stderr or "").strip())
    except Exception as exc:  # noqa: BLE001
        log.warning("skopeo inspect error for %s: %s", tag, exc)
    return None


def extract_nvr(inspect_json: dict) -> tuple[str | None, str | None, str | None, str | None]:
    """Return (nvr, build_date, source_repo, commit_id) from a skopeo inspect payload."""
    labels = (inspect_json or {}).get("Labels") or {}
    nvr = labels.get("com.redhat.art.nvr")
    build_date = (
        labels.get("build-date")
        or labels.get("org.opencontainers.image.created")
    )
    source_repo = labels.get("io.openshift.build.source-location")
    commit_id = labels.get("io.openshift.build.commit.id")
    return nvr, build_date, source_repo, commit_id


# ---------------------------------------------------------------------------
# Business-day staleness
# ---------------------------------------------------------------------------

def parse_quay_timestamp(ts_str: str) -> datetime:
    """Parse Quay last_modified: 'Fri, 09 Oct 2026 19:27:37 -0000'."""
    ts_str = ts_str.rsplit(" ", 1)[0]  # drop tz offset, assume UTC
    dt = datetime.strptime(ts_str, "%a, %d %b %Y %H:%M:%S")
    return dt.replace(tzinfo=timezone.utc)


def business_days_since(dt: datetime) -> int:
    """Count weekdays (Mon–Fri) between dt and now (UTC)."""
    now = datetime.now(timezone.utc)
    if dt > now:
        return 0
    count = 0
    current = dt.date() + timedelta(days=1)
    end = now.date()
    while current <= end:
        if current.weekday() < 5:
            count += 1
        current += timedelta(days=1)
    return count


def staleness_info(days: int) -> tuple[str, str]:
    """Return (level, emoji) based on business-day count."""
    if days >= STALE_RED_DAYS:
        return "red", ":red_circle:"
    if days >= STALE_ORANGE_DAYS:
        return "orange", ":large_orange_circle:"
    if days >= STALE_YELLOW_DAYS:
        return "yellow", ":large_yellow_circle:"
    return "green", ":large_green_circle:"


# ---------------------------------------------------------------------------
# State persistence
# ---------------------------------------------------------------------------

def load_state(state_path: Path) -> dict:
    if state_path.exists():
        with state_path.open() as f:
            return json.load(f)
    return {}


def save_state(state_path: Path, state: dict, dry_run: bool) -> None:
    if dry_run:
        log.info("[dry-run] Would write state to %s", state_path)
        return
    state_path.parent.mkdir(parents=True, exist_ok=True)
    with state_path.open("w") as f:
        json.dump(state, f, indent=2)
        f.write("\n")
    log.info("State saved → %s", state_path)


# ---------------------------------------------------------------------------
# Per-variant result classification
# ---------------------------------------------------------------------------

RESULT_NEW     = "new"
RESULT_OK      = "ok"
RESULT_STALE   = "stale"
RESULT_MISSING = "missing"
RESULT_ERROR   = "error"


def classify_variant(ocp: str, entry: dict | None, prev_variants: dict) -> dict:
    """
    Classify one OCP variant result.
    Returns a dict with keys: ocp, status, digest, last_modified, days, emoji,
    prev_digest, prev_nvr, nvr, source_repo, commit_id (carried forward from prev for unchanged).
    """
    if entry is None:
        return {
            "ocp": ocp, "status": RESULT_MISSING,
            "digest": None, "last_modified": None,
            "days": 0, "emoji": ":warning:", "prev_digest": None, "prev_nvr": None,
            "nvr": None, "source_repo": None, "commit_id": None,
        }

    digest   = entry["manifest_digest"]
    last_mod = entry.get("last_modified", "")
    prev     = prev_variants.get(ocp, {})
    prev_dig = prev.get("digest", "")
    prev_nvr = prev.get("nvr")

    if digest != prev_dig:
        return {
            "ocp": ocp, "status": RESULT_NEW,
            "digest": digest, "last_modified": last_mod,
            "days": 0, "emoji": ":new:", "prev_digest": prev_dig or None,
            "prev_nvr": prev_nvr,
            "nvr": None,  # Will be resolved via skopeo in cmd_monitor
            "source_repo": None, "commit_id": None,
        }

    try:
        dt = parse_quay_timestamp(last_mod)
    except (ValueError, IndexError, TypeError):
        log.warning("  Cannot parse last_modified %r; using start_ts", last_mod)
        dt = datetime.fromtimestamp(entry.get("start_ts", 0), timezone.utc)
    days = business_days_since(dt)

    level, emoji = staleness_info(days)
    status = RESULT_STALE if level != "green" else RESULT_OK
    return {
        "ocp": ocp, "status": status,
        "digest": digest, "last_modified": last_mod,
        "days": days, "emoji": emoji, "prev_digest": None, "prev_nvr": prev_nvr,
        "nvr": prev_nvr,  # Carry forward existing NVR for unchanged variants
        "source_repo": prev.get("source_repo"), "commit_id": prev.get("commit_id"),
    }


# ---------------------------------------------------------------------------
# Slack block builders
# ---------------------------------------------------------------------------

def _short(digest: str | None, n: int = 19) -> str:
    return (digest[:n] + "...") if digest else "(none)"


def _short_nvr(nvr: str | None, digest: str | None) -> str:
    """Prefer the ART NVR; fall back to a short digest when unavailable."""
    if nvr:
        return nvr
    return _short(digest)


def _short_date(last_modified: str | None) -> str:
    """'Fri, 09 Oct 2026 19:27:37 -0000' → '09 Oct 2026'."""
    if not last_modified:
        return "unknown"
    parts = last_modified.split(",", 1)
    rest = parts[1].strip() if len(parts) == 2 else last_modified
    tokens = rest.split()
    return " ".join(tokens[:3]) if len(tokens) >= 3 else rest


def _biz(days: int) -> str:
    """Format business days count: '1 business day' / '4 business days'."""
    unit = "day" if days == 1 else "days"
    return f"{days} business {unit}"


def build_train_message_blocks(
    product: str,
    train_version: str,
    results: list[dict],
    mention: str = "",
    registry: str = "",
    image_suffix: str = "",
    reference_ocp: str = "",
) -> list:
    """
    Build Slack message for a single product train showing all OCP variants.
    
    Two formats:
    - NEW builds: Expanded with Previous→New NVR, floating tag, build details
    - No new builds: Compact with current NVR, health/staleness line
    
    Args:
        product: Product name (acm, mce)
        train_version: Train version (e.g., "5.0", "2.17")
        results: List of variant results (one per OCP version)
        mention: Optional mention string for stale items
        registry: Quay registry (e.g., quay.io/.../art-fbc)
        image_suffix: Image suffix (e.g., multiclusterhub-rhel9)
        reference_ocp: Reference OCP version for this train
    
    Returns:
        Slack blocks for this train
    """
    if not results:
        return []
    
    # Find reference OCP result for version/NVR data
    ref_result = next((r for r in results if r["ocp"] == reference_ocp), None) or results[0]
    
    # Count new builds
    new_count = sum(1 for r in results if r["status"] == RESULT_NEW)
    
    # Worst staleness across all OCP variants
    worst_days = max(r["days"] for r in results)
    level, emoji = staleness_info(worst_days)
    
    # Header prefix: |2.17.X ACM|
    header_prefix = f"|{train_version}.X {product.upper()}|"
    
    # === NEW BUILD PATH (EXPANDED) ===
    if new_count > 0:
        # Header with :label: emoji
        header_text = f"{header_prefix} :label: *{product.upper()} Image Tag Changed!*"
        
        # Previous → New NVR lines (from reference OCP variant)
        prev_nvr = ref_result.get("prev_nvr") or "(none)"
        new_nvr = ref_result.get("nvr") or "(unknown)"
        nvr_lines = f"Previous: `{prev_nvr}`\nNew:      `{new_nvr}`"
        
        # Floating tag line (reference OCP tag)
        floating_tag = f"{product}-{train_version}__v{reference_ocp}__{image_suffix}"
        floating_tag_line = f"Floating Tag: `{floating_tag}`"
        
        # Build Details section
        commit_id = ref_result.get("commit_id")
        source_repo = ref_result.get("source_repo")
        build_date = ref_result.get("build_date") or "(unknown)"
        digest = ref_result.get("digest") or "(unknown)"
        
        if commit_id and source_repo:
            short_commit = commit_id[:7]
            commit_link = f"<{source_repo}/commit/{commit_id}|{short_commit}>"
        else:
            commit_link = "(unknown)"
        
        build_details = (
            f"*Build Details:*\n"
            f"• Commit: {commit_link}\n"
            f"• Built On: `{build_date}`\n"
            f"• Digest: `{digest}`"
        )
        
        # Image builds by OCP (clickable Quay links + timestamps)
        build_items = []
        for r in sorted(results, key=lambda x: x["ocp"]):
            ocp = r["ocp"]
            status = r["status"]
            
            if status == RESULT_MISSING:
                build_items.append(f"{ocp} :warning:")
            elif status == RESULT_ERROR:
                build_items.append(f"{ocp} :x:")
            else:
                # Extract timestamp from NVR
                nvr = r.get("nvr") or ""
                timestamp = ""
                for part in nvr.split('-'):
                    if len(part) >= 14 and part[:14].isdigit():
                        timestamp = part[:14]
                        break
                
                # Build clickable Quay tag link
                tag = f"{product}-{train_version}__v{ocp}__{image_suffix}"
                tag_url = _quay_tag_url(registry, tag)
                ocp_link = f"<{tag_url}|{ocp}>"
                
                if timestamp:
                    build_items.append(f"{ocp_link} {timestamp}")
                else:
                    build_items.append(f"{ocp_link} (unknown)")
        
        builds_line = "*Image builds by OCP:*\n" + " · ".join(build_items)
        
        # Registry link (to reference OCP tag page)
        registry_url = _quay_tag_url(registry, floating_tag)
        registry_line = f"• Registry: <{registry_url}|Quay.io Image Link>"
        
        # Build blocks
        blocks: list[dict] = [
            {"type": "section", "text": {"type": "mrkdwn", "text": header_text}},
            {"type": "section", "text": {"type": "mrkdwn", "text": nvr_lines}},
            {"type": "section", "text": {"type": "mrkdwn", "text": floating_tag_line}},
            {"type": "section", "text": {"type": "mrkdwn", "text": build_details}},
            {"type": "section", "text": {"type": "mrkdwn", "text": builds_line}},
            {"type": "section", "text": {"type": "mrkdwn", "text": registry_line}},
        ]
        
        return blocks
    
    # === NO NEW BUILDS PATH (COMPACT) ===
    else:
        # Header with color emoji
        current_nvr = ref_result.get("nvr") or "(unknown)"
        header_text = f"{header_prefix} {emoji} No new builds. Current tag is `{current_nvr}`"
        
        # Health/staleness line
        last_modified = ref_result.get("last_modified")
        date_str = _short_date(last_modified) if last_modified else "unknown"
        
        if worst_days == 0:
            health_line = f"Build is healthy — Last build was {date_str} (0 business days ago)"
        else:
            health_line = f"Last build was {date_str} ({worst_days} business days ago)"
        
        # Add mention for red items
        if level == "red" and mention:
            health_line += f" {mention}"
        
        # Build blocks
        blocks: list[dict] = [
            {"type": "section", "text": {"type": "mrkdwn", "text": header_text}},
            {"type": "section", "text": {"type": "mrkdwn", "text": health_line}},
        ]
        
        return blocks


def build_ocp_footprint_blocks(
    product: str,
    train_results: list[tuple[str, list[dict]]],
    mention: str = "",
) -> list:
    """
    Build a single Slack message showing operator footprint grouped by target OCP version.

    Layout (one message, one section per OCP target):

      ☸️ ACM ART FBC Builds · 2026-10-14 EST
      📊 Global Footprints: 🟢 9 Current | 🟡 1 Static | 🟠 0 Alerts | 🔴 0 Critical | Quay Registry: Registry
      ────────────────────────────────────
      |4.19.X OCP| 🟢 Current (All platform images match latest upstream commits)
      • ACM 2.15 · `multiclusterhub-operator-fbc-2.15.9-20261014221713.ocp4.19` _(7bf6b36)_
      • ACM 2.16 · `multiclusterhub-operator-fbc-2.16.4-20261014221713.ocp4.19` _(32b90d3)_
      ────────────────────────────────────
      |4.21.X OCP| 🟡 Static (No new code modifications; tracking baseline tag)
      • ACM 2.14 · `...` _(81e23ce, 2 business days ago)_
      • ACM 2.16 · `...` _(32b90d3)_

    Every OCP target is reported, showing the health status of all product trains
    targeting that OCP version. Mention is appended only to 🔴 critical items.
    Returns [] only when there are no trains at all.
    """
    quay_registry_url = "https://quay.io/repository/redhat-user-workloads/ocp-art-tenant/art-fbc"
    registry_link = f"<{quay_registry_url}|Registry>"

    # Regroup train_results by OCP version
    # ocp_map[ocp] = [(train_version, variant_result), ...]
    ocp_map: dict[str, list[tuple[str, dict]]] = {}
    for train_version, results in train_results:
        for r in results:
            ocp = r["ocp"]
            if ocp not in ocp_map:
                ocp_map[ocp] = []
            ocp_map[ocp].append((train_version, r))

    if not ocp_map:
        return []

    # Compute global status counts across all OCP targets
    green_count = sum(1 for variants in ocp_map.values() 
                      if staleness_info(max(v[1]["days"] for v in variants))[0] == "green")
    yellow_count = sum(1 for variants in ocp_map.values() 
                       if staleness_info(max(v[1]["days"] for v in variants))[0] == "yellow")
    orange_count = sum(1 for variants in ocp_map.values() 
                       if staleness_info(max(v[1]["days"] for v in variants))[0] == "orange")
    red_count = sum(1 for variants in ocp_map.values() 
                    if staleness_info(max(v[1]["days"] for v in variants))[0] == "red")
    
    # Current EST date
    from datetime import datetime, timezone, timedelta
    # EST is UTC-5 (note: this doesn't account for EDT/daylight saving, always shows EST)
    now_est = (datetime.now(timezone.utc) + timedelta(hours=-5)).strftime("%Y-%m-%d EST")
    
    header_text = (
        f":wheel_of_dharma: *{product.upper()} ART FBC Builds* · {now_est}\n"
        f":bar_chart: Global Footprints: :large_green_circle: {green_count} Current | "
        f":large_yellow_circle: {yellow_count} Static | "
        f":large_orange_circle: {orange_count} Alerts | "
        f":red_circle: {red_count} Critical | Quay Registry: {registry_link}"
    )

    # Build sections for each OCP target
    ocp_sections: list[str] = []
    for ocp in sorted(ocp_map.keys()):
        variants = ocp_map[ocp]
        
        # Compute worst status among all variants for this OCP
        worst_days = max(v[1]["days"] for v in variants)
        level, emoji = staleness_info(worst_days)
        
        # Status descriptions based on thresholds
        if level == "green":
            status_desc = "Current (All platform images match latest upstream commits)"
        elif level == "yellow":
            status_desc = "Static (No new code modifications; tracking baseline tag)"
        elif level == "orange":
            status_desc = "Stale Alert (Expected build cycle missed; delta exceeds threshold)"
        else:  # red
            status_desc = f"Pipeline Failure (The build orchestration process aborted or failed)"
        
        header = f"|{ocp}.X OCP| {emoji} {status_desc}"
        
        # Build per-train bullets
        bullets: list[str] = []
        for train_version, r in sorted(variants, key=lambda x: x[0]):
            status = r["status"]
            train_label = f"*{product.upper()} {train_version}*"
            
            # Build commit link if available
            commit_id = r.get("commit_id")
            source_repo = r.get("source_repo")
            if commit_id and source_repo:
                short_commit = commit_id[:7]
                commit_link = f"<{source_repo}/commit/{commit_id}|{short_commit}>"
            else:
                commit_link = None
            
            # NVR display (not linked, just in backticks)
            digest = r.get("digest")
            nvr_display = f"`{r.get('nvr') or _short(digest) if digest else '(unknown)'}`"
            
            if status == RESULT_NEW:
                parts = [f"• {train_label} · {nvr_display}"]
                prev_nvr = r.get("prev_nvr")
                if commit_link:
                    parts.append(f"_({commit_link})_")
                else:
                    parts.append(f"_(New build)_")
                bullet_line = " ".join(parts)
                
                # Add "previous" line if there was a prior build
                if prev_nvr:
                    # Extract timestamp from previous NVR if available (format: ...-YYYYMMDDHHMMSS.ocp...)
                    prev_timestamp = ""
                    try:
                        # NVR format: multiclusterhub-operator-fbc-5.0.0-20261014161151.ocp5.0
                        # Extract the timestamp part (20261014161151)
                        nvr_parts = prev_nvr.split('-')
                        for part in nvr_parts:
                            if len(part) == 14 and part.isdigit():
                                # Format: YYYYMMDDHHmmss → YYYY-MM-DD HH:mm:ss
                                prev_timestamp = f" (built {part[:4]}-{part[4:6]}-{part[6:8]} {part[8:10]}:{part[10:12]}:{part[12:14]})"
                                break
                    except Exception:
                        pass
                    
                    bullets.append(f"{bullet_line}\n  ↳ _Previous: `{prev_nvr}`{prev_timestamp}_")
                else:
                    bullets.append(bullet_line)
            elif status == RESULT_MISSING:
                bullets.append(f"• {train_label} · :warning: tag not found")
            elif status == RESULT_ERROR:
                bullets.append(f"• {train_label} · :x: query error")
            else:  # OK or STALE
                days = r["days"]
                variant_level, variant_emoji = staleness_info(days)
                
                # Don't repeat the emoji at train level if already shown in OCP header
                parts = [f"• {train_label} · {nvr_display}"]
                
                # Only show age when > 0 days (redundant to say "today" when header already says "updated within 1 business day")
                if days > 0:
                    age_str = f"{_biz(days)} ago"
                    if commit_link:
                        parts.append(f"_({commit_link}, {age_str})_")
                    else:
                        parts.append(f"_({age_str})_")
                else:
                    # 0 days: just show commit if available
                    if commit_link:
                        parts.append(f"_({commit_link})_")
                
                at = f" {mention}" if variant_level == "red" and mention else ""
                bullets.append(" ".join(parts) + at)
        
        ocp_sections.append(f"{header}\n" + "\n".join(bullets))

    # Slack section blocks cap at 3000 chars; chunk the OCP sections if needed.
    blocks: list[dict] = [
        {"type": "section", "text": {"type": "mrkdwn", "text": header_text}},
        {"type": "divider"},
    ]
    
    for i, section in enumerate(ocp_sections):
        blocks.append({"type": "section", "text": {"type": "mrkdwn", "text": section}})
        # Add divider after each OCP section except the last
        if i < len(ocp_sections) - 1:
            blocks.append({"type": "divider"})

    return blocks


def post_slack(webhook_url: str, blocks: list, dry_run: bool, skip: bool) -> bool:
    """Post blocks to Slack. Returns True on success (or skip/dry-run), False on failure."""
    if skip:
        log.info("[skip-notifications] Slack message suppressed.")
        return True
    if dry_run:
        log.info("[dry-run] Slack payload:\n%s", json.dumps({"blocks": blocks}, indent=2))
        return True
    payload = json.dumps({"blocks": blocks}).encode()
    req = urllib.request.Request(
        webhook_url,
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=15) as resp:
            log.info("Slack response: %s", resp.status)
            return True
    except Exception as exc:
        log.error("Slack POST failed: %s", exc)
        return False


# ---------------------------------------------------------------------------
# Discover subcommand
# ---------------------------------------------------------------------------

def cmd_discover(cfg: dict) -> None:
    product      = cfg["product"]
    registry     = cfg["registry"]
    image_suffix = cfg["image_suffix"]
    print(f"\nDiscovering OCP variants for {product.upper()} trains in {registry}\n")
    for train in cfg["trains"]:
        version  = train["version"]
        variants = discover_ocp_variants(registry, product, version, image_suffix)
        print(f"  {product}-{version}: {variants if variants else '(none found)'}")
    print()


# ---------------------------------------------------------------------------
# Monitor subcommand
# ---------------------------------------------------------------------------

def cmd_monitor(cfg: dict, args: argparse.Namespace) -> int:
    product      = cfg["product"]
    registry     = cfg["registry"]
    image_suffix = cfg["image_suffix"]
    state_path   = Path(args.state)
    webhook_url  = os.environ.get("SLACK_WEBHOOK_URL", "")

    if not webhook_url and not args.skip_notifications and not args.dry_run:
        log.error("SLACK_WEBHOOK_URL is required unless --skip-notifications or --dry-run is set.")
        return 1

    state     = load_state(state_path)
    new_state = {k: dict(v) for k, v in state.items()}  # deep-ish copy
    train_results: list[tuple[str, list[dict], str, str]] = []  # (version, results, reference_ocp, mention)
    # Mention: env var SLACK_TEAM_ID takes precedence over trains.yaml
    default_mention = os.environ.get("SLACK_TEAM_ID") or cfg.get("mention", "")
    # Auto-wrap bare IDs like "S05L3R4SM89" → "<!subteam^S05L3R4SM89>"
    if default_mention and not default_mention.startswith("<"):
        default_mention = f"<!subteam^{default_mention}>"
    errors = 0

    # Filter trains if --version specified
    trains = cfg["trains"]
    if args.version:
        trains = [t for t in trains if t["version"] == args.version]
        if not trains:
            log.error("No train found matching version '%s'", args.version)
            sys.exit(1)

    for train in trains:
        version = train["version"]
        ocp_ref = train["ocp"]  # Reference OCP for this train
        train_mention = train.get("mention", default_mention)  # Per-train override or default
        # Auto-wrap bare IDs in per-train overrides too
        if train_mention and not train_mention.startswith("<"):
            train_mention = f"<!subteam^{train_mention}>"
        key     = f"{product}-{version}"

        log.info("=== %s: discovering OCP variants ...", key)
        try:
            ocp_versions = discover_ocp_variants(registry, product, version, image_suffix)
        except RuntimeError as exc:
            log.error("  %s: discovery failed: %s", key, exc)
            errors += 1
            train_results.append((version, [{"ocp": "*", "status": RESULT_ERROR,
                "digest": None, "last_modified": None, "days": 0,
                "emoji": ":x:", "prev_digest": None, "prev_nvr": None}], ocp_ref, train_mention))
            continue

        if not ocp_versions:
            log.warning("  %s: no active OCP variant tags found", key)
            train_results.append((version, [{"ocp": "*", "status": RESULT_MISSING,
                "digest": None, "last_modified": None, "days": 0,
                "emoji": ":warning:", "prev_digest": None, "prev_nvr": None}], ocp_ref, train_mention))
            continue

        log.info("  variants: %s", ocp_versions)

        prev_variants = state.get(key, {})
        new_variants  = dict(prev_variants)
        results: list[dict] = []

        for ocp in ocp_versions:
            tag = f"{product}-{version}__v{ocp}__{image_suffix}"
            log.info("  Checking %s ...", tag)
            try:
                entry = query_tag(registry, tag)
            except RuntimeError as exc:
                log.error("    query failed: %s", exc)
                results.append({"ocp": ocp, "status": RESULT_ERROR,
                                 "digest": None, "last_modified": None,
                                 "days": 0, "emoji": ":x:",
                                 "prev_digest": None, "prev_nvr": None})
                errors += 1
                continue

            result = classify_variant(ocp, entry, prev_variants)

            # For new builds, resolve the human-readable ART NVR via skopeo.
            # (Only on change — keeps a normal run to zero skopeo invocations.)
            if result["status"] == RESULT_NEW and entry is not None:
                inspect = skopeo_inspect(registry, tag)
                nvr, build_date, source_repo, commit_id = extract_nvr(inspect) if inspect else (None, None, None, None)
                result["nvr"]         = nvr
                result["build_date"]  = build_date
                result["source_repo"] = source_repo
                result["commit_id"]   = commit_id

            results.append(result)

            # Log detail
            if result["status"] == RESULT_NEW:
                log.info("    v%s: NEW BUILD  nvr=%s", ocp, result.get("nvr") or "(unresolved)")
                log.debug("    current  digest=%s  last_modified=%s",
                          _short(result["digest"]), result["last_modified"])
                prev_str = result["prev_nvr"] or (
                    _short(result["prev_digest"]) if result["prev_digest"] else "(none — first run)"
                )
                log.debug("    previous=%s", prev_str)
            elif result["status"] == RESULT_MISSING:
                log.warning("    v%s: tag not found / pruned", ocp)
            elif result["status"] == RESULT_STALE:
                log.info("    v%s: unchanged, %d biz days [%s]",
                         ocp, result["days"], result["status"])
            else:
                log.debug("    current  digest=%s  last_modified=%s",
                          _short(result["digest"]), result["last_modified"])
                log.info("    v%s: ok, %d biz days", ocp, result["days"])

            # Update per-variant state (carry nvr/build_date/source_repo/commit_id forward or store new)
            # Only carry forward prev metadata when digest is unchanged.
            # On RESULT_NEW, a failed skopeo means we have no NVR — don't
            # store the old digest's NVR next to the new digest.
            if entry is not None:
                prev_entry = prev_variants.get(ocp, {})
                carry = result["status"] != RESULT_NEW
                new_variants[ocp] = {
                    "tag":           tag,
                    "digest":        result["digest"],
                    "last_modified": result["last_modified"],
                    "start_ts":      entry.get("start_ts", 0),
                    "nvr":           result.get("nvr") or (prev_entry.get("nvr") if carry else None),
                    "build_date":    result.get("build_date") or (prev_entry.get("build_date") if carry else None),
                    "source_repo":   result.get("source_repo") or (prev_entry.get("source_repo") if carry else None),
                    "commit_id":     result.get("commit_id") or (prev_entry.get("commit_id") if carry else None),
                }

        new_state[key] = new_variants
        train_results.append((version, results, ocp_ref, train_mention))

    # Post one message per train; save state only after successful delivery
    slack_ok = True
    for train_version, results, reference_ocp, train_mention in train_results:
        blocks = build_train_message_blocks(
            product, train_version, results, train_mention,
            registry, image_suffix, reference_ocp
        )
        if blocks:
            if not post_slack(webhook_url, blocks, args.dry_run, args.skip_notifications):
                slack_ok = False
        else:
            log.info("No results for train %s — skipping Slack notification.", train_version)

    if slack_ok:
        save_state(state_path, new_state, args.dry_run)
    else:
        log.error("Slack delivery failed — state NOT saved to avoid missed notifications on next run.")
        errors += 1

    return 1 if errors else 0


def cmd_monitor_collect_data(cfg: dict, args: argparse.Namespace) -> tuple[str, list[tuple[str, list[dict], str, str]], str, str]:
    """
    Monitor builds and return train results data without building Slack blocks.
    Used for consolidated mode where multiple products are combined into one message.
    
    Returns:
        (product_name, train_results_with_ocp, registry, image_suffix) tuple
        where train_results_with_ocp = [(version, results, reference_ocp, mention), ...]
    """
    product      = cfg["product"]
    registry     = cfg["registry"]
    image_suffix = cfg["image_suffix"]
    default_mention = cfg.get("mention", "")
    trains       = cfg["trains"]
    state_path   = Path(args.state)

    # Load previous state
    prev_state = load_state(state_path)
    new_state: dict[str, dict] = {}

    train_results: list[tuple[str, list[dict], str, str]] = []
    
    for train in trains:
        version = train["version"]
        ocp_ref = train["ocp"]  # This is the reference OCP for this train
        train_mention = train.get("mention", default_mention)  # Per-train override or default
        key = f"{product}-{version}"
        prev_variants = prev_state.get(key, {})
        new_variants: dict[str, dict] = {}

        # Discover all OCP variants for this train
        ocp_versions = discover_ocp_variants(registry, product, version, image_suffix)
        
        results: list[dict] = []
        for ocp in ocp_versions:
            tag = f"{product}-{version}__v{ocp}__{image_suffix}"
            entry = query_tag(registry, tag)
            result = classify_variant(ocp, entry, prev_variants)
            
            # For new builds, resolve NVR via skopeo
            if result["status"] == RESULT_NEW and entry is not None:
                inspect = skopeo_inspect(registry, tag)
                nvr, build_date, source_repo, commit_id = extract_nvr(inspect) if inspect else (None, None, None, None)
                result["nvr"]         = nvr
                result["build_date"]  = build_date
                result["source_repo"] = source_repo
                result["commit_id"]   = commit_id

            results.append(result)

            # Update per-variant state
            if entry is not None:
                prev_entry = prev_variants.get(ocp, {})
                carry = result["status"] != RESULT_NEW
                new_variants[ocp] = {
                    "tag":           tag,
                    "digest":        result["digest"],
                    "last_modified": result["last_modified"],
                    "start_ts":      entry.get("start_ts", 0),
                    "nvr":           result.get("nvr") or (prev_entry.get("nvr") if carry else None),
                    "build_date":    result.get("build_date") or (prev_entry.get("build_date") if carry else None),
                    "source_repo":   result.get("source_repo") or (prev_entry.get("source_repo") if carry else None),
                    "commit_id":     result.get("commit_id") or (prev_entry.get("commit_id") if carry else None),
                }

        new_state[key] = new_variants
        train_results.append((version, results, ocp_ref, train_mention))

    # Save state
    save_state(state_path, new_state, args.dry_run)
    
    return (product, train_results, registry, image_suffix)


# ---------------------------------------------------------------------------
# Entrypoint
# ---------------------------------------------------------------------------

def main() -> None:
    parser = argparse.ArgumentParser(description="ART FBC downstream builds monitor")
    parser.add_argument(
        "--trains", default="tools/art-monitor/trains.yaml",
        help="Path to trains YAML config",
    )
    parser.add_argument(
        "--state", default="tools/art-monitor/.state.json",
        help="Path to state JSON file",
    )
    parser.add_argument(
        "--version",
        help="Filter to monitor only this train version (e.g., '2.17', '5.0')",
    )
    parser.add_argument(
        "--consolidated", nargs='+', metavar=("TRAINS_CFG:STATE_FILE"),
        help="Consolidated mode: monitor multiple products and post one message. "
             "Format: --consolidated trains.yaml:.state.json trains-mce.yaml:.state-mce.json",
    )
    parser.add_argument(
        "--dry-run", action="store_true",
        help="Check builds without writing state or sending notifications",
    )
    parser.add_argument(
        "--skip-notifications", action="store_true",
        help="Skip Slack notifications but still update state",
    )
    parser.add_argument(
        "--discover", action="store_true",
        help="List available OCP variants per train and exit",
    )
    parser.add_argument(
        "--log-level", default="INFO",
        choices=["DEBUG", "INFO", "WARNING", "ERROR"],
    )
    args = parser.parse_args()

    logging.basicConfig(
        level=getattr(logging, args.log_level),
        format="%(asctime)s %(levelname)-8s %(message)s",
        datefmt="%Y-%m-%dT%H:%M:%SZ",
    )

    # Consolidated mode: monitor multiple products and post one message per train
    if args.consolidated:
        products_data = []
        for spec in args.consolidated:
            if ':' not in spec:
                log.error("Invalid consolidated spec (expected TRAINS:STATE): %s", spec)
                sys.exit(1)
            trains_path_str, state_path_str = spec.split(':', 1)
            trains_path = Path(trains_path_str)
            if not trains_path.exists():
                log.error("Trains config not found: %s", trains_path)
                sys.exit(1)
            
            with trains_path.open() as f:
                cfg = yaml.safe_load(f)
            
            # Override state path for this product
            product_args = argparse.Namespace(**vars(args))
            product_args.state = state_path_str
            product_args.trains = trains_path_str
            
            # Run monitor for this product and collect data
            log.info("Monitoring %s (trains=%s, state=%s)", 
                     cfg.get("product", "unknown"), trains_path_str, state_path_str)
            product_data = cmd_monitor_collect_data(cfg, product_args)
            products_data.append(product_data)
        
        # Collect all trains across products and sort by version descending, ACM before MCE
        all_trains = []
        for product, train_results, registry, image_suffix in products_data:
            for train_version, results, reference_ocp, mention in train_results:
                all_trains.append((product, train_version, results, mention, registry, image_suffix, reference_ocp))
        
        # Sort: version descending (5.0 > 2.17 > 2.16...), then acm before mce
        def train_sort_key(item):
            product, version, _, _, _, _, _ = item
            # Parse version as float for sorting
            try:
                ver_num = float(version)
            except ValueError:
                ver_num = 0.0
            # Return (negative version for descending, 0 for acm / 1 for mce)
            return (-ver_num, 0 if product == "acm" else 1)
        
        all_trains.sort(key=train_sort_key)
        
        # Post one message per train
        webhook_url = os.environ.get("SLACK_WEBHOOK_URL")
        if not webhook_url:
            log.warning("SLACK_WEBHOOK_URL not set — notifications will be skipped.")
        else:
            for product, train_version, results, mention, registry, image_suffix, reference_ocp in all_trains:
                blocks = build_train_message_blocks(
                    product, train_version, results, mention, 
                    registry, image_suffix, reference_ocp
                )
                if blocks:
                    post_slack(webhook_url, blocks, args.dry_run, args.skip_notifications)
                    # Small delay between messages to avoid rate limiting
                    import time
                    time.sleep(0.5)
        
        sys.exit(0)

    # Single-product mode (original behavior)
    trains_path = Path(args.trains)
    if not trains_path.exists():
        log.error("Trains config not found: %s", trains_path)
        sys.exit(1)

    with trains_path.open() as f:
        cfg = yaml.safe_load(f)

    if args.discover:
        cmd_discover(cfg)
        sys.exit(0)

    sys.exit(cmd_monitor(cfg, args))


if __name__ == "__main__":
    main()
