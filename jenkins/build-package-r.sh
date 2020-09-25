#!/bin/bash
# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0
# The script receives version as a parameter and builds the package producing a tar
# and this tarball is uploaded into the public server
sed -i -e s/"^Version: .*$"/"Version: $VERNO"/g $WORKSPACE/sdk/R/DESCRIPTION
cd $WORKSPACE/sdk/
R CMD build R
scp $WORKSPACE/sdk/ArvadosR_"$VERNO".tar.gz jenkinsapt@r.arvados.org:/var/www/r.arvados.org/src/contrib
ssh jenkinsapt@r.arvados.org 'cd /var/www/r.arvados.org/src/contrib && R -q -e "tools::write_PACKAGES()"'
