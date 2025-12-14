/*
 * Copyright © 2021 Collabora Ltd.
 *
 * Derived from tu_image.c which is:
 * Copyright © 2016 Red Hat.
 * Copyright © 2016 Bas Nieuwenhuizen
 * Copyright © 2015 Intel Corporation
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
 * FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
 * DEALINGS IN THE SOFTWARE.
 */

#include "pan_afbc.h"
#include "pan_props.h"

#include "panvk_device.h"
#include "panvk_device_memory.h"
#include "panvk_entrypoints.h"
#include "panvk_image.h"
#include "panvk_instance.h"
#include "panvk_physical_device.h"

#include "drm-uapi/drm_fourcc.h"
#include "util/u_atomic.h"
#include "util/u_debug.h"
#include "util/u_drm.h"

#include "vk_format.h"
#include "vk_log.h"
#include "vk_object.h"
#include "vk_util.h"

bool
panvk_image_can_use_afbc(
   struct panvk_physical_device *phys_dev, VkFormat fmt,
   VkImageUsageFlags usage, VkImageType type, VkImageTiling tiling,
   VkImageCreateFlags flags)
{
   unsigned arch = pan_arch(phys_dev->kmod.props.gpu_id);
   struct panvk_instance *instance = to_panvk_instance(phys_dev->vk.instance);
   enum pipe_format pfmt = vk_format_to_pipe_format(fmt);

   /* Disallow AFBC if either of these is true
    * - PANVK_DEBUG does not have the 'afbc' flag set
    * - storage image views are requested
    * - host image copies are requested
    * - the GPU doesn't support AFBC
    * - the format is not AFBC-able
    * - tiling is set to linear
    * - this is a 1D image
    * - this is a 3D image on a pre-v7 GPU
    * - this is a mutable format image on v7
    */
   return
      (instance->debug_flags & PANVK_DEBUG_AFBC) &&
      !(usage & (VK_IMAGE_USAGE_STORAGE_BIT |
                 VK_IMAGE_USAGE_HOST_TRANSFER_BIT)) &&
      pan_query_afbc(&phys_dev->kmod.props) &&
      pan_afbc_supports_format(arch, pfmt) &&
      tiling == VK_IMAGE_TILING_OPTIMAL &&
      type != VK_IMAGE_TYPE_1D &&
      (type != VK_IMAGE_TYPE_3D || arch >= 7) &&
      (!(flags & VK_IMAGE_CREATE_MUTABLE_FORMAT_BIT) || arch != 7);
}

