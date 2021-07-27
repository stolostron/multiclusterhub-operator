# Copyright (c) 2021 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project

import os
import shutil

if shutil.which("docker") is None:
    raise Exception("Docker not installed! Go install Docker!")

if shutil.which("helm") is None:
    raise Exception("Helm not installed! Go install Helm 3 CLI!")

# NW this can be set when the code is in the mch operator
_product_version = os.environ.get('PRODUCT_VERSION')
if not _product_version:
    raise Exception("You must export PRODUCT_VERSION!")

_image_registry = os.environ.get('MOCK_IMAGE_REGISTRY')
if not _image_registry:
    raise Exception("You must export MOCK_IMAGE_REGISTRY! (ex: 'MOCK_IMAGE_REGISTRY/MOCK_IMAGE_NAME:MOCK_IMAGE_TAG')")

_image_name = os.environ.get('MOCK_IMAGE_NAME')
if not _image_name:
    _image_name = "hub-mock-component-image"
    os.environ['MOCK_IMAGE_NAME'] = _image_name

_image_tag = os.environ.get('MOCK_IMAGE_TAG')
if not _image_tag:
    _image_tag="mock"
    os.environ['MOCK_IMAGE_TAG'] = _image_tag
