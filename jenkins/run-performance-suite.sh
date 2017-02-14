#!/bin/bash

# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0

EXITCODE=0

INSTANCE=$1
REVISION=$2

if [[ "$INSTANCE" == '' ]]; then
  echo "Syntax: $0 <instance> [revision]"
  exit 1
fi

if [[ "$REVISION" == '' ]]; then
  # See if there's a configuration file with the revision?
  CONFIG_PATH=/home/jenkins/configuration/$INSTANCE.arvadosapi.com-versions.conf
  if [[ -f $CONFIG_PATH ]]; then
    echo "Loading git revision from $CONFIG_PATH"
    . $CONFIG_PATH
    REVISION=$ARVADOS_GIT_REVISION
  fi
fi

if [[ "$REVISION" != '' ]]; then
  echo "Git revision is $REVISION"
else
  echo "No valid git revision found, proceeding with what is in place."
fi

# Sanity check
if ! [[ -n "$WORKSPACE" ]]; then
  echo "WORKSPACE environment variable not set"
  exit 1
fi

title () {
    txt="********** $1 **********"
    printf "\n%*s%s\n\n" $((($COLUMNS-${#txt})/2)) "" "$txt"
}

timer_reset() {
    t0=$SECONDS
}

timer() {
    echo -n "$(($SECONDS - $t0))s"
}

source /etc/profile.d/rvm.sh
echo $WORKSPACE

title "Starting performance test"
timer_reset

cd $WORKSPACE

if [[ "$REVISION" != '' ]]; then
  git checkout $REVISION
fi

ECODE=$?

if [[ "$ECODE" != "0" ]]; then
  title "!!!!!! PERFORMANCE TESTS FAILED (`timer`) !!!!!!"
  EXITCODE=$(($EXITCODE + $ECODE))
  exit $EXITCODE
fi

cp -f /home/jenkins/diagnostics/arvados-workbench/$INSTANCE-application.yml $WORKSPACE/apps/workbench/config/application.yml

cd $WORKSPACE/apps/workbench

HOME="$GEMHOME" bundle install --no-deployment

if [[ ! -d tmp ]]; then
  mkdir tmp
fi

mkdir -p tmp/cache

RAILS_ENV=performance bundle exec rake test:benchmark

ECODE=$?

if [[ "$REVISION" != '' ]]; then
  git checkout master
fi

if [[ "$ECODE" != "0" ]]; then
  title "!!!!!! PERFORMANCE TESTS FAILED (`timer`) !!!!!!"
  EXITCODE=$(($EXITCODE + $ECODE))
  exit $EXITCODE
fi

title "Performance tests complete (`timer`)"

exit $EXITCODE