static bool
panvk_image_can_use_mod(struct panvk_image *image, uint64_t mod)
{
   struct panvk_physical_device *phys_dev =
      to_panvk_physical_device(image->vk.base.device->physical);
   struct panvk_instance *instance =
      to_panvk_instance(image->vk.base.device->physical->instance);
   enum pipe_format pfmt = vk_format_to_pipe_format(image->vk.format);
   bool forced_linear = (instance->debug_flags & PANVK_DEBUG_LINEAR) ||
                        image->vk.tiling == VK_IMAGE_TILING_LINEAR ||
                        image->vk.image_type == VK_IMAGE_TYPE_1D;

   /* If the image is meant to be linear, don't bother testing the
    * other cases. */
   if (forced_linear)
      return mod == DRM_FORMAT_MOD_LINEAR;

   if (drm_is_afbc(mod)) {
      if (!panvk_image_can_use_afbc(
            phys_dev, image->vk.format,
            image->vk.usage | image->vk.stencil_usage, image->vk.image_type,
            image->vk.tiling, image->vk.create_flags))
         return false;

      /* AFBC is not supported for multisample images
       *
       * This is not checked in panvk_image_can_use_afbc because the sample
       * count is not known in vkGetPhysicalDeviceImageFormatProperties2. */
      if (image->vk.samples > 1)
         return false;

      const struct util_format_description *fdesc =
         util_format_description(pfmt);
      bool is_rgb = fdesc->colorspace == UTIL_FORMAT_COLORSPACE_RGB ||
                    fdesc->colorspace == UTIL_FORMAT_COLORSPACE_SRGB;

      if ((mod & AFBC_FORMAT_MOD_YTR) && (!is_rgb || fdesc->nr_channels >= 3))
         return false;

      /* AFBC headers point to their tile with a 32-bit offset, so we can't
       * have a body size that's bigger than UINT32_MAX. */
      uint64_t body_size = (uint64_t)image->vk.extent.width *
                           image->vk.extent.height * image->vk.extent.depth *
                           util_format_get_blocksize(pfmt);
      if (body_size > UINT32_MAX)
         return false;

      /* We assume all other unsupported AFBC modes have been filtered out
       * through pan_best_modifiers[]. */
      return true;
   }

   /* Some formats can only be used with AFBC. */
   if (!pan_u_tiled_or_linear_supports_format(pfmt))
      return false;

   if (mod == DRM_FORMAT_MOD_ARM_16X16_BLOCK_U_INTERLEAVED) {
      /* Multiplanar YUV with U-interleaving isn't supported by the HW. We
       * also need to make sure images that can be aliased to planes of
       * multi-planar images remain compatible with the aliased images, so
       * don't allow U-interleaving for those either.
       */
      if (vk_format_get_plane_count(image->vk.format) > 1 ||
          vk_image_can_be_aliased_to_yuv_plane(&image->vk))
         return false;

      /* If we're dealing with a compressed format that requires non-compressed
       * views we can't use U_INTERLEAVED tiling because the tiling is different
       * between compressed and non-compressed formats. If we wanted to support
       * format re-interpretation we would have to specialize the shaders
       * accessing non-compressed image views (coordinate patching for
       * sampled/storage image, frag_coord patching for color attachments). Let's
       * keep things simple for now and make all compressed images that
       * have VK_IMAGE_CREATE_BLOCK_TEXEL_VIEW_COMPATIBLE_BIT set linear. */
      return !(image->vk.create_flags &
               VK_IMAGE_CREATE_BLOCK_TEXEL_VIEW_COMPATIBLE_BIT);
   }

   /* If we get there, it must be linear to be supported. */
   return mod == DRM_FORMAT_MOD_LINEAR;
}

static uint64_t
panvk_image_get_explicit_mod(
   struct panvk_image *image,
   const VkImageDrmFormatModifierExplicitCreateInfoEXT *explicit)
{
   uint64_t mod = explicit->drmFormatModifier;

   assert(!vk_format_is_depth_or_stencil(image->vk.format));
   assert(image->vk.samples == 1);
   assert(image->vk.array_layers == 1);
   assert(image->vk.image_type != VK_IMAGE_TYPE_3D);
   assert(explicit->drmFormatModifierPlaneCount == 1);
   assert(panvk_image_can_use_mod(image, mod));

   return mod;
}

static uint64_t
panvk_image_get_mod_from_list(struct panvk_image *image,
                              const uint64_t *mods, uint32_t mod_count)
{
   PAN_SUPPORTED_MODIFIERS(supported_mods);

   for (unsigned i = 0; i < ARRAY_SIZE(supported_mods); ++i) {
      if (!panvk_image_can_use_mod(image, supported_mods[i]))
         continue;

      if (!mod_count ||
          drm_find_modifier(supported_mods[i], mods, mod_count))
         return supported_mods[i];
   }

   /* If we reached that point without finding a proper modifier, there's
    * a serious issue. */
   assert(!"Invalid modifier");
   return DRM_FORMAT_MOD_INVALID;
}

