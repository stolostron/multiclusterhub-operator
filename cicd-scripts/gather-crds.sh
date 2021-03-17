#!/usr/bin/bash

# Copyright (c) 2021 Red Hat, Inc.
# Copyright (c) Contributors to the Open Cluster Management project

# This script gathers the CRDS from the Hub CRDs repo into the specified place
# in prep for building the Hub operator image. Its used both in the upstream
# build and in the Red Hat product downstream build.
#
# Arguments:
#
# $1 = Reference to Git repository to clone (https: URL, or git@<repo>, etc).
# $2 = Identifier of what to check out (branch name or commit SHA).
# $3 = Directory into which gathered CRDs are to be placed.  Will be deleted
#      and recreated to ensure its empty to start.
#
# Notes:
# - To run in a ubi 8 container, you'll need to add in (microdnf install)
#   the git packge, and in ubi-minimal the findutils package as well.
#

me=$(basename $0)

if [[ -z "$3" ]]; then
   >&2 echo "ERROR: Required arguments are missing."
   >&2 echo "Syntax: $me <repo-url> <branch-or-sha> <dest-dir>"
   exit 5
fi

if ! command -v git > /dev/null; then
   >&2 echo "ERROR: Git cli not found (need to install git package?)."
   exit 3
fi
if ! command -v find > /dev/null; then
   >&2 echo "ERROR: Find utility not found (need to install findutils package?)."
   exit 3
fi

starting_pwd="$PWD"

crd_repo_clone_url="$1"
what_to_checkout="$2"
dest_dir="$3"

clone_to_spot=$(mktemp -dt  "$me.XXXXXXXX")

# Supress the git CLI's SSH known-host checking in case we use "git@" repo refs.
# (Note: We undo this config change at the end.)

save_git_ssh_cmd=$(git config --global --get core.sshCommand)
git config --global core.sshCommand 'ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no'

# Clone the upstream CRD repo.

echo "Cloning CRD repo."
git clone $crd_repo_clone_url "$clone_to_spot"
if [[ $? -ne 0 ]]; then
   >&2 echo "ERROR: Could not clone upstream Hub CRDs repo."
   exit 3
fi

cd "$clone_to_spot"
git -c advice.detachedHead=false checkout $what_to_checkout
cd "$starting_pwd"

# Gather CRDs from it into destination directory.

echo "Copying CRDs to $dest_dir."
rm -rf "$dest_dir"
mkdir -p "$dest_dir"
if [[ $? -ne 0 ]]; then
   >&2 echo "ERROR: Could not create destination directory."
   exit 3
fi

find "$clone_to_spot" -name '*.yaml' -print0 | xargs -I '{}' -0 cp '{}' "$dest_dir"

# Clean up our temp stuff.

echo "Deleting CRD repo clone."
rm -rf "$clone_to_spot"

# Put git config back to the way we found it.
if [[ -n "$save_git_ssh_cmd" ]]; then
   git config --global core.sshCommand "$save_git_ssh_cmd"
else
   git config --global --unset core.sshCommand || true
fi
