#!/bin/bash

# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: Apache-2.0

set -eo pipefail

# Get the wb2 repository
cd /usr/src
sudo git clone https://git.arvados.org/arvados-workbench2.git

# Build the workbench2-build docker image
cd arvados-workbench2
sudo make workbench2-build-image

cd ..
sudo rm -rf arvados-workbench2
