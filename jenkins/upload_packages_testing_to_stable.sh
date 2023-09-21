#!/bin/bash
# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0

# This script publishes packages from our testing repo to the prod repo
# (#11572)
# Parameters: list of packages, space separated, to copy from *-testing to *

APT_REPO_SERVER="apt.arvados.org"
RPM_REPO_SERVER="rpm.arvados.org"

DEBUG=1
SSH_PORT=2222
ECODE=0

# Convert package list into a regex for publising
# Make sure the variables are set or provide an example of how to use them
if [ -z "${PACKAGES_TO_PUBLISH}" ]; then
  echo "You must provide a list of packages to publish, as obtained with https://dev.arvados.org/projects/ops/wiki/Updating_clusters#Gathering-package-versions."
  exit 254
fi
if [ -z "${LSB_DISTRIB_CODENAMES}" ]; then
  echo "You must provide a space-separated list of LSB distribution codenames to which you want to publish to, ie."
  echo "* Debian/Ubuntu: buster, bullseye, focal"
  echo "* Centos: centos7 (the only one currently supported.)"
  exit 255
fi

# Only numbered package versions are supposed to go into the stable repositories
TMP=$(echo "$PACKAGES_TO_PUBLISH" | sed 's/versions://g;')
VERPATTERN='[0-9]+\.[0-9]+\.[0-9]+(\.[0-9]+)?-[0-9]+'
VALIDATED_PACKAGES_TO_PUBLISH=`echo "$TMP" | sed -nE '/^.*: '"$VERPATTERN"'$/p'`

if [[ "$TMP" != "$VALIDATED_PACKAGES_TO_PUBLISH" ]]; then
  echo "The list of packages has invalid syntax. each line must be of the format:"
  echo
  echo "packagename: $VERPATTERN"
  echo
  exit 253
fi

# Sanitize the vars in a way suitable to be used by the remote 'publish_packages.sh' script
# Just to make copying a single line, and not having to loop over it
PACKAGES_LIST=$(echo ${PACKAGES_TO_PUBLISH} | sed 's/versions://g; s/\([a-z-]*\):[[:blank:]]*\([0-9.-]*\)/\1:\2,/g; s/[[:blank:]]//g; s/,$//g;')

DISTROS=$(echo "${LSB_DISTRIB_CODENAMES}"|sed s/[[:space:]]/,/g |tr '[:upper:]' '[:lower:]')

if ( echo ${LSB_DISTRIB_CODENAMES} |grep -q -E '(centos|rocky)' ); then
  REPO_SERVER=${RPM_REPO_SERVER}
else
  REPO_SERVER=${APT_REPO_SERVER}
fi

REMOTE_CMD="/usr/local/bin/testing_to_stable_publish_packages.sh --distros ${DISTROS} --packages ${PACKAGES_LIST}"

# Now we execute it remotely
TMP_FILE=`mktemp`

ssh -t \
    -l jenkinsapt \
    -p $SSH_PORT \
    -o "StrictHostKeyChecking no" \
    -o "ConnectTimeout 5" \
    ${REPO_SERVER} \
    "${REMOTE_CMD}"
ECODE=$?

exit ${ECODE}