static uint64_t
panvk_image_get_mod(struct panvk_image *image,
                    const VkImageCreateInfo *pCreateInfo)
{
   if (pCreateInfo->tiling == VK_IMAGE_TILING_DRM_FORMAT_MODIFIER_EXT) {
      const VkImageDrmFormatModifierListCreateInfoEXT *mod_list =
         vk_find_struct_const(pCreateInfo->pNext,
                              IMAGE_DRM_FORMAT_MODIFIER_LIST_CREATE_INFO_EXT);
      const VkImageDrmFormatModifierExplicitCreateInfoEXT *explicit_mod =
         vk_find_struct_const(
            pCreateInfo->pNext,
            IMAGE_DRM_FORMAT_MODIFIER_EXPLICIT_CREATE_INFO_EXT);

      if (explicit_mod)
         return panvk_image_get_explicit_mod(image, explicit_mod);

      if (mod_list)
         return panvk_image_get_mod_from_list(image,
                   mod_list->pDrmFormatModifiers,
                   mod_list->drmFormatModifierCount);

      assert(!"Missing modifier info");
   }

   return panvk_image_get_mod_from_list(image, NULL, 0);
}

static enum mali_texture_dimension
panvk_image_type_to_mali_tex_dim(VkImageType type)
{
   switch (type) {
   case VK_IMAGE_TYPE_1D:
      return MALI_TEXTURE_DIMENSION_1D;
   case VK_IMAGE_TYPE_2D:
      return MALI_TEXTURE_DIMENSION_2D;
   case VK_IMAGE_TYPE_3D:
      return MALI_TEXTURE_DIMENSION_3D;
   default:
      unreachable("Invalid image type");
   }
}

static bool
is_disjoint(const struct panvk_image *image)
{
   assert((image->plane_count > 1 &&
           image->vk.format != VK_FORMAT_D32_SFLOAT_S8_UINT) ||
          (image->vk.create_flags & VK_IMAGE_CREATE_ALIAS_BIT) ||
          !(image->vk.create_flags & VK_IMAGE_CREATE_DISJOINT_BIT));
   return image->vk.create_flags & VK_IMAGE_CREATE_DISJOINT_BIT;
}

static bool
strict_import(struct panvk_image *image, uint32_t plane)
{
   /* We can't do strict imports for AFBC because a Vulkan-based compositor
    * might be importing buffers from clients that are relying on the old
    * behavior. The only exception is AFBC(YUV) because support for these
    * formats was added after we started enforcing WSI pitch. */
   if (drm_is_afbc(image->vk.drm_format_mod) &&
       !pan_format_is_yuv(image->planes[plane].image.props.format))
      return false;

   return true;
}

static VkResult
panvk_image_init_layouts(struct panvk_image *image,
                         const VkImageCreateInfo *pCreateInfo)
{
   struct panvk_physical_device *phys_dev =
      to_panvk_physical_device(image->vk.base.device->physical);
   unsigned arch = pan_arch(phys_dev->kmod.props.gpu_id);
   const VkImageDrmFormatModifierExplicitCreateInfoEXT *explicit_info =
      vk_find_struct_const(
         pCreateInfo->pNext,
         IMAGE_DRM_FORMAT_MODIFIER_EXPLICIT_CREATE_INFO_EXT);

   image->plane_count = vk_format_get_plane_count(pCreateInfo->format);

   /* Z32_S8X24 is not supported on v9+, and we don't want to use it
    * on v7- anyway, because it's less efficient than the multiplanar
    * alternative.
    */
   if (image->vk.format == VK_FORMAT_D32_SFLOAT_S8_UINT)
      image->plane_count = 2;

   const struct pan_mod_handler *mod_handler =
      pan_mod_get_handler(arch, image->vk.drm_format_mod);
   struct pan_image_layout_constraints plane_layout = {
      .offset_B = 0,
   };
   for (uint8_t plane = 0; plane < image->plane_count; plane++) {
      VkFormat format;

      if (image->vk.format == VK_FORMAT_D32_SFLOAT_S8_UINT)
         format = plane == 0 ? VK_FORMAT_D32_SFLOAT : VK_FORMAT_S8_UINT;
      else
         format = vk_format_get_plane_format(image->vk.format, plane);

      if (explicit_info) {
         plane_layout = (struct pan_image_layout_constraints){
            .offset_B = explicit_info->pPlaneLayouts[plane].offset,
            .wsi_row_pitch_B = explicit_info->pPlaneLayouts[plane].rowPitch,
         };
      }

      image->planes[plane].image = (struct pan_image){
         .props = {
            .modifier = image->vk.drm_format_mod,
            .format = vk_format_to_pipe_format(format),
            .dim = panvk_image_type_to_mali_tex_dim(image->vk.image_type),
            .extent_px = {
               .width = vk_format_get_plane_width(image->vk.format, plane,
                                                  image->vk.extent.width),
               .height = vk_format_get_plane_height(image->vk.format, plane,
                                                    image->vk.extent.height),
               .depth = image->vk.extent.depth,
            },
            .array_size = image->vk.array_layers,
            .nr_samples = image->vk.samples,
            .nr_slices = image->vk.mip_levels,
         },
         .mod_handler = mod_handler,
         .planes = {&image->planes[plane].plane},
      };

      plane_layout.strict = strict_import(image, plane);
      if (!pan_image_layout_init(arch, &image->planes[plane].image, 0,
                                 &plane_layout)) {
         return panvk_error(image->vk.base.device,
                            VK_ERROR_INITIALIZATION_FAILED);
      }

      if (!is_disjoint(image) && !explicit_info)
         plane_layout.offset_B += image->planes[plane].plane.layout.data_size_B;
   }

   return VK_SUCCESS;
}

