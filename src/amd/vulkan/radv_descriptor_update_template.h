/*
 * Copyright © 2016 Bas Nieuwenhuizen
 *
 * SPDX-License-Identifier: MIT
 */

#ifndef RADV_DESCRIPTOR_UPDATE_TEMPLATE_H
#define RADV_DESCRIPTOR_UPDATE_TEMPLATE_H

#include "vk_object.h"

#include <vulkan/vulkan.h>

struct radv_cmd_buffer;
struct radv_descriptor_set;
struct radv_device;

struct radv_descriptor_update_template_entry {
   VkDescriptorType descriptor_type;

   /* The number of descriptors to update */
   uint32_t descriptor_count;

   /* Into mapped_ptr or dynamic_descriptors, in units of the respective array */
   uint32_t dst_offset;

   /* In dwords. Not valid/used for dynamic descriptors */
   uint32_t dst_stride;

   uint32_t buffer_offset;

   /* Only valid for combined image samplers and samplers */
   uint8_t has_sampler;
   uint8_t has_ycbcr_sampler;

   /* In bytes */
   size_t src_offset;
   size_t src_stride;

   /* For push descriptors */
   const uint32_t *immutable_samplers;
};

struct radv_descriptor_update_template {
   struct vk_object_base base;
   uint32_t entry_count;
   VkPipelineBindPoint bind_point;
   struct radv_descriptor_update_template_entry entry[0];
};

VK_DEFINE_NONDISP_HANDLE_CASTS(radv_descriptor_update_template, base, VkDescriptorUpdateTemplate,
                               VK_OBJECT_TYPE_DESCRIPTOR_UPDATE_TEMPLATE)

void radv_cmd_update_descriptor_set_with_template(struct radv_device *device, struct radv_cmd_buffer *cmd_buffer,
                                                  struct radv_descriptor_set *set,
                                                  VkDescriptorUpdateTemplate descriptorUpdateTemplate,
                                                  const void *pData);

#endif /* RADV_DESCRIPTOR_UPDATE_TEMPLATE_H */
