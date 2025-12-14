/*
 * Copyright © 2025 Imagination Technologies Ltd.
 *
 * SPDX-License-Identifier: MIT
 */

/**
 * \file pco_legalize.c
 *
 * \brief PCO legalizing pass.
 */

#include "pco.h"
#include "pco_builder.h"
#include "pco_internal.h"
#include "util/macros.h"

#include <stdbool.h>

/**
 * \brief Insert a mov to legalize how a hardware register is referenced.
 *
 * \param[in,out] instr PCO instr.
 * \param[in,out] ref Reference to be legalized.
 * \param[in] needs_s124 Whether the mapping needs to use S{1,2,4}
 *                       rather than S{0,2,3}.
 * \return True if progress was made.
 */
static void insert_mov_ref(pco_instr *instr, pco_ref *ref, bool needs_s124)
{
   assert(pco_ref_is_scalar(*ref));
   pco_ref new_ref = pco_ref_new_ssa(instr->parent_func,
                                     pco_ref_get_bits(*ref),
                                     pco_ref_get_chans(*ref));

   pco_ref_xfer_mods(&new_ref, ref, true);

   pco_builder b =
      pco_builder_create(instr->parent_func, pco_cursor_before_instr(instr));

   if (needs_s124)
      pco_movs1(&b, new_ref, *ref);
   else
      pco_mbyp(&b, new_ref, *ref);

   *ref = new_ref;
}

/**
 * \brief Try to legalize an instruction's hardware source mappings.
 *
 * \param[in,out] instr PCO instr.
 * \param[in] info PCO op info.
 * \return True if progress was made.
 */
static bool try_legalize_src_mappings(pco_instr *instr,
                                      const struct pco_op_info *info)
{
   bool progress = false;
   bool needs_s124;

   /* Check dests. */
   pco_foreach_instr_dest (pdest, instr) {
      unsigned dest_index = pdest - instr->dest;
      if (!info->dest_intrn_map[dest_index])
         continue;

      enum pco_io mapped_src = PCO_IO_S0 + info->dest_intrn_map[dest_index] - 1;

      if (ref_src_map_valid(*pdest, mapped_src, &needs_s124))
         continue;

      insert_mov_ref(instr, pdest, needs_s124);
      progress = true;
   }

   /* Check srcs. */
   pco_foreach_instr_src (psrc, instr) {
      unsigned src_index = psrc - instr->src;
      if (!info->src_intrn_map[src_index])
         continue;

      enum pco_io mapped_src = PCO_IO_S0 + info->src_intrn_map[src_index] - 1;

      if (ref_src_map_valid(*psrc, mapped_src, &needs_s124))
         continue;

      insert_mov_ref(instr, psrc, needs_s124);
      progress = true;
   }

   return progress;
}

/**
 * \brief Try to legalizes an instruction.
 *
 * \param[in,out] instr PCO instr.
 * \return True if progress was made.
 */
static bool try_legalize(pco_instr *instr)
{
   const struct pco_op_info *info = &pco_op_info[instr->op];
   bool progress = false;

   /* Skip pseudo instructions. */
   if (info->type == PCO_OP_TYPE_PSEUDO)
      return false;

   progress |= try_legalize_src_mappings(instr, info);

   return progress;
}

/**
 * \brief Legalizes instructions where additional restrictions apply.
 *
 * \param[in,out] shader PCO shader.
 * \return True if the pass made progress.
 */
bool pco_legalize(pco_shader *shader)
{
   bool progress = false;

   assert(!shader->is_grouped);
   assert(!shader->is_legalized);

   pco_foreach_func_in_shader (func, shader) {
      pco_foreach_instr_in_func_safe (instr, func) {
         progress |= try_legalize(instr);
      }
   }

   shader->is_legalized = true;
   return progress;
}
