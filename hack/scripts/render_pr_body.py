#!/usr/bin/env python3
"""
Render a GitHub PR body from a `.github/PULL_REQUEST_TEMPLATE/*.md` template.

Automated workflows (bundle/chart regeneration, OWNERS resync, etc.) use this
to fill in a purpose-built PR template instead of relying on the default text
from the `create-pull-request` action or hand-rolling markdown inline in YAML.

Marker lines in the template of the form:

    <!-- AUTOMATION:NAME -->

are replaced with either a literal string (--set NAME=VALUE) or the contents
of a file (--set-file NAME=PATH). Markers with no value provided fall back to
--fallback (default: "_None reported._") so the template still renders
cleanly if a workflow doesn't populate every section.

Example:
    python3 hack/scripts/render_pr_body.py \\
        --template .github/PULL_REQUEST_TEMPLATE/automated-bundle-update.md \\
        --output /tmp/pr-body.md \\
        --set-file TRIGGER=/tmp/trigger.txt \\
        --set-file DIFFSTAT=/tmp/diffstat.txt \\
        --set-file WARNINGS=/tmp/warnings.txt
"""
import argparse
import re
import sys

MARKER_PATTERN = re.compile(r"<!--\s*AUTOMATION:([A-Z0-9_]+)\s*-->")


def parse_args():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--template", required=True, help="Path to the PR template to render")
    parser.add_argument("--output", required=True, help="Path to write the rendered PR body to")
    parser.add_argument(
        "--set",
        action="append",
        default=[],
        metavar="NAME=VALUE",
        help="Replace marker NAME with a literal string VALUE",
    )
    parser.add_argument(
        "--set-file",
        action="append",
        default=[],
        metavar="NAME=PATH",
        help="Replace marker NAME with the (stripped) contents of the file at PATH",
    )
    parser.add_argument(
        "--fallback",
        default="_None reported._",
        help="Text used when a --set-file value is missing or empty",
    )
    return parser.parse_args()


def collect_values(args):
    values = {}

    for item in args.set:
        if "=" not in item:
            sys.exit(f"error: --set value '{item}' is not in NAME=VALUE format")
        name, _, value = item.partition("=")
        values[name.strip()] = value

    for item in args.set_file:
        if "=" not in item:
            sys.exit(f"error: --set-file value '{item}' is not in NAME=PATH format")
        name, _, path = item.partition("=")
        try:
            with open(path, "r", encoding="utf-8") as f:
                content = f.read().strip()
        except FileNotFoundError:
            content = ""
        values[name.strip()] = content if content else args.fallback

    return values


def render(template_text, values):
    def replace(match):
        name = match.group(1)
        if name not in values:
            sys.stderr.write(f"warning: no value provided for marker '{name}', leaving it blank.\n")
            return ""
        return values[name]

    return MARKER_PATTERN.sub(replace, template_text)


def main():
    args = parse_args()
    values = collect_values(args)

    with open(args.template, "r", encoding="utf-8") as f:
        template_text = f.read()

    rendered = render(template_text, values)

    with open(args.output, "w", encoding="utf-8") as f:
        f.write(rendered)


if __name__ == "__main__":
    main()
