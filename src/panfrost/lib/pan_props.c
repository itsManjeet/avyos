/*
 * Copyright (C) 2019 Collabora, Ltd.
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
 *
 * Authors:
 *   Alyssa Rosenzweig <alyssa.rosenzweig@collabora.com>
 */

#include "util/macros.h"

#include "kmod/pan_kmod.h"
#include "panfrost/util/pan_ir.h"
#include "pan_props.h"

#include <genxml/gen_macros.h>

/* GPU revision (rXpY) */
#define GPU_REV(X, Y) (((X) & 0xf) << 12 | ((Y) & 0xff) << 4)

/* Fixed "minimum revisions" */
#define GPU_REV_NONE (~0)
#define GPU_REV_ALL  GPU_REV(0, 0)
#define GPU_REV_R0P3 GPU_REV(0, 3)
#define GPU_REV_R1P1 GPU_REV(1, 1)

#define MODEL(gpu_prod_id_, gpu_prod_id_mask_, gpu_variant_, shortname,        \
              counters, ...)                                                   \
   {                                                                           \
      .gpu_prod_id = gpu_prod_id_,                                             \
      .gpu_prod_id_mask = gpu_prod_id_mask_,                                   \
      .gpu_variant = gpu_variant_,                                             \
      .name = "Mali-" shortname,                                               \
      .performance_counters = counters,                                        \
      ##__VA_ARGS__,                                                           \
   }

