# Copyright 2025 Valve Corporation
# SPDX-License-Identifier: MIT

from isa import isa
import math
import sys
import binascii

def match_instr(raw):
    for key in isa.instrs:
        instr = isa.instrs[key]
        length = instr.length[instr.length_spec.bits.extract(raw) if instr.length_spec is not None else 0]
        r = raw & ((1 << (length * 8)) - 1)
        if (r & instr.exact.mask) == instr.exact.value:
            return length, instr, r

    return 2, None, raw

def map_value(v, kind, length):
    if kind in isa.enums:
        val = isa.enums[kind].values.get(v)
        return f'unk{v}' if val is None else val.display
    elif kind == 'signed':
        return str(-((1 << length) - v) if (v & (1 << (length - 1))) else v)
    else:
        return str(v)

def map_mod(v, mod):
    if mod.kind in isa.enums:
        return map_value(v, mod.kind, 0)
    else:
        assert(mod.bits.size() == 1)
        return mod.display if v != 0 else None

def decode_float_immediate(n):
	sign = -1.0 if n & 0x80 else 1.0
	e = (n & 0x70) >> 4
	f = n & 0xF
	if e == 0:
		return sign * f / 64.0
	else:
		return sign * float(0x10 | f) * (2.0 ** (e - 7))

def operand(desc, length):
    count = desc.get('count', 1)
    if count == 0:
        count = 4
    if 'mask' in desc:
        count = desc['mask'].bit_count()
    if 'zx' in desc:
        desc['sx'] = desc['zx'] ^ 1

    size = 64 if desc.get('size64', 0) != 0 else 32 if desc.get('size32', 0) != 0 else 16
    base = desc.get('base')
    kind = desc.get('kind')
    cache = desc.get('cache', False)
    discard = desc.get('discard', False)

    hint = desc.get('hint')
    if hint is not None:
        if hint == 3:
            discard = True
        elif hint == 2:
            cache = True
        else:
            assert(hint == 1)

    assert(not (cache and discard))
    hint = '^' if discard else '$' if cache else ''
    suffixes = ''.join(['.' + x for x in ['abs', 'neg', 'sx'] if desc.get(x, 0) != 0])

    if kind == None:
        return None
    elif kind in ['cf', 'ts', 'ss']:
        assert(count == 1)
        return f'{kind}{base}' + suffixes
    elif kind == 'fimm':
        return str(decode_float_immediate(base)) + suffixes
    elif kind in ['imm', 'signed']:
        return map_value(base, kind, length) + suffixes

    def m(offs):
        assert(size == 16 or size == 32 or size == 64)
        reg = base + (offs * (size // 16))
        whole = reg >> 1
        part = reg & 1
        prefix = kind[0]

        if size == 16:
            part_str = 'h' if part != 0 else 'l'
            return f'{prefix}{whole}{part_str}'
        else:
            assert(part == 0)
            return f'{"d" if size == 64 else ""}{prefix}{whole}'

    return hint + '_'.join([m(x) for x in range(count)]) + suffixes

def map_op(v, kind, raw, ins, length):
    for c in isa.encodings[kind].cases:
        if (v & c.exact.mask) == c.exact.value:
            desc = {}
            if c.kind is not None:
                length = c.kind.bits.size()
                desc['kind'] = c.kind.name
                desc['base'] = c.kind.bits.extract(v) << c.kind.shift
                if c.kind.kind == 'signed':
                    desc['kind'] = 'signed'

            for mod in c.modifiers:
                desc[mod.name] = mod.bits.extract(v)

            for prop in c.properties:
                desc[prop] = int(c.properties[prop])

            return operand(desc, length)

def disasm(raw_in):
    length, instr, raw = match_instr(raw_in)
    if instr is None:
        return length, "<unknown instr>"

    imms = [map_value(imm.bits.extract(raw), imm.kind, imm.bits.size()) for imm in instr.immediates]
    mods = [map_mod(mod.bits.extract(raw), mod) for mod in instr.modifiers if mod.name != 'length']
    dests = [map_op(d.bits.extract(raw), d.kind, raw, instr, d.bits.size()) for d in instr.dests]
    srcs = [map_op(s.bits.extract(raw), s.kind, raw, instr, s.bits.size()) for s in instr.sources]

    dests = ['_' if x is None else str(x) for x in dests]
    srcs = [str(x) for x in srcs if x is not None]
    imms = [str(x) for x in imms if x is not None]
    mods = [str(x) for x in mods if x is not None]
    end = ', '.join(dests + srcs + imms + mods)
    if end != "":
        end = " " + end

    asm = f'{instr.display:<8}{end}'.strip()

    unexpected = raw & ~instr.mask()
    unexpected = [i for i in range(length * 8) if unexpected & (1 << i)]
    if len(unexpected) != 0:
        print(f"Unexpected set bits: {unexpected}")
        length = -length

    return length, asm

if __name__ == '__main__':
    prog = open(sys.argv[1], 'rb').read()
    print("")
    i = 0
    while i < len(prog):
        lst = prog[i:i + 16]
        raw = sum([int(x) << (8 * i) for i, x in enumerate(lst)])
        length, asm = disasm(raw)
        lst = lst[0:length]
        print(f'{hex(i)[2:]:>4}: {str(binascii.hexlify(lst).decode()):<20} {asm}')
        if asm.strip() == 'trap':
            break
        i += length
    print("")
