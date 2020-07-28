#!/bin/bash

# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: Apache-2.0

set -eo pipefail

# Install the dependencies for arvados-server
sudo su -c "DEBIAN_FRONTEND=noninteractive apt-get install -y libpam0g-dev golang-1.14"

# Check out a local copy of the arvados repo so we can use it to install the dependencies
cd /usr/src
sudo git clone arvados.git
cd arvados
/usr/lib/go-1.14/bin/go mod download
sudo /usr/lib/go-1.14/bin/go run ./cmd/arvados-server install -type test

# Our Jenkins jobs use this directory to store the temporary files for the tests
mkdir /home/jenkins/tmp
