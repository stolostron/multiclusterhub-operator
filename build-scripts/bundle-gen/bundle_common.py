# Copyright (c) 2020 Red Hat, Inc.

# Some common functions for ACM/OCM bundle building scripts.

# Assumes: Python 3.6+

import os
import shutil
import sys
import yaml

yaml_loader = yaml.SafeLoader


def eprint(*args, **kwargs):
   print(*args, file=sys.stderr, **kwargs)

def emsg(msg, *args):
   eprint("Error: " + msg, *args)

def die(msg, *args):
   eprint("Error: " + msg, *args)
   eprint("Aborting.")
   exit(2)

# Accumulate a set of scalars
def accumulate_set(thing_kind, thing_kind_pl, thing_list, thing_set):

   # Capitalize just first char of first word.
   capitalized_thing_kind = thing_kind[0:1].upper() + thing_kind[1:]

   if not thing_list:
      print("   Info: Source CSV has no %s." % thing_kind_pl)
   else:
      for t in thing_list:
         tt = t.strip()
         if tt not in thing_set:
            print("   %s: %s" % (capitalized_thing_kind, tt))
            thing_set.add(tt)


# Accumulates a collection of keyed things, optionally aborting on dup keys.
def accumulate_keyed(thing_kind, thing_list, thing_map, key_getter, dups_ok=False, another_thing_map=None):

   # Capitalize just first char of first word.
   capitalized_thing_kind = thing_kind[0:1].upper() + thing_kind[1:]

   for thing in thing_list:
      key = key_getter(thing)
      if key not in thing_map:
         print("   %s: %s" % (capitalized_thing_kind, key))
         thing_map[key] = thing
      else:
         if not dups_ok:
            die("Duplicate %s: %s." % (thing_kind, key))

      # Also accomulate into a second map in passed, eg. a per-source-bundle map rather
      # than one that is accumulating over all source bundles.
      if another_thing_map is not None:
         another_thing_map[key] = thing
   #
   return


# Plugs a list of things into into a base stanza, deleting anchoring property if list is empty.
def plug_in_things_quietly(base_map, prop_name, things_map):

   if things_map:
      base_map[prop_name] = list(things_map.values())
   else:
      try:
         del base_map[prop_name]
      except KeyError:
         pass
   return


# Plugs a list of things into into a base stanza, deleting anchoring property if list is empty.
def plug_in_things(thing_kind, base_map, prop_name, things_map, warn_on_none=False):

   plug_in_things_quietly(base_map, prop_name, things_map)
   if not things_map:
      thing_kind_pl = "%ss" % thing_kind
      msg_sev = "WARN" if warn_on_none else "Note"
      print("   %s: Merged CSV has no no %s." % (msg_sev,thing_kind_pl))
   return


# Reconsiles a set of required vs. owned keyed thigns and plugs resulting sets into a stanza.
def reconsile_and_plug_in_things(thing_kind, things, owned_things, required_things):

   thing_kind_pl = "%ss" % thing_kind
   owned_thing_pl    = "owned %s" % thing_kind_pl
   required_thing_pl = "required %s" % thing_kind_pl

   print("Reconsiling required vs. owned %s." % thing_kind_pl)

   # Plug the merged list of owned things(eg. CRDs, API Services) into the output CSV
   plug_in_things(owned_thing_pl, things, "owned", owned_things)

   # Reconsile required things against owned things: We don't want to express a reqruiement
   #  for a needed thing if the merged CSV will be prodiving it.  Plug resulting list into
   #  output CSV.

   req_things_to_remove = list()
   for req_thing_gvk in required_things.keys():
      if req_thing_gvk in owned_things:
         req_things_to_remove.append(req_thing_gvk)
   if req_things_to_remove:
      for req_thing_gvk in req_things_to_remove:
         print("   %s requirement internally satisfied: %s" % (thing_kind, req_thing_gvk))
         del required_things[req_thing_gvk]
      #
   else:
      print("   No %s requirements are internally satisfied." % thing_kind)

   # Plug in the resulting required-things list.
   plug_in_things(required_thing_pl, things, "required", required_things, True)

   return

# Forms a group/version/kind string from a map containg group, kind, name, version properties.
def get_gvk(a_map):

   kind  = a_map["kind"]
   vers  = a_map["version"]

   # Some CRD references might not have a group property, but hopefully they have
   # a name property from which group can be deduced.

   group = None
   try:
      group = a_map["group"]
   except KeyError:
      # No group property, deduce group frmo name property which we assume is
      #  in the form <kinds>.group.
      # TODO: Consult OLM doc on how it handles this case.
      try:
         name = a_map["name"]
         group = name[name.index(".")+1:]
         # Let this blow up with ValueError if name is not in dotted form.
      except KeyError:
         die("Can't determine API group for CRD of kind %s." % kind)
   gvk = "%s/%s/%s" % (group, vers, kind)
   return gvk

# Forms a group/version/kind string from a map containg apiVersion and kind properties.
def get_avk(a_map):

   group_version = a_map["apiVersion"]
   kind = a_map["kind"]
   gvk = "%s/%s" % (group_version, kind)
   return gvk

# Forms a group/version/kind string from a CRD resource:
def get_gvk_for_crd(crd_map):

   spec = crd_map["spec"]
   group = spec["group"]
   vers  = spec["version"]
   kind  = spec["names"]["kind"]
   gvk = "%s/%s/%s" % (group, vers, kind)
   return gvk


