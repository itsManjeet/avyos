#!/usr/bin/env python3

"""
string to string mapping table

License: MIT

Copyright (c) 2020 Simon Ser
Copyright 2022 Collabora, Ltd.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
"""

import sys

def escape_for_c(s):
    l = [c if c.isalnum() or c in ' .,' else '\\%03o' % ord(c) for c in s]
    return ''.join(l)

if len(sys.argv) != 4:
    print('usage: ' + sys.argv[0] + ' <infile> <outfile> <ident>', file=sys.stderr)
    sys.exit(1)

infile = sys.argv[1]
outfile = sys.argv[2]
ident = sys.argv[3]

records = {}

with open(infile, 'r') as f:
    for line in f:
        [pnpid, name] = line.split(maxsplit=1)

        if len(pnpid) != 3:
            print("Warning: skipping invalid PNP ID %s" % (repr(pnpid)), file=sys.stderr)
            continue

        records[pnpid] = escape_for_c(name.strip())

with open(outfile, 'w') as f:
    f.write(
f'''
#include <string.h>
#include <stdint.h>

const char *
{ident}(const char *key);

const char *
{ident}(const char *key)
{{
    size_t len = strlen(key);
    size_t i;
    uint32_t u = 0;

    if (len > 4)
        return NULL;

    for (i = 0; i < len; i++)
        u = (u << 8) | (uint8_t)key[i];

    switch (u) {{
''')
    for id in sorted(records.keys()):
        u = 0
        for c in id:
            u = (u << 8) | ord(c)
        f.write(f'    case {u}: return "{records[id]}";\n')
    f.write(
f'''
    default:
        return NULL;
    }}
}}
''')
