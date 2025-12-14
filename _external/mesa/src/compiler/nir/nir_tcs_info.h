/*
 * Copyright © 2024 Advanced Micro Devices, Inc.
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
 *
 * Authors:
 *    Marek Olšák <marek.olsak@amd.com>
 *
 */

#ifndef NIR_TCS_INFO_H
#define NIR_TCS_INFO_H

#include "compiler/shader_enums.h"
#include "nir_defines.h"

#ifdef __cplusplus
extern "C" {
#endif

typedef struct nir_tcs_info {
    /* The bitmask of patch outputs that are always written by all invocations
     * in all execution paths.
     *
     * This is useful when a pass wants to read patch output values at the end
     * of the shader. If this is true, the pass doesn't have to insert a barrier
     * and use output loads, it can just use the SSA defs that are being stored
     * (or phis thereof) to get the patch output values.
     */
   uint32_t patch_outputs_defined_by_all_invoc;

   /* The bitmask of patch outputs that are only written by invocation 0. */
   uint32_t patch_outputs_only_written_by_invoc0;

   /* The bitmask of patch outputs that are only read by invocation 0. */
   uint32_t patch_outputs_only_read_by_invoc0;

   /* The bitmask of tess level outputs that are written by all invocations.
    * Bit 0 is outer levels, bit 1 is inner levels.
    */
   uint8_t tess_levels_defined_by_all_invoc : 2;

   /* The bitmask of tess level outputs that are only written by invocation 0. */
   uint8_t tess_levels_only_written_by_invoc0 : 2;

   /* The bitmask of tess level outputs that are only read by invocation 0. */
   uint8_t tess_levels_only_read_by_invoc0 : 2;

   /* Whether all tess levels that are written in all invocations. */
   bool all_invocations_define_tess_levels : 1;

   /* Whether any of the outer tess level components is effectively 0, meaning
    * that the shader discards the patch. NaNs and negative values are included
    * in this. If the patch is discarded, inner tess levels have no effect.
    */
   bool all_tess_levels_are_effectively_zero : 1;

   /* Whether all tess levels are effectively 1, meaning that the tessellator
    * behaves as if they were 1. There is a range of values that lead to that
    * behavior depending on the tessellation spacing.
    */
   bool all_tess_levels_are_effectively_one : 1;

   /* Whether the shader uses a barrier synchronizing TCS output stores.
    * For example, passes that write an output at the beginning of the shader
    * and load it at the end can use this to determine whether they have to
    * insert a barrier or whether the shader already contains a barrier.
    */
   bool always_executes_barrier : 1;

   /* Whether outer tess levels <= 0 can be written by the shader. */
   bool can_discard_patches : 1;
} nir_tcs_info;

void
nir_gather_tcs_info(const nir_shader *nir, nir_tcs_info *info,
                    enum tess_primitive_mode prim,
                    enum gl_tess_spacing spacing);

#ifdef __cplusplus
}
#endif

#endif /* NIR_TCS_INFO_H */
