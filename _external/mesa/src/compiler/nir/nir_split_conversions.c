/*
 * Copyright © 2024 Collabora, Ltd.
 *
 * Permission is hereby granted, free of charge, to any person obtaining a
 * copy of this software and associated documentation files (the "Software"),
 * to deal in the Software without restriction, including without limitation
 * the rights to use, copy, modify, merge, publish, distribute, sublicense,
 * and/or sell copies of the Software, and to permit persons to whom the
 * Software is furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice (including the next
 * paragraph) shall be included in all copies or substantial portions of the
 * Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL
 * THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
 * FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS
 * IN THE SOFTWARE.
 */

/* Adapted from intel_nir_lower_conversions.c */

#include "nir.h"
#include "nir_builder.h"

static nir_rounding_mode
op_rounding_mode(nir_op op)
{
   switch (op) {
   case nir_op_f2f16_rtne: return nir_rounding_mode_rtne;
   case nir_op_f2f16_rtz: return nir_rounding_mode_rtz;
   default: return nir_rounding_mode_undef;
   }
}

static bool
split_conversion_instr(nir_builder *b, nir_instr *instr, UNUSED void *_data)
{
   const nir_split_conversions_options *opts = _data;

   if (instr->type != nir_instr_type_alu)
      return false;

   nir_alu_instr *alu = nir_instr_as_alu(instr);

   if (!nir_op_infos[alu->op].is_conversion)
      return false;

   unsigned tmp_bit_size = opts->callback(instr, opts->callback_data);
   if (tmp_bit_size == 0)
      return false;

   unsigned src_bit_size = nir_src_bit_size(alu->src[0].src);
   unsigned dst_bit_size = alu->def.bit_size;
   if (src_bit_size < dst_bit_size)
      assert(src_bit_size < tmp_bit_size && tmp_bit_size < dst_bit_size);
   else
      assert(dst_bit_size < tmp_bit_size && tmp_bit_size < src_bit_size);

   nir_alu_type src_type = nir_op_infos[alu->op].input_types[0];
   nir_alu_type src_full_type = (nir_alu_type) (src_type | src_bit_size);

   nir_alu_type dst_full_type = nir_op_infos[alu->op].output_type;
   assert(nir_alu_type_get_type_size(dst_full_type) == dst_bit_size);
   nir_alu_type dst_type = nir_alu_type_get_base_type(dst_full_type);
   const nir_rounding_mode rounding_mode = op_rounding_mode(alu->op);

   nir_alu_type tmp_type;
   if ((src_full_type == nir_type_float16 && dst_bit_size == 64) ||
       (src_bit_size == 64 && dst_full_type == nir_type_float16)) {
      /* It is important that the intermediate conversion happens through a
       * 32-bit float type so we don't lose range when we convert to/from
       * a 64-bit integer.
       */
      assert(tmp_bit_size == 32);
      tmp_type = nir_type_float32;
   } else {
      /* For fp64 to integer conversions, using an integer intermediate type
       * ensures that rounding happens as part of the first conversion,
       * avoiding any chance of rtne rounding happening before the conversion
       * to integer (which is expected to round towards zero).
       *
       * NOTE: NVIDIA hardware saturates conversions by default and the second
       * conversion will not saturate in this case.  However, GLSL makes OOB
       * values in conversions undefiend.
       *
       * For all other conversions, the conversion from int to int is either
       * lossless or just as lossy as the final conversion.
       */
      tmp_type = dst_type | tmp_bit_size;
   }

   b->cursor = nir_before_instr(&alu->instr);
   nir_def *src = nir_ssa_for_alu_src(b, alu, 0);
   nir_def *tmp;
   if (src_full_type == nir_type_float64 && dst_full_type == nir_type_float16) {
      /* For fp64->fp16 conversions, we need to be careful with the first
       * conversion or else rounding might not accumulate properly.
       */
      assert(tmp_type == nir_type_float32);
      if (rounding_mode == nir_rounding_mode_rtne ||
          rounding_mode == nir_rounding_mode_undef ||
          !opts->has_convert_alu_types) {
         nir_def *src_lo = nir_unpack_64_2x32_split_x(b, src);
         nir_def *src_hi = nir_unpack_64_2x32_split_y(b, src);

         /* RTNE is tricky to get right through a double conversion.  To work
          * around this, we do a little fixup of the fp64 value first.
          *
          * For a 64-bit float, the mantissa bits are as follows:
          *
          *    HHHHHHHHHHHLTFFFFFFFFF FFFDDDDDDDDDDDDDDDDDDDDDDDDDDDDD
          *                           |                              |
          *                           +------- bottom 32 bits -------+
          *
          * Where:
          *  - D are only used for fp64
          *  - T and F are used for fp64 and fp32
          *  - H and L are used for fp64, fp32, and fp16
          *  - L denotes the low bit of the fp16 mantissa
          *  - T is the tie bit
          *
          * The RTNE tie-breaking rules for fp64 -> fp16 can then be described
          * as follows:
          *
          *  - If any F or D bit is non-zero:
          *     - If T == 1, round up
          *     - If T == 0, round down
          *  - If all F and D bits are zero:
          *     - If T == 0, it's already fp16, do nothing
          *     - If T != 0 and L == 0, round down
          *     - If T != 0 and L != 0, round up
          *
          * What's important here is that the only way the F or D bits fit
          * into the algorithm is if any are zero or none are zero.  So we
          * will get the same result if we take all of the bits in the low
          * dword, or them together, and then or that into the low F bits of
          * the high dword.  The result of "all F and D bits are zero" will be
          * the same.  We can also zero the low dword without affecting the
          * final result.  Doing this accomplishes two useful things:
          *
          *  1. The resulting fp64 value is exactly representable as fp32 so
          *     we don't have to care about the rounding of the fp64 -> fp32
          *     conversion.
          *
          *  2. The fp32 -> fp16 conversion will round exactly the same as a
          *     full fp64 -> fp16 conversion on the original data since it now
          *     takes all of the D bits into account as well as the F bits.
          *
          * It's also correct for NaN/INF since those are delineated by the
          * entire mantissa being either zero or non-zero.  For denorms,
          * anything that might be a denorm in fp32 or fp64 will have a
          * sufficiently negative exponent that it will flush to zero when
          * converted to fp16, regardless of what we do here.
          *
          * This same trick works for all the rounding modes.  Even though the
          * actual rounding logic is a bit different, they all treat the F and
          * D bits together based on "all F and D bits are zero" or not.
          *
          * There are many operations we could choose for combining the low
          * dword bits for ORing into the high dword.  We choose umin because
          * it nicely translates to a single fixed-latency instruction on a
          * lot of hardware.
          */
         src_hi = nir_ior(b, src_hi, nir_umin_imm(b, src_lo, 1));
         src_lo = nir_imm_int(b, 0);

         tmp = nir_f2f32(b, nir_pack_64_2x32_split(b, src_lo, src_hi));
      } else {
         /* For round-up, round-down, and round-towards-zero, the rounding
          * accumulates properly as long as we use the same rounding mode for
          * both operations.  This is more efficient if the back-end supports
          * nir_intrinsic_convert_alu_types.
          */
         tmp = nir_convert_alu_types(b, 32, src,
                                     .src_type = nir_type_float64,
                                     .dest_type = tmp_type,
                                     .rounding_mode = rounding_mode,
                                     .saturate = false);
      }
   } else {
      /* This is an up-convert or a convert to integer, in which case we
       * always round towards zero.
       */
      tmp = nir_type_convert(b, src, src_type, tmp_type,
                             nir_rounding_mode_undef);
   }
   nir_def *res = nir_type_convert(b, tmp, tmp_type, dst_full_type,
                                   rounding_mode);
   nir_def_replace(&alu->def, res);

   return true;
}

bool
nir_split_conversions(nir_shader *shader,
                      const nir_split_conversions_options *options)
{
   return nir_shader_instructions_pass(shader, split_conversion_instr,
                                       nir_metadata_control_flow,
                                       (void *)options);
}