static void
panvk_image_pre_mod_select_meta_adjustments(struct panvk_image *image)
{
   const VkImageAspectFlags aspects = vk_format_aspects(image->vk.format);
   const VkImageUsageFlags all_usage =
      image->vk.usage | image->vk.stencil_usage;

   /* We do image blit/resolve with vk_meta, so when an image is flagged as
    * being a potential transfer source, we also need to add the sampled usage.
    */
   if (image->vk.usage & VK_IMAGE_USAGE_TRANSFER_SRC_BIT)
      image->vk.usage |= VK_IMAGE_USAGE_SAMPLED_BIT;
   if (image->vk.stencil_usage & VK_IMAGE_USAGE_TRANSFER_SRC_BIT)
      image->vk.stencil_usage |= VK_IMAGE_USAGE_SAMPLED_BIT;

   /* Similarly, image that can be a transfer destination can be attached
    * as a color or depth-stencil attachment by vk_meta. */
   if (image->vk.usage & VK_IMAGE_USAGE_TRANSFER_DST_BIT) {
      if (aspects & VK_IMAGE_ASPECT_DEPTH_BIT)
         image->vk.usage |= VK_IMAGE_USAGE_DEPTH_STENCIL_ATTACHMENT_BIT;

      if (aspects & VK_IMAGE_ASPECT_COLOR_BIT) {
         image->vk.usage |= VK_IMAGE_USAGE_COLOR_ATTACHMENT_BIT;
         image->vk.usage |= VK_IMAGE_USAGE_STORAGE_BIT;
      }
   }

   if (image->vk.stencil_usage & VK_IMAGE_USAGE_TRANSFER_DST_BIT)
      image->vk.stencil_usage |= VK_IMAGE_USAGE_DEPTH_STENCIL_ATTACHMENT_BIT;

   /* vk_meta creates 2D array views of 3D images. */
   if (all_usage & VK_IMAGE_USAGE_TRANSFER_DST_BIT &&
       image->vk.image_type == VK_IMAGE_TYPE_3D)
      image->vk.create_flags |= VK_IMAGE_CREATE_2D_ARRAY_COMPATIBLE_BIT;

   /* Needed for resolve operations. */
   if (image->vk.usage & VK_IMAGE_USAGE_COLOR_ATTACHMENT_BIT)
      image->vk.usage |= VK_IMAGE_USAGE_SAMPLED_BIT;

   if (image->vk.usage & VK_IMAGE_USAGE_DEPTH_STENCIL_ATTACHMENT_BIT &&
       aspects & VK_IMAGE_ASPECT_DEPTH_BIT)
      image->vk.usage |= VK_IMAGE_USAGE_SAMPLED_BIT;

   if (image->vk.stencil_usage & VK_IMAGE_USAGE_DEPTH_STENCIL_ATTACHMENT_BIT)
      image->vk.stencil_usage |= VK_IMAGE_USAGE_SAMPLED_BIT;

   if ((image->vk.usage &
        (VK_IMAGE_USAGE_TRANSFER_DST_BIT | VK_IMAGE_USAGE_TRANSFER_SRC_BIT)) &&
       vk_format_is_compressed(image->vk.format)) {
      /* We need to be able to create RGBA views of compressed formats for
       * vk_meta copies. */
      image->vk.create_flags |= VK_IMAGE_CREATE_MUTABLE_FORMAT_BIT |
                                VK_IMAGE_CREATE_BLOCK_TEXEL_VIEW_COMPATIBLE_BIT;
   }
}

