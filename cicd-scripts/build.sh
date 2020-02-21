#!/bin/bash
echo "BUILD GOES HERE!"

echo "<repo>/<component>:<tag> : $1"

podmanAvailable=$(podman -v 2>/dev/null)
dockerAvailable=$(docker -v 2>/dev/null)

if [ ! -z "$podmanAvailable" ]
then
  imageBuilder="podman"
elif [ ! -z "$dockerAvailable" ]
then
  imageBuilder="docker"
else
  echo "Must install docker or podman ... Exiting"
  exit 1
fi

operator-sdk build $1 --image-builder $imageBuilder
