/*
 * Copyright (C) 2018 Alyssa Rosenzweig
 * Copyright (C) 2019-2021 Collabora, Ltd.
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

#include "pan_shader.h"
#include "panfrost/clc/pan_compile.h"
#include "pan_blend.h"
#include "pan_format.h"

#include "panfrost/compiler/bifrost_compile.h"
#include "panfrost/midgard/midgard_compile.h"

const nir_shader_compiler_options *
pan_shader_get_compiler_options(unsigned arch)
{
   switch (arch) {
   case 4:
   case 5:
      return &midgard_nir_options;
   case 6:
   case 7:
      return &bifrost_nir_options_v6;
   case 9:
   case 10:
      return &bifrost_nir_options_v9;
   case 11:
   case 12:
   case 13:
      return &bifrost_nir_options_v11;
   default:
      assert(!"Unsupported arch");
      return NULL;
   }
}

void
pan_shader_compile(nir_shader *s, struct pan_compile_inputs *inputs,
                   struct util_dynarray *binary, struct pan_shader_info *info)
{
   unsigned arch = pan_arch(inputs->gpu_id);

   memset(info, 0, sizeof(*info));

   NIR_PASS(_, s, nir_lower_printf_buffer, 0, LIBPAN_PRINTF_BUFFER_SIZE - 8);

   if (arch >= 6)
      bifrost_compile_shader_nir(s, inputs, binary, info);
   else
      midgard_compile_shader_nir(s, inputs, binary, info);

   info->stage = s->info.stage;
   info->contains_barrier =
      s->info.uses_memory_barrier || s->info.uses_control_barrier;
   info->separable = s->info.separate_shader;

   switch (info->stage) {
   case MESA_SHADER_VERTEX:
      info->attributes_read = s->info.inputs_read;
      info->attributes_read_count = util_bitcount64(info->attributes_read);
      info->attribute_count = info->attributes_read_count;

      if (arch <= 5) {
         if (info->midgard.vs.reads_raw_vertex_id)
            info->attribute_count =
               MAX2(info->attribute_count, PAN_VERTEX_ID + 1);

         bool instance_id =
            BITSET_TEST(s->info.system_values_read, SYSTEM_VALUE_INSTANCE_ID);
         if (instance_id)
            info->attribute_count =
               MAX2(info->attribute_count, PAN_INSTANCE_ID + 1);
      }

      info->vs.writes_point_size =
         s->info.outputs_written & VARYING_BIT_PSIZ;

      if (arch >= 9) {
         info->varyings.output_count =
            util_last_bit(s->info.outputs_written >> VARYING_SLOT_VAR0);

         /* Store the mask of special varyings, in case we need to emit ADs
          * later. */
         info->varyings.fixed_varyings =
            pan_get_fixed_varying_mask(s->info.outputs_written);
      }
      break;
   case MESA_SHADER_FRAGMENT:
      if (s->info.outputs_written & BITFIELD64_BIT(FRAG_RESULT_DEPTH))
         info->fs.writes_depth = true;
      if (s->info.outputs_written & BITFIELD64_BIT(FRAG_RESULT_STENCIL))
         info->fs.writes_stencil = true;
      if (s->info.outputs_written & BITFIELD64_BIT(FRAG_RESULT_SAMPLE_MASK))
         info->fs.writes_coverage = true;

      info->fs.outputs_read = s->info.outputs_read >> FRAG_RESULT_DATA0;
      info->fs.outputs_written = s->info.outputs_written >> FRAG_RESULT_DATA0;
      info->fs.sample_shading = s->info.fs.uses_sample_shading;
      info->fs.untyped_color_outputs = s->info.fs.untyped_color_outputs;

      info->fs.can_discard = s->info.fs.uses_discard;
      info->fs.early_fragment_tests = s->info.fs.early_fragment_tests;

      /* List of reasons we need to execute frag shaders when things
       * are masked off */

      info->fs.sidefx = s->info.writes_memory || s->info.fs.uses_discard;

      /* With suitable ZSA/blend, is early-z possible? */
      info->fs.can_early_z = !info->fs.sidefx && !info->fs.writes_depth &&
                             !info->fs.writes_stencil &&
                             !info->fs.writes_coverage;

      /* Similiarly with suitable state, is FPK possible? */
      info->fs.can_fpk = !info->fs.writes_depth && !info->fs.writes_stencil &&
                         !info->fs.writes_coverage && !info->fs.can_discard &&
                         !info->fs.outputs_read;

      /* Requires the same hardware guarantees, so grouped as one bit
       * in the hardware.
       */
      info->contains_barrier |= s->info.fs.needs_coarse_quad_helper_invocations;

      info->fs.reads_frag_coord =
         (s->info.inputs_read & VARYING_BIT_POS) ||
         BITSET_TEST(s->info.system_values_read, SYSTEM_VALUE_FRAG_COORD);
      info->fs.reads_primitive_id =
         (s->info.inputs_read & VARYING_BIT_PRIMITIVE_ID) ||
         BITSET_TEST(s->info.system_values_read, SYSTEM_VALUE_PRIMITIVE_ID);
      info->fs.reads_point_coord =
         s->info.inputs_read & VARYING_BIT_PNTC;
      info->fs.reads_face =
         (s->info.inputs_read & VARYING_BIT_FACE) ||
         BITSET_TEST(s->info.system_values_read, SYSTEM_VALUE_FRONT_FACE);
      if (arch >= 9) {
         info->varyings.input_count =
            util_last_bit(s->info.inputs_read >> VARYING_SLOT_VAR0);

         /* Store the mask of special varyings, in case we need to emit ADs
          * later. */
         info->varyings.fixed_varyings =
            pan_get_fixed_varying_mask(s->info.inputs_read);
      }
      break;
   default:
      /* Everything else treated as compute */
      info->wls_size = s->info.shared_size;
      break;
   }

   info->outputs_written = s->info.outputs_written;
   info->attribute_count += BITSET_LAST_BIT(s->info.images_used);
   info->writes_global = s->info.writes_memory;
   info->ubo_count = s->info.num_ubos;

   info->sampler_count = info->texture_count =
      BITSET_LAST_BIT(s->info.textures_used);

   unsigned execution_mode = s->info.float_controls_execution_mode;
   info->ftz_fp16 = nir_is_denorm_flush_to_zero(execution_mode, 16);
   info->ftz_fp32 = nir_is_denorm_flush_to_zero(execution_mode, 32);

   if (arch >= 9) {
      /* Valhall hardware doesn't have a "flush FP16, preserve FP32" mode, and
       * we don't advertise independent FP16/FP32 denorm modes in panvk, but
       * it's still possible to have shaders that don't specify any denorm mode
       * for FP32. In that case, default to flush FP32. */
      if (info->ftz_fp16 && !info->ftz_fp32) {
         assert(!nir_is_denorm_preserve(execution_mode, 32));
         info->ftz_fp32 = true;
      }
   }

   if (arch >= 6) {
      /* This is "redundant" information, but is needed in a draw-time hot path */
      for (unsigned i = 0; i < ARRAY_SIZE(info->bifrost.blend); ++i) {
         info->bifrost.blend[i].format =
            pan_blend_type_from_nir(info->bifrost.blend[i].type);
      }
   }
}
