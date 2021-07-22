#!/bin/bash

# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: Apache-2.0

set -eo pipefail

sudo su -c "echo ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAgEA3nzI6T6Lpd5xFoRewcx91Dv9sUzNNmdYfwOleemBFz0y3RaQElehUWasyjuIURZw7RL5EjvrqeQq9pe/lO99dO0F9yMuMsMH2t88YrVJQ/z/5Aa4I2zYQotKb/9CCfynsy41y5xywxtOwXiDk2kpo+c9VZCyCeW8Hnc9HaIpkKSLnkqDVhESzlrkYyNKZvUAL1hiFIzmmw/veFgRb7/ol76Ze3xsWugbHUECEIAKoz/8uaevOAAoJrMFhffFIQ8IfClqDZv2lnBBhvh1O9TO0Mg4klcieyQ1RZhMjeP4WnAa9PXP7xZlQHLgO9qO1jDd2sOdkX6EedCfX5jO4Y51HPJpV35uYumw3veftMlpIJFmA2eIQxU19SCYpojWRGXZ5v9WtFIHX2nGy+Gi1bk7TR+HBsCDXOPhhQk4ceIM4OonqEb1NJ57elxh6mFbDAQCZtYhRLqYvcyGpuBdVdLNOJbOZ7vBJY3Kfjxa87rvFIJheT6DXhpdRayeOovLm0vuJ53bZoWxWOqjpuigQqHtSB3OmintrKB916BhNFsHwPeZpK0ahZGFygV63REM/X8m0nOlHqyAY69uzLHyYXM83zAI1L5Y4wuzsVqO+1tnK6PMcfg5/DArjBrn7YqA5tzJY9EgdVVPjpgjD2yYYTOP6b3UUw4uFj3asnOX24dfVzk= ward@countzero" >> /home/jenkins/.ssh/authorized_keys
sudo su -c "echo ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQCfRJenfxGPFuJ/W2KUs6Wf0waiFsSaxd8WUTFXi4ophH2qkUMxd8xvoex/Z+Iq1fA8QCe4DfFnhUOMViP9bBB6+Va/x1Jx521P+DfN8ILqM2kcug0OoC4c5gTyFecJaKIyX6jMeidQThkd/UPHeTdicOnMqUTKIksQVXzIxzve6MQxbZts1viw0VgcO2t16F9LA+JaB8ONOTPfBzJ/BzZRiztfzQ0qW9rpX23icHtv494QrHZtgpvXltB/fy+YUZ80QJH1Iul4xnNL/27/MtZempJSSzvzlZxMIpRbe9F4qn859DZMGjUnoVMQ5v6Aw7fQRm1iMVa++i6i5X0bAI6xR6eIQzPgGdu+ymlanH0TO+5aOhOcuELyL81q2VBl01SBCfZ2OxX2ZL1HCLMe7vtxl8qxbO7Ur/Cq6j6/t7Ghb/jSnxY68rNm7MhOc5bcWL6vPxjmDMPzpk1bKL0lfEOoI4Dij+Rb0p8glC2399lZrMUTspf+/Qz6bP6ev/Z2ZJDeugRBQeLcMdu5dg1CzIKf+x8n3IbSyu5w/ML9BGq6vXpViVVkZ73DYo1BipnUVpzZ+wVGk+JdhoBFD3wlZYm/rDMiMtKrtcYhvPrvp3EhM+vBbCkcAKY40mcVuQD8BhPe5za9vlqjTKHOUTOGLkI4RAiPF3kwjXwELyxuOIki5w== javier@agnes RSA-4096, 20170323" >> /home/jenkins/.ssh/authorized_keys
sudo su -c "echo ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDH8swFWEfEfHhA+C5ezV8SXO/PkzGD1SH5VAQP/XDIrtnUocBZ3CE30lSyqYJI/EVKVqVa/ICQ0YpUwiMK6+3Jr9QQJwVyTmPji2nY3InL+1XAucN6HFJGKY9bYSsNOuKooj22GwBWw3gfJNLg/8qtpVykEq1yRpyh6pGsXT+J5nUZ723vZZTh//sxdN4CM8D8zoDgHc4RbL+zvESnCDrDbtMhg2u1h14RWiFOBAnzYuWcgtVDy2HA9iS0hJFB2UOV50byXLrEetxJ84PTwRsV2irq1y63g58VxwYOUrVZ08MY5qFvHExBjPqeqhRMzE7GufWM5F1CcUuGviOGFWfqMnfG4VOirPkFtRoK2oKRxH+NVPoUXWWxItJQ1dZ9hLDDWgAbxAvLS4Nnl2hvOVAbC7RVpXfoAhIPpL48oS1UprbsZIMxk2ZmRSJB1ykD3aLUvoO4zoD6xADt8uLiPvVYgFWUy1doLxHZqdY1Omc91owgQVPKvQ4vhqsJehQl4ZDS+O+8S7aC5m8sQ/V+NqiiXLH22vN58K7qNrkHWdb1n+rhilMbA5zp3cSKBgwmmNdupyPkJOKvf3IS7i4El+c8RFmRQv4FzGrdjGXAP8LPtt1dWPgHTFYjmrkOHLmfWM/y8cuyPWW/HEp3Y/msPQRlS3Gymce//vAWgN4T9yN46w== lucas@notebook" >> /home/jenkins/.ssh/authorized_keys

# Install a few dependency packages
# First, let's figure out the OS we're working on
OS_ID=$(grep ^ID= /etc/os-release |cut -f 2 -d \")
case ${OS_ID} in
  centos)
    PREINSTALL_CMD="/bin/true"
    INSTALL_CMD="yum -y"
    POSTINSTALL_CMD="/bin/true"
    PKGS="git nmap-ncat java-11-openjdk"
    ;;
  debian|ubuntu)
    echo "deb http://deb.debian.org/debian buster-backports main" | sudo tee /etc/apt/sources.list.d/buster-backports.list

    PREINSTALL_CMD="DEBIAN_FRONTEND=noninteractive apt update"
    INSTALL_CMD="DEBIAN_FRONTEND=noninteractive apt install -y"
    POSTINSTALL_CMD="DEBIAN_FRONTEND=noninteractive apt autopurge -y"
    # SUFFIX packages with - to remove them
    # Remove unattended-upgrades so that it doesn't interfere with our nodes at startup
    PKGS="git netcat-traditional default-jdk unattended-upgrades-"
    ;;
esac

sudo su -c "${PREINSTALL_CMD}"
sudo su -c "${INSTALL_CMD} ${PKGS}"
sudo su -c "${POSTINSTALL_CMD}"

# create a reference repository (bare git repo)
# jenkins will use this to speed up the checkout for each job
cd /usr/src
sudo git clone --mirror https://git.arvados.org/arvados.git
sudo chown jenkins:jenkins arvados.git -R

# Jenkins will use this script to determine when the node is ready for use
sudo mv /tmp/node-ready.sh /usr/local/bin/

# make sure sshd does not start on boot (yes, this is nasty). Jenkins will call
# /tmp/node-ready.sh as a GCP `startup script`, which gets run on node start.
# That script loops until it can connect to git.arvados.org, and then starts
# sshd so that the Jenkins agent can connect. This avoids the race where Jenkins
# tries to start a job before the GCP outbound routing is working, and fails on
# the first thing it needs internet for, the checkout from git.arvados.org
sudo /bin/systemctl disable sshd
