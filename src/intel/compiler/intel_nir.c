/*
 * Copyright (c) 2014-2023 Intel Corporation
 * SPDX-License-Identifier: MIT
 */

#include "intel_nir.h"

unsigned
intel_nir_split_conversions_cb(const nir_instr *instr, void *data)
{
   nir_alu_instr *alu = nir_instr_as_alu(instr);

   unsigned src_bit_size = nir_src_bit_size(alu->src[0].src);
   nir_alu_type src_type = nir_op_infos[alu->op].input_types[0];
   nir_alu_type src_full_type = (nir_alu_type) (src_type | src_bit_size);

   unsigned dst_bit_size = alu->def.bit_size;
   nir_alu_type dst_full_type = nir_op_infos[alu->op].output_type;

   /* BDW PRM, vol02, Command Reference Instructions, mov - MOVE:
    *
    *   "There is no direct conversion from HF to DF or DF to HF.
    *    Use two instructions and F (Float) as an intermediate type.
    *
    *    There is no direct conversion from HF to Q/UQ or Q/UQ to HF.
    *    Use two instructions and F (Float) or a word integer type
    *    or a DWord integer type as an intermediate type."
    *
    * It is important that the intermediate conversion happens through a
    * 32-bit float type so we don't lose range when we convert from
    * a 64-bit integer.
    */
   if ((src_full_type == nir_type_float16 && dst_bit_size == 64) ||
       (src_bit_size == 64 && dst_full_type == nir_type_float16))
      return 32;

   /* SKL PRM, vol 02a, Command Reference: Instructions, Move:
    *
    *   "There is no direct conversion from B/UB to DF or DF to B/UB. Use
    *    two instructions and a word or DWord intermediate type."
    *
    *   "There is no direct conversion from B/UB to Q/UQ or Q/UQ to B/UB.
    *    Use two instructions and a word or DWord intermediate integer
    *    type."
    *
    * It is important that we use a 32-bit integer matching the sign of the
    * destination as the intermediate type so we avoid any chance of rtne
    * rounding happening before the conversion to integer (which is expected
    * to round towards zero) in double to byte conversions.
    */
   if ((src_bit_size == 8 && dst_bit_size == 64) ||
       (src_bit_size == 64 && dst_bit_size == 8))
      return 32;

   return 0;
}

bool
intel_nir_pulls_at_sample(nir_shader *shader)
{
   nir_foreach_function_impl(impl, shader) {
      nir_foreach_block(block, impl) {
         nir_foreach_instr(instr, block) {
            if (instr->type != nir_instr_type_intrinsic)
               continue;

            nir_intrinsic_instr *intrin = nir_instr_as_intrinsic(instr);

            if (intrin->intrinsic == nir_intrinsic_load_barycentric_at_sample)
               return true;
         }
      }
   }

   return false;
}


