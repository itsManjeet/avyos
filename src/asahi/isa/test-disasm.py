# Copyright 2025 Valve Corporation
# SPDX-License-Identifier: MIT

import sys
import os
import textwrap
import binascii
from test.disasm import disasm

errors = False
new_cases = []

for case in open(sys.argv[1]).read().strip().split('\n'):
    first = case.split(' ')[0]
    ref = case[len(first):].strip()

    # Extract bytes
    lst = [int(x, 16) for x in textwrap.wrap(first, 2)]

    # Make sure we don't depend on zeroes in upper bits
    padded = lst + [0xCA, 0xFE, 0xBA, 0xBE] * 16
    raw = sum([x << (8 * i) for i, x in enumerate(padded)])

    error = False
    length, asm = disasm(raw)
    error |= (length != len(lst))

    actual = f'{str(binascii.hexlify(bytes(lst)).decode()):<20} {asm}\n'
    new_cases.append(actual)
    error |= (actual[:-1] != case)

    if error:
        print(case)
        print(actual)
    errors |= error

# If there were errors, optionally update expectations.
if len(sys.argv) >= 3:
    assert(sys.argv[2] in ['-u', '--update'])
    open(sys.argv[1], 'w').write(''.join(new_cases))

if errors:
    print('Fail.')
    sys.exit(1)
else:
    print('Pass.')
