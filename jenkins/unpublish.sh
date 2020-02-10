#!/bin/bash

set -x

VERNO=$1

if [[ -z "$VERNO" ]] ;
   echo "Must provide version number to unpublish"
   exit 1
fi

rm -f $(find /var/www/rpm.arvados.org/CentOS/7/testing -name *-${VERNO}*.rpm)
createrepo /var/www/rpm.arvados.org/CentOS/7/testing/x86_64

for fr in $(ls -d /var/lib/freight/apt/*-testing) ; do
    rm -f $(find ${fr} -name *-${VERNO}*.deb)
    freight cache ${fr}
done
