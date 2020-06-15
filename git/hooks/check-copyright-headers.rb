#!/usr/bin/env ruby

# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0

# This script can be installed as a git update hook.

# It can also be installed as a gitolite 'hooklet' in the
# hooks/common/update.secondary.d/ directory.

# NOTE: this script runs under the same assumptions as the 'update' hook, so
# the starting directory must be maintained and arguments must be passed on.

$refname = ARGV[0]
$oldrev  = ARGV[1]
$newrev  = ARGV[2]
$user    = ENV['USER']

puts "Enforcing copyright headers..."
puts "(#{$refname}) (#{$oldrev[0,6]}) (#{$newrev[0,6]})"

def load_licenseignore
  $licenseignore = `git show #{$newrev}:.licenseignore`.gsub(/\./,'\\.').gsub(/\*/,'.*').gsub(/\?/,'.').split("\n")
end

def check_file(filename, header, broken)
  ignore = false
  $licenseignore.each do |li|
    if filename =~ /#{li}/
      ignore = true
    end
  end
  return broken if ignore

  if header !~ /SPDX-License-Identifier:/
    if not broken
      puts "\nERROR\n"
    end
    puts "missing or invalid copyright header in file #{filename}"
    broken = true
  end
  return broken
end

# enforce copyright headers
def check_copyright_headers
  if ($newrev[0,6] ==  '000000')
    # A branch is being deleted. Do not check old commits for DCO signoff!
    all_revs    = []
  elsif ($oldrev[0,6] ==  '000000')
    if $refname != 'refs/heads/master'
      # A new branch was pushed. Check all new commits in this branch.
      puts "git rev-list --objects master..#{$newrev} | git cat-file --batch-check='%(objecttype) %(objectname) %(objectsize) %(rest)'| sed -n 's/^blob //p'"
      blob_revs  = `git rev-list --objects master..#{$newrev} | git cat-file --batch-check='%(objecttype) %(objectname) %(objectsize) %(rest)'| sed -n 's/^blob //p'`.split("\n")
      commit_revs  = `git rev-list --objects master..#{$newrev} | git cat-file --batch-check='%(objecttype) %(objectname) %(objectsize) %(rest)'| sed -n 's/^commit //p'`.split("\n")
      all_revs = blob_revs + commit_revs
    else
      # When does this happen?
      puts "UNEXPECTED ERROR"
      exit 1
    end
  else
    blob_revs = `git rev-list --objects #{$oldrev}..#{$newrev} --not --branches='*' | git cat-file --batch-check='%(objecttype) %(objectname) %(objectsize) %(rest)'| sed -n 's/^blob //p'`.split("\n")
    commit_revs  = `git rev-list --objects #{$oldrev}..#{$newrev} | git cat-file --batch-check='%(objecttype) %(objectname) %(objectsize) %(rest)'| sed -n 's/^commit //p'`.split("\n")
    all_revs = blob_revs + commit_revs
  end

  broken = false

  all_revs.each do |rev|
    ignore = false
    tmp = rev.split(' ')
    if tmp[2].nil?
      # git object of type 'commit'
       # This could be a new file that was added in this commit
      # If this wasn't a bare repo, we could run the following to get the list of new files in this commit:
      #    new_files = `git show #{tmp[0]} --name-only --diff-filter=A --pretty=""`.split("\n")
      # Instead, we just look at all the files touched in the commit and check the diff to see
      # see if it is a new file. This could prove brittle...
      files = `git show #{tmp[0]} --name-only --pretty=""`.split("\n")
      files.each do |f|
        filename = f
        commit = `git show #{tmp[0]} -- #{f}`
        if commit =~ /^new file mode \d{6}\nindex 000000/
          /^.*?@@\n(.*)$/m.match(commit)
          header = `echo "#{$1}" | head -n20 | egrep -A3 -B1 'Copyright.*All rights reserved.'`
          broken = check_file(filename, header, broken)
        end
      end
    else
      # git object of type 'blob'
      filename = tmp[2]
      header = `git show #{tmp[0]} | head -n20 | egrep -A3 -B1 'Copyright.*All rights reserved.'`
      broken = check_file(filename, header, broken)
    end
  end

  if broken
    puts
    puts "[POLICY] all files must contain copyright headers, for more information see"
    puts
    puts "          https://arvados.org/projects/arvados/wiki/Coding_Standards#Copyright-headers"
    puts
    puts "Enforcing copyright headers: FAIL"
    exit 1

  end
  puts "Enforcing copyright headers: PASS"
end

load_licenseignore
check_copyright_headers
