/*
 * Copyright © 2025 Valve Corporation
 * SPDX-License-Identifier: MIT
 */

#include "nir.h"
#include "nir_builder.h"

static bool
pass(nir_builder *b, nir_instr *instr, void *data)
{
   if (instr->type != nir_instr_type_jump)
      return false;

   nir_jump_instr *jump = nir_instr_as_jump(instr);
   if (jump->type == nir_jump_halt) {
      jump->type = nir_jump_return;
      return true;
   }

   return false;
}

bool
nir_lower_halt_to_return(nir_shader *nir)
{
   return nir_shader_instructions_pass(nir, pass, nir_metadata_all, NULL);
}
