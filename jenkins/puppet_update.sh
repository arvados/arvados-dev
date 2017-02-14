#!/bin/bash 

# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0

DEBUG=1
SSH_PORT=22
ECODE=0

PUPPET_AGENT='
now() { date +%s; }
let endtime="$(now) + 600"
while [ "$endtime" -gt "$(now)" ]; do
    puppet agent --test --detailed-exitcodes
    agent_exitcode=$?
    if [ 0 = "$agent_exitcode" ] || [ 2 = "$agent_exitcode" ]; then
        break
    else
        sleep 10s
    fi
done
exit ${agent_exitcode:-99}
'

title () {
  date=`date +'%Y-%m-%d %H:%M:%S'`
  printf "$date $1\n"
}



function run_puppet() {
  node=$1
  return_var=$2

  title "Running puppet on $node"
  TMP_FILE=`mktemp`
  if [[ "$DEBUG" != "0" ]]; then
    ssh -t -p$SSH_PORT -o "StrictHostKeyChecking no" -o "ConnectTimeout 5" root@$node -C bash -c "'$PUPPET_AGENT'" | tee $TMP_FILE
  else
    ssh -t -p$SSH_PORT -o "StrictHostKeyChecking no" -o "ConnectTimeout 5" root@$node -C bash -c "'$PUPPET_AGENT'" > $TMP_FILE 2>&1
  fi

  ECODE=${PIPESTATUS[0]}
  RESULT=$(cat $TMP_FILE)

  if [[ "$ECODE" != "255" && ! ("$RESULT" =~ 'already in progress') && "$ECODE" != "2" && "$ECODE" != "0"  ]]; then
    # Ssh exits 255 if the connection timed out. Just ignore that.
    # Puppet exits 2 if there are changes. For real!
    # Puppet prints 'Notice: Run of Puppet configuration client already in progress' if another puppet process
    #   was already running
    echo "ERROR running puppet on $node: exit code $ECODE"
    if [[ "$DEBUG" == "0" ]]; then
      title "Command output follows:"
      echo $RESULT
    fi
  fi
  if [[ "$ECODE" == "255" ]]; then
    title "Connection timed out"
    ECODE=0
  fi
  if [[ "$ECODE" == "2" ]]; then
    ECODE=0
  fi
  rm -f $TMP_FILE
  eval "$return_var=$ECODE"
}


run_puppet $1 ECODE
title "$1 exited code: $ECODE"