static uint64_t
panvk_image_get_total_size(const struct panvk_image *image)
{
   uint64_t size = 0;
   for (uint8_t plane = 0; plane < image->plane_count; plane++) {
      const struct pan_image_layout *layout =
         &image->planes[plane].plane.layout;
      size = MAX2(size, layout->slices[0].offset_B + layout->data_size_B);
   }
   return size;
}

static VkResult
panvk_image_init(struct panvk_image *image,
                 const VkImageCreateInfo *pCreateInfo)
{
   /* Add any create/usage flags that might be needed for meta operations.
    * This is run before the modifier selection because some
    * usage/create_flags influence the modifier selection logic. */
   panvk_image_pre_mod_select_meta_adjustments(image);

   /* Now that we've patched the create/usage flags, we can proceed with the
    * modifier selection. */
   image->vk.drm_format_mod = panvk_image_get_mod(image, pCreateInfo);
   return panvk_image_init_layouts(image, pCreateInfo);
}

static VkResult
panvk_image_plane_bind(struct panvk_device *dev,
                       struct panvk_image_plane *plane,
                       struct panvk_device_memory *mem, uint64_t offset)
{
   plane->plane.base = mem->addr.dev + offset;
   plane->mem = mem;
   plane->mem_offset = offset;
   /* Reset the AFBC headers */
   if (drm_is_afbc(plane->image.props.modifier)) {
      /* Transient CPU mapping */
      void *bo_base = pan_kmod_bo_mmap(mem->bo, 0, pan_kmod_bo_size(mem->bo),
                                       PROT_WRITE, MAP_SHARED, NULL);

      if (bo_base == MAP_FAILED)
         return panvk_errorf(dev, VK_ERROR_OUT_OF_HOST_MEMORY,
                             "Failed to CPU map AFBC image plane");

      for (unsigned layer = 0; layer < plane->image.props.array_size;
           layer++) {
         for (unsigned level = 0; level < plane->image.props.nr_slices;
              level++) {
            const struct pan_image_slice_layout *slayout =
               &plane->plane.layout.slices[level];
            uint32_t z_slice_count =
               u_minify(plane->image.props.extent_px.depth, level);

            for (unsigned z = 0; z < z_slice_count; z++) {
               void *header = bo_base + offset +
                              ((uint64_t)slayout->afbc.surface_stride_B * z) +
                              (layer * plane->plane.layout.array_stride_B) +
                              plane->plane.layout.slices[level].offset_B;
               memset(header, 0, slayout->afbc.header.surface_size_B);
            }
         }
      }

      ASSERTED int ret = os_munmap(bo_base, pan_kmod_bo_size(mem->bo));
      assert(!ret);
   }

   return VK_SUCCESS;
}

