#!/bin/bash

# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: Apache-2.0

set -eo pipefail

# Install the dependencies for the package building/testing jobs
sudo env DEBIAN_FRONTEND=noninteractive apt-get install -y docker.io make wget dpkg-dev createrepo-c unzip
sudo usermod -a -G docker jenkins

# Ansible install
sudo install -d -o "$(id -nu)" -g "$(id -ng)" /opt/ansible
python3 -m venv /opt/ansible
/opt/ansible/bin/pip config --quiet --site set global.no-input true
/opt/ansible/bin/pip config --quiet --site set install.no-cache-dir true
/opt/ansible/bin/pip config --quiet --site set install.progress-bar off
/opt/ansible/bin/pip install "pip>=20.3" wheel
/opt/ansible/bin/pip install "ansible~=8.7" "yq~=3.4"
sudo chown -R root: /opt/ansible

# Packer install
cd /tmp
wget https://releases.hashicorp.com/packer/1.8.0/packer_1.8.0_linux_amd64.zip
unzip packer_1.8.0_linux_amd64.zip packer
sudo mv packer /usr/local/bin/

# Install the arvados-dev repo where the Jenkins job expects it
cd /usr/local
sudo git clone --depth 1 https://github.com/arvados/arvados-dev
sudo chown -R jenkins:jenkins /usr/local/arvados-dev/

# React uses a lot of filesystem watchers (via inotify). Increase the default
# so we have a higher limit at runtime.
echo fs.inotify.max_user_watches=524288 | sudo tee -a /etc/sysctl.conf
sudo sysctl -p

# Use 'docker-hub-mirror.arvados.org' instead of pulling directly from
# docker hub to avoid getting rate limit errors from docker.io.
echo '{ "registry-mirrors": ["https://docker-hub-mirror.arvados.org"] }' | sudo tee -a /etc/docker/daemon.json
