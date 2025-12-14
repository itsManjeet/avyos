/*
 * Copyright © 2016 Bas Nieuwenhuizen
 *
 * SPDX-License-Identifier: MIT
 */

#ifndef RADV_DESCRIPTOR_POOL_H
#define RADV_DESCRIPTOR_POOL_H

#include "vk_object.h"

#include <vulkan/vulkan.h>

struct radv_descriptor_set;

struct radv_descriptor_pool_entry {
   uint32_t offset;
   uint32_t size;
   struct radv_descriptor_set *set;
};

struct radv_descriptor_pool {
   struct vk_object_base base;
   struct radeon_winsys_bo *bo;
   uint8_t *host_bo;
   uint8_t *mapped_ptr;
   uint64_t current_offset;
   uint64_t size;

   uint8_t *host_memory_base;
   uint8_t *host_memory_ptr;
   uint8_t *host_memory_end;

   uint32_t entry_count;
   uint32_t max_entry_count;

   union {
      struct radv_descriptor_set *sets[0];
      struct radv_descriptor_pool_entry entries[0];
   };
};

VK_DEFINE_NONDISP_HANDLE_CASTS(radv_descriptor_pool, base, VkDescriptorPool, VK_OBJECT_TYPE_DESCRIPTOR_POOL)

#endif /* RADV_DESCRIPTOR_POOL_H */
