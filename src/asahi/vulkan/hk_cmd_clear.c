/*
 * Copyright 2024 Valve Corporation
 * Copyright 2024 Alyssa Rosenzweig
 * Copyright 2022-2023 Collabora Ltd. and Red Hat Inc.
 * SPDX-License-Identifier: MIT
 */
#include "util/format/u_formats.h"
#include "util/macros.h"
#include "util/u_math.h"
#include "vulkan/vulkan_core.h"
#include "hk_cmd_buffer.h"
#include "layout.h"

#include "hk_device.h"
#include "hk_entrypoints.h"
#include "hk_image.h"
#include "hk_image_view.h"
#include "hk_physical_device.h"

#include "libagx_dgc.h"
#include "libagx_shaders.h"

void
hk_clear_image(struct hk_cmd_buffer *cmd, struct hk_image *image,
               enum pipe_format view_format, const uint32_t *clear_value,
               const VkImageSubresourceRange *range, bool whole_3d)
{
   const uint32_t level_count =
      vk_image_subresource_level_count(&image->vk, range);

   bool z = (range->aspectMask & VK_IMAGE_ASPECT_DEPTH_BIT);
   bool s = (range->aspectMask & VK_IMAGE_ASPECT_STENCIL_BIT);

   unsigned first_plane = (s && !z) ? 1 : 0;
   unsigned last_plane = s ? 1 : 0;

   if (image->plane_count == 1) {
      first_plane = 0;
      last_plane = 0;
   }

   for (unsigned plane = first_plane; plane <= last_plane; ++plane) {
      struct ail_layout *layout = &image->planes[plane].layout;
      perf_debug(cmd, "Image clear (%scompressed)",
                 layout->compressed ? "" : "un");

      for (uint32_t l = 0; l < level_count; l++) {
         const uint32_t level = range->baseMipLevel + l;

         const VkExtent3D level_extent =
            vk_image_mip_level_extent(&image->vk, level);

         uint32_t base_array_layer, layer_count;
         if (image->vk.image_type == VK_IMAGE_TYPE_3D && whole_3d) {
            base_array_layer = 0;
            layer_count = level_extent.depth;
         } else {
            base_array_layer = range->baseArrayLayer;
            layer_count = vk_image_subresource_layer_count(&image->vk, range);
         }

         enum pipe_format format = view_format ? view_format : layout->format;
         bool stencil = format == PIPE_FORMAT_S8_UINT;

         if (stencil) {
            format = PIPE_FORMAT_R8_UINT;
         } else if (format == PIPE_FORMAT_Z16_UNORM) {
            format = PIPE_FORMAT_R16_UNORM;
         } else if (format == PIPE_FORMAT_Z32_FLOAT) {
            format = PIPE_FORMAT_R32_FLOAT;
         }

         uint32_t c[4];
         util_format_pack_rgba(format, c, &clear_value[stencil ? 1 : 0], 1);

         unsigned blocksize_B = util_format_get_blocksize(format);
         assert(util_is_power_of_two_nonzero(blocksize_B) && blocksize_B <= 16);

         /* Splat out to 128-bit */
         uint8_t *bytes = (uint8_t *)c;
         for (unsigned i = 1; i < 16; ++i) {
            bytes[i] = bytes[i % blocksize_B];
         }

         uint64_t address =
            image->planes[plane].addr +
            ail_get_layer_level_B(layout, base_array_layer, level);

         uint32_t size = ail_get_level_size_B(layout, level);

         assert((layout->layer_stride_B % 16) == 0 && "aligned");
         uint32_t layer_stride_uint4 = layout->layer_stride_B / 16;

         if (ail_is_level_logically_compressed(layout, level)) {
            assert((layout->compression_layer_stride_B % 16) == 0 && "aligned");
            uint32_t meta_layer_stride_tl =
               layout->compression_layer_stride_B / 8;

            uint64_t meta_addr =
               image->planes[plane].addr + layout->metadata_offset_B +
               (base_array_layer * layout->compression_layer_stride_B) +
               layout->level_offsets_compressed_B[level];

            uint32_t word = (uint32_t)ail_tile_mode_solid(format);

            struct agx_grid grid =
               agx_3d(ail_metadata_width_tl(layout, level),
                      ail_metadata_height_tl(layout, level), layer_count);

            struct ail_tile tilesize = layout->tilesize_el[level];

            libagx_fast_clear(cmd, grid, AGX_BARRIER_ALL, meta_addr, address,
                              grid.count[0], grid.count[1], tilesize.width_el,
                              tilesize.height_el, meta_layer_stride_tl,
                              layer_stride_uint4, util_logbase2(blocksize_B),
                              util_logbase2(layout->sample_count_sa), c[0],
                              c[1], c[2], c[3], word);
         } else {
            libagx_fill_uint4(cmd, agx_2d(DIV_ROUND_UP(size, 16), layer_count),
                              AGX_BARRIER_ALL, address, layer_stride_uint4,
                              c[0], c[1], c[2], c[3]);
         }
      }
   }
}

VKAPI_ATTR void VKAPI_CALL
hk_CmdClearColorImage(VkCommandBuffer commandBuffer, VkImage _image,
                      VkImageLayout imageLayout,
                      const VkClearColorValue *pColor, uint32_t rangeCount,
                      const VkImageSubresourceRange *pRanges)
{
   VK_FROM_HANDLE(hk_cmd_buffer, cmd, commandBuffer);
   VK_FROM_HANDLE(hk_image, image, _image);

   for (uint32_t r = 0; r < rangeCount; r++) {
      hk_clear_image(cmd, image, PIPE_FORMAT_NONE, pColor->uint32, &pRanges[r],
                     true /* whole 3D */);
   }
}

VKAPI_ATTR void VKAPI_CALL
hk_CmdClearDepthStencilImage(VkCommandBuffer commandBuffer, VkImage _image,
                             VkImageLayout imageLayout,
                             const VkClearDepthStencilValue *pDepthStencil,
                             uint32_t rangeCount,
                             const VkImageSubresourceRange *pRanges)
{
   VK_FROM_HANDLE(hk_cmd_buffer, cmd, commandBuffer);
   VK_FROM_HANDLE(hk_image, image, _image);

   for (uint32_t r = 0; r < rangeCount; r++) {
      uint32_t colour[4] = {fui(pDepthStencil->depth), pDepthStencil->stencil};
      hk_clear_image(cmd, image, PIPE_FORMAT_NONE, colour, &pRanges[r],
                     true /* whole 3D */);
   }
}
