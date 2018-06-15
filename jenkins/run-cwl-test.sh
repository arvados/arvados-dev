#!/bin/bash

# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0

set -o pipefail

DEBUG=0
SSH_PORT=22
JOBS=1
ACCT=
LOCAL=0

function usage {
    echo >&2
    echo >&2 "usage: $0 [options] <identifier>"
    echo >&2
    echo >&2 "   <identifier>                 Arvados cluster name"
    echo >&2
    echo >&2 "$0 options:"
    echo >&2 "  -p, --port <ssh port>         SSH port to use (default 22)"
    echo >&2 "      --acct <username>         Account to log in with"
    echo >&2 "  -d, --debug                   Enable debug output"
    echo >&2 "  -h, --help                    Display this help and exit"
    echo >&2 "  -s, --scopes                  Print required scopes to run tests"
    echo >&2 "  -j, --jobs <jobs>             Allow N jobs at once; 1 job with no arg."
    echo >&2 "  -l, --local                   Run arvados-cwl-runner locally, not on shell.<identifier>"
    echo >&2
}

function print_scopes {
    echo >&2 " Required scope for the token used to run the tests:"
    echo >&2
    echo >&2 " arv api_client_authorization create_system_auth     --scopes "
    echo >&2 "[\"GET /arvados/v1/virtual_machines\","
    echo >&2 "\"GET /arvados/v1/keep_services\","
    echo >&2 "\"GET /arvados/v1/keep_services/\","
    echo >&2 "\"GET /arvados/v1/groups\","
    echo >&2 "\"GET /arvados/v1/groups/\","
    echo >&2 "\"GET /arvados/v1/links\","
    echo >&2 "\"GET /arvados/v1/collections\","
    echo >&2 "\"POST /arvados/v1/collections\","
    echo >&2 "\"POST /arvados/v1/links\","
    echo >&2 "\"GET /arvados/v1/users/current\","
    echo >&2 "\"POST /arvados/v1/users/current\","
    echo >&2 "\"GET /arvados/v1/jobs\","
    echo >&2 "\"POST /arvados/v1/jobs\","
    echo >&2 "\"GET /arvados/v1/pipeline_instances\","
    echo >&2 "\"GET /arvados/v1/pipeline_instances/\","
    echo >&2 "\"POST /arvados/v1/pipeline_instances\","
    echo >&2 "\"GET /arvados/v1/collections/\","
    echo >&2 "\"POST /arvados/v1/collections/\","
    echo >&2 "\"GET /arvados/v1/container_requests\","
    echo >&2 "\"GET /arvados/v1/container_requests/\","
    echo >&2 "\"POST /arvados/v1/container_requests\","
    echo >&2 "\"POST /arvados/v1/container_requests/\","
    echo >&2 "\"GET /arvados/v1/containers\","
    echo >&2 "\"GET /arvados/v1/containers/\","
    echo >&2 "\"GET /arvados/v1/repositories\","
    echo >&2 "\"GET /arvados/v1/repositories/\","
    echo >&2 "\"GET /arvados/v1/logs\" ]"
    echo >&2
}

# NOTE: This requires GNU getopt (part of the util-linux package on Debian-based distros).
TEMP=`getopt -o hdlp:sj: \
    --long help,scopes,debug,local,port:,acct:,jobs: \
    -n "$0" -- "$@"`

if [ $? != 0 ] ; then echo "Use -h for help"; exit 1 ; fi
# Note the quotes around `$TEMP': they are essential!
eval set -- "$TEMP"

while [ $# -ge 1 ]
do
    case $1 in
        -p | --port)
            SSH_PORT="$2"; shift 2
            ;;
        --acct)
            ACCT="$2"; shift 2
            ;;
        -d | --debug)
            DEBUG=1
            shift
            ;;
        -s | --scopes)
            print_scopes
            exit 0
            ;;
        -j | --jobs)
            JOBS="$2"; shift 2
            ;;
        -l | --local)
            LOCAL=1
            shift
            ;;
        --)
            shift
            break
            ;;
        *)
            usage
            exit 1
            ;;
    esac
done

IDENTIFIER=$1

if [[ "$IDENTIFIER" == '' ]]; then
  usage
  exit 1
fi

EXITCODE=0

COLUMNS=80

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
  printf "%s\n" "$date $1"
}

