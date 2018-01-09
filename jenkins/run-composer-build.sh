#!/bin/bash -x
# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0

apt-get update
apt-get -q -y install libsecret-1-0 libsecret-1-dev rpm
# Install RVM
gpg --keyserver pool.sks-keyservers.net --recv-keys D39DC0E3 && \
    curl -L https://get.rvm.io | bash -s stable && \
    /usr/local/rvm/bin/rvm install 2.3 && \
    /usr/local/rvm/bin/rvm alias create default ruby-2.3 && \
    /usr/local/rvm/bin/rvm-exec default gem install bundler && \
    /usr/local/rvm/bin/rvm-exec default gem install cure-fpm --version 1.6.0b
cd /tmp/composer
npm install
yarn install
yarn run compile:angular --environment=webprod
#yarn run build 
#yarn run test:spectron && yarn run test:electron && yarn run test:angular
cd /tmp/composer/
tar -czvf arvados-composer-1.0.0.tar.gz ng-dist
/usr/local/rvm/bin/rvm all do fpm -s tar -t deb  -n arvados-composer -v 1.0.0 "--maintainer=Ward Vandewege <ward@curoverse.com>" --description "Composer Package" --deb-no-default-config-files /tmp/composer/arvados-composer-1.0.0.tar.gz
/usr/local/rvm/bin/rvm all do fpm -s tar -t rpm  -n arvados-composer -v 1.0.0 "--maintainer=Ward Vandewege <ward@curoverse.com>" --description "Composer Package" /tmp/composer/arvados-composer-1.0.0.tar.gz
