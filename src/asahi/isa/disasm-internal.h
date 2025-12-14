/*
 * Copyright 2025 Valve Corporation
 * SPDX-License-Identifier: MIT
 */
#include <assert.h>
#include <inttypes.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include "asahi/isa/agx_minifloat.h"
#include "util/bitset.h"
#include "util/u_math.h"
#include "disasm.h"

struct disasm_ctx {
   FILE *fp;
   bool any_operands;
   bool error;
};

static inline uint64_t
bits(BITSET_WORD *word, unsigned start, unsigned size, unsigned shift)
{
   return ((uint64_t)__bitset_extract(word, start, size)) << shift;
}

typedef void (*disasm_instr)(struct disasm_ctx *ctx, BITSET_WORD *code);

struct disasm_spec {
   const char *display;
   disasm_instr disassemble;
   unsigned length_bit;
   unsigned length_short;
   unsigned length_long;
   BITSET_WORD mask[4];
   BITSET_WORD exact[4];
   BITSET_WORD known[4];
};

static void
introduce_operand(struct disasm_ctx *ctx)
{
   if (ctx->any_operands) {
      fprintf(ctx->fp, ", ");
   } else {
      fprintf(ctx->fp, " ");
   }

   ctx->any_operands = true;
}

enum operand_kind {
   KIND_NONE = 0,
   KIND_REG,
   KIND_UNIFORM,
   KIND_CF,
   KIND_TS,
   KIND_SS,
   KIND_IMM,
   KIND_FIMM,
};

struct operand_desc {
   enum operand_kind kind;
   int32_t value;
   unsigned hint, count;
   bool optional, size16, size32, size64, abs, neg, sx, cache, lu;
};

static void
_print_operand(struct disasm_ctx *ctx, struct operand_desc d)
{
   if (d.kind == KIND_NONE) {
      if (!d.optional) {
         introduce_operand(ctx);
         fprintf(ctx->fp, "_");
      }

      return;
   }

   unsigned size = d.size64 ? 64 : d.size32 ? 32 : 16;
   bool cache = d.cache || d.hint == 2;
   bool lu = d.lu || d.hint == 3;
   unsigned count = MAX2(d.count, 1);

   introduce_operand(ctx);

   if (lu && cache) {
      fprintf(ctx->fp, "XXX invalid cache+lu set\n");
      ctx->error = true;
   }

   if (lu)
      fprintf(ctx->fp, "^");
   if (cache)
      fprintf(ctx->fp, "$");

   const char *prefixes[][2] = {
      [KIND_REG] = {"r", "dr"}, [KIND_UNIFORM] = {"u", "du"},
      [KIND_CF] = {"cf"},       [KIND_TS] = {"ts"},
      [KIND_SS] = {"ss"},
   };

   if (d.kind == KIND_IMM) {
      fprintf(ctx->fp, "%d", d.value);
   } else if (d.kind == KIND_FIMM) {
      float f = agx_minifloat_decode(d.value);

      /* Match python's float printing */
      if (f == (int)f)
         fprintf(ctx->fp, "%g.0", agx_minifloat_decode(d.value));
      else
         fprintf(ctx->fp, "%g", agx_minifloat_decode(d.value));
   } else if (d.kind == KIND_CF || d.kind == KIND_TS || d.kind == KIND_SS) {
      fprintf(ctx->fp, "%s%u", prefixes[d.kind][0], d.value);
   } else {
      for (unsigned i = 0; i < count; ++i) {
         if (i)
            fprintf(ctx->fp, "_");

         unsigned reg = d.value + (i * (size / 16));
         unsigned whole = reg >> 1;
         unsigned part = reg & 1;
         const char *prefix = prefixes[d.kind][size == 64];

         if (size == 16) {
            fprintf(ctx->fp, "%s%u%c", prefix, whole, "lh"[part]);
         } else {
            if (part != 0) {
               fprintf(ctx->fp, "# 32-bit must be expected, but got raw %u\n",
                       reg);
               ctx->error = true;
            }

            fprintf(ctx->fp, "%s%u", prefix, whole);
         }
      }
   }

   if (d.abs)
      fprintf(ctx->fp, ".abs");
   if (d.neg)
      fprintf(ctx->fp, ".neg");
   if (d.sx)
      fprintf(ctx->fp, ".sx");
}

static void
print_immediate(struct disasm_ctx *ctx, bool is_signed, uint32_t value)
{
   introduce_operand(ctx);
   fprintf(ctx->fp, is_signed ? "%d" : "%u", value);
}

static void
_print_enum(struct disasm_ctx *ctx, const char **arr, unsigned n,
            unsigned value)
{
   if (value >= n || arr[value] == NULL) {
      introduce_operand(ctx);
      fprintf(ctx->fp, "XXX: Unknown enum value %u", value);
      ctx->error = true;
   } else if (arr[value][0] != 0) {
      introduce_operand(ctx);
      fprintf(ctx->fp, "%s", arr[value]);
   }
}

#define print_enum(ctx, arr, value)                                            \
   _print_enum(ctx, arr, ARRAY_SIZE(arr), value)

static void
print_modifier(struct disasm_ctx *ctx, const char *display, unsigned value)
{
   if (value) {
      introduce_operand(ctx);
      fprintf(ctx->fp, "%s", display);
   }
}

static signed
_disassemble_instr(BITSET_WORD *code, FILE *fp, struct disasm_spec *specs,
                   unsigned nr_specs, unsigned offset, bool verbose)
{
   BITSET_WORD tmp[4];
   memcpy(tmp, code, sizeof(tmp));
   struct disasm_spec *spec = NULL;
   unsigned n = 0;
   bool match = false;
   BITSET_WORD masked[4] = {0};

   for (unsigned i = 0; i < nr_specs; ++i) {
      spec = &specs[i];

      n = spec->length_short;
      if (BITSET_TEST(tmp, spec->length_bit)) {
         n = spec->length_long;
      }

      match = true;
      for (unsigned i = 0; i < ARRAY_SIZE(tmp); ++i) {
         masked[i] = tmp[i];

         uint32_t bytes_left = (n - (i * 4));
         if (bytes_left < 4) {
            masked[i] &= ((1 << (bytes_left * 8)) - 1);
         }

         if ((masked[i] & spec->mask[i]) != spec->exact[i]) {
            match = false;
            break;
         }
      }

      if (match)
         break;
   }

   struct disasm_ctx ctx = {.fp = fp};

   if (match) {
      BITSET_WORD unknown[4];
      __bitset_andnot(unknown, masked, spec->known, ARRAY_SIZE(masked));
      int i;
      BITSET_FOREACH_SET(i, unknown, n * 8) {
         fprintf(fp, "# XXX: Unknown bit set %u\n", i);
         ctx.error = true;
      }
   } else {
      n = 2;
   }

   if (verbose) {
      fprintf(fp, "%4x: ", offset);
      for (unsigned i = 0; i < n; ++i) {
         fprintf(fp, "%02x", __bitset_extract(masked, i * 8, 8));
      }
      for (unsigned i = n; i < 11; ++i) {
         fprintf(fp, "  ");
      }
      fprintf(fp, " ");
   }

   if (!match) {
      fprintf(fp, "<unknown instr>\n");
      return -n;
   }

   fprintf(fp, "%s", spec->display);

   if (spec->disassemble)
      spec->disassemble(&ctx, masked);

   fprintf(fp, "\n");

   return ctx.error ? -n : n;
}