#define MIDGARD_MODEL(gpu_prod_id, shortname, counters, ...)                   \
   MODEL(gpu_prod_id << 16, 0xffff0000, 0, shortname, counters, ##__VA_ARGS__)

#define BIFROST_MODEL(gpu_prod_id, shortname, counters, ...)                   \
   MODEL(gpu_prod_id << 16, ARCH_MAJOR | ARCH_MINOR | PRODUCT_MAJOR, 0,        \
         shortname, counters, ##__VA_ARGS__)

#define VALHALL_MODEL(gpu_prod_id, gpu_variant, shortname, counters, ...)      \
   MODEL(gpu_prod_id << 16, ARCH_MAJOR | ARCH_MINOR | PRODUCT_MAJOR,           \
         gpu_variant, shortname, counters, ##__VA_ARGS__)

#define AVALON_MODEL(gpu_prod_id, gpu_variant, shortname, counters, ...)       \
   MODEL(gpu_prod_id << 16, ARCH_MAJOR | ARCH_MINOR | PRODUCT_MAJOR,           \
         gpu_variant, shortname, counters, ##__VA_ARGS__)

#define MODEL_ANISO(rev) .min_rev_anisotropic = GPU_REV_##rev

#define MODEL_TB_SIZES(color_tb_size, z_tb_size)                               \
   .tilebuffer = {                                                             \
      .color_size = color_tb_size,                                             \
      .z_size = z_tb_size,                                                     \
   }

#define MODEL_RATES(pixel_rate, texel_rate, fma_rate)                          \
   .rates = {                                                                  \
      .pixel = pixel_rate,                                                     \
      .texel = texel_rate,                                                     \
      .fma = fma_rate,                                                         \
   }

#define MODEL_QUIRKS(...) .quirks = {__VA_ARGS__}

/* Table of supported Mali GPUs */
/* clang-format off */
const struct pan_model pan_model_list[] = {
   MIDGARD_MODEL(0x600,     "T600",   "T60x", MODEL_ANISO(NONE), MODEL_TB_SIZES( 8192,  8192),
                                              MODEL_QUIRKS( .max_4x_msaa = true )),
   MIDGARD_MODEL(0x620,     "T620",   "T62x", MODEL_ANISO(NONE), MODEL_TB_SIZES( 8192,  8192)),
   MIDGARD_MODEL(0x720,     "T720",   "T72x", MODEL_ANISO(NONE), MODEL_TB_SIZES( 8192,  8192),
                                              MODEL_QUIRKS( .no_hierarchical_tiling = true, .max_4x_msaa = true )),
   MIDGARD_MODEL(0x750,     "T760",   "T76x", MODEL_ANISO(NONE), MODEL_TB_SIZES( 8192,  8192)),
   MIDGARD_MODEL(0x820,     "T820",   "T82x", MODEL_ANISO(NONE), MODEL_TB_SIZES( 8192,  8192),
                                              MODEL_QUIRKS( .no_hierarchical_tiling = true, .max_4x_msaa = true )),
   MIDGARD_MODEL(0x830,     "T830",   "T83x", MODEL_ANISO(NONE), MODEL_TB_SIZES( 8192,  8192),
                                              MODEL_QUIRKS( .no_hierarchical_tiling = true, .max_4x_msaa = true )),
   MIDGARD_MODEL(0x860,     "T860",   "T86x", MODEL_ANISO(NONE), MODEL_TB_SIZES( 8192,  8192)),
   MIDGARD_MODEL(0x880,     "T880",   "T88x", MODEL_ANISO(NONE), MODEL_TB_SIZES( 8192,  8192)),

   BIFROST_MODEL(0x6000,    "G71",    "TMIx", MODEL_ANISO(NONE), MODEL_TB_SIZES( 8192,  8192)),
   BIFROST_MODEL(0x6201,    "G72",    "THEx", MODEL_ANISO(R0P3), MODEL_TB_SIZES(16384,  8192)),
   BIFROST_MODEL(0x7000,    "G51",    "TSIx", MODEL_ANISO(R1P1), MODEL_TB_SIZES( 8192,  8192)),
   BIFROST_MODEL(0x7003,    "G31",    "TDVx", MODEL_ANISO(ALL),  MODEL_TB_SIZES( 8192,  8192)),
   BIFROST_MODEL(0x7201,    "G76",    "TNOx", MODEL_ANISO(ALL),  MODEL_TB_SIZES(16384,  8192)),
   BIFROST_MODEL(0x7202,    "G52",    "TGOx", MODEL_ANISO(ALL),  MODEL_TB_SIZES(16384,  8192)),
   BIFROST_MODEL(0x7402,    "G52 r1", "TGOx", MODEL_ANISO(ALL),  MODEL_TB_SIZES( 8192,  8192)),

   VALHALL_MODEL(0x9001, 0, "G57",    "TNAx", MODEL_ANISO(ALL),  MODEL_TB_SIZES(16384,  8192),
                                              MODEL_RATES(2, 4,  32)),
   VALHALL_MODEL(0x9003, 0, "G57",    "TNAx", MODEL_ANISO(ALL),  MODEL_TB_SIZES(16384,  8192),
                                              MODEL_RATES(2, 4,  32)),
   VALHALL_MODEL(0xa807, 0, "G610",   "TVIx", MODEL_ANISO(ALL),  MODEL_TB_SIZES(32768, 16384),
                                              MODEL_RATES(4, 8,  64)),
   VALHALL_MODEL(0xac04, 0, "G310",   "TVAx", MODEL_ANISO(ALL),  MODEL_TB_SIZES(16384,  8192),
                                              MODEL_RATES(2, 2,  16)),
   VALHALL_MODEL(0xac04, 1, "G310",   "TVAx", MODEL_ANISO(ALL),  MODEL_TB_SIZES(16384,  8192),
                                              MODEL_RATES(2, 4,  32)),
   VALHALL_MODEL(0xac04, 2, "G310",   "TVAx", MODEL_ANISO(ALL),  MODEL_TB_SIZES(16384,  8192),
                                              MODEL_RATES(4, 4,  48)),
   VALHALL_MODEL(0xac04, 3, "G310",   "TVAx", MODEL_ANISO(ALL),  MODEL_TB_SIZES(32768, 16384),
                                              MODEL_RATES(4, 8,  48)),
   VALHALL_MODEL(0xac04, 4, "G310",   "TVAx", MODEL_ANISO(ALL),  MODEL_TB_SIZES(32768, 16384),
                                              MODEL_RATES(4, 8,  64)),

   AVALON_MODEL( 0xc800, 4, "G720",   "TTIx", MODEL_ANISO(ALL),  MODEL_TB_SIZES(65536, 32768),
                                              MODEL_RATES(4, 8, 128)),
   AVALON_MODEL( 0xd800, 4, "G725",   "TKRx", MODEL_ANISO(ALL),  MODEL_TB_SIZES(65536, 65536),
                                              MODEL_RATES(4, 8, 128)),
};
/* clang-format on */

#undef GPU_REV
#undef GPU_REV_NONE
#undef GPU_REV_ALL
#undef GPU_REV_R0P3
#undef GPU_REV_R1P1

#undef MIDGARD_MODEL
#undef BIFROST_MODEL
#undef VALHALL_MODEL
#undef AVALON_MODEL
#undef MODEL

#undef MODEL_ANISO
#undef MODEL_TB_SIZES
#undef MODEL_RATES
#undef MODEL_QUIRKS

/*
 * Look up a supported model by its GPU ID, or return NULL if the model is not
 * supported at this time.
 */
const struct pan_model *
pan_get_model(uint32_t gpu_id, uint32_t gpu_variant)
{
   for (unsigned i = 0; i < ARRAY_SIZE(pan_model_list); ++i) {
      uint32_t gpu_prod_id = gpu_id & pan_model_list[i].gpu_prod_id_mask;

      if (pan_model_list[i].gpu_prod_id == gpu_prod_id &&
          pan_model_list[i].gpu_variant == gpu_variant)
         return &pan_model_list[i];
   }

   return NULL;
}

unsigned
pan_query_l2_slices(const struct pan_kmod_dev_props *props)
{
   /* L2_SLICES is MEM_FEATURES[11:8] minus(1) */
   return ((props->mem_features >> 8) & 0xF) + 1;
}

struct pan_tiler_features
pan_query_tiler_features(const struct pan_kmod_dev_props *props)
{
   /* Default value (2^9 bytes and 8 levels) to match old behaviour */
   uint32_t raw = props->tiler_features;

   /* Bin size is log2 in the first byte, max levels in the second byte */
   return (struct pan_tiler_features){
      .bin_size = (1 << (raw & BITFIELD_MASK(5))),
      .max_levels = (raw >> 8) & BITFIELD_MASK(4),
   };
}

unsigned
pan_query_core_count(const struct pan_kmod_dev_props *props,
                     unsigned *core_id_range)
{
   /* On older kernels, worst-case to 16 cores */

   unsigned mask = props->shader_present;

   /* Some cores might be absent. In some cases, we care
    * about the range of core IDs (that is, the greatest core ID + 1). If
    * the core mask is contiguous, this equals the core count.
    */
   *core_id_range = util_last_bit(mask);

   /* The actual core count skips overs the gaps */
   return util_bitcount(mask);
}

unsigned
pan_query_thread_tls_alloc(const struct pan_kmod_dev_props *props)
{
   return props->max_tls_instance_per_core ?: props->max_threads_per_core;
}

unsigned
pan_compute_max_thread_count(const struct pan_kmod_dev_props *props,
                             unsigned work_reg_count)
{
   unsigned aligned_reg_count;

   /* 4, 8 or 16 registers per shader on Midgard
    * 32 or 64 registers per shader on Bifrost
    */
   if (pan_arch(props->gpu_id) <= 5) {
      aligned_reg_count = util_next_power_of_two(MAX2(work_reg_count, 4));
      assert(aligned_reg_count <= 16);
   } else {
      aligned_reg_count = work_reg_count <= 32 ? 32 : 64;
   }

   return MIN3(props->max_threads_per_wg, props->max_threads_per_core,
               props->num_registers_per_core / aligned_reg_count);
}

uint32_t
pan_query_compressed_formats(const struct pan_kmod_dev_props *props)
{
   return props->texture_features[0];
}

/* Check for AFBC hardware support. AFBC is introduced in v5. Implementations
 * may omit it, signaled as a nonzero value in the AFBC_FEATURES property. */

bool
pan_query_afbc(const struct pan_kmod_dev_props *props)
{
   unsigned reg = props->afbc_features;

   return (pan_arch(props->gpu_id) >= 5) && (reg == 0);
}

/* Check for AFRC hardware support. AFRC is introduced in v10. Implementations
 * may omit it, signaled in bit 25 of TEXTURE_FEATURES_0 property. */

bool
pan_query_afrc(const struct pan_kmod_dev_props *props)
{
   return (pan_arch(props->gpu_id) >= 10) &&
          (props->texture_features[0] & (1 << 25));
}

/*
 * To pipeline multiple tiles, a given tile may use at most half of the tile
 * buffer. This function returns the optimal size (assuming pipelining).
 *
 * For Mali-G510 and Mali-G310, we will need extra logic to query the tilebuffer
 * size for the particular variant. The CORE_FEATURES register might help.
 */
unsigned
pan_query_optimal_tib_size(const struct pan_model *model)
{
   /* Preconditions ensure the returned value is a multiple of 1 KiB, the
    * granularity of the colour buffer allocation field.
    */
   assert(model->tilebuffer.color_size >= 2048);
   assert(util_is_power_of_two_nonzero(model->tilebuffer.color_size));

   return model->tilebuffer.color_size / 2;
}

unsigned
pan_query_optimal_z_tib_size(const struct pan_model *model)
{
   /* Preconditions ensure the returned value is a multiple of 1 KiB, the
    * granularity of the colour buffer allocation field.
    */
   assert(model->tilebuffer.z_size >= 1024);
   assert(util_is_power_of_two_nonzero(model->tilebuffer.z_size));

   return model->tilebuffer.z_size / 2;
}

uint64_t
pan_clamp_to_usable_va_range(const struct pan_kmod_dev *dev, uint64_t va)
{
   struct pan_kmod_va_range user_va_range =
      pan_kmod_dev_query_user_va_range(dev);

   if (va < user_va_range.start)
      return user_va_range.start;
   else if (va > user_va_range.start + user_va_range.size)
      return user_va_range.start + user_va_range.size;

   return va;
}
