#!/usr/bin/env python3
# Copyright (c) 2020 Red Hat, Inc.

# Assumes: Python 3.6+

# Takes an "unbound" CSV bundle abd configures it for a release by:
#
# - Updating CSV version and CSV name (to match version)
# - Setting the "replaces" property.
# - Overriding image refernces
# - Setting the createdAt timestamp
#
# Note:
# - We declare our Pyton requirement as 3.6+ to gain use of the inseration-oder preserving
#   implementation of dict() to have a generated CSV ordering that matches that of the
#   template CSV.  (Python 3.7+ makes this order preserving a part of the language spec, btw).

from bundle_common import *

import argparse
import datetime
import os


# --- Main ---

def main():

   # Handle args:

   parser = argparse.ArgumentParser()

   parser.add_argument("--source-bundle-dir", dest="source_bundle_pathn", required=True)

   parser.add_argument("--pkg-dir",  dest="pkg_dir_pathn", required=True)
   parser.add_argument("--pkg-name", dest="pkg_name", required=True)

   parser.add_argument("--use-bundle-image-format", dest="use_bundle_image_format", action="store_true")

   parser.add_argument("--default-channel",    dest="default_channel", required=True)
   parser.add_argument("--replaces-channel",   dest="replaces_channel")
   parser.add_argument("--additional-channel", dest="other_channels", action="append")

   parser.add_argument("--csv-vers",  dest="csv_vers", required=True)
   parser.add_argument("--prev-vers", dest="prev_vers")

   parser.add_argument("--image-override", dest="image_overrides", action="append", required=True)

   args = parser.parse_args()

   source_bundle_pathn = args.source_bundle_pathn

   operator_name  = args.pkg_name
   pkg_name       = args.pkg_name
   pkg_dir_pathn  = args.pkg_dir_pathn

   use_bundle_image_format = args.use_bundle_image_format

   replaces_channel = args.replaces_channel
   other_channels   = args.other_channels
   default_channel  = args.default_channel

   csv_vers  = args.csv_vers
   prev_vers = args.prev_vers

   image_override_list = args.image_overrides

   csv_name = "%s.v%s" % (pkg_name, csv_vers)
   csv_fn   = "%s.clusterserviceversion.yaml" % (csv_name)

   # The package directory is the directory in which we place a version-named
   # sub-directory for the new bundle.  Make sure the package directory exists,
   # and then create (or empty out) a bundle directory under it.

   if not os.path.exists(pkg_dir_pathn):
      die("Output package directory doesn't exist: %s" % pkg_dir_pathn)
   elif not os.path.isdir(pkg_dir_pathn):
      die("Output package path exists but isn't a directory: %s" % pkg_dir_pathn)

   # There seem to be several formats for a bundle directore, depending on how they
   # are being published: being placed in an operator bundle image, or made available
   # some other way (eg. via a Quay.io Application Repo).
   #
   # When the bundle is in bundle-image format, the bundle directory has a manifests
   # subdirectory containing all CSV, CRD manifests, and a metadata directory containing
   # an annotations manifest. When other formats these subdirectories do not exist and
   # all manifests are in the bundle directory itself.  (The bits of metadata are instead
   # in a package.yamml in the package directory containing the bundle.)

   if use_bundle_image_format:
      bundle_pathn = os.path.join(pkg_dir_pathn, csv_vers, "manifests")
   else:
      bundle_pathn = os.path.join(pkg_dir_pathn, csv_vers)
   create_or_empty_directory("outout bundle manifests", bundle_pathn)


   # Load or create the (output) package manifest.

   pkg_manifest_pathn = os.path.join(pkg_dir_pathn, "package.yaml")
   pkg_manifest = load_pkg_manifest(pkg_manifest_pathn, pkg_name)
   if default_channel:
      pkg_manifest["defaultChannel"] = default_channel
   else:
      # TODO/Idea: Use default channel already in package manifest if any?
      if use_bundle_image_format:
         emsg("A default channel is required when using bundle-image format.")

   # See if this CSV is to replace an existing one as determeined by:
   #
   # - Previous version speicified by invocation arg, or if none
   # - Current CSV listed on the replaces channel for the package.

   prev_csv_name = None
   if prev_vers:
      prev_csv_name = "%s.v%s" % (pkg_name, prev_vers)
   else:
      if replaces_channel:
         chan = find_channel_entry(pkg_manifest, replaces_channel)
         if chan is not None:
            prev_csv_name = chan["currentCSV"]

   print("New CSV name: %s" % csv_name)
   if prev_csv_name:
      print("Replaces previous CSV: %s" % prev_csv_name)
   else:
      print("NOTE: New CSV does not replace a previous one.")

   channels_to_update = list()
   if replaces_channel:
      channels_to_update.append(replaces_channel)
   if default_channel:
      channels_to_update.append(default_channel)
   if other_channels:
      channels_to_update.extend(other_channels)

   # Load all manifests in the source bundle directory.  For non-CSV manifests,
   # just copy over to output bundle.  Hold onto the CSV for later manipulation.

   csv = None

   manifests = load_all_manifests(source_bundle_pathn)
   for manifest_fn, manifest in manifests.items():
      kind = manifest["kind"]
      if kind == "ClusterServiceVersion":
         if csv is None:
            csv = manifest
         else:
            die("More than one CSV manifest found in bundle directory.")
      else:
         print("Copying manifest file unchanged: %s" % manifest_fn)
         copy_file(manifest_fn, source_bundle_pathn, bundle_pathn)
   #

   # Adjust CSV name and creation timestamp in metadata

   metadata = csv["metadata"]
   metadata["name"] = csv_name

   created_at = datetime.datetime.now().strftime("%Y-%m-%dT%H:%M:%SZ")

   annotations = metadata["annotations"]
   annotations["createdAt"] = created_at

   # Plug in version and previous CSV ("replaces") if any.

   spec = csv["spec"]
   spec["version"]  = csv_vers
   if prev_csv_name is not None:
      spec["replaces"] = prev_csv_name
   else:
      try:
         del spec["replaces"]
      except KeyError:
         pass

   # Adjust image refs in deployment specs according to overrides:

   image_overrides = create_image_override_map(image_override_list)

   install_spec = spec["install"]["spec"]
   deployments = install_spec["deployments"]

   if image_overrides:
      print("Updating image references...")
      for deployment in deployments:
         update_image_refs_in_deployment(deployment, image_overrides)

   # Write out the updated CSV

   csv_pathn = os.path.join(bundle_pathn, csv_fn)

   print("Writing CSV mainfest: %s" % csv_fn)
   dump_manifest("bound CSV", csv_pathn, csv)

   # Generate metadata/annotatoins

   if use_bundle_image_format:
      metadata_pathn = os.path.join(pkg_dir_pathn, csv_vers, "metadata")
      create_or_empty_directory("outout bundle metadata", metadata_pathn)

      annotations_manifest = dict()
      annotations_manifest["annotations"] = dict()
      annot = annotations_manifest["annotations"]

      annot["operators.operatorframework.io.bundle.mediatype.v1"] = "registry+v1"
      annot["operators.operatorframework.io.bundle.manifests.v1"] = "manifests/"
      annot["operators.operatorframework.io.bundle.metadata.v1"]  = "metadata/"

      annot["operators.operatorframework.io.bundle.package.v1"] = pkg_name

      channels_list = ','.join(sorted(list(channels_to_update)))
      annot["operators.operatorframework.io.bundle.channels.v1"] = channels_list

      # Observation: Having a default channel annotation be a property of a bundle
      # (representing a specific version of a CSV) seems odd, as this is really a
      # property of the package, not a paritcular CSV.  I wonder what the result is if
      # multiple CSVs decleare different defaults??
      annot["operators.operatorframework.io.bundle.channel.default.v1"] = default_channel

      print("Writing bundle metadata.")
      bundle_annotations_pathn = os.path.join(metadata_pathn, "annotations.yaml")
      dump_manifest("bundle metadata", bundle_annotations_pathn, annotations_manifest)

   # Update the package manifest to point to the new CSV

   print("Updating package manifest.")
   update_pkg_manifest(pkg_manifest, channels_to_update, csv_name)
   dump_manifest("package manifest", pkg_manifest_pathn, pkg_manifest)

   exit(0)

if __name__ == "__main__":
   main()

#-30-

