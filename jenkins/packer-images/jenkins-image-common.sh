#!/bin/bash

# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: Apache-2.0

set -eo pipefail

# Wait for cloud-init to finish
cloud-init status --wait

sudo tee -a ~jenkins/.ssh/authorized_keys >/dev/null <<EOF
ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDH8swFWEfEfHhA+C5ezV8SXO/PkzGD1SH5VAQP/XDIrtnUocBZ3CE30lSyqYJI/EVKVqVa/ICQ0YpUwiMK6+3Jr9QQJwVyTmPji2nY3InL+1XAucN6HFJGKY9bYSsNOuKooj22GwBWw3gfJNLg/8qtpVykEq1yRpyh6pGsXT+J5nUZ723vZZTh//sxdN4CM8D8zoDgHc4RbL+zvESnCDrDbtMhg2u1h14RWiFOBAnzYuWcgtVDy2HA9iS0hJFB2UOV50byXLrEetxJ84PTwRsV2irq1y63g58VxwYOUrVZ08MY5qFvHExBjPqeqhRMzE7GufWM5F1CcUuGviOGFWfqMnfG4VOirPkFtRoK2oKRxH+NVPoUXWWxItJQ1dZ9hLDDWgAbxAvLS4Nnl2hvOVAbC7RVpXfoAhIPpL48oS1UprbsZIMxk2ZmRSJB1ykD3aLUvoO4zoD6xADt8uLiPvVYgFWUy1doLxHZqdY1Omc91owgQVPKvQ4vhqsJehQl4ZDS+O+8S7aC5m8sQ/V+NqiiXLH22vN58K7qNrkHWdb1n+rhilMbA5zp3cSKBgwmmNdupyPkJOKvf3IS7i4El+c8RFmRQv4FzGrdjGXAP8LPtt1dWPgHTFYjmrkOHLmfWM/y8cuyPWW/HEp3Y/msPQRlS3Gymce//vAWgN4T9yN46w== lucas@notebook
EOF

# Install a few dependency packages
. /etc/os-release
PREINSTALL_CMD='echo "error: unknown distro" >&2; false'
for OS_ID in ${ID:-} ${ID_LIKE:-}; do
  case ${OS_ID} in
    rhel)
      PREINSTALL_CMD="/bin/true"
      INSTALL_CMD="yum install -y"
      POSTINSTALL_CMD="/bin/true"
      PKGS="git nmap-ncat java-11-openjdk"
      break
      ;;
    debian)
      if [[ "$VERSION_CODENAME" == buster ]]; then
        echo "deb http://deb.debian.org/debian buster-backports main" | sudo tee /etc/apt/sources.list.d/buster-backports.list
      fi
      PREINSTALL_CMD="env DEBIAN_FRONTEND=noninteractive apt-get update"
      INSTALL_CMD="env DEBIAN_FRONTEND=noninteractive apt-get install -y"
      POSTINSTALL_CMD="env DEBIAN_FRONTEND=noninteractive apt-get purge --autoremove -y"
      # SUFFIX packages with - to remove them
      # Remove unattended-upgrades so that it doesn't interfere with our nodes at startup
      PKGS="git netcat-traditional default-jdk unattended-upgrades-"
      break
      ;;
  esac
done

sudo ${PREINSTALL_CMD}
sudo ${INSTALL_CMD} ${PKGS}
sudo ${POSTINSTALL_CMD}

# create a reference repository (bare git repo)
# jenkins will use this to speed up the checkout for each job
cd /usr/src
sudo git clone --mirror git://git.arvados.org/arvados.git
sudo chown jenkins:jenkins arvados.git -R

# Jenkins will use this script to determine when the node is ready for use
sudo mv /tmp/node-ready.sh /usr/local/bin/

# make sure sshd does not start on boot (yes, this is nasty). Jenkins will call
# /tmp/node-ready.sh as a GCP `startup script`, which gets run on node start.
# That script loops until it can connect to git.arvados.org, and then starts
# sshd so that the Jenkins agent can connect. This avoids the race where Jenkins
# tries to start a job before the GCP outbound routing is working, and fails on
# the first thing it needs internet for, the checkout from git.arvados.org
sudo /bin/systemctl disable ssh
