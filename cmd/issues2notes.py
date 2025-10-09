#!/usr/bin/env python3
# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0
"""issues2notes.py – Build a Markdown outline from Redmine issues

How to use this script:

1. On Redmine, search issues that match what you want to report, like all
   issues for a release. Usually you'll do this after associating all issues
   with a release, like using `art redmine issues find-and-associate`.

2. At the bottom of the page, use the CSV link to download that issues
   list. When prompted, choose to export all columns and use UTF-8 encoding.

3. Run this script on the CSV you download:

        ./issues2notes.py [-O ISSUES.md] ISSUES.csv

   You can use the `-k` option to select columns that will group issues
   together in the outline.

4. Edit the outline into full release notes. The ticket links are suitable to
   include: private tickets will not be linked. We typically don't include
   ticket subjects directly.
"""

import argparse
import csv
import dataclasses
import itertools
import os
import re
import sys
import typing as t
import urllib.parse

from collections import abc
from pathlib import PurePosixPath

RedmineIssue = t.TypedDict('RedmineIssue', {
    # Columns in the Redmine issues CSV export
    '#': str,
    'Project': str,
    'Tracker': str,
    'Parent task': str,
    'Parent task subject': str,
    'Status': str,
    'Priority': str,
    'Subject': str,
    'Author': str,
    'Assigned To': str,
    'Updated': str,
    'Category': str,
    'Target version': str,
    'Start date': str,
    'Due date': str,
    'Created': str,
    'Closed': str,
    'Last updated by': str,
    'Related issues': str,
    'Files': str,
    'Private': str,
    'Story points': str,
    'Velocity based estimate': str,
    'Position': str,
    'Remaining (hours)': str,
    'Release': str,
    'Backlog type': str,
    # Synthesized by the Output class for use in format strings
    'IssueNumber': str,
    'URL': str,
}, total=False)


@dataclasses.dataclass
class Output:
    redmine_url: urllib.parse.ParseResult
    section_keys: abc.Sequence[str]
    private_key: str = 'Project'
    private_re: t.Pattern | str = re.compile(
        r'\b ( ops | private ) \b',
        re.IGNORECASE | re.VERBOSE,
    )
    _last_section: t.Sequence[str] = ()

    PUBLIC_FMT = "[#{IssueNumber}]({URL}) [{Subject}]"
    PRIVATE_FMT = "#{IssueNumber} [PRIVATE: [{Subject}]({URL})]"

    def _get_section(self, issue: RedmineIssue) -> t.Sequence[str]:
        return tuple(issue[key] for key in self.section_keys)

    def _write_section(self, issue: RedmineIssue, f: t.TextIO) -> None:
        section = self._get_section(issue)
        same = True
        for key, last_val, this_val, level in zip(
                self.section_keys,
                self._last_section or itertools.repeat(None),
                section,
                itertools.chain(range(2, 6), itertools.repeat(6)),
        ):
            if not (same := same and last_val == this_val):
                print('#' * level, this_val or f"(No {key})", end="\n\n", file=f)
        self._last_section = section

    def _write_issue(self, issue: RedmineIssue, f: t.TextIO) -> None:
        if re.search(self.private_re, issue[self.private_key]):
            issue_fmt = self.PRIVATE_FMT
        else:
            issue_fmt = self.PUBLIC_FMT
        print(issue_fmt.format_map(issue), end="\n\n", file=f)

    def write(self, issues: abc.Iterable[RedmineIssue], f: t.TextIO) -> None:
        for issue in sorted(issues, key=self._get_section):
            issue['IssueNumber'] = issue['#']
            url_path = PurePosixPath(self.redmine_url.path, 'issues', issue['#'])
            url = self.redmine_url._replace(path=str(url_path))
            issue['URL'] = urllib.parse.urlunparse(url)
            self._write_section(issue, f)
            self._write_issue(issue, f)


class ArgumentParser(argparse.ArgumentParser):
    class HowtoAction(argparse.Action):
        def __call__(self, parser, namespace, values, option_string=None):
            parser.exit(message=__doc__)


    def __init__(self):
        super().__init__(
            usage="%(prog)s [-O ISSUES.md] [other options ...] ISSUES.csv",
        )
        self.add_argument(
            '--howto',
            action=self.HowtoAction,
            nargs=0,
            help="Print instructions for the process to use this script",
        )
        self.add_argument(
            '--output-file', '-O',
            metavar='PATH',
            type=argparse.FileType('w'),
            default=sys.stdout,
            help="Path to write output Markdown. Default stdout.",
        )
        self.add_argument(
            '--section-key', '-k',
            metavar='KEY',
            action='append',
            dest='section_keys',
            help="""Organize the release notes into sections based on the named
column in the CSV. Can be specified multiple times.
""")
        self.add_argument(
            '--redmine-endpoint',
            metavar='URL',
            type=urllib.parse.urlparse,
            default=urllib.parse.urlparse(
                os.environ.get('REDMINE_ENDPOINT') or 'https://dev.arvados.org'
            ),
            help="Base URL of the Redmine instance. Uses $REDMINE_ENDPOINT if set.",
        )
        self.add_argument(
            'issues_csv',
            # This encoding properly handles the BOM in Redmine's download
            type=argparse.FileType('r', encoding='utf-8-sig'),
            help="CSV file exported from Redmine issues",
        )

    def parse_args(self, args=None, namespace=None):
        retargs = super().parse_args(args, namespace)
        if not retargs.section_keys:
            retargs.section_keys = ['Assigned To', 'Category']
        elif bad_keys := [
                repr(key)
                for key in retargs.section_keys
                if key not in RedmineIssue.__optional_keys__
        ]:
            self.error(f"unknown section key {', '.join(bad_keys)}")
        return retargs


def main(args: argparse.Namespace):
    output = Output(args.redmine_endpoint, args.section_keys)
    with args.output_file, args.issues_csv:
        issues = t.cast(t.Iterable[RedmineIssue], csv.DictReader(args.issues_csv))
        output.write(issues, args.output_file)
    return os.EX_OK


if __name__ == '__main__':
    try:
        args = ArgumentParser().parse_args()
        returncode = main(args)
    except OSError as err:
        if err.filename:
            filename = f' on `{err.filename}`'
        else:
            filename = ''
        print(f"I/O error{filename}: {err.strerror}", file=sys.stderr)
        returncode = os.EX_IOERR
    except Exception as err:
        print(f"internal {err.__class__.__name__}: {err}", file=sys.stderr)
        returncode = os.EX_SOFTWARE
    exit(returncode)
