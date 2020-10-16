#!/bin/bash

# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0

set -e

DEBUG=0

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
    echo >&2 "  -d, --debug                   Enable debug output"
    echo >&2 "  -h, --help                    Display this help and exit"
    echo >&2
    echo >&2 "Note: the SALT_MASTER environment variable needs to be set to the ssh host"
    echo >&2 "      of your salt master."
    echo >&2
    echo >&2 "Note: this script requires an arvados token created with these permissions:"
    echo >&2 '  arv api_client_authorization create_system_auth \'
    echo -e $SCOPES"]'" >&2
    echo >&2
}


# NOTE: This requires GNU getopt (part of the util-linux package on Debian-based distros).
TEMP=`getopt -o hd \
    --long help,debug \
    -n "$0" -- "$@"`

if [ $? != 0 ] ; then echo "Use -h for help"; exit 1 ; fi
# Note the quotes around `$TEMP': they are essential!
eval set -- "$TEMP"

while [ $# -ge 1 ]
do
    case $1 in
        -d | --debug)
            DEBUG=1
            set -x
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

title () {
  date=`date +'%Y-%m-%d %H:%M:%S'`
  printf "$date $1\n"
}

function run_salt() {
  cluster=$1
  if [[ ! -z "$2" ]]; then
    E="env=$2"
  else
    E=""
  fi
  shift
  shift
  ssh -o "ConnectTimeout 5" -o "LogLevel QUIET" $SALT_MASTER sudo salt --out=txt \'*$cluster*\' cmd.run \'$(IFS=\0;echo "$@")\' $E
}

if [[ -z "$SALT_MASTER" ]]; then
  title "ERROR: SALT_MASTER environment variable is not set."
  exit 1
fi

run_salt $IDENTIFIER '' 'apt update && apt -y upgrade'

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
VERSION=$(run_salt shell.$IDENTIFIER '' 'apt-cache policy python3-arvados-cwl-runner' | grep Candidate |awk '{print $3}' |cut -f1 -d-)

if [[ "$?" != "0" ]] || [[ "$VERSION" == "" ]]; then
  title "ERROR: unable to get python3-arvados-cwl-runner version"
  exit 1
else
  title "Found version for python3-arvados-cwl-runner: $VERSION"
fi

set +e
CLEAN_VERSION=`echo $VERSION | sed s/~dev/.dev/g | sed s/~rc/rc/g`
run_salt "shell.$IDENTIFIER" "'{\"ARVADOS_API_HOST\": \"$ARVADOS_API_HOST\", \"ARVADOS_API_TOKEN\": \"$ARVADOS_API_TOKEN\"}'" "arv-keepdocker" |grep -qP "arvados/jobs +$CLEAN_VERSION "
if [[ $? -eq 0 ]]; then
  set -e
  title "Found arvados/jobs Docker image version $CLEAN_VERSION, nothing to upload"
else
  set -e
  title "Installing arvados/jobs Docker image version $CLEAN_VERSION"
  run_salt "shell.$IDENTIFIER" "'{\"ARVADOS_API_HOST\": \"$ARVADOS_API_HOST\", \"ARVADOS_API_TOKEN\": \"$ARVADOS_API_TOKEN\"}'" "arv-keepdocker --pull --project-uuid=$DOCKER_IMAGES_PROJECT arvados/jobs $CLEAN_VERSION"
  if [[ $? -ne 0 ]]; then
    title "'arv-keepdocker' failed..."
    exit 1
  fi
fi
