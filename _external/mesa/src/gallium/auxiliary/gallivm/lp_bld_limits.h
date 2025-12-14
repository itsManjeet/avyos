/**************************************************************************
 *
 * Copyright 2010-2012 VMware, Inc.
 * All Rights Reserved.
 *
 * Permission is hereby granted, free of charge, to any person obtaining a
 * copy of this software and associated documentation files (the
 * "Software"), to deal in the Software without restriction, including
 * without limitation the rights to use, copy, modify, merge, publish,
 * distribute, sub license, and/or sell copies of the Software, and to
 * permit persons to whom the Software is furnished to do so, subject to
 * the following conditions:
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NON-INFRINGEMENT. IN NO EVENT SHALL
 * THE COPYRIGHT HOLDERS, AUTHORS AND/OR ITS SUPPLIERS BE LIABLE FOR ANY CLAIM,
 * DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR
 * OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE
 * USE OR OTHER DEALINGS IN THE SOFTWARE.
 *
 * The above copyright notice and this permission notice (including the
 * next paragraph) shall be included in all copies or substantial portions
 * of the Software.
 *
 **************************************************************************/


#ifndef LP_BLD_LIMITS_H_
#define LP_BLD_LIMITS_H_


#include <limits.h>

#include "pipe/p_state.h"
#include "pipe/p_defines.h"
#include "util/u_cpu_detect.h"

/*
 * llvmpipe shader limits
 */

#define LP_MAX_TGSI_TEMPS 4096

#define LP_MAX_TGSI_ADDRS 16

#define LP_MAX_TGSI_IMMEDIATES 4096

#define LP_MAX_TGSI_CONSTS 4096

#define LP_MAX_TGSI_CONST_BUFFERS 16

#define LP_MAX_TGSI_CONST_BUFFER_SIZE (LP_MAX_TGSI_CONSTS * sizeof(float[4]))

#define LP_MAX_TGSI_SHADER_BUFFERS 32

#define LP_MAX_TGSI_SHADER_BUFFER_SIZE (1 << 27)

#define LP_MAX_TGSI_SHADER_IMAGES 64

/*
 * For quick access we cache registers in statically
 * allocated arrays. Here we define the maximum size
 * for those arrays.
 */
#define LP_MAX_INLINED_TEMPS 256

#define LP_MAX_INLINED_IMMEDIATES 256

/**
 * Maximum control flow nesting
 *
 * Vulkan CTS tests seem to have up to 76 levels. Add a few for safety.
 * SM4.0 requires 64 (per subroutine actually, subroutine nesting itself is 32)
 * SM3.0 requires 24 (most likely per subroutine too)
 * add 2 more (some translation could add one more)
 */
#define LP_MAX_TGSI_NESTING 80

static inline bool
lp_has_fp16(void)
{
   return util_get_cpu_caps()->has_f16c || DETECT_ARCH_AARCH64;
}

/**
 * Some of these limits are actually infinite (i.e., only limited by available
 * memory), however advertising INT_MAX would cause some test problems to
 * actually try to allocate the maximum and run out of memory and crash.  So
 * stick with something reasonable here.
 */
static inline void
gallivm_init_shader_caps(struct pipe_shader_caps *caps)
{
   caps->max_instructions =
   caps->max_alu_instructions =
   caps->max_tex_instructions =
   caps->max_tex_indirections = 1 * 1024 * 1024;
   caps->max_control_flow_depth = LP_MAX_TGSI_NESTING;
   caps->max_inputs = 32;
   caps->max_outputs = 32;
   caps->max_const_buffer0_size = LP_MAX_TGSI_CONST_BUFFER_SIZE;
   caps->max_const_buffers = LP_MAX_TGSI_CONST_BUFFERS;
   caps->max_temps = LP_MAX_TGSI_TEMPS;
   caps->cont_supported = true;
   caps->indirect_temp_addr = true;
   caps->indirect_const_addr = true;
   caps->subroutines = true;
   caps->integers = true;
   caps->fp16 =
   caps->fp16_derivatives = lp_has_fp16();
   //enabling this breaks GTF-GL46.gtf21.GL2Tests.glGetUniform.glGetUniform
   caps->fp16_const_buffers = false;
   caps->int16 = true;
   caps->glsl_16bit_consts = true;
   caps->glsl_16bit_load_dst = true;
   caps->max_texture_samplers = PIPE_MAX_SAMPLERS;
   caps->max_sampler_views = PIPE_MAX_SHADER_SAMPLER_VIEWS;
   caps->supported_irs = (1 << PIPE_SHADER_IR_TGSI) | (1 << PIPE_SHADER_IR_NIR);
   caps->tgsi_sqrt_supported = true;
   caps->tgsi_any_inout_decl_range = true;
   caps->max_shader_buffers = LP_MAX_TGSI_SHADER_BUFFERS;
   caps->max_shader_images = LP_MAX_TGSI_SHADER_IMAGES;
}


#endif /* LP_BLD_LIMITS_H_ */
