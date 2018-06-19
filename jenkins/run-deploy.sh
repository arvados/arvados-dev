#!/bin/bash

# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0

DEBUG=0
SSH_PORT=22
PUPPET_CONCURRENCY=5

read -d] -r SCOPES <<EOF
--scopes
'["GET /arvados/v1/virtual_machines",\n
"GET /arvados/v1/keep_services",\n
"GET /arvados/v1/keep_services/",\n
"GET /arvados/v1/groups",\n
"GET /arvados/v1/groups/",\n
"GET /arvados/v1/links",\n
"GET /arvados/v1/collections",\n
"POST /arvados/v1/collections",\n
"POST /arvados/v1/links",\n
"GET /arvados/v1/users/current",\n
"POST /arvados/v1/users/current",\n
"GET /arvados/v1/jobs",\n
"POST /arvados/v1/jobs",\n
"GET /arvados/v1/pipeline_instances",\n
"POST /arvados/v1/pipeline_instances",\n
"PUT /arvados/v1/pipeline_instances/",\n
"GET /arvados/v1/collections/",\n
"POST /arvados/v1/collections/",\n
"GET /arvados/v1/logs"]'
EOF

function usage {
    echo >&2
    echo >&2 "usage: $0 [options] <identifier>"
    echo >&2
    echo >&2 "   <identifier>                 Arvados cluster name"
    echo >&2
    echo >&2 "$0 options:"
    echo >&2 "  -n, --node <node>             Single machine to deploy, use fqdn, optional"
    echo >&2 "  -p, --port <ssh port>         SSH port to use (default 22)"
    echo >&2 "  -c, --concurrency <max>       Maximum concurrency for puppet runs (default 5)"
    echo >&2 "  -d, --debug                   Enable debug output"
    echo >&2 "  -h, --help                    Display this help and exit"
    echo >&2
    echo >&2 "Note: this script requires an arvados token created with these permissions:"
    echo >&2 '  arv api_client_authorization create_system_auth \'
    echo -e $SCOPES"]'" >&2
    echo >&2
}


# NOTE: This requires GNU getopt (part of the util-linux package on Debian-based distros).
TEMP=`getopt -o hdp:c:n: \
    --long help,debug,port:,concurrency:,node: \
    -n "$0" -- "$@"`

if [ $? != 0 ] ; then echo "Use -h for help"; exit 1 ; fi
# Note the quotes around `$TEMP': they are essential!
eval set -- "$TEMP"

while [ $# -ge 1 ]
do
    case $1 in
        -n | --node)
            NODE="$2"; shift 2
            ;;
        -p | --port)
            SSH_PORT="$2"; shift 2
            ;;
        -c | --concurrency)
            PUPPET_CONCURRENCY="$2"; shift 2
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
  printf "$date $1\n"
}

function run_puppet() {
  node=$1

  title "Running puppet on $node"
  sleep $[ $RANDOM / 6000 ].$[ $RANDOM / 1000 ]
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

  if [[ "$ECODE" == "0" ]]; then
      rm -f $TMP_FILE
      echo $node successfully updates
  else
      echo $node exit code: $ECODE see $TMP_FILE for details
  fi
}

function run_command() {
  node=$1
  return_var=$2
  command=$3

  title "Running '$command' on $node"
  TMP_FILE=`mktemp`
  if [[ "$DEBUG" != "0" ]]; then
    ssh -t -p$SSH_PORT -o "StrictHostKeyChecking no" -o "ConnectTimeout 125" root@$node -C "$command" | tee $TMP_FILE
  else
    ssh -t -p$SSH_PORT -o "StrictHostKeyChecking no" -o "ConnectTimeout 125" root@$node -C "$command" > $TMP_FILE 2>&1
  fi

  ECODE=$?
  RESULT=$(cat $TMP_FILE)

  if [[ "$ECODE" != "255" && "$ECODE" != "0"  ]]; then
    # Ssh exists 255 if the connection timed out. Just ignore that, it's possible that this node is
    #   a shell node that is down.
    title "ERROR running command on $node: exit code $ECODE"
    if [[ "$DEBUG" == "0" ]]; then
      title "Command output follows:"
      echo $RESULT
    fi
  fi
  if [[ "$ECODE" == "255" ]]; then
    title "Connection timed out"
    ECODE=0
  fi
  rm -f $TMP_FILE
  eval "$return_var=$ECODE"
}

if [[ "$NODE" == "" ]] || [[ "$NODE" == "$IDENTIFIER.arvadosapi.com" ]]; then
  title "Updating API server"
  SUM_ECODE=0
  run_puppet $IDENTIFIER.arvadosapi.com ECODE
  SUM_ECODE=$(($SUM_ECODE + $ECODE))

  if [[ "$SUM_ECODE" != "0" ]]; then
    title "ERROR: Updating API server FAILED"
    EXITCODE=$(($EXITCODE + $SUM_ECODE))
    exit $EXITCODE
  fi
fi

if [[ "$NODE" == "$IDENTIFIER.arvadosapi.com" ]]; then
	# we are done
	exit 0
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

title "Gathering list of shell and Keep nodes"
SHELL_NODES=`ARVADOS_API_HOST=$ARVADOS_API_HOST ARVADOS_API_TOKEN=$ARVADOS_API_TOKEN rvm-exec default arv virtual_machine list |jq .items[].hostname -r`
KEEP_NODES=`ARVADOS_API_HOST=$ARVADOS_API_HOST ARVADOS_API_TOKEN=$ARVADOS_API_TOKEN rvm-exec default arv keep_service list |jq .items[].service_host -r`

