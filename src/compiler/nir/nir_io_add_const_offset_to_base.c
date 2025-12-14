/*
 * Copyright © 2014 Intel Corporation
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

/**
 * This pass adds constant offsets to instr->const_index[0] for input/output
 * intrinsics, and resets the offset source to 0.  Non-constant offsets remain
 * unchanged - since we don't know what part of a compound variable is
 * accessed, we allocate storage for the entire thing. For drivers that use
 * nir_lower_io_vars_to_temporaries() before nir_lower_io(), this guarantees that
 * the offset source will be 0, so that they don't have to add it in manually.
 */

#include "nir.h"
#include "nir_builder.h"

static bool
is_input(nir_intrinsic_instr *intrin)
{
   return intrin->intrinsic == nir_intrinsic_load_input ||
          intrin->intrinsic == nir_intrinsic_load_per_primitive_input ||
          intrin->intrinsic == nir_intrinsic_load_input_vertex ||
          intrin->intrinsic == nir_intrinsic_load_per_vertex_input ||
          intrin->intrinsic == nir_intrinsic_load_interpolated_input ||
          intrin->intrinsic == nir_intrinsic_load_fs_input_interp_deltas;
}

static bool
is_output(nir_intrinsic_instr *intrin)
{
   return intrin->intrinsic == nir_intrinsic_load_output ||
          intrin->intrinsic == nir_intrinsic_load_per_vertex_output ||
          intrin->intrinsic == nir_intrinsic_load_per_view_output ||
          intrin->intrinsic == nir_intrinsic_load_per_primitive_output ||
          intrin->intrinsic == nir_intrinsic_store_output ||
          intrin->intrinsic == nir_intrinsic_store_per_vertex_output ||
          intrin->intrinsic == nir_intrinsic_store_per_view_output ||
          intrin->intrinsic == nir_intrinsic_store_per_primitive_output;
}

static bool
is_dual_slot(nir_intrinsic_instr *intrin)
{
   if (intrin->intrinsic == nir_intrinsic_store_output ||
       intrin->intrinsic == nir_intrinsic_store_per_vertex_output ||
       intrin->intrinsic == nir_intrinsic_store_per_view_output ||
       intrin->intrinsic == nir_intrinsic_store_per_primitive_output) {
      return nir_src_bit_size(intrin->src[0]) == 64 &&
             nir_src_num_components(intrin->src[0]) >= 3;
   }

   return intrin->def.bit_size == 64 &&
          intrin->def.num_components >= 3;
}

static bool
add_const_offset_to_base_block(nir_block *block, nir_builder *b,
                               nir_variable_mode modes)
{
   bool progress = false;
   nir_foreach_instr_safe(instr, block) {
      if (instr->type != nir_instr_type_intrinsic)
         continue;

      nir_intrinsic_instr *intrin = nir_instr_as_intrinsic(instr);

      if (((modes & nir_var_shader_in) && is_input(intrin)) ||
          ((modes & nir_var_shader_out) && is_output(intrin))) {
         nir_io_semantics sem = nir_intrinsic_io_semantics(intrin);

         /* NV_mesh_shader: ignore MS primitive indices. */
         if (b->shader->info.stage == MESA_SHADER_MESH &&
             sem.location == VARYING_SLOT_PRIMITIVE_INDICES &&
             !(b->shader->info.per_primitive_outputs &
               VARYING_BIT_PRIMITIVE_INDICES))
            continue;

         nir_src *offset = nir_get_io_offset_src(intrin);

         /* TODO: Better handling of per-view variables here */
         if (nir_src_is_const(*offset) &&
             !nir_intrinsic_io_semantics(intrin).per_view) {
            unsigned off = nir_src_as_uint(*offset);

            if (off) {
               nir_intrinsic_set_base(intrin, nir_intrinsic_base(intrin) + off);

               sem.location += off;
               b->cursor = nir_before_instr(&intrin->instr);
               nir_src_rewrite(offset, nir_imm_int(b, 0));
               progress = true;
            }
            /* non-indirect indexing should reduce num_slots */
            sem.num_slots = is_dual_slot(intrin) ? 2 : 1;
            nir_intrinsic_set_io_semantics(intrin, sem);
         }
      }
   }

   return progress;
}

bool
nir_io_add_const_offset_to_base(nir_shader *nir, nir_variable_mode modes)
{
   bool progress = false;

   nir_foreach_function_impl(impl, nir) {
      bool impl_progress = false;
      nir_builder b = nir_builder_create(impl);
      nir_foreach_block(block, impl) {
         impl_progress |= add_const_offset_to_base_block(block, &b, modes);
      }
      progress |= impl_progress;
      nir_progress(impl_progress, impl, nir_metadata_control_flow);
   }

   return progress;
}
