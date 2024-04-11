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
  echo "* Debian: --packages=arvados-ws:0.1.20170906144951.22418ed6e-1,keep-exercise:1.2.3-1"
  exit 254
fi
if [ -z "${distros}" ]; then
  echo "You must provide a space-separated list of LSB distribution codenames to which you want to publish to, ie."
  echo "* Debian: --distros=jessie,xenial,stretch,etc."
  echo "* CentOS/Rocky: --distros=centos7,etc."
  exit 255
fi

DIST_LIST=$(echo ${distros} | sed s/,/' '/g |tr '[:upper:]' '[:lower:]')
CENTOS_PACKAGES=$(echo ${packages} | sed 's/\([a-z-]*\):[[:blank:]]*\([0-9.-]*\)/\1*\2*/g; s/,/ /g;')
DEBIAN_PACKAGES=$(echo ${packages} | sed 's/\([a-z-]*\):[[:blank:]]*\([0-9.-]*\)/\1 (= \2)/g;')

for DISTNAME in ${DIST_LIST}; do
    echo
    echo "### Publishing packages for ${DISTNAME} ###"
    echo
    if ( echo ${DISTNAME} |grep -q -E '(centos|rocky)' ); then
	case ${DISTNAME} in
	    'centos7')
		DIST_DIR_TEST='7/testing/x86_64'
		DIST_DIR_PROD='7/os/x86_64'
		;;
	    'rocky8')
		DIST_DIR_TEST='8/testing/x86_64'
		DIST_DIR_PROD='8/os/x86_64'
		;;
	    *)
		echo "Only centos7 and rocky8 are accepted right now"
		exit 253
		;;
	esac
	cd ${RPM_REPO_BASE_DIR}
	mkdir -p ${RPM_REPO_BASE_DIR}/CentOS/${DIST_DIR_PROD}
	echo "Copying packages ..."
	for P in ${CENTOS_PACKAGES}; do
	    cp ${RPM_REPO_BASE_DIR}/CentOS/${DIST_DIR_TEST}/${P} ${RPM_REPO_BASE_DIR}/CentOS/${DIST_DIR_PROD}/
	    if [ $? -ne 0 ]; then
		FAILED_PACKAGES="${FAILED_PACKAGES} ${P}"
	    fi
	done
	echo "Recreating repo CentOS/${DIST_DIR_PROD} ..."
	createrepo_c ${RPM_REPO_BASE_DIR}/CentOS/${DIST_DIR_PROD}
    else
	echo "Copying packages ..."
	OLDIFS=$IFS
	IFS=$','
	for P in ${DEBIAN_PACKAGES}; do
	    aptly repo search ${DISTNAME}-testing "${P}"
	    if [ $? -ne 0 ]; then
		echo "ERROR: unable to find a match for '${P}' in ${DISTNAME}-testing"
		FAILED_PACKAGES="${FAILED_PACKAGES} ${DISTNAME}-testing:${P}"
	    else
		aptly repo copy ${DISTNAME}-testing ${DISTNAME} "${P}"
		if [ $? -ne 0 ]; then
		    echo "ERROR: unable to copy '${P}' from ${DISTNAME}-testing to ${DISTNAME}"
		    FAILED_PACKAGES="${FAILED_PACKAGES} ${DISTNAME}-testing:${P}"
		fi
	    fi
	done
	IFS=$OLDIFS
	echo "Publishing ${DISTNAME} repository..."
	aptly publish update ${DISTNAME} filesystem:${DISTNAME}:
    fi
done

if [ "${FAILED_PACKAGES}" != "" ]; then
  echo "PACKAGES THAT FAILED TO PUBLISH"
  echo "${FAILED_PACKAGES}"
  exit 252
else
  echo "All packages published correctly"
fi
