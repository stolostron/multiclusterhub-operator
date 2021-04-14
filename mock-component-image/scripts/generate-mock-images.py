import shutil
import os
from check_env_vars import _product_version, _image_registry, _image_name, _image_tag

_full_image="{}/{}:{}".format(_image_registry, _image_name, _image_tag)
os.system('docker build . -t {}'.format(_full_image))
os.system('docker push {}'.format(_full_image) )
print ("pushed: {}".format(_full_image))
