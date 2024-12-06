#!/bin/bash

# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: Apache-2.0

# GCP nodes sometimes have no outbound working network for the first few
# seconds/minutes.
#
# This script will wait until git.arvados.org is reachable.
set -eo pipefail

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

# All set! Start sshd so jenkins can start the agent...
systemctl list-units --quiet "ssh*.service" |
    awk '($1 !~ /@/ && $2 == "loaded") { print $1; }' |
    while read ssh_service; do
        systemctl start "$ssh_service"
    done

echo "Completed node-ready.sh"
# Log a timestamp
date
