#!/bin/bash
#
# Requires:
#
# - readlink
# - Python 3.6 (for underlying scripts)
#
# This script drives the creation of the OCM Hub operator bundle.
#
# Pre-reqs:
#
# - operator-sdk (or other means) has generated the operator's owned CRDs,
#   requierd CRDs, roles and deployment manifests and left them in the
#   deploy directory at the top of this GHE repo.
#
# Assumptions:
#
# - We assume this directory is located two subdirs below the top of the repo.
#
# Cautions:
#
# - Tested on Linux, not Mac.

my_dir=$(dirname $(readlink -f $0))
top_of_repo=$(readlink  -f $my_dir/../..)

image_rgy_ns_and_repo="quay.io/open-cluster-management/multiclusterhub-operator"

deploy_dir=$top_of_repo/deploy
pkg_dir=$top_of_repo/operator-bundles/open-cluster-management-hub
csv_template=$my_dir/ocm-hub-csv-template.yaml

csv_release="$1"
image_tag_or_digest="$2"
snapshot_id="$3"

if [[ -z $image_tag_or_digest ]]; then
   >&2 echo "Syntax: <csv_version> <image_tag_or_digest> [<snapshot_id>]"
   exit 1
fi

# CSV release identifier is assumed to be in x.y.z format.

old_IFS=$IFS
IFS=. vvv=(${csv_release##*-})
rel_maj=${vvv[0]}
rel_min=${vvv[1]}
rel_z=${vvv[2]}
IFS=$old_IFS


if [[ -z $snapshot_id ]]; then
   csv_vers="$csv_release"
   image_suffix="$image_tag_or_digest"
else
   csv_vers="$csv_release-$snapshot_id"
   if [[ $image_tag_or_digest == sha256* ]]; then
      image_suffix="$image_tag_or_digest"
   else
      image_suffix="$image_tag_or_digest-SHAPSHOT-$snapshot_id"
   fi
fi

# Proposed channel structure:
#
# Version channel (eg. latest-v1)
#   A channel that will have a CSV-replacement chain from most recent CSV for the major
#   version all the way back to the last one for the major version, based on semantic
#   versioning indicaitng upgrades can be done across all major-version releases.
#   To be used by customers that would want to allow automatic feature-release upgrades.
#
#   The CSV replacement chain will be managed using this channel in the package.
#
# Feature-Release channel (eg. latest-1.0, latest-1.1, etc.)
#   A channel that will have a CSV_replacement chain from most recent CSV for a
#   major.minor feature release (eg. 1.0) to the first such for the feature release, but
#   not have CSVs for other feature releases.  To be used by custoemrs that would want
#   automatic upgrades for fixes within a feature release but not from one feature
#   release to the next.
#
# Release-snapshots channel (eg. snapshots-1.0.0, snapshots-1.0.1, snapshots--1.1.0, etc.)
#   A channel that will have a sequence of CSV for snapshots for a specific x.y.z release only.
#   To be used in dev, not likely one that would be published to customers.
#
# Snapshot channel (eg. snapshot-2020-04-15-01-02-03)
#   A channel taht will only every point to exactly one snapshot.  To be used in dev
#   to force installation of a particular CSV snapshot.  Not a channel that would be
#   made available to customers.
#
#   We will probably want some pruning-off of shapshot channels eg. based on age. That could
#   become an additional responsibility of the create-csv Python script, or handled by some
#   separate process.
#
# Other ideas:
#
#   Pipeline-stage channels:  edge-1.0.0, stable-1.0.0, etc.

version_channel="latest-v$rel_maj"
feature_release_channel="latest-$rel_maj.$rel_min"
specific_release_channel="snapshots-$csv_release"
snapshot_channel="snapshot-$csv_vers"

if [[ $image_suffix == sha256* ]]; then
   image_suffix="@$image_suffix"
else
   image_suffix=":$image_suffix"
fi

operator_image_override="$image_rgy_ns_and_repo$image_suffix"

if [[ ! -d $pkg_dir ]]; then
   mkdir -p $pkg_dir
fi

# TODO: Resolve this:
# Since changes in major version (x of x.y.z) indicates a progression for which
# automatic upgrades are not going to be provided, perhaps we should have a different
# OLM package for each such version, eg. by appending the version number to the
# package name (eg. advanced-cluster-management-v1)?

$my_dir/create-ocm-hub-bundle.py \
   --deploy-dir $deploy_dir --pkg-dir $pkg_dir --csv-template $csv_template \
   --csv-vers $csv_vers \
   --replaces-channel "$version_channel" \
   --other-channel "$feature_release_channel" \
   --other-channel "$specific_release_channel" \
   --other-channel "$snapshot_channel" \
   --image-override $operator_image_override

