#! /usr/bin/env python3
# Copyright (C) 2025 fontconfig Authors
# SPDX-License-Identifier: HPND

import sys
import re
import argparse


def gen_header():
    return '''/* Copyright (C) 2025 fontconfig Authors */
/* SPDX-License-Identifier: HPND */

'''


def parse_list(lfile):
    with open(lfile) as f:
        lines = []
        for s in f:
            x = s.split()
            if not s.startswith('#') and len(x) > 0:
                lines.append(x)
    return sorted(lines, key=lambda a: a[0].lower())


def parse_fcobjh(bfile):
    with open(bfile) as f:
        s = f.read()
        s = re.sub(re.compile(r'//.*?$|/\*.*?\*/|\'(?:\\.|[^\\\'])*\'|"(?:\\.|[^\\"])*"',
                              re.DOTALL | re.MULTILINE),
                   lambda m: " " if m.group(0).startswith('/') else s,
                   s)
        l = [ss.strip().replace('FC_OBJECT (', '').replace(')','').split() for ss in s.splitlines()]
        ll = [x for x in l if x]
        # 0 is reserved for FC_INVALID_OBJECT
        ret = dict([('FC_' + ll[n][0].strip(','), n + 1) for n in range(0, len(ll))])
        rret = dict([(n + 1, 'FC_' + ll[n][0].strip(',')) for n in range(0, len(ll))])
        ret['FC_INVALID_OBJECT'] = 0
        rret[0] = 'FC_INVALID_OBJECT'
        return ret, rret


def gen_constsym(sym_list, max_sym):
    ret = ['typedef struct _FcConstIndex {',
           '    FcObject object;',
           '    int      idx_obj;',
           '    int      idx_variant;',
           '} FcConstIndex;',
           '',
           'typedef struct _FcConstSymbolMap {',
           '    const FcChar8 *name;',
           f'    FcConstIndex   values[{max_sym+1}];',
           '} FcConstSymbolMap;',
           '',
           'static const FcConstSymbolMap _FcBaseConstantSymbols[] = {']
    for k in sorted(sym_list.keys()):
        a = sym_list[k]
        ret.append('    {')
        ret.append(f'        (const FcChar8 *) "{k}",')
        ret.append('        {')
        for v in a:
            ret.append(f'            {{ {v[0]}_OBJECT, {v[1]}, {v[2]} }},')
        ret.append('            { FC_INVALID_OBJECT, 0, 0 },')
        ret.append('        },')
        ret.append('    },')
    ret.append('};')
    ret.append('')
    ret.append('#define NUM_FC_CONST_SYMBOLS (sizeof (_FcBaseConstantSymbols) / sizeof (_FcBaseConstantSymbols[0]))')
    ret.append('')
    return ret


def gen_baseconstobj_body(objs, enum_list):
    max_objs = 0
    ret = []
    for i, a in enumerate(objs):
        if not a:
            ret.append('    {{{ NULL, NULL, 0 }}}    /* ' + enum_list[i] + ' */,')
        else:
            ret.append('    {{')
            n = 0
            for v in a:
                ret.append('        { (const FcChar8 *) "' + v[0] + '", ' + enum_list[i] + ', ' + v[1] + '},')
                n += 1
            ret.append('        { NULL, NULL, 0 },')
            max_objs = max([n + 1, max_objs])
            ret.append('    }},')
    return ret, max_objs


def gen_decl_baseconstobj(max_objs):
    return ['typedef struct _fcConstObjects {',
            f'    FcConstant values[%d];' % max_objs,
            '} FcConstantObjects;',
            '',
            'static const FcConstantObjects _FcBaseConstantObjects[FC_MAX_BASE_OBJECT+1] = {'
            ]


def gen_baseconstobj(objs, enum_list):
    body, max_objs = gen_baseconstobj_body(objs, enum_list)
    decl = gen_decl_baseconstobj(max_objs)
    return decl + body + ['};', '', '#define NUM_FC_CONST_OBJS (sizeof (_FcBaseConstantObjects) / sizeof (_FcBaseConstantObjects[0]))', '']


def gen_body(lfile, bfile):
    const_list = parse_list(lfile)
    enum_list, reverse_enum_list = parse_fcobjh(bfile)
    const_list_by_objs = [None] * len(enum_list)
    sym_list_by_const = {}
    max_sym = 0
    for i, a in enumerate(const_list):
        idx = enum_list[a[1]]
        if not isinstance(const_list_by_objs[idx], list):
            const_list_by_objs[idx] = []
        const_list_by_objs[idx].append([a[0], a[2]])
        if a[0] not in sym_list_by_const:
            sym_list_by_const[a[0]] = []
        sym_list_by_const[a[0]].append([a[1], idx, len(const_list_by_objs[idx]) - 1])
        max_sym = max([len(sym_list_by_const[a[0]]), max_sym])
    sym = gen_constsym(sym_list_by_const, max_sym)
    objs = gen_baseconstobj(const_list_by_objs, reverse_enum_list)

    return '\n'.join(sym) + '\n'+ '\n'.join(objs)


def gen_test_body(lfile, bfile):
    ret = []
    const_list = parse_list(lfile)
    enum_list, reverse_enum_list = parse_fcobjh(bfile)
    ret.append('#include <stdio.h>')
    ret.append('#include "fontconfig/fontconfig.h"')
    ret.append('')
    ret.append('int test (void) {')
    ret.append('    int ret = 0;')
    ret.append('    const FcConstant *c;')
    for i, a in enumerate(const_list):
        ret.append(f'    c = FcNameGetConstantFor ((const FcChar8 *)"{a[0]}", {a[1]});')
        ret.append(f'    if (!c || c->value != {a[2]}) {{')
        ret.append(f'        fprintf (stderr, "failed: (%s, %s)\\n", "{a[0]}", _FC_STRINGIFY({a[1]}));')
        ret.append('        ret++;')
        ret.append('    }')
    ret.append('    return ret;')
    ret.append('}')
    ret.append('')
    ret.append('int main(void) { return test(); }')
    ret.append('')
    return '\n'.join(ret)


def main():
    parser = argparse.ArgumentParser(
        description='fcconst.h generator',
        formatter_class=argparse.ArgumentDefaultsHelpFormatter,
    )
    parser.add_argument('-o', '--output',
                        type=argparse.FileType('w'),
                        default='-',
                        help='Output file')
    parser.add_argument('-t', '--test',
                        action='store_true',
                        help='Generate test case')
    parser.add_argument('list',
                        help='list file for constant names')
    parser.add_argument('header',
                        help='fcobjs.h file')

    args = parser.parse_args()
    with args.output:
        args.output.write(gen_header())
        if args.test:
            args.output.write(gen_test_body(args.list, args.header))
        else:
            args.output.write(gen_body(args.list, args.header))

    sys.exit(0)


if __name__ == '__main__':
    main()
