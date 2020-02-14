#!/bin/bash

set -x

if [[ -z "$1" ]] ; then
    echo "Must provide version number to unpublish"
    exit 1
fi

VERNO=$1
PACKAGE="*"

if [[ -n "$2" ]] ; then
    PACKAGE="$2"
fi

rm -f $(find /var/www/rpm.arvados.org/CentOS/7/testing -name "${PACKAGE}-${VERNO}*.rpm")
sudo -u jenkinsapt createrepo /var/www/rpm.arvados.org/CentOS/7/testing/x86_64

for fr in $(ls -d /var/lib/freight/apt/*-testing) ; do
    rm -f $(find ${fr} -name "${PACKAGE}_${VERNO}*.deb")
    sudo -u jenkinsapt freight cache ${fr}
done

