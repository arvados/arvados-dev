#!/bin/bash

# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: Apache-2.0

set -eo pipefail

# Install the dependencies for arvados-server
sudo su -c "DEBIAN_FRONTEND=noninteractive apt-get install -y libpam0g-dev wget build-essential"

# Install docker (used in our tests and also for package building/testing)
sudo su -c "DEBIAN_FRONTEND=noninteractive apt-get install -y docker.io make wget dpkg-dev createrepo-c unzip"
sudo usermod -a -G docker jenkins

# Check out a local copy of the arvados repo so we can use it to install the dependencies
cd /usr/src
sudo git clone arvados.git

if [[ "$GIT_HASH" != "" ]]; then
  echo "GIT_HASH is set to $GIT_HASH, checking out that revision..."
  (cd arvados && sudo git checkout $GIT_HASH)
fi

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

# FUSE must be configured with the 'user_allow_other' option enabled for Crunch to set up Keep mounts that are readable by containers.
# This is used in our test suite.
echo user_allow_other | sudo tee -a /etc/fuse.conf

# React uses a lot of filesystem watchers (via inotify). Increase the default
# so we have a higher limit at runtime.
echo fs.inotify.max_user_watches=524288 | sudo tee -a /etc/sysctl.conf

# Our Jenkins jobs use this directory to store the temporary files for the tests
mkdir /home/jenkins/tmp

# Preseed the run-tests.sh cache. This is a little bit silly (a lot of this
# stuff was already done by the call to `./cmd/arvados-server install -type
# test` above, but they do not share a cache.
sudo chown jenkins:jenkins /home/jenkins -R
sudo chown jenkins:jenkins /usr/src/arvados -R
sudo -u jenkins ./build/run-tests.sh WORKSPACE=/usr/src/arvados --temp /home/jenkins/tmp --only install

# Install the arvados-dev repo where the jenkins `test-provision-multinode` job
# expects it
cd /usr/local
sudo git clone --depth 1 https://github.com/arvados/arvados-dev
sudo chown -R jenkins:jenkins /usr/local/arvados-dev/
