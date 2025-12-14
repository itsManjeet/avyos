/*
 * Copyright 2025 Google LLC
 * SPDX-License-Identifier: MIT
 */

#ifndef GFXSTREAM_VULKAN_MAPPER_H
#define GFXSTREAM_VULKAN_MAPPER_H

#include <memory>
#include <optional>
#include <unordered_map>

#include "util/u_dl.h"
#include "vk_dispatch_table.h"
#include "vulkan/vulkan_core.h"

struct VulkanMapperData {
    // in
    int64_t handle;
    int32_t handleType;
    uint32_t memoryIdx;
    uint64_t size;

    // out
    VkDeviceMemory memory = VK_NULL_HANDLE;
    uint8_t* ptr = nullptr;
};

struct DeviceId {
    uint8_t deviceUUID[16];
    uint8_t driverUUID[16];
};

class GfxStreamVulkanMapper {
   public:
    ~GfxStreamVulkanMapper();
    GfxStreamVulkanMapper();

    static GfxStreamVulkanMapper* getInstance(std::optional<DeviceId> deviceId = std::nullopt);
    int32_t map(struct VulkanMapperData* mapData);
    void unmap(struct VulkanMapperData* mapData);

   private:
    bool initialize(DeviceId& deviceId);

    struct util_dl_library* mLoaderLib = nullptr;
    struct vk_uncompacted_dispatch_table mVk;
    VkInstance mInstance;
    VkDevice mDevice;
};

#endif
