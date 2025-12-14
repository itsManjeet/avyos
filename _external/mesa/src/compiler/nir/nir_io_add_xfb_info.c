/*
 * Copyright 2025 Advanced Micro Devices, Inc.
 * SPDX-License-Identifier: MIT
 */

#include "nir.h"
#include "nir_xfb_info.h"

bool
nir_io_add_intrinsic_xfb_info(nir_shader *nir)
{
   nir_function_impl *impl = nir_shader_get_entrypoint(nir);
   bool progress = false;

   for (unsigned i = 0; i < NIR_MAX_XFB_BUFFERS; i++)
      nir->info.xfb_stride[i] = nir->xfb_info->buffers[i].stride / 4;

   nir_foreach_block(block, impl) {
      nir_foreach_instr_safe(instr, block) {
         if (instr->type != nir_instr_type_intrinsic)
            continue;

         nir_intrinsic_instr *intr = nir_instr_as_intrinsic(instr);

         if (!nir_intrinsic_has_io_xfb(intr))
            continue;

         /* No indirect indexing allowed. The index is implied to be 0. */
         ASSERTED nir_src offset = *nir_get_io_offset_src(intr);
         assert(nir_src_is_const(offset) && nir_src_as_uint(offset) == 0);

         /* Calling this pass for the second time shouldn't do anything. */
         if (nir_intrinsic_io_xfb(intr).out[0].num_components ||
             nir_intrinsic_io_xfb(intr).out[1].num_components ||
             nir_intrinsic_io_xfb2(intr).out[0].num_components ||
             nir_intrinsic_io_xfb2(intr).out[1].num_components)
            continue;

         nir_io_semantics sem = nir_intrinsic_io_semantics(intr);
         unsigned writemask = nir_intrinsic_write_mask(intr) << nir_intrinsic_component(intr);

         nir_io_xfb xfb[2];
         memset(xfb, 0, sizeof(xfb));

         for (unsigned i = 0; i < nir->xfb_info->output_count; i++) {
            nir_xfb_output_info *out = &nir->xfb_info->outputs[i];
            if (out->location == sem.location) {
               unsigned xfb_mask = writemask & out->component_mask;

               /*fprintf(stdout, "output%u: buffer=%u, offset=%u, location=%u, "
                           "component_offset=%u, component_mask=0x%x, xfb_mask=0x%x, slots=%u\n",
                       i, out->buffer,
                       out->offset,
                       out->location,
                       out->component_offset,
                       out->component_mask,
                       xfb_mask, sem.num_slots);*/

               while (xfb_mask) {
                  int start, count;
                  u_bit_scan_consecutive_range(&xfb_mask, &start, &count);

                  xfb[start / 2].out[start % 2].num_components = count;
                  xfb[start / 2].out[start % 2].buffer = out->buffer;
                  /* out->offset is relative to the first stored xfb component */
                  /* start is relative to component 0 */
                  xfb[start / 2].out[start % 2].offset =
                     out->offset / 4 - out->component_offset + start;

                  progress = true;
               }
            }
         }

         nir_intrinsic_set_io_xfb(intr, xfb[0]);
         nir_intrinsic_set_io_xfb2(intr, xfb[1]);
      }
   }

   nir_no_progress(impl);
   return progress;
}
