#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.

set -e

_script_dir=$(dirname "$0")
echo 'mode: atomic' > cover.out
echo '' > cover.tmp

_pkgs=$(go list ./... | grep -v /build | grep -v /vendor)
echo "$_pkgs" | xargs -n1 -I{} "$_script_dir"/test_package.sh {}
