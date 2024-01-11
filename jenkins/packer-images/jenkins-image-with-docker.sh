#!/bin/bash

# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: Apache-2.0

set -eo pipefail

# Install the dependencies for the package building/testing jobs
sudo su -c "DEBIAN_FRONTEND=noninteractive apt-get install -y docker.io make wget dpkg-dev createrepo-c unzip"
sudo usermod -a -G docker jenkins

# Install the arvados-dev repo where the Jenkins job expects it
cd /usr/local
sudo git clone --depth 1 https://github.com/arvados/arvados-dev
sudo chown -R jenkins:jenkins /usr/local/arvados-dev/