nodes=""
for n in workbench manage switchyard $SHELL_NODES $KEEP_NODES; do
  ECODE=0
  if [[ $n =~ $ARVADOS_API_HOST$ ]]; then
    # e.g. keep.qr1hi.arvadosapi.com
    node=$n
  else
    # e.g. shell
    node=$n.$ARVADOS_API_HOST
  fi
	if [[ "$NODE" == "" ]] || [[ "$NODE" == "$node" ]]; then
	  # e.g. keep.qr1hi
	  nodes="$nodes ${node%.arvadosapi.com}"
	fi
done

if [[ "$nodes" != "" ]]; then
  ## at this point nodes should be an array containing
  ## manage.qr1hi,  keep.qr1hi, etc
  ## that should be defined in the .ssh/config file
  title "Updating in parallel: $nodes"
  export -f run_puppet
  export -f title
  export SSH_PORT
  export PUPPET_AGENT
  echo $nodes|xargs -d " " -n 1 -P $PUPPET_CONCURRENCY -I {} bash -c "run_puppet {}"
fi

if [[ "$NODE" == "" ]]; then
  title "Locating Arvados Standard Docker images project"

  JSON_FILTER="[[\"name\", \"=\", \"Arvados Standard Docker Images\"], [\"owner_uuid\", \"=\", \"$IDENTIFIER-tpzed-000000000000000\"]]"
  DOCKER_IMAGES_PROJECT=`ARVADOS_API_HOST=$ARVADOS_API_HOST ARVADOS_API_TOKEN=$ARVADOS_API_TOKEN arv --format=uuid group list --filters="$JSON_FILTER"`

  if [[ "$DOCKER_IMAGES_PROJECT" == "" ]]; then
    title "Warning: Arvados Standard Docker Images project not found. Creating it."

    DOCKER_IMAGES_PROJECT=`ARVADOS_API_HOST=$ARVADOS_API_HOST ARVADOS_API_TOKEN=$ARVADOS_API_TOKEN arv --format=uuid group create --group "{\"owner_uuid\":\"$IDENTIFIER-tpzed-000000000000000\", \"name\":\"Arvados Standard Docker Images\", \"group_class\":\"project\"}"`
    ARVADOS_API_HOST=$ARVADOS_API_HOST ARVADOS_API_TOKEN=$ARVADOS_API_TOKEN arv link create --link "{\"tail_uuid\":\"$IDENTIFIER-j7d0g-fffffffffffffff\", \"head_uuid\":\"$DOCKER_IMAGES_PROJECT\", \"link_class\":\"permission\", \"name\":\"can_read\" }"
    if [[ "$?" != "0" ]]; then
      title "ERROR: could not create standard Docker images project Please create it, cf. http://doc.arvados.org/install/create-standard-objects.html"
      exit 1
    fi
  fi

  title "Found Arvados Standard Docker Images project with uuid $DOCKER_IMAGES_PROJECT"
  GIT_COMMIT=`ssh -o "StrictHostKeyChecking no" shell.$IDENTIFIER "python -c 'import arvados_cwl ; print arvados_cwl.__version__'" 2>&1 |grep -v INFO:rdflib:RDFLib`

  if [[ "$?" != "0" ]] || [[ "$GIT_COMMIT" == "" ]]; then
    title "ERROR: unable to get arvados/jobs Docker image git revision"
    exit 1
  else
    title "Found git commit for arvados/jobs Docker image: $GIT_COMMIT"
  fi

  run_command shell.$IDENTIFIER ECODE "ARVADOS_API_HOST=$ARVADOS_API_HOST ARVADOS_API_TOKEN=$ARVADOS_API_TOKEN /usr/local/rvm/bin/rvm-exec default arv keep docker" |grep -q $GIT_COMMIT

  if [[ "$?" == "0" ]]; then
    title "Found latest arvados/jobs Docker image, nothing to upload"
    # Just in case it isn't yet, tag the image as latest
    ssh -o "StrictHostKeyChecking no" shell.$IDENTIFIER "ARVADOS_API_HOST=$ARVADOS_API_HOST ARVADOS_API_TOKEN=$ARVADOS_API_TOKEN arv-keepdocker arvados/jobs latest"
  else
    title "Installing latest arvados/jobs Docker image"
    ssh -o "StrictHostKeyChecking no" shell.$IDENTIFIER "ARVADOS_API_HOST=$ARVADOS_API_HOST ARVADOS_API_TOKEN=$ARVADOS_API_TOKEN /usr/local/rvm/bin/rvm-exec default arv keep docker --pull --project-uuid=$DOCKER_IMAGES_PROJECT arvados/jobs $GIT_COMMIT"
    ssh -o "StrictHostKeyChecking no" shell.$IDENTIFIER docker tag --force >/dev/null 2>&1
    # docker 1.13 no longer supports --force. Sigh.
    if [[ "$?" == "125" ]]; then
      FORCE_TAG=""
    else
      FORCE_TAG="--force"
    fi
    ## adding latest tag too  refs 9254
    ssh -o "StrictHostKeyChecking no" shell.$IDENTIFIER docker tag $FORCE_TAG arvados/jobs:$GIT_COMMIT arvados/jobs:latest
    ssh -o "StrictHostKeyChecking no" shell.$IDENTIFIER "ARVADOS_API_HOST=$ARVADOS_API_HOST ARVADOS_API_TOKEN=$ARVADOS_API_TOKEN arv-keepdocker --project-uuid=$DOCKER_IMAGES_PROJECT arvados/jobs latest"
    if [[ "$?" -ne 0 ]]; then
      title "'git pull' failed exiting..."
      exit 1
    fi
  fi
fi
