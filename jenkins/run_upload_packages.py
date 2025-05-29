#!/usr/bin/env python3

# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0

import argparse
import errno
import functools
import glob
import locale
import logging
import os
import re
import shlex
import shutil
import subprocess
import sys
import time

def run_and_grep(cmd, read_output, *regexps,
                 encoding=locale.getpreferredencoding(), **popen_kwargs):
    """Run a subprocess and capture output lines matching regexps.

    Arguments:
    * cmd: The command to run, as a list or string, as for subprocess.Popen.
    * read_output: 'stdout' or 'stderr', the name of the output stream to read.
    Remaining arguments are regexps to match output, as strings or compiled
    regexp objects.  Output lines matching any regexp will be captured.

    Keyword arguments:
    * encoding: The encoding used to decode the subprocess output.
    Remaining keyword arguments are passed directly to subprocess.Popen.

    Returns 2-tuple (subprocess returncode, list of matched output lines).
    """
    regexps = [regexp if hasattr(regexp, 'search') else re.compile(regexp)
               for regexp in regexps]
    popen_kwargs[read_output] = subprocess.PIPE
    proc = subprocess.Popen(cmd, **popen_kwargs)
    with open(getattr(proc, read_output).fileno(), encoding=encoding) as output:
        matched_lines = []
        for line in output:
            if any(regexp.search(line) for regexp in regexps):
                matched_lines.append(line)
            if read_output == 'stderr':
                print(line, file=sys.stderr, end='')
    return proc.wait(), matched_lines


class TimestampFile:
    def __init__(self, path):
        self.path = path
        # Make sure the dirname for `path` exists
        p = os.path.dirname(path)
        try:
            os.makedirs(p)
        except OSError as exc:
            if exc.errno == errno.EEXIST and os.path.isdir(p):
                pass
            else:
                raise
        self.start_time = time.time()

    def last_upload(self):
        try:
            return os.path.getmtime(self.path)
        except EnvironmentError:
            return -1

    def update(self):
        try:
            os.close(os.open(self.path, os.O_CREAT | os.O_APPEND))
            os.utime(self.path, (time.time(), self.start_time))
        except:
            # when the packages directory is created/populated by a build in a
            # docker container, as root, the script that runs the upload
            # doesn't always have permission to touch a timestamp file there.
            # In production, we build/upload from ephemeral machines, which
            # means that the timestamp mechanism is not used. We print a
            # warning and move on without erroring out.
            print("Warning: unable to update timestamp file",self.path,"permission problem?")
            pass

class PackageSuite:
    NEED_SSH = False

    def __init__(self, glob_root, rel_globs):
        logger_part = getattr(self, 'LOGGER_PART', os.path.basename(glob_root))
        self.logger = logging.getLogger('arvados-dev.upload.' + logger_part)
        self.globs = [os.path.join(glob_root, rel_glob)
                      for rel_glob in rel_globs]

    def files_to_upload(self, since_timestamp):
        for abs_glob in self.globs:
            for path in glob.glob(abs_glob):
                if os.path.getmtime(path) >= since_timestamp:
                    yield path

    def upload_file(self, path):
        raise NotImplementedError("PackageSuite.upload_file")

    def upload_files(self, paths):
        for path in paths:
            self.logger.info("Uploading %s", path)
            self.upload_file(path)

    def post_uploads(self, paths):
        pass

    def update_packages(self, since_timestamp):
        upload_paths = list(self.files_to_upload(since_timestamp))
        if upload_paths:
            self.upload_files(upload_paths)
            self.post_uploads(upload_paths)


class PythonPackageSuite(PackageSuite):
    LOGGER_PART = 'python'

    def upload_file(self, path):
        subprocess.run([
            'twine', 'upload',
            '--disable-progress-bar',
            '--non-interactive',
            '--skip-existing',
            path,
        ], stdin=subprocess.DEVNULL, check=True)


class GemPackageSuite(PackageSuite):
    LOGGER_PART = 'gems'
    REUPLOAD_REGEXP = re.compile(r'^Repushing of gem versions is not allowed\.$')

    def upload_file(self, path):
        cmd = ['gem', 'push', path]
        push_returncode, repushed = run_and_grep(cmd, 'stdout', self.REUPLOAD_REGEXP)
        if (push_returncode != 0) and not repushed:
            raise subprocess.CalledProcessError(push_returncode, cmd)


