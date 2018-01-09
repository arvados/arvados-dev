#!/usr/bin/env ruby

# This script can be installed as a git update hook.

# It can also be installed as a gitolite 'hooklet' in the
# hooks/common/update.secondary.d/ directory.

# NOTE: this script runs under the same assumptions as the 'update' hook, so
# the starting directory must be maintained and arguments must be passed on.

$refname = ARGV[0]
$oldrev  = ARGV[1]
$newrev  = ARGV[2]
$user    = ENV['USER']

puts "Enforcing DCO signoff: (#{$refname}) (#{$oldrev[0,6]}) (#{$newrev[0,6]})"

$regex = /\[ref: (\d+)\]/

$arvados_DCO = /Arvados-DCO-1.1-Signed-off-by:/

# enforced DCO signoff in commit message
def check_message_format
  if ($newrev[0,6] ==  '000000')
    # A branch is being deleted. Do not check old commits for DCO signoff!
    all_revs    = []
  elsif ($oldrev[0,6] ==  '000000')
    if $refname != 'refs/heads/master'
      # A new branch was pushed. Check all new commits in this branch.
      all_revs  = `git log --pretty=format:%H master..#{$newrev}`.split("\n")
    else
      # When does this happen?
      all_revs  = [$newrev]
    end
  else
    all_revs    = `git rev-list --first-parent #{$oldrev}..#{$newrev}`.split("\n")
  end

  broken = false

  all_revs.each do |rev|
    message = `git cat-file commit #{rev} | sed '1,/^$/d' | grep -E "Arvados-DCO-1.1-Signed-off-by: .+@.+\..+"`

    if ! $arvados_DCO.match(message)
      puts "\n[POLICY] Rejected commit: missing Arvados-DCO-1.1-Signed-off-by line"
      puts "\n******************************************************************\n"
      puts "\nOffending commit: #{rev}\n"
      puts "\nOffending commit message:\n\n"
      puts `git cat-file commit #{rev} | sed '1,/^$/d'`
      puts "\n******************************************************************\n"
      puts "\n\n"
      puts "\nFor more information, see\n"
      puts "\n  https://dev.arvados.org/projects/arvados/wiki/Developer_Certificate_Of_Origin\n"
      puts "\n\n"
      broken = true
    end
  end

  if broken
    exit 1
  end
end

check_message_format
