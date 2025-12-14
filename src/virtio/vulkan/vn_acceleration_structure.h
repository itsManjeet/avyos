/*
 * Copyright 2025 Google LLC
 * SPDX-License-Identifier: MIT
 */

#ifndef VN_ACCELERATION_STRUCTURE_H
#define VN_ACCELERATION_STRUCTURE_H

#include "vn_common.h"

struct vn_acceleration_structure {
   struct vn_object_base base;
};
VK_DEFINE_NONDISP_HANDLE_CASTS(vn_acceleration_structure,
                               base.vk,
                               VkAccelerationStructureKHR,
                               VK_OBJECT_TYPE_ACCELERATION_STRUCTURE_KHR)

#endif /* VN_ACCELERATION_STRUCTURE_H */
