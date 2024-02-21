#!/bin/bash

# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: Apache-2.0

set -eo pipefail

# Get the arvados repository
cd /usr/src
sudo git clone https://git.arvados.org/arvados.git
cd arvados/services/workbench2

if [[ "$GIT_HASH" != "" ]]; then
  echo "GIT_HASH is set to $GIT_HASH, checking out that revision..."
  sudo git checkout $GIT_HASH
fi

# React uses a lot of filesystem watchers (via inotify). Increase the default
# so we have a higher limit at runtime.
echo fs.inotify.max_user_watches=524288 | sudo tee -a /etc/sysctl.conf

# Build the workbench2-build docker image
sudo make workbench2-build-image

cd ../../../
sudo rm -rf arvados