class DistroPackageSuite(PackageSuite):
    NEED_SSH = True
    REMOTE_DEST_DIR = 'tmp'

    def __init__(self, glob_root, rel_globs, target, ssh_host, ssh_opts):
        super().__init__(glob_root, rel_globs)
        self.target = target
        self.ssh_host = ssh_host
        self.ssh_opts = ['-o' + opt for opt in ssh_opts]
        if not self.logger.isEnabledFor(logging.INFO):
            self.ssh_opts.append('-q')

    def _build_cmd(self, base_cmd, *args):
        cmd = [base_cmd]
        cmd.extend(self.ssh_opts)
        cmd.extend(args)
        return cmd

    def _paths_basenames(self, paths):
        return (os.path.basename(path) for path in paths)

    def _run_script(self, script, *args):
        # SSH will use a shell to run our bash command, so we have to
        # quote our arguments.
        # self.__class__.__name__ provides $0 for the script, which makes a
        # nicer message if there's an error.
        subprocess.check_call(self._build_cmd(
                'ssh', self.ssh_host, 'bash', '-ec', shlex.quote(script),
                self.__class__.__name__, *(shlex.quote(s) for s in args)))

    def upload_files(self, paths):
        dest_dir = os.path.join(self.REMOTE_DEST_DIR, self.target)
        mkdir = self._build_cmd('ssh', self.ssh_host, 'install', '-d', dest_dir)
        subprocess.check_call(mkdir)
        cmd = self._build_cmd('scp', *paths)
        cmd.append('{}:{}'.format(self.ssh_host, dest_dir))
        subprocess.check_call(cmd)


class DebianPackageSuite(DistroPackageSuite):
    APT_SCRIPT = """
set -e
cd "$1"; shift
DISTNAME=$1; shift
# We increase database open attempts to accommodate parallel upload jobs.
aptly() {
  command aptly -db-open-attempts=60 "$@"
}
for package in "$@"; do
  if aptly repo search "$DISTNAME" "${package%.deb}" >/dev/null; then
    echo "Not adding $package, it is already present in repo $DISTNAME"
    rm "$package"
  else
    aptly repo add -remove-files "$DISTNAME" "$package"
  fi
done
aptly publish update "$DISTNAME" filesystem:"${DISTNAME%-*}":
"""

    def __init__(self, glob_root, rel_globs, target, ssh_host, ssh_opts, repo):
        super().__init__(glob_root, rel_globs, target, ssh_host, ssh_opts)
        self.TARGET_DISTNAMES = {
            'debian10': 'buster-'+repo,
            'debian11': 'bullseye-'+repo,
            'debian12': 'bookworm-'+repo,
            'ubuntu1804': 'bionic-'+repo,
            'ubuntu2004': 'focal-'+repo,
            'ubuntu2204': 'jammy-'+repo,
            'ubuntu2404': 'noble-'+repo,
            }

    def post_uploads(self, paths):
        self._run_script(self.APT_SCRIPT, self.REMOTE_DEST_DIR + '/' + self.target,
                         self.TARGET_DISTNAMES[self.target],
                         *self._paths_basenames(paths))


class RedHatPackageSuite(DistroPackageSuite):
    CREATEREPO_SCRIPT = """
cd "$1"; shift
REPODIR=$1; shift
rpmsign --addsign "$@" </dev/null
mv "$@" "$REPODIR"
createrepo_c -c ~/.createrepo-cache --update "$REPODIR"
"""
    REPO_ROOT = '/var/www/rpm.arvados.org/'

    def __init__(self, glob_root, rel_globs, target, ssh_host, ssh_opts, repo):
        super().__init__(glob_root, rel_globs, target, ssh_host, ssh_opts)
        self.TARGET_REPODIRS = {
            'centos7': 'RHEL/7/%s/x86_64/' % repo,
            'rocky8': 'RHEL/8/%s/x86_64/' % repo,
        }

    def post_uploads(self, paths):
        repo_dir = os.path.join(self.REPO_ROOT,
                                self.TARGET_REPODIRS[self.target])
        self._run_script(self.CREATEREPO_SCRIPT, self.REMOTE_DEST_DIR + '/' + self.target,
                         repo_dir, *self._paths_basenames(paths))


def _define_suite(suite_class, *rel_globs, **kwargs):
    return functools.partial(suite_class, rel_globs=rel_globs, **kwargs)

