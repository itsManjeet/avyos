# Copyright 2025 Valve Corporation
# SPDX-License-Identifier: MIT

from dataclasses import dataclass
from functools import reduce
import xml.etree.ElementTree as ET
import sys
import copy
import os

def ensure(cond, msg):
    if not cond:
        print(msg)
        sys.exit(1)

class BitRange:
    def __init__(self, start, end):
        assert(start <= end)
        self.start = start
        self.end = end

        if self.start >= 0:
            self.size = end - start + 1
            self.size_mask = (1 << self.size) - 1
            self.mask = self.size_mask << start

    def parse(spec: str):
        if ':' in spec:
            start, end = [int(x) for x in spec.split(':')]
            return BitRange(start, end)
        else:
            return BitRange(int(spec), int(spec))

    def specialize(self, length):
        return BitRange(self.start % length, self.end % length)

    def __repr__(self):
        if self.start == self.end:
            return f'{self.start}'
        else:
            return f'{self.start}:{self.end}'

class Bits:
    def __init__(self, spec = None):
        if spec is not None:
            self.ranges = [BitRange.parse(r) for r in spec.split(' ')]
        else:
            self.ranges = []

    def size(self) -> int:
        return sum([x.size for x in self.ranges])

    def mask(self) -> int:
        masks = [r.mask for r in self.ranges]
        union = 0
        for mask in masks:
            union |= mask

        # No overlap
        assert(sum(masks) == union)
        return union

    def extract(self, x):
        accum = 0
        n = 0

        for r in self.ranges:
            accum |= ((x >> r.start) & r.size_mask) << n
            n += r.size

        return accum

    def specialize(self, length):
        obj = copy.copy(self)
        obj.ranges = [r.specialize(length) for r in obj.ranges]
        return obj

    def __repr__(self):
        return ' '.join([str(r) for r in self.ranges])

def parse_bitstring(bits):
    value = 0

    # Reverse to deal with the convention
    bits = bits[::-1]

    for i, bit in enumerate(bits):
        assert(bit in ['0', '1'])
        if bit == '1':
            value |= (1 << i)

    return value

# Spread the bits of value out according to the 1s in mask.
# Similar to pdep in x86.
def deposit_bits(mask, value):
    accum = 0
    value_i = 0

    assert(mask < (1 << 128))
    for m in range(128):
        if mask & (1 << m):
            if (value & (1 << value_i)) != 0:
                accum |= (1 << m)

            value_i += 1

    assert((accum & ~mask) == 0)
    return accum

def substitute(text, substitutions):
    if text in substitutions:
        return substitutions[text]
    else:
        return text

class Exact:
    def __init__(self, el = None, subst = None):
        bits = 0
        if el is not None:
            self.mask = Bits(el.attrib['bit']).mask()
            if el.tag != 'zero':
                text = substitute(el.text, subst)
                bits = parse_bitstring(text)
                assert(len(text) == self.mask.bit_count())
        else:
            self.mask = 0

        self.value = deposit_bits(self.mask, bits)

    def union(self, other):
        self.value |= other.value
        self.mask |= other.mask

class EnumValue:
    def __init__(self, el):
        self.name = el.text
        self.label = el.attrib.get('label')
        self.default = bool(el.attrib.get('default', False))
        self.display = None if self.default else self.name

class Enum:
    def __init__(self, el):
        self.name = el.attrib['name']
        self.kind = el.attrib['kind']
        self.values = {}

        for child in el:
            self.values[int(child.attrib['value'])] = EnumValue(child)

class Immediate:
    def __init__(self, el, name):
        self.name = name
        self.kind = el.attrib.get('kind', 'int')
        self.bits = Bits(el.attrib['bit'])
        self.shift = int(el.attrib.get('shift', 0))

class Operand:
    def __init__(self, el, is_dest):
        self.kind = el.attrib['kind']
        self.bits = Bits(el.attrib['bit'])
        self.is_dest = is_dest

    def specialize(self, length):
        obj = copy.copy(self)
        obj.bits = obj.bits.specialize(length)
        return obj

class EncodingCase:
    def __init__(self, el):
        self.exact = Exact()
        self.modifiers = []
        self.kind = None
        self.properties = {}

        for ch in el:
            if ch.tag in ["exact", "zero"]:
                self.exact.union(Exact(ch, {}))
            elif ch.tag == "immediate":
                assert(self.kind is None)
                self.kind = Immediate(ch, ch.attrib['name'])
            elif ch.tag == "modifier":
                self.modifiers.append(Modifier(ch, ch.attrib['name']))
            elif ch.tag == "property":
                self.properties[ch.attrib['name']] = ch.text or '1'

    def union(self, other):
        self.exact.union(other.exact)
        self.modifiers = other.modifiers + self.modifiers
        self.properties.update(other.properties)

        assert(self.kind is None or other.kind is None)
        self.kind = self.kind or other.kind