VKAPI_ATTR VkResult VKAPI_CALL
panvk_CreateImage(VkDevice device, const VkImageCreateInfo *pCreateInfo,
                  const VkAllocationCallbacks *pAllocator, VkImage *pImage)
{
   VK_FROM_HANDLE(panvk_device, dev, device);
   struct panvk_physical_device *phys_dev =
      to_panvk_physical_device(dev->vk.physical);

   const VkImageSwapchainCreateInfoKHR *swapchain_info =
      vk_find_struct_const(pCreateInfo->pNext, IMAGE_SWAPCHAIN_CREATE_INFO_KHR);
   if (swapchain_info && swapchain_info->swapchain != VK_NULL_HANDLE) {
      return wsi_common_create_swapchain_image(&phys_dev->wsi_device,
                                               pCreateInfo,
                                               swapchain_info->swapchain,
                                               pImage);
   }

   struct panvk_image *image =
      vk_image_create(&dev->vk, pCreateInfo, pAllocator, sizeof(*image));
   if (!image)
      return panvk_error(device, VK_ERROR_OUT_OF_HOST_MEMORY);

   VkResult result = panvk_image_init(image, pCreateInfo);
   if (result != VK_SUCCESS) {
      vk_image_destroy(&dev->vk, pAllocator, &image->vk);
      return result;
   }

   /*
    * From the Vulkan spec:
    *
    *    If the size of the resultant image would exceed maxResourceSize, then
    *    vkCreateImage must fail and return VK_ERROR_OUT_OF_DEVICE_MEMORY.
    */
   if (panvk_image_get_total_size(image) > UINT32_MAX) {
      vk_image_destroy(&dev->vk, pAllocator, &image->vk);
      return panvk_error(device, VK_ERROR_OUT_OF_DEVICE_MEMORY);
   }

   *pImage = panvk_image_to_handle(image);
   return VK_SUCCESS;
}

VKAPI_ATTR void VKAPI_CALL
panvk_DestroyImage(VkDevice _device, VkImage _image,
                   const VkAllocationCallbacks *pAllocator)
{
   VK_FROM_HANDLE(panvk_device, device, _device);
   VK_FROM_HANDLE(panvk_image, image, _image);

   if (!image)
      return;

   vk_image_destroy(&device->vk, pAllocator, &image->vk);
}

static void
get_image_subresource_layout(const struct panvk_image *image,
                             const VkImageSubresource2 *subres2,
                             VkSubresourceLayout2 *layout2)
{
   const VkImageSubresource *subres = &subres2->imageSubresource;
   VkSubresourceLayout *layout = &layout2->subresourceLayout;
   unsigned plane = panvk_plane_index(image->vk.format, subres->aspectMask);
   assert(plane < PANVK_MAX_PLANES);

   const struct pan_image_slice_layout *slice_layout =
      &image->planes[plane].plane.layout.slices[subres->mipLevel];

   layout->offset =
      slice_layout->offset_B +
      (subres->arrayLayer * image->planes[plane].plane.layout.array_stride_B);
   layout->size = slice_layout->size_B;
   layout->arrayPitch = image->planes[plane].plane.layout.array_stride_B;

   if (drm_is_afbc(image->vk.drm_format_mod)) {
      /* row/depth pitch expressed in AFBC superblocks. */
      layout->rowPitch = pan_afbc_stride_blocks(
         image->vk.drm_format_mod, slice_layout->afbc.header.row_stride_B);
      layout->depthPitch = pan_afbc_stride_blocks(
         image->vk.drm_format_mod, slice_layout->afbc.header.surface_size_B);
   } else {
      layout->rowPitch = slice_layout->tiled_or_linear.row_stride_B;
      layout->depthPitch = slice_layout->tiled_or_linear.surface_stride_B;
   }

   VkSubresourceHostMemcpySize *memcpy_size =
      vk_find_struct(layout2->pNext, SUBRESOURCE_HOST_MEMCPY_SIZE);
   if (memcpy_size) {
      /* When copying to/from a D24S8 image, we can't use the normal memcpy
       * path because we need to interleave the depth/stencil components. For
       * the stencil aspect, the copied data only needs 1 byte/px instead of 4.
       */
      if (image->vk.format == VK_FORMAT_D24_UNORM_S8_UINT) {
         switch (subres->aspectMask) {
            case VK_IMAGE_ASPECT_DEPTH_BIT:
               memcpy_size->size = slice_layout->size_B;
               break;
            case VK_IMAGE_ASPECT_STENCIL_BIT:
               memcpy_size->size = slice_layout->size_B / 4;
               break;
            default:
               unreachable("invalid aspect");
         }
      } else {
         memcpy_size->size = slice_layout->size_B;
      }
   }
}

