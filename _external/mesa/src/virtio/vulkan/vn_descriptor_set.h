/*
 * Copyright 2019 Google LLC
 * SPDX-License-Identifier: MIT
 *
 * based in part on anv and radv which are:
 * Copyright © 2015 Intel Corporation
 * Copyright © 2016 Red Hat.
 * Copyright © 2016 Bas Nieuwenhuizen
 */

#ifndef VN_DESCRIPTOR_SET_H
#define VN_DESCRIPTOR_SET_H

#include "vn_common.h"

enum vn_descriptor_type {
   VN_DESCRIPTOR_TYPE_SAMPLER,
   VN_DESCRIPTOR_TYPE_COMBINED_IMAGE_SAMPLER,
   VN_DESCRIPTOR_TYPE_SAMPLED_IMAGE,
   VN_DESCRIPTOR_TYPE_STORAGE_IMAGE,
   VN_DESCRIPTOR_TYPE_UNIFORM_TEXEL_BUFFER,
   VN_DESCRIPTOR_TYPE_STORAGE_TEXEL_BUFFER,
   VN_DESCRIPTOR_TYPE_UNIFORM_BUFFER,
   VN_DESCRIPTOR_TYPE_STORAGE_BUFFER,
   VN_DESCRIPTOR_TYPE_UNIFORM_BUFFER_DYNAMIC,
   VN_DESCRIPTOR_TYPE_STORAGE_BUFFER_DYNAMIC,
   VN_DESCRIPTOR_TYPE_INPUT_ATTACHMENT,
   VN_DESCRIPTOR_TYPE_INLINE_UNIFORM_BLOCK,
   VN_DESCRIPTOR_TYPE_MUTABLE_EXT,
   VN_DESCRIPTOR_TYPE_ACCELERATION_STRUCTURE_KHR,

   /* add new enum types before this line */
   VN_NUM_DESCRIPTOR_TYPES,
};

struct vn_descriptor_set_layout_binding {
   enum vn_descriptor_type type;
   uint32_t count;
   bool has_immutable_samplers;
   BITSET_DECLARE(mutable_descriptor_types, VN_NUM_DESCRIPTOR_TYPES);
};

struct vn_descriptor_set_layout {
   struct vn_object_base base;

   struct vn_refcount refcount;

   uint32_t last_binding;
   bool has_variable_descriptor_count;
   bool is_push_descriptor;

   /* bindings must be the last field in the layout */
   struct vn_descriptor_set_layout_binding bindings[];
};
VK_DEFINE_NONDISP_HANDLE_CASTS(vn_descriptor_set_layout,
                               base.vk,
                               VkDescriptorSetLayout,
                               VK_OBJECT_TYPE_DESCRIPTOR_SET_LAYOUT)

struct vn_descriptor_pool_state {
   uint32_t set_count;
   uint32_t iub_binding_count;
   uint32_t descriptor_counts[VN_NUM_DESCRIPTOR_TYPES];
};

struct vn_descriptor_pool_state_mutable {
   uint32_t max;
   uint32_t used;
   BITSET_DECLARE(types, VN_NUM_DESCRIPTOR_TYPES);
};

enum vn_async_set_alloc_state {
   /* Must do synchronous set alloc. It comes from either no_async_set_alloc
    * perf option or transition from below conditional states.
    */
   VN_ASYNC_SET_ALLOC_NONE,
   /* Can still do async set alloc, but the pool might become fragmented after
    * freeing up any descriptor set and that will change the state to NONE.
    */
   VN_ASYNC_SET_ALLOC_BEFORE_FREE,
   /* Can still do async set alloc, and the sets allocated are with the same
    * number of descriptors and types. Allocating a set with different number
    * of descriptors or types will change the state to BEFORE_FREE.
    */
   VN_ASYNC_SET_ALLOC_SAME_ALLOC,
   /* Will always do async set alloc, since the pool is not created with
    * VK_DESCRIPTOR_POOL_CREATE_FREE_DESCRIPTOR_SET_BIT.
    */
   VN_ASYNC_SET_ALLOC_ALWAYS,
};