# Get a sequence property, defaulting to an empty one.
def get_seq(from_map, prop_name):
   try:
      s = from_map[prop_name]
   except KeyError:
      s = list()
   return s

# Get a map property, defaulting to an empty one.
def get_map(from_map, prop_name):
   try:
      m = from_map[prop_name]
   except KeyError:
      m = dict()
   return m

# GEt a scalar property, defaulting to None.
def get_scalar(from_map, prop_name):
   try:
      s = from_map[prop_name]
   except KeyError:
      s = None
   return s


# Load a manifest (YAML) file.
def load_manifest(manifest_type, pathn):

   if not pathn.endswith(".yaml"):
      return None
   try:
      with open(pathn, "r") as f:
         return yaml.load(f, yaml_loader)
   except FileNotFoundError:
      cap_manifest_type= manifest_type[0:1].upper() + manifest_type[1:]
      die("%s not found: %s" % (cap_manifest_type, pathn))


# Loads all YAML manifests found in a directory.
def load_all_manifests(dir_pathn):

   manifests = dict()

   all_fns = os.listdir(dir_pathn)
   for fn in all_fns:
      if not fn.endswith(".yaml"):
         continue
      manifest = load_manifest("file", os.path.join(dir_pathn, fn))
      manifests[fn] = manifest
   #
   return manifests

# Write out a YAML manifest
def dump_manifest(manifest_type, pathn, manifest):

   with open(pathn, "w") as f:
      yaml.dump(manifest, f, width=100, default_flow_style=False, sort_keys=False)
   return

# Copy a file from a source directory to a destination directory.
def copy_file(fn, from_dir_pathn, to_dir_pathn):

   src_pathn  = os.path.join(from_dir_pathn, fn)
   dest_pathn = os.path.join(to_dir_pathn,   fn)
   shutil.copy(src_pathn, dest_pathn)

   return

# Creates a directory, or empties out contents if directory exists.
def create_or_empty_directory(dir_type, pathn):

   try:
      os.makedirs(pathn)
   except FileExistsError:
      if os.path.isdir(pathn):
         for fn in os.listdir(pathn):
            fpathn = os.path.join(pathn, fn)
            os.unlink(fpathn)
      else:
         cap_dir_type= dir_type[0:1].upper() + dir_type[1:]
         die("%s directory path exists but isn't a directory: %s" % (cap_dir_type, bundle_dir_pathn))

   return


# Creates an image-overide map from a list of overrides (from args).
def create_image_override_map(image_override_specs):

   image_overrides = dict()
   if image_override_specs:
      for o in image_override_specs:
         parsed_ref = parse_image_ref(o)
         repository = parsed_ref["repository"]
         image_overrides[repository] = o
   return image_overrides


# Parse a container image reference.
def parse_image_ref(image_ref):

   # Image ref:  [registry-and-ns/]repository-name[:tag][@digest]

   parsed_ref = dict()

   remaining_ref = image_ref
   at_pos = remaining_ref.rfind("@")
   if at_pos > 0:
      parsed_ref["digest"] = remaining_ref[at_pos+1:]
      remaining_ref = remaining_ref[0:at_pos]
   else:
      parsed_ref["digest"] = None
   colon_pos = remaining_ref.rfind(":")
   if colon_pos > 0:
      parsed_ref["tag"] = remaining_ref[colon_pos+1:]
      remaining_ref = remaining_ref[0:colon_pos]
   else:
      parsed_ref["tag"] = None
   slash_pos = remaining_ref.rfind("/")
   if slash_pos > 0:
      parsed_ref["repository"] = remaining_ref[slash_pos+1:]
      parsed_ref["registry_and_namespace"] = remaining_ref[0:slash_pos]
   else:
      parsed_ref["repository"] = remaining_ref
      parsed_ref["registry_and_namespace"] = None

   return parsed_ref

def update_image_refs_in_deployment(deployment, image_overrides=None):

   containers = deployment["spec"]["template"]["spec"]["containers"]
   for container in containers:
      image_ref = container["image"]
      parsed_ref = parse_image_ref(image_ref)

      if not image_overrides:
         print("  Image (no overrides): %s" % image_ref)
      else:
         repository = parsed_ref["repository"]
         try:
            new_image_ref = image_overrides[repository]
            container["image"] = new_image_ref
            print("   Image override:  %s" % new_image_ref)
         except KeyError:
            print("   WARN: No image override for: %s" % image_ref)


# Loads or creates a package manifest:
def load_pkg_manifest(pathn, pkg_name):

   if os.path.exists(pathn):
      pkg_manifest = load_manifest("package manifest", pathn)
   else:
      pkg_manifest = dict()
      pkg_manifest["packageName"] = pkg_name
      pkg_manifest["channels"] = list()
   return pkg_manifest


def find_channel_entry(manifest, channel):

   pkg_channels = manifest["channels"]
   for pc in pkg_channels:
      if pc["name"] == channel:
         return pc
   return None

# Updates the current CSV pointers in a package manifest map.
def update_pkg_manifest(manifest, for_channels, current_csv_name):

   pkg_channels = manifest["channels"]
   for chan_name in for_channels:
      chan = find_channel_entry(manifest, chan_name)
      if chan is None:
         chan = dict()
         chan["name"] = chan_name
         pkg_channels.append(chan)
      chan["currentCSV"] = current_csv_name
   return