VKAPI_ATTR void VKAPI_CALL
panvk_GetImageSubresourceLayout2(VkDevice device, VkImage image,
                                 const VkImageSubresource2 *pSubresource,
                                 VkSubresourceLayout2 *pLayout)
{
   VK_FROM_HANDLE(panvk_image, img, image);

   get_image_subresource_layout(img, pSubresource, pLayout);
}

VKAPI_ATTR void VKAPI_CALL
panvk_GetDeviceImageSubresourceLayoutKHR(
   VkDevice device, const VkDeviceImageSubresourceInfoKHR *pInfo,
   VkSubresourceLayout2KHR *pLayout)
{
   VK_FROM_HANDLE(panvk_device, dev, device);
   struct panvk_image image = {0};

   vk_image_init(&dev->vk, &image.vk, pInfo->pCreateInfo);
   panvk_image_init(&image, pInfo->pCreateInfo);
   get_image_subresource_layout(&image, pInfo->pSubresource, pLayout);
   vk_image_finish(&image.vk);
}

VKAPI_ATTR void VKAPI_CALL
panvk_GetImageMemoryRequirements2(VkDevice device,
                                  const VkImageMemoryRequirementsInfo2 *pInfo,
                                  VkMemoryRequirements2 *pMemoryRequirements)
{
   VK_FROM_HANDLE(panvk_image, image, pInfo->image);

   const uint64_t alignment = 4096;
   const VkImagePlaneMemoryRequirementsInfo *plane_info =
      vk_find_struct_const(pInfo->pNext, IMAGE_PLANE_MEMORY_REQUIREMENTS_INFO);
   const bool disjoint = is_disjoint(image);
   const VkImageAspectFlags aspects =
      plane_info ? plane_info->planeAspect : image->vk.aspects;
   uint8_t plane = panvk_plane_index(image->vk.format, aspects);
   const uint64_t size =
      disjoint ? image->planes[plane].plane.layout.data_size_B :
      panvk_image_get_total_size(image);

   pMemoryRequirements->memoryRequirements.memoryTypeBits = 1;
   pMemoryRequirements->memoryRequirements.alignment = alignment;
   pMemoryRequirements->memoryRequirements.size = size;

   vk_foreach_struct_const(ext, pMemoryRequirements->pNext) {
      switch (ext->sType) {
      case VK_STRUCTURE_TYPE_MEMORY_DEDICATED_REQUIREMENTS: {
         VkMemoryDedicatedRequirements *dedicated = (void *)ext;
         dedicated->requiresDedicatedAllocation = false;
         dedicated->prefersDedicatedAllocation = dedicated->requiresDedicatedAllocation;
         break;
      }
      default:
         vk_debug_ignored_stype(ext->sType);
         break;
      }
   }
}

VKAPI_ATTR void VKAPI_CALL
panvk_GetDeviceImageMemoryRequirements(VkDevice device,
                                       const VkDeviceImageMemoryRequirements *pInfo,
                                       VkMemoryRequirements2 *pMemoryRequirements)
{
   VK_FROM_HANDLE(panvk_device, dev, device);

   struct panvk_image image;
   vk_image_init(&dev->vk, &image.vk, pInfo->pCreateInfo);
   panvk_image_init(&image, pInfo->pCreateInfo);

   VkImageMemoryRequirementsInfo2 info2 = {
      .sType = VK_STRUCTURE_TYPE_IMAGE_MEMORY_REQUIREMENTS_INFO_2,
      .image = panvk_image_to_handle(&image),
   };
   panvk_GetImageMemoryRequirements2(device, &info2, pMemoryRequirements);
   vk_image_finish(&image.vk);
}

