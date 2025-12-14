/*
 * Copyright © 2021 Collabora Ltd.
 * SPDX-License-Identifier: MIT
 */

#ifndef PANVK_IMAGE_H
#define PANVK_IMAGE_H

#include "vk_image.h"

#include "pan_image.h"

#define PANVK_MAX_PLANES 3

struct panvk_device_memory;
struct panvk_physical_device;

/* Right now, planar YUV images are treated as N different images, hence the 1:1
 * association between pan_image and pan_image_plane, but this can be optimized
 * once planar YUV support is hooked up. */
struct panvk_image_plane {
   struct pan_image image;
   struct pan_image_plane plane;

   struct panvk_device_memory *mem;

   /* Plane offset inside the memory object. */
   uint64_t mem_offset;
};

struct panvk_image {
   struct vk_image vk;

   uint8_t plane_count;
   struct panvk_image_plane planes[PANVK_MAX_PLANES];
};

VK_DEFINE_NONDISP_HANDLE_CASTS(panvk_image, vk.base, VkImage,
                               VK_OBJECT_TYPE_IMAGE)

/* Check whether it is possible that images in a given configuration may use
 * AFBC tiling. This function does not have access to all of the relevant
 * image configuration, and returns true if any images with the specified
 * configuration subset may use AFBC. */
bool panvk_image_can_use_afbc(
   struct panvk_physical_device *phys_dev, VkFormat fmt,
   VkImageUsageFlags usage, VkImageType type, VkImageTiling tiling,
   VkImageCreateFlags flags);

static inline unsigned
panvk_plane_index(VkFormat format, VkImageAspectFlags aspect_mask)
{
   switch (aspect_mask) {
   default:
      return 0;
   case VK_IMAGE_ASPECT_PLANE_1_BIT:
      return 1;
   case VK_IMAGE_ASPECT_PLANE_2_BIT:
      return 2;
   case VK_IMAGE_ASPECT_STENCIL_BIT:
      return format == VK_FORMAT_D32_SFLOAT_S8_UINT;
   }
}

#endif
