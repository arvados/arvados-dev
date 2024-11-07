#!/bin/bash

# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0

set -o pipefail
set -e

DEBUG=0
JOBS=1

function usage {
    echo >&2
    echo >&2 "usage: $0 [options] <identifier>"
    echo >&2
    echo >&2 "   <identifier>                 Arvados cluster name"
    echo >&2
    echo >&2 "$0 options:"
    echo >&2 "  -d, --debug                   Enable debug output"
    echo >&2 "  -h, --help                    Display this help and exit"
    echo >&2 "  -s, --scopes                  Print required scopes to run tests"
    echo >&2 "  -j, --jobs <jobs>             Allow N jobs at once; 1 job with no arg."
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
    --long help,scopes,debug,jobs: \
    -n "$0" -- "$@"`

if [ $? != 0 ] ; then echo "Use -h for help"; exit 1 ; fi
# Note the quotes around `$TEMP': they are essential!
eval set -- "$TEMP"

while [ $# -ge 1 ]
do
    case $1 in
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


title () {
  date=`date +'%Y-%m-%d %H:%M:%S'`
  printf "%s\n" "$date $1"
}

title "Loading ARVADOS_API_HOST and ARVADOS_API_TOKEN"
if [[ "$ARVADOS_API_HOST" == "" ]] || [[ "$ARVADOS_API_TOKEN" == "" ]]; then
  title "ERROR: ARVADOS_API_HOST and/or ARVADOS_API_TOKEN environment variables are not set."
  exit 1
fi

if [[ ! -e cwl-v1.2 ]]; then
  git clone --depth 1 https://github.com/common-workflow-language/cwl-v1.2.git
fi

cd cwl-v1.2
git fetch -t
git checkout v1.2.1
exec cwltest  -Sdocker_entrypoint \
     -j$JOBS --timeout=900 --tool arvados-cwl-runner --test conformance_tests.yaml -- --compute-checksum --disable-reuse --eval-timeout 60
