#!/usr/bin/env python3

# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0

import argparse
import functools
import glob
import locale
import logging
import os
import pipes
import re
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
        self.start_time = time.time()

    def last_upload(self):
        try:
            return os.path.getmtime(self.path)
        except EnvironmentError:
            return -1

    def update(self):
        os.close(os.open(self.path, os.O_CREAT | os.O_APPEND))
        os.utime(self.path, (time.time(), self.start_time))


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
    REUPLOAD_REGEXPS = [
        re.compile(
            r'^error: Upload failed \(400\): A file named "[^"]+" already exists\b'),
        re.compile(
            r'^error: Upload failed \(400\): File already exists\b'),
        re.compile(
            r'^error: Upload failed \(400\): Only one sdist may be uploaded per release\b'),
    ]

    def __init__(self, glob_root, rel_globs):
        super().__init__(glob_root, rel_globs)
        self.seen_packages = set()

    def upload_file(self, path):
        src_dir = os.path.dirname(os.path.dirname(path))
        if src_dir in self.seen_packages:
            return
        self.seen_packages.add(src_dir)
        # NOTE: If we ever start uploading Python 3 packages, we'll need to
        # figure out some way to adapt cmd to match.  It might be easiest
        # to give all our setup.py files the executable bit, and run that
        # directly.
        # We also must run `sdist` before `upload`: `upload` uploads any
        # distributions previously generated in the command.  It doesn't
        # know how to upload distributions already on disk.  We write the
        # result to a dedicated directory to avoid interfering with our
        # timestamp tracking.
        cmd = ['python2.7', 'setup.py']
        if not self.logger.isEnabledFor(logging.INFO):
            cmd.append('--quiet')
        cmd.extend(['sdist', '--dist-dir', '.upload_dist', 'upload'])
        upload_returncode, repushed = run_and_grep(
            cmd, 'stderr', *self.REUPLOAD_REGEXPS, cwd=src_dir)
        if (upload_returncode != 0) and not repushed:
            raise subprocess.CalledProcessError(upload_returncode, cmd)
        shutil.rmtree(os.path.join(src_dir, '.upload_dist'))


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
                'ssh', self.ssh_host, 'bash', '-ec', pipes.quote(script),
                self.__class__.__name__, *(pipes.quote(s) for s in args)))

    def upload_files(self, paths):
        dest_dir = os.path.join(self.REMOTE_DEST_DIR, self.target)
        mkdir = self._build_cmd('ssh', self.ssh_host, 'install', '-d', dest_dir)
        subprocess.check_call(mkdir)
        cmd = self._build_cmd('scp', *paths)
        cmd.append('{}:{}'.format(self.ssh_host, dest_dir))
        subprocess.check_call(cmd)


class DebianPackageSuite(DistroPackageSuite):
    FREIGHT_SCRIPT = """
cd "$1"; shift
DISTNAME=$1; shift
freight add "$@" "apt/$DISTNAME"
freight cache "apt/$DISTNAME"
rm "$@"
"""
    TARGET_DISTNAMES = {
        'debian8': 'jessie-dev',
        'debian9': 'stretch-dev',
        'ubuntu1204': 'precise-dev',
        'ubuntu1404': 'trusty-dev',
        'ubuntu1604': 'xenial-dev',
        }

    def post_uploads(self, paths):
        self._run_script(self.FREIGHT_SCRIPT, self.REMOTE_DEST_DIR + '/' + self.target,
                         self.TARGET_DISTNAMES[self.target],
                         *self._paths_basenames(paths))


class RedHatPackageSuite(DistroPackageSuite):
    CREATEREPO_SCRIPT = """
cd "$1"; shift
REPODIR=$1; shift
rpmsign --addsign "$@" </dev/null
mv "$@" "$REPODIR"
createrepo "$REPODIR"
"""
    REPO_ROOT = '/var/www/rpm.arvados.org/'
    TARGET_REPODIRS = {
        'centos7': 'CentOS/7/dev/x86_64/',
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
                            'sdk/pam/dist/*.tar.gz',
                            'sdk/python/dist/*.tar.gz',
                            'sdk/cwl/dist/*.tar.gz',
                            'services/nodemanager/dist/*.tar.gz',
                            'services/fuse/dist/*.tar.gz',
                        ),
    'gems': _define_suite(GemPackageSuite,
                          'sdk/ruby/*.gem',
                          'sdk/cli/*.gem',
                          'services/login-sync/*.gem',
                      ),
    }
for target in ['debian8', 'debian9', 'ubuntu1204', 'ubuntu1404', 'ubuntu1604']:
    PACKAGE_SUITES[target] = _define_suite(
        DebianPackageSuite, os.path.join('packages', target, '*.deb'),
        target=target)
for target in ['centos7']:
    PACKAGE_SUITES[target] = _define_suite(
        RedHatPackageSuite, os.path.join('packages', target, '*.rpm'),
        target=target)

def parse_arguments(arguments):
    parser = argparse.ArgumentParser(
        prog="run_upload_packages.py",
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
        'targets', nargs='*', default=['all'], metavar='target',
        help="Upload packages to these targets (default all)\nAvailable targets: " +
        ', '.join(sorted(PACKAGE_SUITES.keys())))
    args = parser.parse_args(arguments)
    if 'all' in args.targets:
        args.targets = list(PACKAGE_SUITES.keys())

    if args.workspace is None:
        parser.error("workspace not set from command line or environment")
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
