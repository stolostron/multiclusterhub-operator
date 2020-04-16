#!/bin/bash

timestamp="$(date "+%Y-%m-%d-%H-%M-%S")"

./gen-ocm-hub-bundle.sh 1.0.0 sha256:13579 $timestamp
