#!/bin/bash
#
# Requires:
#
# - readlink
# - Python 3.6 (for underlying scripts)
#
# This script creates an "unbound" operator bundle for the OCM Hub operator based on
# a template and operator-sdk generated artifacts.
#
# By "unbound bundle" (*), we mean one that is structurally complete except that:
#
# - Image references iwthin the CSV have not been updated/bound to specify a specific/pinned
#   operator image (for a snapshot, or an actual release), and
#
# - The bundle/CSV name and the CSV's replaces property have not been set in a way that
#   positions this bundle in replaces-chain sequence of released instances of the operator.
#
# (*) Suggestions for better terminology cheerfully considered.
#
# Pre-reqs:
#
# - operator-sdk (or other means) has generated the operator's owned CRDs, requierd CRDs, roles
#   and deployment manifests and left them in the deploy directory at the top of this GHE repo.
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

deploy_dir=$top_of_repo/deploy
pkg_dir=$top_of_repo/operator-bundles/unbound/open-cluster-management-hub
csv_template=$my_dir/ocm-hub-csv-template.yaml

csv_vers="$1"

if [[ -z $csv_vers ]]; then
   >&2 echo "Syntax: <csv_version>"
   exit 1
fi

channel="latest"

if [[ ! -d $pkg_dir ]]; then
   mkdir -p $pkg_dir
fi

$my_dir/create-unbound-ocm-hub-bundle.py \
   --deploy-dir $deploy_dir --pkg-dir $pkg_dir --csv-template $csv_template \
   --csv-vers $csv_vers --channel $channel