class Encoding:
    def __init__(self, el):
        base = EncodingCase(el)
        cases = [EncodingCase(ch) for ch in el if ch.tag == 'case']

        if len(cases) == 0:
            self.cases = [base]
        else:
            self.cases = cases
            for c in self.cases:
                c.union(base)

class Modifier:
    def __init__(self, el, name):
        self.name = name
        self.display = el.get('display', name)
        self.kind = el.get('kind', 'int')
        self.bits = Bits(el.attrib['bit'])

class Instruction:
    def __init__(self, xml, name, display, substitutions, defs):
        length = [int(x) for x in xml.attrib['length'].split('/')]
        exact = Exact()
        mods = []
        dests = []
        srcs = []
        imms = []

        for el in xml:
            if el.tag in ["exact", "zero"]:
                exact.union(Exact(el, substitutions))
            elif el.tag == "immediate":
                imms.append(Immediate(el, el.attrib['name']))
            elif el.tag == "src":
                srcs.append(Operand(el, False))
            elif el.tag == "dest":
                dests.append(Operand(el, True))
            elif el.tag == "modifier":
                mods.append(Modifier(el, el.attrib['name']))
            elif el.tag in defs:
                d = defs[el.tag]
                if isinstance(d, Operand):
                    spec = d.specialize(length[-1] * 8)
                    if spec.is_dest:
                        dests.append(spec)
                    else:
                        srcs.append(spec)
                elif isinstance(d, Modifier):
                    mods.append(d)
                elif isinstance(d, Immediate):
                    imms.append(d)
                else:
                    assert(0)
            elif el.tag == 'ins':
                assert(xml.tag == 'group')
            else:
                print(name)
                assert(0)

        # Infer length bit if needed
        if len(length) > 1 and not any([x.name == 'length' for x in mods]):
            mods.append(defs['length'])

        self.length_spec = None
        self.length_bit = 0
        for mod in mods:
            if mod.name == 'length':
                self.length_spec = mod
                self.length_bit = mod.bits.ranges[0].start
        self.name = name
        self.display = display or name
        self.length = length
        self.exact = exact
        self.modifiers = mods
        self.dests = dests
        self.sources = srcs
        self.immediates = imms

    def mask(self):
        mask = self.exact.mask
        parts = self.modifiers + self.immediates + self.sources + self.dests
        bits = [x.bits.mask() for x in parts]
        return self.exact.mask | reduce(lambda x, y: x | y, bits, 0)

class ISA:
    def __init__(self, el):
        self.enums = {}
        self.instrs = {}
        self.encodings = {}
        defs = {}

        for child in el:
            name = child.get('name')

            if child.tag == "define":
                if 'src' in child.attrib:
                    defs[child.attrib['src']] = Operand(child, False)
                elif 'dest' in child.attrib:
                    defs[child.attrib['dest']] = Operand(child, True)
                elif 'modifier' in child.attrib:
                    name = child.attrib['modifier']
                    defs[child.attrib['modifier']] = Modifier(child, name)
                elif 'immediate' in child.attrib:
                    name = child.attrib['immediate']
                    defs[child.attrib['immediate']] = Immediate(child, name)
                else:
                    assert(0)
            elif child.tag == "encoding":
                self.encodings[child.attrib['name']] = Encoding(child)
            elif child.tag == "enum":
                enum = Enum(child)
                self.enums[enum.kind] = enum
            elif child.tag == "ins":
                ins = Instruction(child, name, child.get('display'), {}, defs)
                ensure(ins.name not in self.instrs, f'Multiple instructions named {ins.name}')
                self.instrs[ins.name] = ins
            elif child.tag == "group":
                ensure(any([c.tag == "ins" for c in child]), f'No instructions in group {name}')

                for c in child:
                    if c.tag == "ins":
                        ins = Instruction(child, c.attrib['name'], c.get('display'), c.attrib, defs)
                        ensure(ins.name not in self.instrs, f'Multiple instructions named {ins.name}')
                        self.instrs[ins.name] = ins

xmlfile = os.path.join(os.path.dirname(__file__), 'AGX2.xml')
isa = ISA(ET.parse(xmlfile).getroot())
