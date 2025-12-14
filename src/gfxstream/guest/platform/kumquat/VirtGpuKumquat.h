/*
 * Copyright 2023 Google
 * SPDX-License-Identifier: MIT
 */

#pragma once

#include "GfxStreamVulkanMapper.h"
#include "VirtGpu.h"

#include "virtgpu_kumquat_ffi.h"

class VirtGpuKumquatResource : public std::enable_shared_from_this<VirtGpuKumquatResource>,
                               public VirtGpuResource {
   public:
    VirtGpuKumquatResource(struct virtgpu_kumquat* virtGpu, uint32_t blobHandle,
                           uint32_t resourceHandle, uint64_t size);
    ~VirtGpuKumquatResource();

    uint32_t getResourceHandle() const override;
    uint32_t getBlobHandle() const override;
    uint64_t getSize() const override;

    int wait() override;

    VirtGpuResourceMappingPtr createMapping(void) override;
    int exportBlob(struct VirtGpuExternalHandle& handle) override;

    int transferFromHost(uint32_t x, uint32_t y, uint32_t w, uint32_t h) override;
    int transferToHost(uint32_t x, uint32_t y, uint32_t w, uint32_t h) override;

   private:
    // Not owned.  Really should use a ScopedFD for this, but doesn't matter since we have a
    // singleton deviceimplemenentation anyways.
    struct virtgpu_kumquat* mVirtGpu = nullptr;

    uint32_t mBlobHandle;
    uint32_t mResourceHandle;
    uint64_t mSize;
};

class VirtGpuKumquatResourceMapping : public VirtGpuResourceMapping {
   public:
    VirtGpuKumquatResourceMapping(VirtGpuResourcePtr blob, struct virtgpu_kumquat* virtGpu,
                                  uint8_t* ptr, uint64_t size);

    VirtGpuKumquatResourceMapping(VirtGpuResourcePtr blob, struct virtgpu_kumquat* virtGpu,
                                  struct VulkanMapperData& data, uint64_t size);

    ~VirtGpuKumquatResourceMapping(void);

    uint8_t* asRawPtr(void) override;

   private:
    VirtGpuResourcePtr mBlob;
    struct virtgpu_kumquat* mVirtGpu = nullptr;
    struct VulkanMapperData mVulkanData;
    uint8_t* mPtr = nullptr;
    uint64_t mSize = 0;
};

class VirtGpuKumquatDevice : public VirtGpuDevice {
   public:
    VirtGpuKumquatDevice(enum VirtGpuCapset capset, int fd = -1);
    virtual ~VirtGpuKumquatDevice();

    virtual int64_t getDeviceHandle(void);

    virtual struct VirtGpuCaps getCaps(void);

    VirtGpuResourcePtr createBlob(const struct VirtGpuCreateBlob& blobCreate) override;
    VirtGpuResourcePtr createResource(uint32_t width, uint32_t height, uint32_t stride,
                                      uint32_t size, uint32_t virglFormat, uint32_t target,
                                      uint32_t bind) override;

    virtual VirtGpuResourcePtr importBlob(const struct VirtGpuExternalHandle& handle);
    virtual int execBuffer(struct VirtGpuExecBuffer& execbuffer, const VirtGpuResource* blob);

   private:
    int32_t mDescriptor = 0;
    struct virtgpu_kumquat* mVirtGpu = nullptr;
    struct VirtGpuCaps mCaps;
};
