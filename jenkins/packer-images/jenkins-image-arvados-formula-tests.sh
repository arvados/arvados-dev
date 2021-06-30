#!/bin/bash

# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: Apache-2.0

set -eo pipefail

# Install the dependencies for the package building/testing jobs
sudo su -c "DEBIAN_FRONTEND=noninteractive apt-get install -y docker.io git ruby ruby-dev make gcc g++"
sudo usermod -a -G docker jenkins

cd /tmp
wget -c https://git.arvados.org/arvados-formula.git/blob_plain/refs/heads/main:/Gemfile
wget -c https://git.arvados.org/arvados-formula.git/blob_plain/refs/heads/main:/Gemfile.lock
sudo su -c "gem install bundler"
sudo su -c "bundler install"
sudo su -c "gem install kitchen-docker"
