/*
 * Copyright (c) 2025 Intel Corporation
 * SPDX-License-Identifier: MIT
 */

#include "compiler/nir/nir_builder.h"
#include "brw_nir.h"

static bool
brw_nir_lower_fs_msaa_intel_instr(nir_builder *b,
                                  nir_intrinsic_instr *intrin,
                                  void *data)
{
   if (intrin->intrinsic != nir_intrinsic_load_fs_msaa_intel)
      return false;

   b->cursor = nir_before_instr(&intrin->instr);

   const struct brw_wm_prog_key *key = data;

   uint32_t fs_msaa_flags =
      (key->multisample_fbo == INTEL_ALWAYS ?
       INTEL_MSAA_FLAG_MULTISAMPLE_FBO : 0) |
      (key->persample_interp == INTEL_ALWAYS ?
       (INTEL_MSAA_FLAG_PERSAMPLE_DISPATCH |
        INTEL_MSAA_FLAG_PERSAMPLE_INTERP) : 0) |
      (key->alpha_to_coverage == INTEL_ALWAYS ?
       INTEL_MSAA_FLAG_ALPHA_TO_COVERAGE : 0) |
      (key->provoking_vertex_last == INTEL_ALWAYS ?
       INTEL_MSAA_FLAG_PROVOKING_VERTEX_LAST : 0);

   nir_def_replace(&intrin->def, nir_imm_int(b, fs_msaa_flags));

   return true;
}

bool
brw_nir_lower_fs_msaa(nir_shader *shader,
                      const struct brw_wm_prog_key *key)
{
   if (brw_wm_prog_key_is_dynamic(key))
      return false;

   return nir_shader_intrinsics_pass(shader,
                                     brw_nir_lower_fs_msaa_intel_instr,
                                     nir_metadata_control_flow |
                                     nir_metadata_live_defs |
                                     nir_metadata_divergence,
                                     (void *)key);
}