struct vn_descriptor_pool {
   struct vn_object_base base;

   VkAllocationCallbacks allocator;
   struct list_head descriptor_sets;

   enum vn_async_set_alloc_state initial_state;
   enum vn_async_set_alloc_state current_state;

   struct vn_descriptor_set_layout *last_layout;

   struct vn_descriptor_pool_state max;
   struct vn_descriptor_pool_state used;

   uint32_t mutable_states_count;
   struct vn_descriptor_pool_state_mutable *mutable_states;
};
VK_DEFINE_NONDISP_HANDLE_CASTS(vn_descriptor_pool,
                               base.vk,
                               VkDescriptorPool,
                               VK_OBJECT_TYPE_DESCRIPTOR_POOL)

struct vn_descriptor_set {
   struct vn_object_base base;

   struct vn_descriptor_set_layout *layout;
   uint32_t last_binding_descriptor_count;

   struct list_head head;
};
VK_DEFINE_NONDISP_HANDLE_CASTS(vn_descriptor_set,
                               base.vk,
                               VkDescriptorSet,
                               VK_OBJECT_TYPE_DESCRIPTOR_SET)

struct vn_descriptor_update_template {
   struct vn_object_base base;

   struct {
      VkPipelineBindPoint pipeline_bind_point;
      struct vn_descriptor_set_layout *set_layout;
   } push;

   uint32_t entry_count;
   uint32_t img_info_count;
   uint32_t buf_info_count;
   uint32_t bview_count;
   uint32_t iub_count;
   uint32_t accel_count;
   VkDescriptorUpdateTemplateEntry entries[];
};
VK_DEFINE_NONDISP_HANDLE_CASTS(vn_descriptor_update_template,
                               base.vk,
                               VkDescriptorUpdateTemplate,
                               VK_OBJECT_TYPE_DESCRIPTOR_UPDATE_TEMPLATE)

struct vn_descriptor_set_writes {
   VkWriteDescriptorSet *writes;
   VkDescriptorImageInfo *img_infos;
};

struct vn_descriptor_set_update {
   uint32_t write_count;
   VkWriteDescriptorSet *writes;
   VkDescriptorImageInfo *img_infos;
   VkDescriptorBufferInfo *buf_infos;
   VkBufferView *bview_handles;
   VkWriteDescriptorSetInlineUniformBlock *iubs;
   VkWriteDescriptorSetAccelerationStructureKHR *accels;
};

uint32_t
vn_descriptor_set_count_write_images(uint32_t write_count,
                                     const VkWriteDescriptorSet *writes);

const VkWriteDescriptorSet *
vn_descriptor_set_get_writes(uint32_t write_count,
                             const VkWriteDescriptorSet *writes,
                             VkPipelineLayout pipeline_layout_handle,
                             struct vn_descriptor_set_writes *local);

void
vn_descriptor_set_fill_update_with_template(
   struct vn_descriptor_update_template *templ,
   VkDescriptorSet set_handle,
   const uint8_t *data,
   struct vn_descriptor_set_update *update);

void
vn_descriptor_set_layout_destroy(struct vn_device *dev,
                                 struct vn_descriptor_set_layout *layout);

static inline struct vn_descriptor_set_layout *
vn_descriptor_set_layout_ref(struct vn_device *dev,
                             struct vn_descriptor_set_layout *layout)
{
   vn_refcount_inc(&layout->refcount);
   return layout;
}

static inline void
vn_descriptor_set_layout_unref(struct vn_device *dev,
                               struct vn_descriptor_set_layout *layout)
{
   if (vn_refcount_dec(&layout->refcount))
      vn_descriptor_set_layout_destroy(dev, layout);
}

#endif /* VN_DESCRIPTOR_SET_H */
