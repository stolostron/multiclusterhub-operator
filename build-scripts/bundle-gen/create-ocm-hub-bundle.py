#!/usr/bin/env python3
# Assumes: Python 3.6+

# Creates the OCM Hub bundle from parts in the hub operator repo's deploy directory.

# Note:
#
# - Besides creating the CSV/bindle, this script also maintains the current-on-channel
#   pointer for a specified channel in the package manifest.  If the package already
#   points to a particular CSV for the channel, we assume the new CSV replaces that
#   one (i.e. use currentcSV on entry  as the replacesCSV value in the new CSV).
#
# - We declare our Pyton requirement as 3.6+ to gain use of the inseration-oder preserving
#   implementation of dict() to have a generated CSV ordering that matches that of the
#   template CSV.  (Python 3.7+ makes this order preserving a part of the language spec, btw).

from bundle_common import *

import argparse
import datetime
import os
import yaml


# --- Main ---

def main():

   default_pkg_name  = "open-cluster-management-hub"
   default_channel   = "community-1.0"
   default_csv_template_pathn ="./ocm-hub-csv-template.yaml"

   # Handle args:

   parser = argparse.ArgumentParser()

   parser.add_argument("--deploy-dir", dest="deploy_dir_pathn", required=True)

   parser.add_argument("--pkg-dir",  dest="pkg_dir_pathn", required=True)
   parser.add_argument("--pkg-name", dest="pkg_name", default=default_pkg_name)

   parser.add_argument("--replaces-channel", dest="replaces_channel", required=True)
   parser.add_argument("--other-channel",    dest="other_channels", action="append")

   parser.add_argument("--csv-vers",  dest="csv_vers", required=True)

   parser.add_argument("--image-override", dest="image_overrides", action="append")

   parser.add_argument("--csv-template", dest="csv_template_pathn", default=default_csv_template_pathn)

   args = parser.parse_args()

   csv_template_pathn = args.csv_template_pathn

   operator_name  = args.pkg_name
   pkg_name       = args.pkg_name
   pkg_dir_pathn  = args.pkg_dir_pathn

   replaces_channel = args.replaces_channel
   other_channels   = args.other_channels

   csv_vers  = args.csv_vers

   deploy_dir_pathn = args.deploy_dir_pathn

   owned_crds_dir_pathn = os.path.join(deploy_dir_pathn, "crds")
   req_crds_dir_pathn   = os.path.join(deploy_dir_pathn, "req_crds")

   operator_deployment_pathns = [os.path.join(deploy_dir_pathn, "operator.yaml")]
   operator_role_pathns       = [os.path.join(deploy_dir_pathn, "role.yaml")]

   image_override_list = args.image_overrides


   # And now on to the show...

   csv_name  = "%s.v%s" % (pkg_name, csv_vers)
   csv_fn    = "%s.clusterserviceversion.yaml" % (csv_name)

   # The package directory is the directory in which we place a version-named
   # sub-directory for the new bundle.  Make sure the package directory exists,
   # and then create (or empty out) a bundle directory under it.

   if not os.path.exists(pkg_dir_pathn):
      die("Output package directory doesn't exist: %s" % pkg_dir_pathn)
   elif not os.path.isdir(pkg_dir_pathn):
      die("Output package path exists but isn't a directory: %s" % pkg_dir_pathn)

   bundle_pathn = os.path.join(pkg_dir_pathn, csv_vers)
   try:
      os.mkdir(bundle_pathn)
   except FileExistsError:
      if os.path.isdir(bundle_pathn):
         for fn in os.listdir(bundle_pathn):
            fpathn = os.path.join(bundle_pathn, fn)
            os.unlink(fpathn)
      else:
         die("Output bundle directory path exists but isn't a directory: %s" % bundle_dir_pathn)
   #

   csv_pathn = "%s/%s" % (bundle_pathn, csv_fn)


   # Load or create the package manifest.

   pkg_manifest_pathn = os.path.join(pkg_dir_pathn, "package.yaml")
   pkg_manifest = load_pkg_manifest(pkg_manifest_pathn, pkg_name)

   # See if this CSV is to replace an existing one.

   chan = find_channel_entry(pkg_manifest, replaces_channel)
   if chan is not None:
      prev_csv_name = chan["currentCSV"]
   else:
      prev_csv_name = None

   channels_to_update = [replaces_channel]
   channels_to_update.extend(other_channels)

   # Reformat image overrides into a map for easy lookup

   image_overrides = create_image_override_map(image_override_list)


   # Load/parse the base template for the CSV we're generating.  This template provides
   # various boilerplate we're going to use as-in the output CSV.

   o_csv = load_manifest("CSV template", csv_template_pathn)
   o_spec = o_csv["spec"]

   # Process the owned-CRDs directory to determine those CRDs and related ALM examples.

   print("Processing owned-CRDs directory: %s..." % owned_crds_dir_pathn)

   alm_examples = dict()
   owned_crds   = dict()

   manifests = load_all_manifests(owned_crds_dir_pathn)
   for manifest_fn, manifest in manifests.items():
      # Each manifest file contains a single CRD/CR example
      kind = manifest["kind"]
      if kind == "CustomResourceDefinition":
         accumulate_keyed("owned CRD", [manifest], owned_crds, get_gvk_for_crd)
         print("   Copying CRD manifest file: %s" % manifest_fn)
         copy_file(manifest_fn, owned_crds_dir_pathn, bundle_pathn)
      else:
         accumulate_keyed("ALM example", [manifest], alm_examples, get_avk)

   if not owned_crds:
      die("No owned CRDs found.")
   if not alm_examples:
      print("   WARN: No CR examples (ALM examples) found.")

   # Process the required-CRDs directory to gather our required CRD info.

   print("Processing required-CRDs directory: %s..." % req_crds_dir_pathn)

   required_crds = dict()

   manifests = load_all_manifests(req_crds_dir_pathn)
   for manifest_fn, manifest in manifests.items():
      # Each manifest file contains a list of required CRD references
      accumulate_keyed("required CRD", manifest, required_crds, get_gvk)

   # Colelct up owned/requied API Service info.
   # TGBD: Implement me when needed.

   owned_api_svcs = dict()
   required_api_svcs   = dict()

   # Collect up install permission info from role manifests..

   print("Picking up operator permissions (roles/service accounts)...")

   cluster_perms = dict()
   ns_perms      = dict()

   for manifest_pathn in operator_role_pathns:
      manifest = load_manifest("role manifest", manifest_pathn)

      csv_perm = {"name": manifest["metadata"]["name"]}
      csv_perm["rules"]   = manifest["rules"]

      k = manifest["kind"]
      if k == "ClusterRole":
         accumulate_keyed("cluster permission", [csv_perm], cluster_perms, lambda e: e["name"])
      elif k == "Role":
         accumulate_keyed("namespace permission", [csv_perm], cluster_perms, lambda e: e["name"])
      else:
         die("Unrecognized kind of role: %s" % k)

   if not cluster_perms:
      print("   Note: No cluster-wide permissions found.")
   if not ns_perms:
      print("   Note: No namespace permissions found.")
   if not (cluster_perms or ns_perms):
      die("No cluster or namespace permissions found.")


   # Collect up install deployment info from deployment manifests.

   print("Picking up operator install deployment...")

   deployments = dict()

   for manifest_pathn in operator_deployment_pathns:
      manifest = load_manifest("operator manifest", manifest_pathn)
      csv_deployment = {"name": manifest["metadata"]["name"]}
      csv_deployment["spec"]   = manifest["spec"]
      accumulate_keyed("install deployment", [csv_deployment], deployments, lambda e: e["name"])

   if not deployments:
      die("No install deployments found.")


   # Adjust image refs in deployment specs according to overrides:

   print("Updating image references...")

   for deployment_name, deployment in deployments.items():
      update_image_refs_in_deployment(deployment, image_overrides)
   #

   # --- Form the output CSV ---

   o_metadata = o_csv["metadata"]
   o_metadata["name"] = csv_name

   created_at = datetime.datetime.now().strftime("%Y-%m-%dT%H:%M:%SZ")

   o_annotations = o_metadata["annotations"]
   o_annotations["createdAt"] = created_at

   # Convert ALM examples into a sting representation and plug into annotations.

   o_alm_examples = list(alm_examples.values())
   o_alm_examples_str = yaml.dump(o_alm_examples, width=100, default_flow_style=False, sort_keys=False)
   o_annotations["alm-examples"] = o_alm_examples_str

   # Plug in version and previous CSV version, if any.

   o_spec["version"]  = csv_vers
   if prev_csv_name is not None:
      o_spec["replaces"] = prev_csv_name  # TODO: Should this be a list (if allowing skipped versions)?
   else:
      try:
         del o_spec["replaces"]
      except KeyError:
         pass

   # Plug in owned/required CRDs

   o_crds = o_spec["customresourcedefinitions"]
   plug_in_things_quietly(o_crds, "owned",    owned_crds)
   plug_in_things_quietly(o_crds, "required", required_crds)

   # Tidy up: If no CRD info at all, remove the spec stanza.
   if not o_crds:
      del o_spec["customresourcedefinitions"]

   # Plug in owned/required API Services

   o_api_svcs = o_spec["apiservicedefinitions"]
   plug_in_things_quietly(o_api_svcs, "owned",    owned_api_svcs)
   plug_in_things_quietly(o_api_svcs, "required", required_api_svcs)

   # Tidy up: If no API Services info at all, remove the spec stanza.
   if not o_api_svcs:
      del o_spec["apiservicedefinitions"]

   # Now plug in spec.install contents...

   o_install = o_spec["install"]
   o_install["strategy"] = "deployment"
   o_install_spec  = o_install["spec"]

   plug_in_things_quietly(o_install_spec, "clusterPermissions", cluster_perms)
   plug_in_things_quietly(o_install_spec, "permissions",        ns_perms)
   plug_in_things_quietly(o_install_spec, "deployments",        deployments)


   # --- Write out the resutling CSV ---

   print("\nWriting CSV mainfest: %s" % csv_fn)
   dump_manifest("merged CSV", csv_pathn, o_csv)


   # --- Update the package manifest to point to the new CSV ---

   print("Updating package manifest.")
   update_pkg_manifest(pkg_manifest, channels_to_update, csv_name)
   dump_manifest("package manifest", pkg_manifest_pathn, pkg_manifest)

   exit(0)

if __name__ == "__main__":
   main()

#-30-

