#!/usr/bin/env python3

# Given a package directory (eg. as is found in the community-operators repo),
# find the bundle directory for the current CSV in a specified channel of a package.

import os
import yaml
import sys

yaml_loader = yaml.SafeLoader

def eprint(*args, **kwargs):
   print(*args, file=sys.stderr, **kwargs)

def emsg(msg, *args):
   eprint("Error: " + msg, *args)

# --- Main ---

def main():

   if len(sys.argv) != 3:
      eprint("Syntax: %s <channel_name> <package_dir>" % sys.argv[0])
      exit(1)

   selected_channel = sys.argv[1]
   pkg_pathn        = sys.argv[2]

   # The package directory should have a single yaml file

   pkg_yamls = []
   bundle_dirs = []
   try:
      pkg_fns = os.listdir(pkg_pathn)
   except FileNotFoundError:
      emsg("Package directory not found: %s." % pkg_pathn)
      exit(1)
   except NotADirectoryError:
      emsg("Not a directory: %s." % pkg_pathn)
      exit(1)

   for fn in pkg_fns:
      pathn = os.path.join(pkg_pathn, fn)
      if os.path.isdir(pathn):
         bundle_dirs.append(pathn)
      elif pathn.endswith(".yaml"):
         pkg_yamls.append(pathn)
   #

   if len(pkg_yamls) == 0:
      emsg("Package manifest (.yaml) not found in %s." % pkg_pathn)
      exit(1)
   elif len(pkg_yamls) > 1:
      emsg("More than one .yaml file found in %s." % pkg_pathn)
      exit(1)

   # Determine the current CSV for the selected channel.

   with open(pkg_yamls[0], "r") as f:
      pkg = yaml.load(f, yaml_loader)

   pkg_channels = pkg["channels"]
   cur_csv = None
   for c in pkg_channels:
      if c["name"] == selected_channel:
         cur_csv = c["currentCSV"]
         break
   #
   if cur_csv is None:
      emsg("Channel %s not found in package." % selected_channel)
      exit(1)

   # Look through all of the bundle directories to find the one containing the CSV.
   # We do so by looking at the contents of the manifests so we avoid any depnedency
   # on directory/manifest file naming patterns.

   the_bundle_dir = None
   for bundle_pathn in bundle_dirs:
      bundle_files = os.listdir(bundle_pathn)

      found_csv = False
      for fn in bundle_files:
         if not fn.endswith(".yaml"):
            continue
         file_pathn = os.path.join(bundle_pathn, fn)
         with open(file_pathn, "r") as f:
            manifest = yaml.load(f, yaml_loader)
         #
         kind = manifest["kind"]
         if kind == "ClusterServiceVersion":
            found_csv = True
            break
      if not found_csv:
         emsg("CSV manifest not found in bundle %s." % bundle_pathn)
         exit(1)

      csv_name = manifest["metadata"]["name"]
      if csv_name == cur_csv:
         the_bundle_dir = bundle_pathn
         break
   # end-for

   if the_bundle_dir is None:
      emsg("Bundle containing CSV %s not found." % cur_csv)
      exit(1)

   print(the_bundle_dir)
   exit(0)

if __name__ == "__main__":
   main()

#-30-

