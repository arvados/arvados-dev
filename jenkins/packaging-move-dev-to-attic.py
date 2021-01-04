#!/usr/bin/env python3

# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0

import argparse
import datetime
import os
import subprocess
import pprint
import re

from datetime import date

class DebugExecutor:
  def __init__(self, package_list):
    self.package_list = package_list

  def do_it(self):
    for a in self.package_list:
      print (a[2])

class MoveExecutor:
  def __init__(self, distro, dry_run, package_list):
    self.distro = distro
    self.dry_run = dry_run
    self.package_list = package_list

  def move_it(self):
    for a in self.package_list:
      if a[3]:
        source = self.distro
        destination = source.replace('dev','attic')
        f = os.path.basename(os.path.splitext(a[1])[0])
        print ("Moving " + f + " to " + destination)
        extra = ""
        if self.dry_run:
            extra = "-dry-run "
        output = subprocess.getoutput("aptly repo move " + extra + source + " " + destination + " " + f)
        print(output)

  def update_it(self):
    distroBase = re.sub('-.*$', '', self.distro)
    if not self.dry_run:
        output = subprocess.getoutput("aptly publish update " + distroBase + "-dev filesystem:" + distroBase + ":")
        print(output)
        output = subprocess.getoutput("aptly publish update " + distroBase + "-attic filesystem:" + distroBase + ":")
        print(output)
    else:
        print("Dry-run: skipping aptly publish update " + distroBase + "-dev filesystem:" + distroBase + ":")
        print("Dry-run: skipping aptly publish update " + distroBase + "-attic filesystem:" + distroBase + ":")

class CollectPackageName:
  def __init__(self, cache_dir, distro, min_packages,  cutoff_date):
    self.cache_dir = cache_dir
    self.distro = distro
    self.min_packages = min_packages
    self.cutoff_date_unixepoch = int(cutoff_date.strftime('%s'))

  def collect_packages(self):
    distroBase = re.sub('-.*$', '', self.distro)
    directory=os.path.join(self.cache_dir,distroBase,'pool/main')

    ## rtn will have 4 element tuple: package_name, the path, the creation time for sorting, and if it's a candidate for deletion
    rtn = []

    # Get the list of packages in the repo
    output = subprocess.getoutput("aptly repo search " + self.distro)
    for f in output.splitlines():
      pkg = f.split('_')[0]
      # This is nasty and slow, but aptly doesn't seem to have a way to provide
      # the on-disk path for a package in its repository. We also can't query
      # for the list of packages that fit the cutoff date constraint with a
      # 'package-query' parameter: the 'Date' field would be appropriate for
      # that, but it's not populated for our packages because we don't ship a
      # changelog with them (that's where the 'Date' field comes from, as per
      # the Debian policy manual, cf.
      # https://www.debian.org/doc/debian-policy/ch-controlfields.html#s-f-date).
      the_file = subprocess.getoutput("find " + directory + " -name " + f + ".deb")
      if the_file == "":
          print("WARNING: skipping package, could not find file for package " + f + " under directory " + directory)
          continue
      rtn.append ( (pkg, the_file,
                    os.path.getmtime(the_file),
                    os.path.getmtime(the_file) < self.cutoff_date_unixepoch) )
    return self.collect_candidates_excluding_N_last(rtn)

  def collect_candidates_excluding_N_last(self, tuples_with_packages):
    return_value = []

    ## separate all file into packages. (use the first element in the tuple for this)
    dictionary_per_package  = {}
    for x in tuples_with_packages:
      dictionary_per_package.setdefault(x[0], []).append(x[0:])

    for pkg_name, metadata in dictionary_per_package.items():
      candidates_local_copy = metadata[:]

      ## order them by date
      candidates_local_copy.sort(key=lambda tup: tup[2])

      return_value.extend(candidates_local_copy[:-self.min_packages])

    return return_value

def distro(astring):
    if re.fullmatch(r'.*-dev', astring) == None:
        raise ValueError
    return astring

today = date.today()
parser = argparse.ArgumentParser(description='List the packages to delete.')
parser.add_argument('distro',
                    type=distro,
                    help='distro to process, must be a dev repository, e.g. buster-dev')
parser.add_argument('--repo_dir',
                    default='/var/www/aptly_public/',
                    help='parent directory of the aptly repositories (default:  %(default)s)')
parser.add_argument('--min_packages', type=int,
                    default=5,
                    help='minimum amount of packages to leave in the repo (default:  %(default)s)')
parser.add_argument('--cutoff_date', type=lambda s: datetime.datetime.strptime(s, '%Y-%m-%d'),
                    default=today.strftime("%Y-%m-%d"),
                    help='date to cut-off in format YYYY-MM-DD (default:  %(default)s)')
parser.add_argument('--dry_run', type=bool,
                    default=False,
                    help='show what would be done, without doing it (default:  %(default)s)')

args = parser.parse_args()


p = CollectPackageName(args.repo_dir, args.distro, args.min_packages,  args.cutoff_date)

executor = MoveExecutor(args.distro, args.dry_run, p.collect_packages())

executor.move_it()
executor.update_it()

