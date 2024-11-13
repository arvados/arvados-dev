#!/bin/bash
# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0
# The script receives version as a parameter and builds the package producing a tar
# and this tarball is uploaded into the public server
set -xe
cd "$WORKSPACE/sdk/R"
sed -i -e s/"^Version: .*$"/"Version: $VERNO"/g DESCRIPTION
if [ -e Makefile ]; then
    make package
else
    R CMD build .  # Fallback for pre-3.0 releases
fi
scp -o "StrictHostKeyChecking no" "ArvadosR_$VERNO.tar.gz" jenkinsapt@r.arvados.org:/var/www/r.arvados.org/src/contrib
ssh -o "StrictHostKeyChecking no" jenkinsapt@r.arvados.org 'cd /var/www/r.arvados.org/src/contrib && R -q -e "tools::write_PACKAGES()"'
