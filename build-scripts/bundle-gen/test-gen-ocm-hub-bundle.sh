#!/bin/bash

# Quick hack to drive OCM Hub Bundle gen script.

# This might be the form used if generaating a to-be-released bundle for a snapshot.
timestamp="$(date "+%Y-%m-%d-%H-%M-%S")"
./gen-bound-ocm-hub-bundle.sh 1.0.0 sha256:13579 $timestamp

# This might be the form used if generating a "source" CSV bundle.
#./gen-unbound-ocm-hub-bundle.sh 1.0.0

