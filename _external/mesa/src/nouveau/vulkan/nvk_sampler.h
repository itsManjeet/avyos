/*
 * Copyright © 2022 Collabora Ltd. and Red Hat Inc.
 * SPDX-License-Identifier: MIT
 */
#ifndef NVK_SAMPLER_H
#define NVK_SAMPLER_H 1

#include "nvk_private.h"
#include "nvk_physical_device.h"

#include "vk_sampler.h"
#include "vk_ycbcr_conversion.h"

#include "vk_format.h"

struct nvk_sampler_header {
   uint32_t bits[8];
};

struct nvk_sampler {
   struct vk_sampler vk;

   uint8_t plane_count;

   struct {
      uint32_t desc_index;
   } planes[NVK_MAX_SAMPLER_PLANES];
};

VK_DEFINE_NONDISP_HANDLE_CASTS(nvk_sampler, vk.base, VkSampler,
                               VK_OBJECT_TYPE_SAMPLER)

struct nvk_sampler_capture {
   struct {
      uint32_t desc_index;
   } planes[NVK_MAX_SAMPLER_PLANES];
};

struct nvk_sampler_header
nvk_txf_sampler_header(const struct nvk_physical_device *pdev);

#endif
