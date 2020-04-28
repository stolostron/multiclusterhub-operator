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
# TODO: Make these input args or derive in some way???
new_csv_vers="1.0.0"
#prev_csv_ver="1.0.0"
#--- END SCAFFODLING ---

tmp_root="/tmp/gen-acm-bundle"
mkdir -p "$tmp_root"

tmp_dir="$tmp_root/work"
rm -rf "$tmp_dir"
mkdir -p "$tmp_dir"

source_bundles="$tmp_dir/source-bundles"
mkdir -p "$source_bundles"


#--- SCAFFOLDING ---
# TEMP: For now this is a temp spot.  Maybe it should be a place in a repo clone
# and then committed to preserve the generated bundles to for posterity?
unbound_acm_pkg_dir="$tmp_root/operator-bundles/unbound"
rm -rf $unbound_acm_pkg_dir
mkdir -p $unbound_acm_pkg_dir
#--- END SCAFFODLING ---


# To generate the composite ACM bundle, we need some source bundles as input.
#
# For Hive, we generate the source bundle using the Hive operator repo and
# a generation script found there.
#
# For the OCM hub, we use a source bundle found within the operator repo.
#
# For the application subscription operator, for the moment we grab the source bundle
# from what is posted as a comomunity operato.  But in order to syncronize the
# bundle with the code snapshot being used for downstream build, we should change this
# to either (a) generate usng a repo-provided script, or (b) pick up an already
# generated bundle from within the repo.

clone_spot="$tmp_dir/repo-clones"

# App Sub:

community_repo_spot="$clone_spot/community-operators"
git clone "$github:operator-framework/community-operators.git" "$community_repo_spot"
app_sub_pkg="$community_repo_spot/community-operators/multicluster-operators-subscription"
app_sub_channel="alpha"

app_sub_bundle=$($my_dir/find-bundle-dir.py $app_sub_channel $app_sub_pkg)
if [[ $? -ne 0 ]]; then
   >&2 echo "Error: Could not find source bundle directory for Multicluster Subscription."
   >&2 echo "Aborting."
   exit 2
fi
app_sub_bundle_spot="$source_bundles/app-sub"
ln -s "$app_sub_bundle" $app_sub_bundle_spot

# Hive:

hive_repo_spot="$clone_spot/hive"
hive_stable_release_branch="ocm-4.4.0"
git clone -b "$hive_stable_release_branch" "$github:openshift/hive.git" $hive_repo_spot

hive_bundle_work=$tmp_dir/hive-bundle
mkdir -p "$hive_bundle_work"

echo "Generating Hive source bundle."

# Seems the generation script assumes CWD is top of repo.

save_cwd=$PWD
cd $hive_repo_spot
hive_image_placeholder="quay.io/openshift-hive/hive:dont-care"
python2.7 ./hack/generate-operator-bundle.py $hive_bundle_work dont-care 0 "-none" "$hive_image_placeholder"
if [[ $? -ne 0 ]]; then
   >&2 echo "Error: Could not generate Hive source bundle."
   >&2 echo "Aborting."
   exit 2
fi
cd $save_cwd

hive_bundle_spot="$source_bundles/hive"
ln -s "$hive_bundle_work/0.1.0-sha-none" $hive_bundle_spot

# OCM Hub:

hub_repo_spot=$top_of_repo
hub_pkg="$hub_repo_spot/operator-bundles/unbound/open-cluster-management-hub"
hub_channel="latest"

hub_bundle=$($my_dir/find-bundle-dir.py $hub_channel $hub_pkg)
if [[ $? -ne 0 ]]; then
   >&2 echo "Error: Could not find source bundle directory for OCM Hub."
   >&2 echo "Aborting."
   exit 2
fi

hub_bundle_spot="$source_bundles/ocm-hub"
ln -s "$hub_bundle" "$hub_bundle_spot"


# Generate the unbound composite bundle, which will be the source for producing
# the bound one.

$my_dir/create-unbound-acm-bundle.py \
   --pkg-name "advanced-cluster-management" --pkg-dir $unbound_acm_pkg_dir \
   --csv-vers "x.y.z" --channel "latest" \
   --csv-template $my_dir/acm-csv-template.yaml \
   --source-bundle-dir $hub_bundle_spot \
   --source-bundle-dir $hive_bundle_spot \
   --source-bundle-dir $app_sub_bundle_spot

