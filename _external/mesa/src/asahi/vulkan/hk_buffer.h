/*
 * Copyright 2024 Valve Corporation
 * Copyright 2024 Alyssa Rosenzweig
 * Copyright 2022-2023 Collabora Ltd. and Red Hat Inc.
 * SPDX-License-Identifier: MIT
 */
#pragma once

#include "hk_device_memory.h"
#include "hk_private.h"

#include "agx_abi.h"
#include "vk_buffer.h"

struct hk_device_memory;
struct hk_physical_device;

struct hk_buffer {
   struct vk_buffer vk;

   /** Reserved VA for sparse buffers, NULL otherwise. */
   struct agx_va *va;
};

VK_DEFINE_NONDISP_HANDLE_CASTS(hk_buffer, vk.base, VkBuffer,
                               VK_OBJECT_TYPE_BUFFER)

uint64_t hk_buffer_address(const struct hk_buffer *buffer, uint64_t offset,
                           bool read_only);

static inline uint64_t
hk_buffer_address_rw(const struct hk_buffer *buffer, uint64_t offset)
{
   return hk_buffer_address(buffer, offset, false);
}

static inline uint64_t
hk_buffer_address_ro(const struct hk_buffer *buffer, uint64_t offset)
{
   return hk_buffer_address(buffer, offset, true);
}

static inline struct hk_addr_range
hk_buffer_addr_range(const struct hk_buffer *buffer, uint64_t offset,
                     uint64_t range, bool read_only)
{
   /* If range == 0, return a pointer to the zero page. That allows
    * eliding robustness2 bounds checks for index = 0, even without soft fault.
    */
   if (buffer == NULL || range == 0) {
      return (struct hk_addr_range){.addr = AGX_ZERO_PAGE_ADDRESS, .range = 0};
   }

   return (struct hk_addr_range){
      .addr = hk_buffer_address(buffer, offset, read_only),
      .range = vk_buffer_range(&buffer->vk, offset, range),
   };
}

VkResult hk_bind_scratch(struct hk_device *dev, struct agx_va *va,
                         unsigned offs_B, size_t size_B);
