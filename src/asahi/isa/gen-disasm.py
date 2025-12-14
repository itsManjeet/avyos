# Copyright 2016 Intel Corporation
# Copyright 2016 Broadcom
# Copyright 2020 Collabora, Ltd.
# Copyright 2025 Valve Corporation
# SPDX-License-Identifier: MIT

from mako.template import Template
from mako import exceptions
from isa import isa

# XXX: deduplicate
def to_alphanum(name):
    substitutions = {
        ' ': '_',
        '/': '_',
        '[': '',
        ']': '',
        '(': '',
        ')': '',
        '-': '_',
        ':': '',
        '.': '',
        ',': '',
        '=': '',
        '>': '',
        '#': '',
        '&': '',
        '*': '',
        '"': '',
        '+': '',
        '\'': '',
    }

    for i, j in substitutions.items():
        name = name.replace(i, j)

    return name

def safe_name(name):
    name = to_alphanum(name)
    if not name[0].isalpha():
        name = '_' + name

    return name.lower()

def chunk(x):
    chunks = [hex((x >> (32 * i)) & ((1 << 32) - 1)) for i in range(4)]
    while len(chunks) > 1 and chunks[-1] == '0x0':
        chunks.pop()

    return f'{{{", ".join(chunks)}}}'

def instr_is_trivial(I):
    return len(I.dests + I.sources + I.immediates + I.modifiers) == 0

def extract_bits(x, bitset="r"):
    parts = []
    n = 0

    for r in x.bits.ranges:
        parts.append(f'bits({bitset}, {r.start}, {r.size}, {n})')
        n += r.size

    return ' | '.join(parts)

template = """
#include "disasm-internal.h"

% for en in isa.enums:
UNUSED static const char *enum_${safe_name(isa.enums[en].kind)}[] = {
% for n, v in isa.enums[en].values.items():
    [${n}] = "${v.display or ''}",
% endfor
};
% endfor

% for k, enc in isa.encodings.items():
static void
print_${k}(struct disasm_ctx *ctx, BITSET_WORD *orig, uint64_t v, bool source)
{
   BITSET_WORD r[] = { v, v >> 32 };
% for i, case in enumerate(enc.cases):
   ${"} else " if i > 0 else ""}if ((v & ${hex(case.exact.mask)}ull) == ${hex(case.exact.value)}ull) {
% for mod in case.modifiers:
% if mod.name == 'hint':
       if (${extract_bits(mod)} == 0) {
          fprintf(ctx->fp, "# missing hint");
       }

% endif
% endfor
       _print_operand(ctx, (struct operand_desc){
% if case.kind is not None:
          .kind = KIND_${case.kind.name.upper()},
% if case.kind.kind == 'signed':
          .value = util_sign_extend(${extract_bits(case.kind)} << ${case.kind.shift}, ${case.kind.bits.size()}),
% else:
          .value = ${extract_bits(case.kind)} << ${case.kind.shift},
% endif
% endif
% for mod in case.modifiers:
% if mod.name == 'mask':
          .count = util_bitcount(${extract_bits(mod)}),
% elif mod.name == 'count':
          .count = (${extract_bits(mod)}) ?: 4,
% elif mod.name == 'zx':
          .sx = ${extract_bits(mod)} ^ 1,
% else:
          .${mod.name} = ${extract_bits(mod)},
% endif
% endfor
% for prop, v in case.properties.items():
          .${prop} = ${v},
% endfor
          .optional = source,
       });
% endfor
   } else {
       fprintf(ctx->fp, "# XXX: Invalid value 0x%"PRIx64" for ${k}", v);
       ctx->error = true;
   }
}

% endfor

% for k, instr in isa.instrs.items():
% if not instr_is_trivial(instr):
static void
print_${instr.name}(struct disasm_ctx *ctx, BITSET_WORD *r)
{
% for i, op in enumerate(instr.dests):
   print_${op.kind}(ctx, r, ${extract_bits(op)}, false);
% endfor
% for i, op in enumerate(instr.sources):
   print_${op.kind}(ctx, r, ${extract_bits(op)}, true);
% endfor
% for imm in instr.immediates:
   print_immediate(ctx, ${'true' if imm.kind == 'signed' else 'false'}, ${extract_bits(imm)});
% endfor
% for mod in instr.modifiers:
% if mod.name != 'length':
% if mod.kind in isa.enums:
   print_enum(ctx, enum_${mod.kind}, ${extract_bits(mod)});
% else:
   print_modifier(ctx, "${mod.display}", ${extract_bits(mod)});
% endif
% endif
% endfor
}

% endif
% endfor
struct disasm_spec decodings[] = {
% for k, instr in isa.instrs.items():
    { "${instr.display if instr_is_trivial(instr) else f'{instr.display:<8}'}", ${'NULL' if instr_is_trivial(instr) else f'print_{instr.name}'}, ${instr.length_bit}, ${instr.length[0]}, ${instr.length[-1]}, ${chunk(instr.exact.mask)}, ${chunk(instr.exact.value)}, ${chunk(instr.mask())} },
% endfor
};

signed
agx2_disassemble_instr(BITSET_WORD *code, FILE *fp, unsigned offset, bool verbose)
{
  return _disassemble_instr(code, fp, decodings, ARRAY_SIZE(decodings), offset, verbose);
}"""

try:
    print(Template(template).render(isa = isa, safe_name = safe_name, chunk = chunk, instr_is_trivial = instr_is_trivial, extract_bits = extract_bits))
except:
    print(exceptions.text_error_template().render())
