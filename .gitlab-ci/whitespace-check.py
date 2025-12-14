#!/usr/bin/env python3
# SPDX-License-Identifier: MIT

from pathlib import Path
from dataclasses import dataclass

import argparse
import itertools
import os
import sys


@dataclass
class WhitespaceError:
    message: str
    lineno: int
    nlines: int = 1
    column: None | int = None
    ncolumns: int = 1


def test_duplicate_empty_lines(lines: list[str]) -> list[WhitespaceError]:
    errors = []
    for idx, (l1, l2) in enumerate(itertools.pairwise(lines)):
        if not l1 and not l2:
            errors.append(WhitespaceError("Duplicated empty lines", idx, nlines=2))
    return errors


def test_tab_after_space(lines: list[str]) -> list[WhitespaceError]:
    errors = []
    for idx, l in enumerate(lines):
        index = l.find(" \t")
        if index > -1:
            errors.append(
                WhitespaceError(
                    "Tab after space", idx, nlines=index, column=index, ncolumns=2
                )
            )
    return errors


def test_trailing_whitespace(lines: list[str]) -> list[WhitespaceError]:
    errors = []
    for idx, l in enumerate(lines):
        if l.rstrip() != l:
            errors.append(WhitespaceError("Trailing whitespace", idx))
    return errors


def test_empty_line_between_braces(lines: list[str]) -> list[WhitespaceError]:
    errors = []
    for idx in range(len(lines) - 3):
        l1 = lines[idx]
        l2 = lines[idx + 1]
        l3 = lines[idx + 2]
        if l1.strip() == "}" and l3.strip() == "}" and l2.strip() == "":
            errors.append(WhitespaceError("Empty line between closing braces", idx + 1))
    return errors


def main():
    parser = argparse.ArgumentParser(description="Whitespace checker script")
    parser.add_argument(
        "files",
        metavar="FILES",
        type=Path,
        nargs="+",
        help="The files to check",
    )

    args = parser.parse_args()

    have_errors: bool = False

    if os.isatty(sys.stderr.fileno()):
        red = "\x1b[0;31m"
        reset = "\x1b[0m"
    else:
        red = ""
        reset = ""

    for file in args.files:
        lines = [l.rstrip("\n") for l in file.open().readlines()]

        errors = []
        errors.extend(test_tab_after_space(lines))
        errors.extend(test_trailing_whitespace(lines))
        if any(file.name.endswith(suffix) for suffix in [".c", ".h"]):
            if not file.parts[0] == "include":
                errors.extend(test_duplicate_empty_lines(lines))
                errors.extend(test_empty_line_between_braces(lines))

        for e in errors:
            print(f"{red}ERROR: {e.message} in {file}:{reset}", file=sys.stderr)
            print(f"{'-' * 72}", file=sys.stderr)
            lineno = max(0, e.lineno - 5)
            for idx, l in enumerate(lines[lineno : lineno + 10]):
                if e.lineno <= lineno + idx < e.lineno + e.nlines:
                    prefix = "->"
                    hl = red
                    nohl = reset
                else:
                    prefix = "  "
                    hl = ""
                    nohl = ""
                print(f"{hl}{lineno + idx:3d}: {prefix} {l.rstrip()}{nohl}")

            print(f"{'-' * 72}", file=sys.stderr)

        if errors:
            have_errors = True

    if have_errors:
        sys.exit(1)


if __name__ == "__main__":
    main()
