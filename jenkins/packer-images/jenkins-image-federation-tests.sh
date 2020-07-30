#!/bin/bash

# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: Apache-2.0

sudo su -c "DEBIAN_FRONTEND=noninteractive apt-get install -y docker.io virtualenv curl libcurl4-gnutls-dev build-essential libgnutls28-dev python2.7-dev python3-dev"
sudo usermod -a -G docker jenkins
