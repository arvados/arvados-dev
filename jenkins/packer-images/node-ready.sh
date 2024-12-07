#!/bin/bash

# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: Apache-2.0

# GCP nodes sometimes have no outbound working network for the first few
# seconds/minutes. This script will wait until git.arvados.org is reachable.
#
# This script does NOT use `set -euo pipefail` because any failure means we
# fail to start SSH and the node is an unusable waste of money. This should be
# reasonably low risk as long as the script has a minimum of intermediate state.

# Send all stdout/stderr to the log and to the terminal
exec > >(tee -a /tmp/boot-wait.log) 2>&1

# Log a timestamp
date
echo "Starting node-ready.sh"

while ! /bin/nc -w1 -z git.arvados.org 22; do
  echo "Connect failed, waiting 1 second..."
  sleep 1
done
echo "Connected!"

# Debian calls it ssh.service; RHEL calls it sshd.service. Try both;
# it's fine as long as the one we want starts.
systemctl start ssh.service sshd.service

echo "Completed node-ready.sh"
# Log a timestamp
date
