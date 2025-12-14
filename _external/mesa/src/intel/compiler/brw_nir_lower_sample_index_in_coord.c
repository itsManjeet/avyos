/*
 * Copyright (c) 2015-2025 Intel Corporation
 * SPDX-License-Identifier: MIT
 */

#include "brw_nir.h"
#include "compiler/nir/nir_builder.h"

/* Put the sample index in the 4th component of coords since multisampled
 * images don't support mipmapping.
 */
static bool
lower_image_sample_index_in_coord(nir_builder *b,
                                  nir_intrinsic_instr *intrin)
{
   b->cursor = nir_before_instr(&intrin->instr);

   nir_def *coord = intrin->src[1].ssa;
   nir_def *sample_index = intrin->src[2].ssa;

   nir_def *new_coord;
   if (nir_intrinsic_image_array(intrin)) {
      new_coord = nir_vec4(b, nir_channel(b, coord, 0),
                           nir_channel(b, coord, 1), nir_channel(b, coord, 2),
                           sample_index);
   } else {
      new_coord = nir_vec4(b, nir_channel(b, coord, 0),
                           nir_channel(b, coord, 1), nir_imm_int(b, 0),
                           sample_index);
   }

   nir_src_rewrite(&intrin->src[1], new_coord);
   return true;
}

static bool
lower_image_sample_index_in_coord_instr(nir_builder *b,
                                        nir_instr *instr,
                                        void *cb_data)
{
   if (instr->type != nir_instr_type_intrinsic)
      return false;

   nir_intrinsic_instr *intrin = nir_instr_as_intrinsic(instr);

   switch (intrin->intrinsic) {
   case nir_intrinsic_image_load:
   case nir_intrinsic_bindless_image_load:
   case nir_intrinsic_image_store:
   case nir_intrinsic_bindless_image_store:
      if (nir_intrinsic_image_dim(intrin) != GLSL_SAMPLER_DIM_MS)
         return false;
      return lower_image_sample_index_in_coord(b, intrin);

   default:
      return false;
   }
}

bool
brw_nir_lower_sample_index_in_coord(nir_shader *shader)
{
   return nir_shader_instructions_pass(shader,
                                       lower_image_sample_index_in_coord_instr,
                                       nir_metadata_none, NULL);
}
