#!/bin/bash

# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: Apache-2.0

set -eo pipefail

# Get the wb2 repository
cd /usr/src
sudo git clone https://git.arvados.org/arvados-workbench2.git
cd arvados-workbench2

if [[ "$GIT_HASH" != "" ]]; then
  echo "GIT_HASH is set to $GIT_HASH, checking out that revision..."
  sudo git checkout $GIT_HASH
fi

# Build the workbench2-build docker image
sudo make workbench2-build-image

cd ..
sudo rm -rf arvados-workbench2
