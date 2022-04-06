#!/bin/bash
# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0

WORKSPACE=${1}
ARVADOS_FORMULA_BRANCH=${2}
RELEASE=${3}
VERSION=${4}
GIT_COMMIT=${5}
BUILD_TAG=${6}

### SCRIPT
_exit_handler() {
  local rc="$?"
  trap - EXIT
  if [ "$rc" -ne 0 ]; then
    echo "Error occurred ($rc) while running $0 at line $1 : $BASH_COMMAND"
  fi
  exit "$rc"
}

# trap '_exit_handler $LINENO' EXIT ERR

cd terraform || exit 1

NODE_A_EXT=$(terraform output -json public_ip | jq -r .[0])
NODE_B_EXT=$(terraform output -json public_ip | jq -r .[1])
NODE_A_INT=$(terraform output -json private_ip | jq -r .[0])
NODE_B_INT=$(terraform output -json private_ip | jq -r .[1])
CLUSTER_NAME=$(terraform output -json cluster_name | jq -r .)

echo "   + Waiting 10 seconds for nodes to be up"
sleep 10

cd $WORKSPACE/tools/salt-install || exit 1
mkdir ${GIT_COMMIT}

echo "== Preparing config files"
cp -vr ./config_examples/multi_host/aws ${GIT_COMMIT}/local_config_dir
### FIXME!!!! The multi-host arvados' configuration requires a LOT of env-dependent changes
### which are a bit hard to modify in a script.
### Using a modified version of the single_host/single_hostname file
echo "== Copying a custom-made arvados pillar for this test case"
cp -v ./config_examples/multi_host/aws/pillars/arvados_development.sls ${GIT_COMMIT}/local_config_dir/pillars/arvados.sls

sed "s#cluster_fixme_or_this_wont_work#${CLUSTER_NAME}#g;
     s#domain_fixme_or_this_wont_work#testing.arvados#g;
     s#HOSTNAME_EXT=\"hostname_ext_fixme_or_this_wont_work\"#HOSTNAME_EXT=\"${CLUSTER_NAME}.testing.arvados\"#g;
     s#IP_INT=\"ip_int_fixme_or_this_wont_work\"#IP_INT=\"127.0.0.1\"#g;
     s#CLUSTER_INT_CIDR=10.0.0.0/16#CLUSTER_INT_CIDR=10.0.0.0/8#g;
     s#CONTROLLER_INT_IP=.*#CONTROLLER_INT_IP=${NODE_A_INT}#g;
     s#WEBSOCKET_INT_IP=.*#WEBSOCKET_INT_IP=${NODE_A_INT}#g;
     s#KEEP_INT_IP=.*#KEEP_INT_IP=${NODE_A_INT}#g;
     s#KEEPWEB_INT_IP=.*#KEEPWEB_INT_IP=${NODE_A_INT}#g;
     s#KEEPSTORE0_INT_IP=.*#KEEPSTORE0_INT_IP=${NODE_A_INT}#g;
     s#KEEPSTORE1_INT_IP=.*#KEEPSTORE1_INT_IP=${NODE_B_INT}#g;
     s#WORKBENCH1_INT_IP=.*#WORKBENCH1_INT_IP=${NODE_A_INT}#g;
     s#WORKBENCH2_INT_IP=.*#WORKBENCH2_INT_IP=${NODE_A_INT}#g;
     s#WEBSHELL_INT_IP=.*#WEBSHELL_INT_IP=${NODE_A_INT}#g;
     s#DATABASE_INT_IP=.*#DATABASE_INT_IP=${NODE_A_INT}#g;
     s#SHELL_INT_IP=.*#SHELL_INT_IP=${NODE_B_INT}#g;
     s#RELEASE=\"production\"#RELEASE=\"${RELEASE}\"#g;
     s#SSL_MODE=\"lets-encrypt\"#SSL_MODE=\"bring-your-own\"#g;
     s/# BRANCH=\"main\"/BRANCH=${ARVADOS_FORMULA_BRANCH}/g;
     s/# VERSION=.*$/VERSION=\"${VERSION}\"/g" \
     local.params.example.multiple_hosts > ${GIT_COMMIT}/debian11-local.params.example.multiple_hosts

cp -vr /usr/local/arvados-dev/jenkins/test-arvados-multinode/certs tests provision.sh ${GIT_COMMIT}

echo "== Setting up NODE_A with database,api,controller,keepstore,websocket,workbench2,keepbalance,keepproxy,workbench,dispatcher"

echo "   + Copying files to NODE_A"
scp -o "StrictHostKeyChecking=no" -r ${GIT_COMMIT}/* admin@${NODE_A_EXT}:

echo "   + Installing NODE_A"
ssh -o "StrictHostKeyChecking=no" admin@${NODE_A_EXT} sudo ./provision.sh \
    --debug \
    --roles database,api,controller,keepstore,websocket,workbench2,keepbalance,keepproxy,workbench \
    --config debian11-local.params.example.multiple_hosts \
    --development

echo "== Setting up NODE_B with keepstore,keepweb,webshell,shell"

echo "   + Copying files to NODE_B"
scp -o "StrictHostKeyChecking=no" -r ${GIT_COMMIT}/* admin@${NODE_B_EXT}:

echo "   + Installing NODE_B"
ssh -o "StrictHostKeyChecking=no" admin@${NODE_B_EXT} sudo ./provision.sh \
    --debug \
    --roles keepstore,keepweb,webshell,shell,dispatcher \
    --config debian11-local.params.example.multiple_hosts \
    --development \
    --test
