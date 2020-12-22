#!/bin/bash
# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0

# This script publishes packages from our testing repo to the prod repo (#11572 and #11878)
# Parameters (both are mandatory):
#   --packages list of packages, comma separated, to move from dev_repo_dir to prod_repo_dir
#   --distros  list of distros, comma separated, to which to publish packages

RPM_REPO_BASE_DIR="/var/www/rpm.arvados.org"

###  MAIN  ####################################################################
[ $# -lt 2 ] && {
  echo "usage: "
  echo "    $0 --distros <distroA,distroB,...,distroN> --packages <packageA,packageB,...,packageN>"
  echo "    (both parameters are mandatory)"
  echo ""
  exit 1
}

# Parse options
while [ ${#} -gt 0 ]; do
    case ${1} in
        --packages)   packages="$2";    shift 2 ;;
        --distros)    distros="$2";     shift 2 ;;
        *)            echo "$0: Unrecognized option: $1" >&2; exit 1;
    esac
done

# Make sure the variables are set or provide an example of how to use them
if [ -z "${packages}" ]; then
  echo "You must provide a comma-separated list of packages to publish, ie."
  echo "* Debian: --packages=arvados-ws_0.1.20170906144951.22418ed6e-1_amd64.deb,keep-exercise_*,*170906144951*"
  echo "* Centos: --packages=arvados-ws_0.1.20170906144951.22418ed6e-1.x86_64.rpm,keep-exercise_0.1.20170906144951.22418ed6e-1.x86_64.rpm,crunch-dispatch-local_0.1.20170906144951.22418ed6e-1.x86_64.rpm"
  exit 254
fi
if [ -z "${distros}" ]; then
  echo "You must provide a space-separated list of LSB distribution codenames to which you want to publish to, ie."
  echo "* Debian: --distros=jessie,xenial,stretch,etc."
  echo "* Centos: --distros=centos7,etc."
  exit 255
fi

DIST_LIST=$(echo ${distros} | sed s/,/' '/g |tr '[:upper:]' '[:lower:]')
PACKAGES=$(echo ${packages} | sed s/,/' '/g)

if ( echo ${DIST_LIST} |grep -q centos ); then
  for DISTNAME in ${DIST_LIST}; do 
    case ${DISTNAME} in
      'centos7')
        DIST_DIR_TEST='7/testing/x86_64'
        DIST_DIR_PROD='7/os/x86_64'
      ;;
      *)
        echo "Only centos7 is accepted right now"
        exit 253
      ;;
    esac
    cd ${RPM_REPO_BASE_DIR}
    mkdir -p ${RPM_REPO_BASE_DIR}/CentOS/${DIST_DIR_PROD}
    echo "Copying packages ..."
    for P in ${PACKAGES}; do
      cp ${RPM_REPO_BASE_DIR}/CentOS/${DIST_DIR_TEST}/${P} ${RPM_REPO_BASE_DIR}/CentOS/${DIST_DIR_PROD}/
      if [ $? -ne 0 ]; then
        FAILED_PACKAGES="${FAILED_PACKAGES} ${P}"
      fi
    done
    echo "Recreating repo CentOS/${DIST_DIR_PROD} ..."
    createrepo ${RPM_REPO_BASE_DIR}/CentOS/${DIST_DIR_PROD}
  done
else
  for DISTNAME in ${DIST_LIST}; do
    ADDED=()
    echo "Copying packages ..."
    for P in ${PACKAGES}; do
      aptly repo copy ${DISTNAME}-testing ${DISTNAME} $(basename ${P})
    done
    echo "Publishing ${DISTNAME} repository..."
    aptly publish update ${DISTNAME} filesystem:${DISTNAME}:
  done
fi

if [ "${FAILED_PACKAGES}" != "" ]; then
  echo "PACKAGES THAT FAILED TO PUBLISH"
  echo "${FAILED_PACKAGES}"
else
  echo "All packages published correctly"
fi