VKAPI_ATTR void VKAPI_CALL
panvk_GetImageSparseMemoryRequirements2(
   VkDevice device, const VkImageSparseMemoryRequirementsInfo2 *pInfo,
   uint32_t *pSparseMemoryRequirementCount,
   VkSparseImageMemoryRequirements2 *pSparseMemoryRequirements)
{
   /* Sparse images are not yet supported. */
   *pSparseMemoryRequirementCount = 0;
}

VKAPI_ATTR void VKAPI_CALL
panvk_GetDeviceImageSparseMemoryRequirements(VkDevice device,
                                             const VkDeviceImageMemoryRequirements *pInfo,
                                             uint32_t *pSparseMemoryRequirementCount,
                                             VkSparseImageMemoryRequirements2 *pSparseMemoryRequirements)
{
   /* Sparse images are not yet supported. */
   *pSparseMemoryRequirementCount = 0;
}

static VkResult
panvk_image_bind(struct panvk_device *dev,
                 const VkBindImageMemoryInfo *bind_info) {
   VK_FROM_HANDLE(panvk_image, image, bind_info->image);
   VK_FROM_HANDLE(panvk_device_memory, mem, bind_info->memory);
   uint64_t offset = bind_info->memoryOffset;

   if (!mem) {
#if DETECT_OS_ANDROID
      /* TODO handle VkNativeBufferANDROID when we support ANB */
      unreachable("VkBindImageMemoryInfo with no memory");
#else
      const VkBindImageMemorySwapchainInfoKHR *swapchain_info =
         vk_find_struct_const(bind_info->pNext,
                              BIND_IMAGE_MEMORY_SWAPCHAIN_INFO_KHR);
      assert(swapchain_info && swapchain_info->swapchain != VK_NULL_HANDLE);
      VkDeviceMemory mem_handle = wsi_common_get_memory(
         swapchain_info->swapchain, swapchain_info->imageIndex);
      mem = panvk_device_memory_from_handle(mem_handle);
      offset = 0;
#endif
   }

   assert(mem);
   if (is_disjoint(image)) {
      const VkBindImagePlaneMemoryInfo *plane_info =
         vk_find_struct_const(bind_info->pNext, BIND_IMAGE_PLANE_MEMORY_INFO);
      const uint8_t plane =
         panvk_plane_index(image->vk.format, plane_info->planeAspect);
      return panvk_image_plane_bind(dev, &image->planes[plane], mem, offset);
   } else {
      for (unsigned plane = 0; plane < image->plane_count; plane++) {
         VkResult result =
            panvk_image_plane_bind(dev, &image->planes[plane], mem, offset);
         if (result != VK_SUCCESS)
            return result;
      }
   }

   return VK_SUCCESS;
}

VKAPI_ATTR VkResult VKAPI_CALL
panvk_BindImageMemory2(VkDevice device, uint32_t bindInfoCount,
                       const VkBindImageMemoryInfo *pBindInfos)
{
   VK_FROM_HANDLE(panvk_device, dev, device);
   VkResult result = VK_SUCCESS;

   for (uint32_t i = 0; i < bindInfoCount; i++) {
      const VkBindMemoryStatus *bind_status =
         vk_find_struct_const(&pBindInfos[i], BIND_MEMORY_STATUS);
      VkResult bind_result = panvk_image_bind(dev, &pBindInfos[i]);
      if (bind_status)
         *bind_status->pResult = bind_result;
      if (bind_result != VK_SUCCESS)
         result = bind_result;
   }

   return result;
}
