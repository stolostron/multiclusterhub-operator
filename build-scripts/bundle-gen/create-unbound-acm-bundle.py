#!/usr/bin/env python3
# Assumes: Python 3.6+

# Creates the composite ACM bundle by merging CSVs and other manifests from one or more
# ssource bundles into an output bundle.  Boilerplate info for the output CSV is obtained
# from a template.
#
# Note:
#
# - The main inputs to this script are already-formed OLM/operator bundles, so there is no
#   depedency on eg. repo structures, whether operator-sdk is being used or not, etc.
#
# - Except for a few arg defaults, hopefully this script is not really ACM specific.
#
# - We declare our Pyton requirement as 3.6+ to gain use of the inseration-oder preserving
#   implementation of dict() to have a generated CSV ordering that matches that of the
#   template CSV.  (Python 3.7+ makes this order preserving a part of the language spec, btw).

from bundle_common import *

import argparse
import datetime
import os
import sys
import yaml


# --- Main ---

def main():

   default_pkg_name  = "advanced-cluster-management"
   default_csv_template_pathn ="./acm-csv-template.yaml"

   # Handle args:

   parser = argparse.ArgumentParser()

   parser.add_argument("--pkg-dir",  dest="pkg_dir_pathn", required=True)
   parser.add_argument("--pkg-name", dest="pkg_name",      default=default_pkg_name)
   parser.add_argument("--channel",  dest="for_channels",  required=True, action="append")

   parser.add_argument("--csv-vers",  dest="csv_vers", default="x.y.z")
   parser.add_argument("--prev-vers", dest="prev_vers")

   parser.add_argument("--source-bundle-dir", dest="source_bundle_pathns", required=True, action="append")

   parser.add_argument("--csv-template", dest="csv_template_pathn", default=default_csv_template_pathn)

   args = parser.parse_args()

   csv_template_pathn = args.csv_template_pathn

   operator_name = args.pkg_name
   pkg_name      = args.pkg_name
   pkg_dir_pathn = args.pkg_dir_pathn
   for_channels  = args.for_channels

   csv_vers  = args.csv_vers
   prev_vers = args.prev_vers

   source_bundle_pathns = args.source_bundle_pathns


   # And now on to the show...

   csv_name  = "%s.v%s" % (operator_name, csv_vers)
   csv_fn    = "%s.clusterserviceversion.yaml" % (csv_name)

   # The package directory is the directory in which we place a version-named
   # sub-directory for the new bundle.  Make sure the package directory exists,
   # and then create (or empty out) a bundle directory under it.

   if not os.path.exists(pkg_dir_pathn):
      die("Output package directory doesn't exist: %s" % pkg_dir_pathn)
   elif not os.path.isdir(pkg_dir_pathn):
      die("Output package path exists but isn't a directory: %s" % pkg_dir_pathn)

   bundle_pathn = os.path.join(pkg_dir_pathn, csv_vers)
   create_or_empty_directory("output bundle", bundle_pathn)

   csv_pathn = "%s/%s" % (bundle_pathn, csv_fn)


   # Load or create the package manifest.

   pkg_manifest_pathn = os.path.join(pkg_dir_pathn, "package.yaml")
   pkg_manifest = load_pkg_manifest(pkg_manifest_pathn, pkg_name)


   # Load/parse the base template for the CSV we're generating.  This template provides various
   # boilerplate we're going to use as-is in the output CSV we're generating.

   o_csv = load_manifest("CSV template", csv_template_pathn)

   # Check that the specified bundle directories exist.
   for s_bundle_pathn in source_bundle_pathns:
      if not os.path.isdir(s_bundle_pathn):
         die("Source bundle directory doesn't exist or isn't a directory: %s" % s_bundle_pathn)
   #

   o_spec = o_csv["spec"]

   # Holds info that is accumulated over the set of source bundles:
   m_categories        = set()
   m_keywords          = set()
   m_alm_examples      = dict()
   m_owned_crds        = dict()
   m_required_crds     = dict()
   m_owned_api_svcs    = dict()
   m_required_api_svcs = dict()
   m_deployments       = dict()
   m_cluster_perms     = dict()
   m_ns_perms          = dict()

   bundle_fns = set() # Used to ensure no dups/overlays iin file names added to buundle

   # Process each of the source bundles:

   first_bundle = True
   for s_bundle_pathn in source_bundle_pathns:

      if not first_bundle:
         print("\n------------\n")
      first_bundle = False

      print("Processing bundle: %s...\n" % s_bundle_pathn)

      s_owned_crds_map = dict()

      # Load all bundle manifests

      s_csv_fn = None
      s_csv    = None
      s_other_manifests = dict()

      manifests = load_all_manifests(s_bundle_pathn)
      for fn, manifest in manifests.items():
         kind = manifest["kind"]
         if kind == "ClusterServiceVersion":
            if s_csv is None:
               s_csv = manifest
               s_csv_fn = fn
            else:
               die("Too many CSV manifests found in %s." % s_bundle_pathn)
         else:
            s_other_manifests[fn] = manifest

      #--- Consume the bundle's CSV ---

      # Make sure we have only one CSV.

      if s_csv is None:
         die("No CSV manifest found in %s." % s_bundle_pathn)

      print("Found source CSV manifest: %s" % s_csv_fn)

      s_spec = get_map(s_csv, "spec")
      if not s_spec:
         die("Source CSV doesn't have a (non-empty) spec.")

      s_metadata = get_map(s_csv, "metadata")
      if not s_metadata:
         die("Source CSV doesn't have any metadata.")

      s_annotations = get_map(s_metadata, "annotations")
      if not s_annotations:
         print("   WARN: Source CSV doesn't have any annotations.")

      # Accumulate categories into the output set.
      s_cat_str = get_scalar(s_annotations, "categories")
      if s_cat_str is None:
         print("   WARN: Source CSV has no categories.")
      else:
         # Categories are specified as a common-separated string.  Spint and accumulate.
         accumulate_set("category", "categories", s_cat_str.split(","), m_categories)

      # Accumulate CR examples (ALM-Examples) into the output set.
      s_alm_examples_str = get_scalar(s_annotations, "alm-examples")
      if s_alm_examples_str is None:
         print("   WARN: Source CSV has no ALM examples.")
      else:
         # ALM examples contains a string representation of a YML sequence of mappings.
         s_alm_examples = yaml.load(s_alm_examples_str, Loader=yaml_loader)
         accumulate_keyed("ALM example", s_alm_examples, m_alm_examples, get_avk)

      # Accumulate keywords into the output set.
      s_keywords = get_seq(s_spec, "keywords")
      accumulate_set("keyword", "keywords", s_keywords, m_keywords)

      # Add owned CRds from this CSV into the list we're accumulating.  Keep track
      # of them by GVK so we can reconsile against required CRDs later.
      try:
         s_crds = s_spec["customresourcedefinitions"]
         s_owned_crds_list = s_crds["owned"]
         accumulate_keyed("owned CRD", s_owned_crds_list, m_owned_crds, get_gvk, another_thing_map=s_owned_crds_map)
      except KeyError:
         print("   WARN: Source CSV specs no owned CRDs. (???)")
         s_owned_crds = []

      # Nowc collect up the required CRDs.
      try:
         s_crds = s_spec["customresourcedefinitions"]
         s_required_crds = s_crds["required"]
         accumulate_keyed("required CRD", s_required_crds, m_required_crds, get_gvk, dups_ok=True)
      except KeyError:
         # No warn msg as its perfectly fine for a CSV to not defined any required CRDs.
         s_required_crds = []

      # Collect up spec.install stanzas...

      s_install = s_spec["install"]
      s_install_strategy = s_install["strategy"]
      if s_install_strategy != "deployment":
         die("Source CSV specs unsupported install stragegy (%s)." % s_install_strategy)

      s_install_spec = s_install["spec"]

      # Cluster and namespace Permissions (Service Accounts):
      try:
         s_cluster_perms = s_install_spec["clusterPermissions"]
         accumulate_keyed("cluster permission", s_cluster_perms, m_cluster_perms, lambda e: e["serviceAccountName"])
      except KeyError:
         s_cluster_perms = []

      try:
         s_ns_perms = s_install_spec["permissions"]
         accumulate_keyed("namespace permission", s_perms, m_ns_perms, lambda e: e["serviceAccountName"])
      except KeyError:
         s_ns_perms = []

      if not (s_cluster_perms or s_ns_perms):
         print("   WARN: Source CSV defines neither cluster nor namespace permissions/service accounts.")

      # Deployments:
      try:
         s_deployments = s_install_spec["deployments"]
         accumulate_keyed("install deployment", s_deployments, m_deployments, lambda e: e["name"])
      except KeyError:
         print("   WARN: Source CSV specs no install deployments. (???)")
         s_deployments = []

      #--- Copy the source budnle's non-CSV manifests to the output bundle ---

      print("\nHandling non-CSV manifests in the budnle")

      expected_crds = set(s_owned_crds_map.keys())

      for fn, manifest in s_other_manifests.items():

         kind = manifest["kind"]
         if kind == "CustomResourceDefinition":
            crd_gvk = get_gvk_for_crd(manifest)

            # Check that the CRD is expected (listed as owned in CSV) and if so, take
            # it out of the list of expected ones not seen yet.
            crd_is_expected = crd_gvk in expected_crds
            if crd_is_expected:
               expected_crds.remove(crd_gvk)

            k = "CRD" if crd_is_expected else "*Unlisted* CRD"
            print("   Copying manifest file (%s): %s" % (k, fn))

         else:
            # We have a manifest file for something other than a CRD???
            print("***TBD???: %s in %s" % (kind, fn))

         if fn not in bundle_fns:
            copy_file(fn, s_bundle_pathn, bundle_pathn)
            bundle_fns.add(fn)
         else:
            die("Duplicate mainfest filename: %s." % t_manifest_fn)
      #

      # Check that we found manifests for all CRDs owned by this source bundle
      if expected_crds:
         for crd_gvk in expected_crds:
            die("No manifest found for expected CRD: %s" % crd_gvk)
      else:
         print("   Note: Manfests were copied for all expected CRDs.")

      #
   # End for each source-bundle

   print("\n============\n")
   print("Creating merged CSV...")


   # --- Reconsile and generate output CSV properties ---

   # Plug in simple metadata

   o_metadata    = o_csv["metadata"]
   o_annotations = o_metadata["annotations"]

   created_at = datetime.datetime.now().strftime("%Y-%m-%dT%H:%M:%SZ")

   o_metadata["name"] = csv_name
   o_annotations["createdAt"] = created_at

   # Convert categories into a common-separated string and plug into annotations
   o_annotations["categories"] = ','.join(sorted(list(m_categories)))

   # Convert ALM examples into a sting representation and plug into annotations.
   o_alm_examples = list(m_alm_examples.values())
   o_alm_examples_str = yaml.dump(o_alm_examples, width=100, default_flow_style=False, sort_keys=False)
   o_annotations["alm-examples"] = o_alm_examples_str

   o_spec["version"]  = csv_vers

   if prev_vers:
      prev_csv_name = "%s.v%s" % (pkg_name, prev_vers)
      o_spec["replaces"] = prev_csv_name
   else:
      try:
         del o_spec["replaces"]
      except KeyError:
         pass

   # Plug in the merged keyword list (no dups)
   o_spec["keywords"] = list(sorted(m_keywords))

   # Plug in reconsiled/merged CRD info...

   o_crds = o_spec["customresourcedefinitions"]
   reconsile_and_plug_in_things("CRD", o_crds, m_owned_crds, m_required_crds)

   # Tidy up: If no CRD info at all, remove the spec stanza.
   if not o_crds:
      del o_spec["customresourcedefinitions"]


   #-Plug in reconsiled/merged API service info...
   o_api_svcs = o_spec["apiservicedefinitions"]
   reconsile_and_plug_in_things("API service", o_api_svcs, m_owned_api_svcs, m_required_api_svcs)

   # Tidy up: If no API service definitions info at all, remove the spec stanza.
   if not o_api_svcs:
      del o_spec["apiservicedefinitions"]

   # Now plug in merged/editedspec.install contents...

   o_install = o_spec["install"]
   o_install["strategy"] = "deployment"  # The only strategy we currently support.
   o_install_spec  = o_install["spec"]

   print("Plugging in install permissions...")
   plug_in_things("cluster permission",   o_install_spec, "clusterPermissions", m_cluster_perms)
   plug_in_things("naemspace permission", o_install_spec, "permissions",        m_ns_perms)

   print("Plugging in install deployments...")
   plug_in_things("deployment",           o_install_spec, "deployments",        m_deployments, True)

   # --- Write out the resutling merged CSV ---

   if csv_fn not in bundle_fns:
      print("\nWriting merged CSV mainfest: %s" % csv_fn)
      dump_manifest("merged CSV", csv_pathn, o_csv)
      bundle_fns.add(csv_fn)
   else:
      die("Duplicate manifest filename (for the CSV): %s." % csv_fn)

   # --- Update the package manifest to point to the new CSV ---

   print("Updating package manifest.")
   update_pkg_manifest(pkg_manifest, for_channels, csv_name)
   dump_manifest("package manifest", pkg_manifest_pathn, pkg_manifest)

   return

if __name__ == "__main__":
   main()

#-30-

