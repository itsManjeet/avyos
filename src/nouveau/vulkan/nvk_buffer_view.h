/*
 * Copyright © 2022 Collabora Ltd. and Red Hat Inc.
 * SPDX-License-Identifier: MIT
 */
#ifndef NVK_BUFFER_VIEW_H
#define NVK_BUFFER_VIEW_H 1

#include "nvk_private.h"

#include "nvk_descriptor_types.h"

#include "vk_buffer_view.h"

struct nvk_physical_device;

VkFormatFeatureFlags2
nvk_get_buffer_format_features(const struct nvk_physical_device *pdev,
                               VkFormat format);

struct nvk_buffer_view {
   struct vk_buffer_view vk;

   struct nvk_buffer_view_descriptor desc;

   /* Used for uniform texel buffers on Kepler and everything on Maxwell+ */
   struct nvk_edb_buffer_view_descriptor edb_desc;

   /* Used for storage texel buffers on Kepler */
   struct nil_su_info su_info;
};

VK_DEFINE_NONDISP_HANDLE_CASTS(nvk_buffer_view, vk.base, VkBufferView,
                               VK_OBJECT_TYPE_BUFFER_VIEW)

#endif
