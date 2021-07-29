#!/bin/bash

# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0

set -eo pipefail

DISTRO=$1

set -u

if [[ `id -u -n` != "jenkinsapt" ]]; then
  echo "This script must be run as the `jenkinsapt` user"
  exit 1
fi

if [[ -z "$DISTRO" ]]; then
  echo "Syntax: $0 <distro>"
  exit 1
fi

# Check that a FileSystemPublishEndpoint has been added in ~jenkinsapt/.aptly.conf
set +e
TMP=`cat ~jenkinsapt/.aptly.conf | jq .FileSystemPublishEndpoints.$DISTRO |grep rootDir`
set -e
if [[ "$TMP" == "" ]]; then
  echo "Please edit ~jenkinsapt/.aptly.conf and add a FileSystemPublishEndpoint for $DISTRO first"
  echo
  echo "Aborting..."
  exit 2
fi

# Double check if this repository is already defined
set +e
EXISTING=`aptly repo list 2>&1 |grep $DISTRO`
set -e

if [[ "$EXISTING" != "" ]]; then
	echo "This release exists already, judging by the output of 'aptly repo list'"
	echo
	echo "Aborting..."
	exit 2
fi


echo "WARNING! Please use the human name for the distribution release, e.g. bullseye for Debian 11, or focal for Ubuntu 20.04"

echo
read -r -p "Create the repositories for $DISTRO with aptly, are you sure? (y/n) " response
echo

case "$response" in
	[yY])
	    ;;
	*)
	    echo "Aborting."
	    exit 0
	    ;;
esac

# Development builds
aptly repo create --comment "Development builds" --distribution "$DISTRO-dev" $DISTRO-dev
aptly repo edit -architectures="amd64" $DISTRO-dev
aptly repo show $DISTRO-dev

# Testing builds
aptly repo create --comment "Testing builds" --distribution "$DISTRO-testing" $DISTRO-testing
aptly repo edit -architectures="amd64" $DISTRO-testing
aptly repo show $DISTRO-testing

# Release builds
aptly repo create --comment "Release builds" --distribution "$DISTRO" $DISTRO
aptly repo edit -architectures="amd64" $DISTRO
aptly repo show $DISTRO

# Attic
aptly repo create --comment "Attic" --distribution "$DISTRO-attic" $DISTRO-attic
aptly repo edit -architectures="amd64" $DISTRO-attic
aptly repo show $DISTRO-attic


# Publish dev
aptly publish repo -architectures "amd64, arm64" $DISTRO-dev filesystem:$DISTRO:.
aptly publish show $DISTRO-dev filesystem:$DISTRO:.

# Publish testing
aptly publish repo -architectures "amd64, arm64" $DISTRO-testing filesystem:$DISTRO:.
aptly publish show $DISTRO-testing filesystem:$DISTRO:.

# Publish release
aptly publish repo -architectures "amd64, arm64" $DISTRO filesystem:$DISTRO:.
aptly publish show $DISTRO filesystem:$DISTRO:.

# Publish attic
aptly publish repo -architectures "amd64, arm64" $DISTRO-attic filesystem:$DISTRO:.
aptly publish show $DISTRO-attic filesystem:$DISTRO:.

# Show all the published repos
aptly publish list |grep $DISTRO