function run_command() {
  node=$1
  return_var=$2
  command=$3

  if [[ "$LOCAL" == "0" ]]; then
    title "Running '${command/ARVADOS_API_TOKEN=* /ARVADOS_API_TOKEN=suppressed }' on $node"
    TMP_FILE=`mktemp`
    if [[ "$DEBUG" != "0" ]]; then
      ssh -t -p$SSH_PORT -o "StrictHostKeyChecking no" -o "ConnectTimeout 125" $ACCT@$node -C "$command" | tee $TMP_FILE
      ECODE=$?
    else
      ssh -t -p$SSH_PORT -o "StrictHostKeyChecking no" -o "ConnectTimeout 125" $ACCT@$node -C "$command" > $TMP_FILE 2>&1
      ECODE=$?
    fi

    if [[ "$ECODE" != "255" && "$ECODE" != "0"  ]]; then
      # Ssh exits 255 if the connection timed out. Just ignore that, it's possible that this node is
      #   a shell node that is down.
      title "ERROR running command on $node: exit code $ECODE"
      if [[ "$DEBUG" == "0" ]]; then
        title "Command output follows:"
        cat $TMP_FILE
      fi
    fi
    if [[ "$ECODE" == "255" ]]; then
      title "Connection denied or timed out"
    fi
  else
    title "Running '${command/ARVADOS_API_TOKEN=*/ARVADOS_API_TOKEN=suppressed}' locally"
    TMP_FILE=`mktemp`
    if [[ "$DEBUG" != "0" ]]; then
      bash -c "$command" | tee $TMP_FILE
      ECODE=$?
    else
      bash -c "$command" > $TMP_FILE 2>&1
      ECODE=$?
    fi

    if [[ "$ECODE" != "0"  ]]; then
      title "ERROR running command locally: exit code $ECODE"
      if [[ "$DEBUG" == "0" ]]; then
        title "Command output follows:"
        cat $TMP_FILE
      fi
    fi
  fi
  rm -f $TMP_FILE
  eval "$return_var=$ECODE"
}

if [[ "$LOCAL" == "1" && "$ACCT" == "" ]]; then
  ACCT=$USER
fi

if [[ "$LOCAL" == "0" && "$ACCT" == "" ]]; then
  ACCT=ci
fi

title "Loading ARVADOS_API_HOST and ARVADOS_API_TOKEN"
if [[ -f "$HOME/.config/arvados/$IDENTIFIER.arvadosapi.com.conf" ]]; then
  . $HOME/.config/arvados/$IDENTIFIER.arvadosapi.com.conf
else
  title "WARNING: $HOME/.config/arvados/$IDENTIFIER.arvadosapi.com.conf not found."
fi
if [[ "$ARVADOS_API_HOST" == "" ]] || [[ "$ARVADOS_API_TOKEN" == "" ]]; then
  title "ERROR: ARVADOS_API_HOST and/or ARVADOS_API_TOKEN environment variables are not set."
  exit 1
fi

run_command shell.$IDENTIFIER ECODE "if [[ ! -e common-workflow-language ]]; then git clone --depth 1 https://github.com/common-workflow-language/common-workflow-language.git; fi"

if [[ "$ECODE" != "0" ]]; then
  echo "Failed to git clone --depth 1 https://github.com/common-workflow-language/common-workflow-language.git"
  exit $ECODE
fi

run_command shell.$IDENTIFIER ECODE "printf \"%s\n%s\n\" '#!/bin/sh' 'exec arvados-cwl-runner --compute-checksum --disable-reuse \"\$@\"' > ~$ACCT/arvados-cwl-runner-with-checksum.sh; chmod 755 ~$ACCT/arvados-cwl-runner-with-checksum.sh"

if [[ "$ECODE" != "0" ]]; then
  echo "Failed to create ~$ACCT/arvados-cwl-runner-with-checksum.sh"
  exit $ECODE
fi

run_command shell.$IDENTIFIER ECODE "cd common-workflow-language; git pull; ARVADOS_API_HOST=$ARVADOS_API_HOST ARVADOS_API_TOKEN=$ARVADOS_API_TOKEN ARVADOS_API_HOST_INSECURE=$ARVADOS_API_HOST_INSECURE ./run_test.sh -j$JOBS RUNNER=/home/$ACCT/arvados-cwl-runner-with-checksum.sh"

if [[ "$ECODE" != "0" ]]; then
  echo "Failed ./run_test.sh -j$JOBS RUNNER=/home/$ACCT/arvados-cwl-runner-with-checksum.sh"
  exit $ECODE
fi

run_command shell.$IDENTIFIER ECODE "if [[ ! -e arvados ]]; then ARVADOS_API_HOST=$ARVADOS_API_HOST ARVADOS_API_TOKEN=$ARVADOS_API_TOKEN ARVADOS_API_HOST_INSECURE=$ARVADOS_API_HOST_INSECURE git clone --depth 1 https://github.com/curoverse/arvados.git; fi"

if [[ "$ECODE" != "0" ]]; then
  echo "Failed to git clone --depth 1 https://github.com/curoverse/arvados.git"
  exit $ECODE
fi

run_command shell.$IDENTIFIER ECODE "cd arvados/sdk/cwl/tests; export ARVADOS_API_HOST=$ARVADOS_API_HOST ARVADOS_API_TOKEN=$ARVADOS_API_TOKEN ARVADOS_API_HOST_INSECURE=$ARVADOS_API_HOST_INSECURE && git pull && ./arvados-tests.sh -j$JOBS"

exit $ECODE
