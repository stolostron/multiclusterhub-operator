FULL_IMAGE_NAME=$1

podmanAvailable=$(podman -v 2>/dev/null)
dockerAvailable=$(docker -v 2>/dev/null)

if [ ! -z "$podmanAvailable" ]
then
  containerCli=$(which podman)
elif [ ! -z "$dockerAvailable" ]
then
  containerCli=$(which docker) 
else
  echo "Must install docker or podman ... Exiting"
  exit 1
fi

$containerCli login "$FULL_IMAGE_NAME" -u "$DOCKER_USER" -p "$DOCKER_PASS"

$containerCli push "$FULL_IMAGE_NAME"
