#!/bin/bash

# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: Apache-2.0

set -eo pipefail

# Install the dependencies for arvados-server
sudo su -c "DEBIAN_FRONTEND=noninteractive apt-get install -y libpam0g-dev wget build-essential"

# Check out a local copy of the arvados repo so we can use it to install the dependencies
cd /usr/src
sudo git clone arvados.git

# Install the correct version of Go
GO_VERSION=`grep 'const goversion =' /usr/src/arvados/lib/install/deps.go |awk -F'"' '{print $2}'`
cd /usr/src
sudo wget https://golang.org/dl/go${GO_VERSION}.linux-amd64.tar.gz
sudo tar xzf go${GO_VERSION}.linux-amd64.tar.gz
sudo ln -s /usr/src/go/bin/go /usr/local/bin/go-${GO_VERSION}
sudo ln -s /usr/src/go/bin/gofmt /usr/local/bin/gofmt-${GO_VERSION}
sudo ln -s /usr/local/bin/go-${GO_VERSION} /usr/local/bin/go
sudo ln -s /usr/local/bin/gofmt-${GO_VERSION} /usr/local/bin/gofmt

# Preseed our dependencies
cd arvados
sudo go mod download
sudo go run ./cmd/arvados-server install -type test

# Our Jenkins jobs use this directory to store the temporary files for the tests
mkdir /home/jenkins/tmp

# Preseed the run-tests.sh cache. This is a little bit silly (a lot of this
# stuff was already done by the call to `./cmd/arvados-server install -type
# test` above, but they do not share a cache.
sudo chown jenkins:jenkins /home/jenkins -R
sudo chown jenkins:jenkins /usr/src/arvados -R
sudo -u jenkins ./build/run-tests.sh WORKSPACE=/usr/src/arvados --temp /home/jenkins/tmp --only install
