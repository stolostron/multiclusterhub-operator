#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.

#
# Assumptions:
#
# - We assume this directory is located two subdirs below the top of the repo.
#
# Cautions:
#
# - Tested on Linux, not Mac.

my_dir=$(dirname $(readlink -f $0))
top_of_repo=$(readlink -f $my_dir/../..)

github="git@joegpro.github.com"

#--- SCAFFOLDING ---
# TODO: Make these input args or derive from state in some way?
new_csv_vers="1.0.0"
prev_csv_ver="1.0.0"
default_channel="stable-1.0"
additional_channels="stable-v1"
#--- END SCAFFODLING ---

tmp_root="/tmp/gen-acm-bundle"
mkdir -p "$tmp_root"

tmp_dir="$tmp_root/work"
rm -rf "$tmp_dir"
mkdir -p "$tmp_dir"


#--- SCAFFOLDING ---
# TEMP: For now this is a temp spot.  Maybe it should be a place in a repo cloe that
# we then commit the resulting bundles to for posterity?
unbound_acm_pkg_dir="$tmp_root/operator-bundles/unbound"
bound_acm_pkg_dir="$tmp_root/operator-bundles/bound"
mkdir -p "$bound_acm_pkg_dir"
#--- END SCAFFODLING ---

# Ensure the specified input and output directories for the budnles exist.

if [[ ! -d $unbound_acm_pkg_dir ]]; then
   >&2 echo "Error: Input package directory $unbound_acm_pkg_dir doesn't exist."
   >&2 echo "Aborting."
   exit 2
fi

if [[ ! -d $bound_acm_pkg_dir ]]; then
   >&2 echo "Error: Output package directory $bound_acm_pkg_dir doesn't exist."
   >&2 echo "Aborting."
   exit 2
fi

unbound_acm_bundle=$($my_dir/find-bundle-dir.py "latest" $unbound_acm_pkg_dir)
if [[ $? -ne 0 ]]; then
   >&2 echo "Error: Could not find source bundle directory for unbound ACM bundle."
   >&2 echo "Aborting."
   exit 2
fi

if [[ -n "$prev_csv_vers" ]]; then
   prev_option="--prev-csv $prev_csv_vers"
fi

addl_channel_optons=""
for c in $additional_channels; do
   addl_channel_options="$additional_channel_options --additional-channel $c"
done

$my_dir/create-bound-bundle.py \
   --pkg-name "advanced-cluster-management" --pkg-dir $bound_acm_pkg_dir \
   --source-bundle-dir $unbound_acm_bundle \
   --csv-vers "$new_csv_vers" $prev_option \
   --default-channel $default_channel $addl_channel_options \
   --image-override quay.io/hive/hive@sha256:1111 \
   --image-override quay.io/open-cluster-management/multicluster-operators-placementrule@sha256:2222 \
   --image-override quay.io/open-cluster-management/multicluster-operators-subscription@sha256:3333 \
   --image-override quay.io/open-cluster-management/multicluster-operators-deployable@sha256:4444 \
   --image-override quay.io/open-cluster-management/multicluster-operators-channel@sha256:5555 \
   --image-override quay.io/open-cluster-management/multicluster-operators-application@sha256:6666 \
   --image-override quay.io/open-cluster-management/multiclusterhub-operator@sha256:7878


