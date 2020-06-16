#!/bin/bash

get_operator_sdk() {
  # Download the operator-sdk binary only if it is not already available
  # We do not validate the version of operator-sdk if it is available already
  if type operator-sdk >/dev/null 2>&1; then
    which operator-sdk
    return
  fi

  DOWNLOAD_DIR=/tmp/operator-sdk
  # TODO: Make this download the same version we have in go dependencies in gomod
  wget -O $DOWNLOAD_DIR https://github.com/operator-framework/operator-sdk/releases/download/v0.18.1/operator-sdk-v0.18.1-x86_64-linux-gnu >/dev/null  && chmod +x /tmp/operator-sdk || return
  echo $DOWNLOAD_DIR
}

# This function runs operator-sdk run --olm/cleanup depending on the given parameters
# Parameters:
# 1: command to run [run/cleanup]
# 2: path to the operator-sdk binary to use
# 3: OPTIONAL path to the directory holding the operator manifests
OSDK_WMCO_management() {
  if [ "$#" -lt 2 ]; then
    echo incorrect parameter count for OSDK_WMCO_management $#
    return 1
  fi
  if [[ "$1" != "run" && "$1" != "cleanup" ]]; then
    echo $1 does not match either run or cleanup
    return 1
  fi

  local COMMAND=$1
  local OSDK_PATH=$2
  local INCLUDE=""

  if [[ "$1" = "run" ]]; then
    INCLUDE="--include "$3"/windows-machine-config-operator/manifests/windows-machine-config-operator.clusterserviceversion.yaml"
  fi

  # Currently this fails even on successes, adding this check to ignore the failure
  # https://github.com/operator-framework/operator-sdk/issues/2938
  if ! $OSDK_PATH $COMMAND packagemanifests --olm-namespace openshift-operator-lifecycle-manager --operator-namespace windows-machine-config-operator \
  --operator-version 0.0.0 $INCLUDE; then
    echo operator-sdk $1 failed
  fi
}
