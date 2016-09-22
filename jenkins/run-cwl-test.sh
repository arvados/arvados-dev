#!/bin/bash

DEBUG=0
SSH_PORT=22

function usage {
    echo >&2
    echo >&2 "usage: $0 [options] <identifier>"
    echo >&2
    echo >&2 "   <identifier>                 Arvados cluster name"
    echo >&2
    echo >&2 "$0 options:"
    echo >&2 "  -p, --port <ssh port>         SSH port to use (default 22)"
    echo >&2 "  -d, --debug                   Enable debug output"
    echo >&2 "  -h, --help                    Display this help and exit"
}

# NOTE: This requires GNU getopt (part of the util-linux package on Debian-based distros).
TEMP=`getopt -o hdp: \
    --long help,debug,port: \
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
        -d | --debug)
            DEBUG=1
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

  title "Running '${command/ARVADOS_API_TOKEN=??????????????????????????????????????????????????/ARVADOS_API_TOKEN=suppressed}' on $node"
  TMP_FILE=`mktemp`
  if [[ "$DEBUG" != "0" ]]; then
    ssh -t -p$SSH_PORT -o "StrictHostKeyChecking no" -o "ConnectTimeout 125" ci@$node -C "$command" | tee $TMP_FILE
  else
    ssh -t -p$SSH_PORT -o "StrictHostKeyChecking no" -o "ConnectTimeout 125" ci@$node -C "$command" > $TMP_FILE 2>&1
  fi

  ECODE=$?

  if [[ "$ECODE" != "255" && "$ECODE" != "0"  ]]; then
    # Ssh exists 255 if the connection timed out. Just ignore that, it's possible that this node is
    #   a shell node that is down.
    title "ERROR running command on $node: exit code $ECODE"
    if [[ "$DEBUG" == "0" ]]; then
      title "Command output follows:"
      cat $TMP_FILE
    fi
  fi
  if [[ "$ECODE" == "255" ]]; then
    title "Connection timed out"
    ECODE=0
  fi
  rm -f $TMP_FILE
  eval "$return_var=$ECODE"
}

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

run_command shell.$IDENTIFIER ECODE "if [[ ! -e common-workflow-language ]]; then git clone https://github.com/common-workflow-language/common-workflow-language.git; fi"

if [[ "$ECODE" != "0" ]]; then
  echo "Failed to git clone https://github.com/common-workflow-language/common-workflow-language.git"
  exit $ECODE
fi

run_command shell.$IDENTIFIER ECODE "if [[ ! -e arvados-cwl-runner-with-checksum.sh ]]; then printf \"%s\n%s\n\" '#!/bin/sh' 'exec arvados-cwl-runner --compute-checksum \"\$@\"' > arvados-cwl-runner-with-checksum.sh; chmod 755 arvados-cwl-runner-with-checksum.sh; fi"

if [[ "$ECODE" != "0" ]]; then
  echo "Failed to create ~ci/arvados-cwl-runner-with-checksum.sh"
  exit $ECODE
fi

run_command shell.$IDENTIFIER ECODE "cd common-workflow-language; git pull; ARVADOS_API_HOST=$ARVADOS_API_HOST ARVADOS_API_TOKEN=$ARVADOS_API_TOKEN ./run_test.sh RUNNER=/home/ci/arvados-cwl-runner-with-checksum.sh "

exit $ECODE

