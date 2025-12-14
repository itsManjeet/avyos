#!/usr/bin/env python3

# Copyright © 2024 Valve Corporation
# SPDX-License-Identifier: MIT

import argparse
import collections
import subprocess
import os
import re
import sys
import tempfile
import textwrap
from pathlib import Path

def trim_blank_lines(string, trailing):
    lines = string.split('\n')
    string = ''
    empty_line = True
    for i in range(len(lines)):
        line_index = len(lines) - 1 - i if trailing else i
        if empty_line and lines[line_index].strip() == '':
            continue

        write_newline = not empty_line if trailing else line_index < len(lines) - 1
        newline = '\n' if write_newline else ''
        if trailing:
            string = lines[line_index] + newline + string
        else:
            string += lines[line_index] + newline

        empty_line = False

    return string

class TestFileChange:
    def __init__(self, expected, result):
        self.expected = expected

        # Apply the indentation of the expectation to the result
        indentation = 1000
        for expected_line in expected.split('\n'):
            if match := re.match(r'^(\s*)\S', expected_line):
                line_indentation = len(match.group(1))
                if indentation > line_indentation:
                    indentation = line_indentation

        self.result = ''
        result = result.split('\n')
        for i in range(len(result)):
            result_line = result[i]
            indentation_str = '' if result_line.strip() == '' else ' '*indentation
            self.result += indentation_str + result_line + ('\n' if i < len(result) - 1 else '')

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--build-dir', '-B', required=False)
    parser.add_argument('--test-filter', '-f', required=False)
    parser.add_argument('--update-all', '-u', action='store_true')
    args = parser.parse_args()

    bin_path = 'src/compiler/nir/nir_tests'
    if args.build_dir:
        bin_path = args.build_dir + '/' + bin_path

    if not os.path.isfile(bin_path):
        print(f'{bin_path} \033[91m does not exist!\033[0m')
        exit(1)

    build_args = ['meson', 'compile']
    if args.build_dir:
        build_args.append(f'-C{args.build_dir}')
    subprocess.run(build_args)

    test_args = [bin_path]
    if args.test_filter:
        test_args.append(f'--gtest_filter={args.test_filter}')

    env = os.environ.copy()
    if args.update_all:
        env['NIR_TEST_DUMP_SHADERS'] = 'true'

    output = subprocess.run(test_args, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, universal_newlines=True, env=env)

    expected_pattern = re.compile(r'BEGIN EXPECTED\(([\d\w\W/.-_]+)\)')

    expectations = collections.defaultdict(list)

    current_file = None
    current_result = None
    current_expected = None
    inside_result = False
    inside_expected = False

    # Parse the output of the test binary and gather the changed shaders.
    for output_line in output.stdout.split('\n'):
        if output_line.startswith('BEGIN RESULT'):
            inside_result = True
            current_result = ''
            continue

        if output_line.startswith('BEGIN EXPECTED'):
            match = expected_pattern.match(output_line)
            current_file = match.group(1).removeprefix('../')
            inside_expected = True
            current_expected = ''
            continue

        if output_line.startswith('END'):
            if current_result is not None and current_expected is not None:
                # remove trailing and leading blank lines
                current_result = trim_blank_lines(current_result, True)
                current_result = trim_blank_lines(current_result, False)
                current_expected = trim_blank_lines(current_expected, True)
                current_expected = trim_blank_lines(current_expected, False)

                expectations[current_file].append(TestFileChange(current_expected, current_result))

                current_result = None
                current_expected = None

            inside_result = False
            inside_expected = False
            continue

        if inside_result:
            current_result += output_line + '\n'

        if inside_expected:
            current_expected += output_line + '\n'

    patches = []

    # Generate patches for the changed shaders.
    for file in expectations:
        changes = expectations[file]

        updated_test_file = ''

        with open(file) as test_file:
            updated_test_file = str(test_file.read())

            for change in changes:
                updated_test_file = updated_test_file.replace(change.expected, change.result)

                # change.expected == change.result can be the case when using NIR_TEST_DUMP_SHADERS.
                if change.expected in updated_test_file and change.expected != change.result:
                    print(f'Duplicate test case in {file}!')
                    exit(1)

        with tempfile.NamedTemporaryFile(delete_on_close=False) as tmp:
            tmp.write(bytes(updated_test_file, encoding="utf-8"))
            tmp.close()

            diff = subprocess.run(
                ['git', 'diff', '--no-index', file, tmp.name],
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
                universal_newlines=True,
            )
            patch = diff.stdout.replace(tmp.name, '/' + file)

            print(patch)

            patches.append(patch)

    if len(patches) != 0:
        sys.stdout.write('\033[96mApply the changes listed above?\033[0m [Y/n]')
        response = None
        try:
            response = input()
        except KeyboardInterrupt:
            print()
            sys.exit(1)

        if response in ['', 'y', 'Y']:
            for patch in patches:
                apply = subprocess.Popen(
                    ['git', 'apply', '--allow-empty'],
                    stdin=subprocess.PIPE,
                )
                apply.communicate(input=bytes(patch, encoding="utf-8"))
