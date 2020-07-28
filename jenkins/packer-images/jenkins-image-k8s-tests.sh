#!/bin/bash

# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: Apache-2.0

set -eo pipefail

echo "deb http://apt.arvados.org/ buster main" | sudo  tee /etc/apt/sources.list.d/arvados.list

# Install a few dependencies
sudo DEBIAN_FRONTEND=noninteractive apt-get -y --no-install-recommends install gnupg2 wget git default-jdk docker.io netcat

sudo usermod -a -G docker jenkins

cat /tmp/1078ECD7.asc | sudo apt-key add -
sudo DEBIAN_FRONTEND=noninteractive apt-get update
# Install Arvados Packages
# the python3 version is currently broken, see #16434, update to python3 when 2.0.3 is out
#    python3-arvados-cwl-runner \
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y \
    python-arvados-cwl-runner \
    python3-arvados-python-client \

# Install kubectl + helm
# GCE provides the latest kubectl via apt, automatically
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y kubectl
cd /usr/src
sudo wget https://get.helm.sh/helm-v3.2.1-linux-amd64.tar.gz
sudo tar xzf helm-v3.2.1-linux-amd64.tar.gz
sudo mv linux-amd64/helm /usr/bin/

# The rest of this script is what's needed for testing with minikube minikube
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends dnsmasq

# Install KVM
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends qemu-kvm libvirt-clients libvirt-daemon-system

# Add the jenkins user to the libvirt group
sudo usermod -a -G libvirt jenkins

# Install minikube
sudo wget -O /usr/local/bin/minikube https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64
sudo chmod +x /usr/local/bin/minikube

# default to the kvm2 driver *for the jenkins user* (hence, no sudo)
minikube config set driver kvm2

sudo DEBIAN_FRONTEND=noninteractive apt-get clean
