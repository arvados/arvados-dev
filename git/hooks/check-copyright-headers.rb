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
  $licenseignore = `git show #{$newrev}:.licenseignore 2>/dev/null`.gsub(/\./,'\\.').gsub(/\*/,'.*').gsub(/\?/,'.').split("\n")
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
    all_objects = []
    commits = []
  elsif ($oldrev[0,6] ==  '000000')
    if $refname != 'refs/heads/main'
      # A new branch was pushed. Check all new commits in this branch.
      puts "git rev-list --objects main..#{$newrev} | git cat-file --batch-check='%(objecttype) %(objectname) %(objectsize) %(rest)'| sed -n 's/^blob //p'"
      blob_objects  = `git rev-list --objects main..#{$newrev} | git cat-file --follow-symlinks --batch-check='%(objecttype) %(objectname) %(objectsize) %(rest)'| sed -n 's/^blob //p'`.split("\n")
      commit_objects  = `git rev-list --objects main..#{$newrev} | git cat-file --batch-check='%(objecttype) %(objectname) %(objectsize) %(rest)'| sed -n 's/^commit //p'`.split("\n")
      all_objects = blob_objects + commit_objects
      commits = `git rev-list main..#{$newrev}`.split("\n")
    else
      # When does this happen?
      puts "UNEXPECTED ERROR"
      exit 1
    end
  else
    blob_objects = `git rev-list --objects #{$oldrev}..#{$newrev} --not --branches='*' | git cat-file --follow-symlinks --batch-check='%(objecttype) %(objectname) %(objectsize) %(rest)'| sed -n 's/^blob //p'`.split("\n")
    commit_objects  = `git rev-list --objects #{$oldrev}..#{$newrev} | git cat-file --batch-check='%(objecttype) %(objectname) %(objectsize) %(rest)'| sed -n 's/^commit //p'`.split("\n")
    all_objects = blob_objects + commit_objects
    commits = `git rev-list #{$oldrev}..#{$newrev}`.split("\n")
  end

  broken = false

  all_objects.each do |rev|
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
        # Only consider files, not symlinks (mode 120000)
        if commit =~ /^new file mode (100644|10755)\nindex 000000/
          headerCount = 0
          lineCount = 0
          header = ""
          previousLine = ""
          commit.each_line do |line|
            if ((headerCount == 0) and (line =~ /Copyright.*All rights reserved./))
              header = previousLine
              header += line
              headerCount = 1
            elsif ((headerCount > 0) and (headerCount < 3))
              header += line
              headerCount += 1
            elsif (headerCount == 3)
              break
            end
            previousLine = line
            lineCount += 1
            if lineCount > 50
              break
            end
          end
          broken = check_file(filename, header, broken)
        end
      end
    else
      # git object of type 'blob'
      filename = tmp[2]
      # test if this is a symlink.
      # Get the tree for each revision we are considering, find the blob hash in there, check the mode at start of line.
      # Stop looking at revisions once we have a match.
      symlink = false
      commits.each do |r|
        tree = `git cat-file -p #{r}^{tree}`
        if tree =~ /#{tmp[0]}/
           if tree =~ /^120000.blob.#{tmp[0]}/
            symlink = true
          end
          break
        end
      end
      if symlink == false
        header = `git show #{tmp[0]} | head -n20 | egrep -A3 -B1 'Copyright.*All rights reserved.'`
        broken = check_file(filename, header, broken)
      else
        #puts "#{filename} is a symbolic link, skipping"
      end
    end
  end

  if broken
    puts
    puts "[POLICY] all files must contain copyright headers, for more information see"
    puts
    puts "      https://dev.arvados.org/projects/arvados/wiki/Coding_Standards#Copyright-headers"
    puts
    puts "Enforcing copyright headers: FAIL"
    exit 1

  end
  puts "Enforcing copyright headers: PASS"
end

load_licenseignore
check_copyright_headers
