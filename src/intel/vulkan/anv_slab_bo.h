/* Copyright © 2025 Intel Corporation
 * SPDX-License-Identifier: MIT
 */

#pragma once

#include "anv_private.h"

bool anv_slab_bo_init(struct anv_device *device);
void anv_slab_bo_deinit(struct anv_device *device);

struct anv_bo *
anv_slab_bo_alloc(struct anv_device *device, const char *name, uint64_t size,
                  uint32_t alignment, enum anv_bo_alloc_flags alloc_flags);
void anv_slab_bo_free(struct anv_device *device, struct anv_bo *bo);
