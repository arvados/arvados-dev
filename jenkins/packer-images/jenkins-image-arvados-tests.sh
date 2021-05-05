#!/bin/bash

# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: Apache-2.0

set -eo pipefail

# Install the dependencies for arvados-server
sudo su -c "DEBIAN_FRONTEND=noninteractive apt-get install -y libpam0g-dev wget"

# Get Go 1.16.3
cd /usr/src
sudo wget https://golang.org/dl/go1.16.3.linux-amd64.tar.gz
sudo tar xzf go1.16.3.linux-amd64.tar.gz
sudo ln -s /usr/src/go/bin/go /usr/local/bin/go-1.16.3
sudo ln -s /usr/src/go/bin/gofmt /usr/local/bin/gofmt-1.16.3
sudo ln -s /usr/local/bin/go-1.16.3 /usr/local/bin/go
sudo ln -s /usr/local/bin/gofmt-1.16.3 /usr/local/bin/gofmt

# Check out a local copy of the arvados repo so we can use it to install the dependencies
cd /usr/src
sudo git clone arvados.git
cd arvados
sudo go mod download
sudo go run ./cmd/arvados-server install -type test

# Our Jenkins jobs use this directory to store the temporary files for the tests
mkdir /home/jenkins/tmp
