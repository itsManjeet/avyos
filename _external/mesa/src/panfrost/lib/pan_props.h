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

#ifndef PAN_PROPS_H
#define PAN_PROPS_H

#include <stdbool.h>
#include <stdint.h>

#include "util/macros.h"

struct pan_kmod_dev;
struct pan_kmod_dev_props;

/** Implementation-defined tiler features */
struct pan_tiler_features {
   /** Number of bytes per tiler bin */
   unsigned bin_size;

   /** Maximum number of levels that may be simultaneously enabled.
    * Invariant: bitcount(hierarchy_mask) <= max_levels */
   unsigned max_levels;
};

#define ARCH_MAJOR     BITFIELD_RANGE(28, 4)
#define ARCH_MINOR     BITFIELD_RANGE(24, 4)
#define ARCH_REV       BITFIELD_RANGE(20, 4)
#define PRODUCT_MAJOR  BITFIELD_RANGE(16, 4)
#define VERSION_MAJOR  BITFIELD_RANGE(12, 4)
#define VERSION_MINOR  BITFIELD_RANGE(4, 8)
#define VERSION_STATUS BITFIELD_RANGE(0, 4)

struct pan_model {
   /* GPU product ID */
   uint32_t gpu_prod_id;

   /* Mask to apply to the GPU ID to get a product ID. */
   uint32_t gpu_prod_id_mask;

   /* GPU variant. */
   uint32_t gpu_variant;

   /* Marketing name for the GPU, used as the GL_RENDERER */
   const char *name;

   /* Set of associated performance counters */
   const char *performance_counters;

   /* Minimum GPU revision required for anisotropic filtering. ~0 and 0
    * means "no revisions support anisotropy" and "all revisions support
    * anistropy" respectively -- so checking for anisotropy is simply
    * comparing the reivsion.
    */
   uint32_t min_rev_anisotropic;

   struct {
      /* Default tilebuffer size in bytes for the model. */
      uint32_t color_size;

      /* Default tilebuffer depth size in bytes for the model. */
      uint32_t z_size;
   } tilebuffer;

   /* Maximum number of pixels, texels, and FMA ops, per clock per shader
    * core, or 0 if it can't be determined for the given GPU. */
   struct {
      uint32_t pixel;
      uint32_t texel;
      uint32_t fma;
   } rates;

   struct {
      /* The GPU lacks the capability for hierarchical tiling, without
       * an "Advanced Tiling Unit", instead requiring a single bin
       * size for the entire framebuffer be selected by the driver
       */
      bool no_hierarchical_tiling;
      bool max_4x_msaa;
   } quirks;
};

const struct pan_model *pan_get_model(uint32_t gpu_id, uint32_t gpu_variant);

unsigned pan_query_l2_slices(const struct pan_kmod_dev_props *props);

struct pan_tiler_features
pan_query_tiler_features(const struct pan_kmod_dev_props *props);

unsigned pan_query_thread_tls_alloc(const struct pan_kmod_dev_props *props);

uint32_t pan_query_compressed_formats(const struct pan_kmod_dev_props *props);

unsigned pan_query_core_count(const struct pan_kmod_dev_props *props,
                              unsigned *core_id_range);

bool pan_query_afbc(const struct pan_kmod_dev_props *props);

bool pan_query_afrc(const struct pan_kmod_dev_props *props);

unsigned pan_query_optimal_tib_size(const struct pan_model *model);

unsigned pan_query_optimal_z_tib_size(const struct pan_model *model);

uint64_t pan_clamp_to_usable_va_range(const struct pan_kmod_dev *dev,
                                      uint64_t va);

unsigned pan_compute_max_thread_count(const struct pan_kmod_dev_props *props,
                                      unsigned work_reg_count);

/* Returns the architecture version given a GPU ID, either from a table for
 * old-style Midgard versions or directly for new-style Bifrost/Valhall
 * versions */

static inline unsigned
pan_arch(unsigned gpu_id)
{
   switch (gpu_id >> 16) {
   case 0x600:
   case 0x620:
   case 0x720:
      return 4;
   case 0x750:
   case 0x820:
   case 0x830:
   case 0x860:
   case 0x880:
      return 5;
   default:
      return gpu_id >> 28;
   }
}

static inline unsigned
pan_max_effective_tile_size(unsigned arch)
{
   if (arch >= 12)
      return 64 * 64;

   if (arch >= 10)
      return 32 * 32;

   return 16 * 16;
}

static inline unsigned
pan_meta_tile_size(unsigned arch)
{
   if (arch >= 12)
      return 64;

   return 32;
}

/* Returns the maximum usable color tilebuffer-size. This is *usually* twice
 * the optimal tilebuffer-size, but not always.
 */
static inline unsigned
pan_get_max_tib_size(unsigned arch, const struct pan_model *model)
{
   unsigned tib_size = pan_query_optimal_tib_size(model);

   /* On V5, as well as V6 and later, we can disable pipelining to gain some
    * extra tib memory.
    */
   if (arch > 4 && arch != 6)
      return tib_size * 2;

   return tib_size;
}

static inline uint32_t
pan_get_max_cbufs(unsigned arch, unsigned max_tib_size)
{
   if (arch < 5)
      return 1;

   const unsigned min_msaa = 4;            /* Vulkan *requires* at least 4x MSAA support */
   const unsigned max_cbuf_format = 4 * 4; /* R32G32B32A32 */
   const unsigned min_tile_size = 4 * 4;

   unsigned max_cbufs =
      max_tib_size / (min_msaa * max_cbuf_format * min_tile_size);

   return MIN2(max_cbufs, 8);
}

static inline unsigned
pan_get_max_msaa(unsigned arch, unsigned max_tib_size, unsigned max_cbuf_atts,
                 unsigned format_size)
{
   if (arch < 5)
      return 8;

   assert(max_cbuf_atts > 0);
   assert(format_size > 0);

   /* When using an internal format with less than 32-bit per pixels, we're
    * currently using either AU (Additional precision, Unorm) or PU (Padded
    * precision, Unorm), meaning that we need additional bits in the tilebuffer
    * that's used by dithering.
    */
   format_size = MAX2(format_size, 4);

   const unsigned min_tile_size = 4 * 4;
   unsigned max_msaa = max_tib_size / (max_cbuf_atts * format_size *
                                       min_tile_size);
   return MIN2(max_msaa, 16);
}

#endif
