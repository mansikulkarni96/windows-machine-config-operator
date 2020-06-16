#!/bin/bash

# run-local.sh - run/cleanup the operator with OLM
#
# USAGE
#    run-local.sh -a run/cleanup
# OPTIONS
#    -a     Action     run/cleanup the operator installation
#?

# container tool to use with operator-sdk
CONTAINER_TOOL=podman

function stop-exec() {
    echo "Error: $*" >&2
    exit 1
}

# Options
while getopts "a:" opt; do
    case "$opt" in
	a) action="$OPTARG" ;;
	?) stop-exec "Unknown option"
    esac
done

if [[ ! "$action" =~ ^run|cleanup$ ]]; then
    stop-exec "-a Action must be \"run\" or \"cleanup\""
fi

if [ -z "$AWS_SHARED_CREDENTIALS_FILE" ]; then
    stop-exec "env AWS_SHARED_CREDENTIALS_FILE not found"
fi

if [ -z "$KUBE_SSH_KEY_PATH" ]; then
    stop-exec "env KUBE_SSH_KEY_PATH not found"
fi

if [ -z "$CONTAINER_REPO" ]; then
    stop-exec "env CONTAINER_REPO not found"
fi

WMCO_ROOT=$(dirname "${BASH_SOURCE}")/..
source $WMCO_ROOT/hack/common.sh

cd $WMCO_ROOT
OSDK=$(get_operator_sdk)

# Builds the container image and pushes it to repository
# Uses this built image to run the operator on the cluster
# containers are tagged by branch name and it is user's responsibility to clean old/unused containers in
# container repository as well as local system.
case "$action" in
    run)

  TAG=$(git symbolic-ref --short HEAD)
  OPERATOR_IMAGE=$CONTAINER_REPO:$TAG

  $OSDK build "$OPERATOR_IMAGE" --image-builder $CONTAINER_TOOL
  $CONTAINER_TOOL push "$OPERATOR_IMAGE"

  # Create a temporary directory to hold the edited manifests which will be removed on exit
  MANIFEST_LOC=`mktemp -d`
  trap "rm -r $MANIFEST_LOC" EXIT
  cp -r deploy/olm-catalog/windows-machine-config-operator/ $MANIFEST_LOC
  sed -i "s|REPLACE_IMAGE|$OPERATOR_IMAGE|g" $MANIFEST_LOC/windows-machine-config-operator/manifests/windows-machine-config-operator.clusterserviceversion.yaml

  # Verify the operator bundle manifests
  $OSDK bundle validate "$MANIFEST_LOC"/windows-machine-config-operator/

  oc apply -f deploy/namespace.yaml
  if ! oc create secret generic cloud-credentials --from-file=credentials=$AWS_SHARED_CREDENTIALS_FILE -n windows-machine-config-operator; then
    echo "secret already present"
  fi
  if ! oc create secret generic cloud-private-key --from-file=private-key.pem=$KUBE_SSH_KEY_PATH -n windows-machine-config-operator; then
    echo "cloud-private-key already present"
  fi

  # Run the operator in the windows-machine-config-operator namespace
  OSDK_WMCO_management run $OSDK $MANIFEST_LOC
	;;
    cleanup)

  # Remove the operator from windows-machine-config-operator namespace
  OSDK_WMCO_management cleanup $OSDK
	;;
esac
