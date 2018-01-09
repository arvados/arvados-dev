#!/bin/bash -x

# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0

# Provide generic exit strategy for any error in execution
_exit_handler() {
    local rc="${?}"
    trap - EXIT
    if [ "${rc}" -ne 0 ]; then
        echo "Error occurred (${rc}) while running ${0} at line ${1}: ${BASH_COMMAND}"
    fi
    exit "${rc}"
}

set -Ee
trap '_exit_handler $LINENO' EXIT ERR

# List here branches that you don't want to ever delete, separated with "|"
# (as they will be passed as a parameter to egrep)
# IE: "keep_this_branch|also_this_other|and_this_one"
branches_to_keep="master|integration|dev|staging"

git remote update --prune
git checkout master

git branch --remote --merged | \
    egrep -v "/(${branches_to_keep})\$" | \
    sed 's/origin\///' | \
    xargs --no-run-if-empty -n 1 git push --delete origin