PACKAGE_SUITES = {
    'python': _define_suite(PythonPackageSuite,
                            'sdk/cwl/dist/*.tar.gz',
                            'sdk/cwl/dist/*.whl',
                            'sdk/python/dist/*.tar.gz',
                            'sdk/python/dist/*.whl',
                            'services/fuse/dist/*.tar.gz',
                            'services/fuse/dist/*.whl',
                            'tools/crunchstat-summary/dist/*.tar.gz',
                            'tools/crunchstat-summary/dist/*.whl',
                            'tools/user-activity/dist/*.tar.gz',
                            'tools/user-activity/dist/*.whl',
                            'tools/cluster-activity/dist/*.tar.gz',
                            'tools/cluster-activity/dist/*.whl',
                        ),
    'gems': _define_suite(GemPackageSuite,
                          'sdk/ruby-google-api-client/*.gem',
                          'sdk/ruby/*.gem',
                          'sdk/cli/*.gem',
                          'services/login-sync/*.gem',
                      ),
    }

def parse_arguments(arguments):
    parser = argparse.ArgumentParser(
        description="Upload Arvados packages to various repositories")
    parser.add_argument(
        '--workspace', '-W', default=os.environ.get('WORKSPACE'),
        help="Arvados source directory with built packages to upload")
    parser.add_argument(
        '--ssh-host', '-H',
        help="Host specification for distribution repository server")
    parser.add_argument('-o', action='append', default=[], dest='ssh_opts',
                         metavar='OPTION', help="Pass option to `ssh -o`")
    parser.add_argument('--verbose', '-v', action='count', default=0,
                        help="Log more information and subcommand output")
    parser.add_argument(
        '--repo', choices=['dev', 'testing'],
        help="Whether to upload to dev (nightly) or testing (release candidate) repository")

    parser.add_argument(
        'targets', nargs='*', default=['all'], metavar='target',
        help="Upload packages to these targets (default all)\nAvailable targets: " +
        ', '.join(sorted(PACKAGE_SUITES.keys())))
    args = parser.parse_args(arguments)
    if 'all' in args.targets:
        args.targets = list(PACKAGE_SUITES.keys())

    if args.workspace is None:
        parser.error("workspace not set from command line or environment")

    for target in [
            'debian10', 'debian11', 'debian12',
            'ubuntu1804', 'ubuntu2004', 'ubuntu2204', 'ubuntu2404',
    ]:
        PACKAGE_SUITES[target] = _define_suite(
            DebianPackageSuite, os.path.join('packages', target, '*.deb'),
            target=target, repo=args.repo)
    for target in ['centos7', 'rocky8']:
        PACKAGE_SUITES[target] = _define_suite(
            RedHatPackageSuite, os.path.join('packages', target, '*.rpm'),
            target=target, repo=args.repo)

    for target in args.targets:
        try:
            suite_class = PACKAGE_SUITES[target].func
        except KeyError:
            parser.error("unrecognized target {!r}".format(target))
        if suite_class.NEED_SSH and (args.ssh_host is None):
            parser.error(
                "--ssh-host must be specified to upload distribution packages")
    return args

def setup_logger(stream_dest, args):
    log_handler = logging.StreamHandler(stream_dest)
    log_handler.setFormatter(logging.Formatter(
            '%(asctime)s %(name)s[%(process)d] %(levelname)s: %(message)s',
            '%Y-%m-%d %H:%M:%S'))
    logger = logging.getLogger('arvados-dev.upload')
    logger.addHandler(log_handler)
    logger.setLevel(max(1, logging.WARNING - (10 * args.verbose)))

def build_suite_and_upload(target, since_timestamp, args):
    suite_def = PACKAGE_SUITES[target]
    kwargs = {}
    if suite_def.func.NEED_SSH:
        kwargs.update(ssh_host=args.ssh_host, ssh_opts=args.ssh_opts)
    suite = suite_def(args.workspace, **kwargs)
    suite.update_packages(since_timestamp)

def main(arguments, stdout=sys.stdout, stderr=sys.stderr):
    args = parse_arguments(arguments)
    setup_logger(stderr, args)

    for target in args.targets:
        ts_file = TimestampFile(os.path.join(args.workspace, 'packages',
                                             '.last_upload_%s' % target))
        last_upload_ts = ts_file.last_upload()
        build_suite_and_upload(target, last_upload_ts, args)
        ts_file.update()

if __name__ == '__main__':
    main(sys.argv[1:])
