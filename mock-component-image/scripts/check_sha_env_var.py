import os
import shutil

_image_sha = os.environ.get('MOCK_IMAGE_SHA')
if not _image_sha:
    raise Exception("You must export MOCK_IMAGE_SHA! (ex: 'MOCK_IMAGE_REGISTRY/MOCK_IMAGE_NAME@sha:MOCK_IMAGE_SHA')")