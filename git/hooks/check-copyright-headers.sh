#!/bin/bash
# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0

# This script is intended to be run as a git update hook. It ensures that all
# commits adhere to the copyright header convention for the Arvados project,
# which is documented at
#
#    https://dev.arvados.org/projects/arvados/wiki/Coding_Standards#Copyright-headers

REFNAME=$1
OLDREV=$2
NEWREV=$3

EXITCODE=0

echo "Enforcing copyright headers..."

# Load the .licenseignore file
LICENSEIGNORE=`mktemp`
git show ${NEWREV}:.licenseignore > "${LICENSEIGNORE}" 2>/dev/null
if [[ "$?" != "0" ]]; then
  # e.g., .licenseignore does not exist
  ignores=()
else
  IFS=$'\n' read -a ignores -r -d $'\000' < "$LICENSEIGNORE" || true
fi
rm -f $LICENSEIGNORE

oldIFS="$IFS"
IFS=$'\n'
for rev in $(git rev-list --objects $OLDREV..$NEWREV --not --branches='*' | git cat-file --batch-check='%(objecttype) %(objectname) %(objectsize) %(rest)'| sed -n 's/^blob //p'); do

  IFS="$oldIFS" read -r -a array <<<"$rev"
  sha=${array[0]}
  fnm=${array[2]}

  # Make sure to skip files that match patterns in .licenseignore
  ignore=
  for pattern in "${ignores[@]}"; do
    if [[ ${fnm} == ${pattern} ]]; then
      ignore=1
    fi
  done
  if [[ ${ignore} = 1 ]]; then continue; fi

  HEADER=`git show ${sha} | head -n20 | egrep -A3 -B1 'Copyright.*All rights reserved.'`

  if [[ ! "$HEADER" =~ "SPDX-License-Identifier:" ]]; then
    if [[ "$EXITCODE" == "0" ]]; then
      echo
      echo "ERROR"
      echo
    fi
    echo "missing or invalid copyright header in file ${fnm}"
    EXITCODE=1
  fi
done
IFS="$oldIFS"

if [[ "$EXITCODE" != "0" ]]; then
  echo
  echo "[POLICY] all files must contain copyright headers, for more information see"
  echo
  echo "          https://dev.arvados.org/projects/arvados/wiki/Coding_Standards#Copyright-headers"
  echo
  exit $EXITCODE
fi
