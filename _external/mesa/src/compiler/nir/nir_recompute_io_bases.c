/*
 * Copyright (C) 2020 Google, Inc.
 * Copyright (C) 2021 Advanced Micro Devices, Inc.
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
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

#include "nir.h"

/**
 * Return the intrinsic if it matches the mask in "modes", else return NULL.
 */
nir_intrinsic_instr *
nir_get_io_intrinsic(nir_instr *instr, nir_variable_mode modes,
                     nir_variable_mode *out_mode)
{
   if (instr->type != nir_instr_type_intrinsic)
      return NULL;

   nir_intrinsic_instr *intr = nir_instr_as_intrinsic(instr);

   switch (intr->intrinsic) {
   case nir_intrinsic_load_input:
   case nir_intrinsic_load_per_primitive_input:
   case nir_intrinsic_load_input_vertex:
   case nir_intrinsic_load_interpolated_input:
   case nir_intrinsic_load_per_vertex_input:
      *out_mode = nir_var_shader_in;
      return modes & nir_var_shader_in ? intr : NULL;
   case nir_intrinsic_load_output:
   case nir_intrinsic_load_per_vertex_output:
   case nir_intrinsic_load_per_view_output:
   case nir_intrinsic_store_output:
   case nir_intrinsic_store_per_vertex_output:
   case nir_intrinsic_store_per_view_output:
   case nir_intrinsic_store_per_primitive_output:
      *out_mode = nir_var_shader_out;
      return modes & nir_var_shader_out ? intr : NULL;
   default:
      return NULL;
   }
}

/**
 * Recompute the IO "base" indices from scratch to remove holes or to fix
 * incorrect base values due to changes in IO locations by using IO locations
 * to assign new bases. The mapping from locations to bases becomes
 * monotonically increasing.
 */
bool
nir_recompute_io_bases(nir_shader *nir, nir_variable_mode modes)
{
   nir_function_impl *impl = nir_shader_get_entrypoint(nir);

   BITSET_DECLARE(inputs, NUM_TOTAL_VARYING_SLOTS);
   BITSET_DECLARE(per_prim_inputs, NUM_TOTAL_VARYING_SLOTS);  /* FS only */
   BITSET_DECLARE(dual_slot_inputs, NUM_TOTAL_VARYING_SLOTS); /* VS only */
   BITSET_DECLARE(outputs, NUM_TOTAL_VARYING_SLOTS);
   BITSET_ZERO(inputs);
   BITSET_ZERO(per_prim_inputs);
   BITSET_ZERO(dual_slot_inputs);
   BITSET_ZERO(outputs);

   /* Gather the bitmasks of used locations. */
   nir_foreach_block_safe(block, impl) {
      nir_foreach_instr_safe(instr, block) {
         nir_variable_mode mode;
         nir_intrinsic_instr *intr = nir_get_io_intrinsic(instr, modes, &mode);
         if (!intr)
            continue;

         nir_io_semantics sem = nir_intrinsic_io_semantics(intr);
         unsigned num_slots = sem.num_slots;
         if (sem.medium_precision)
            num_slots = (num_slots + sem.high_16bits + 1) / 2;

         if (mode == nir_var_shader_in) {
            for (unsigned i = 0; i < num_slots; i++) {
               if (intr->intrinsic == nir_intrinsic_load_per_primitive_input)
                  BITSET_SET(per_prim_inputs, sem.location + i);
               else
                  BITSET_SET(inputs, sem.location + i);

               if (sem.high_dvec2)
                  BITSET_SET(dual_slot_inputs, sem.location + i);
            }
         } else if (!sem.dual_source_blend_index) {
            for (unsigned i = 0; i < num_slots; i++)
               BITSET_SET(outputs, sem.location + i);
         }
      }
   }

   const unsigned num_normal_inputs = BITSET_COUNT(inputs) + BITSET_COUNT(dual_slot_inputs);

   /* Renumber bases. */
   bool changed = false;

   nir_foreach_block_safe(block, impl) {
      nir_foreach_instr_safe(instr, block) {
         nir_variable_mode mode;
         nir_intrinsic_instr *intr = nir_get_io_intrinsic(instr, modes, &mode);
         if (!intr)
            continue;

         nir_io_semantics sem = nir_intrinsic_io_semantics(intr);
         unsigned num_slots = sem.num_slots;
         if (sem.medium_precision)
            num_slots = (num_slots + sem.high_16bits + 1) / 2;

         if (mode == nir_var_shader_in) {
            if (intr->intrinsic == nir_intrinsic_load_per_primitive_input) {
               nir_intrinsic_set_base(intr,
                                      num_normal_inputs +
                                         BITSET_PREFIX_SUM(per_prim_inputs, sem.location));
            } else {
               nir_intrinsic_set_base(intr,
                                      BITSET_PREFIX_SUM(inputs, sem.location) +
                                         BITSET_PREFIX_SUM(dual_slot_inputs, sem.location) +
                                         (sem.high_dvec2 ? 1 : 0));
            }
         } else if (sem.dual_source_blend_index) {
            nir_intrinsic_set_base(intr,
                                   BITSET_PREFIX_SUM(outputs, NUM_TOTAL_VARYING_SLOTS));
         } else {
            nir_intrinsic_set_base(intr,
                                   BITSET_PREFIX_SUM(outputs, sem.location));
         }
         changed = true;
      }
   }

   nir_progress(changed, impl, nir_metadata_control_flow);

   if (modes & nir_var_shader_in)
      nir->num_inputs = BITSET_COUNT(inputs) + BITSET_COUNT(per_prim_inputs);
   if (modes & nir_var_shader_out)
      nir->num_outputs = BITSET_COUNT(outputs);

   return changed;
}
