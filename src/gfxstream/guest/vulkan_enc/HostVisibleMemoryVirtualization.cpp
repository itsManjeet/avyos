/*
 * Copyright 2018 Google
 * SPDX-License-Identifier: MIT
 */
#include "HostVisibleMemoryVirtualization.h"

#include <set>

#include "ResourceTracker.h"
#include "Resources.h"
#include "VkEncoder.h"
#include "util/detect_os.h"
#include "util/log.h"

namespace gfxstream {
namespace vk {

CoherentMemory::CoherentMemory(VirtGpuResourceMappingPtr blobMapping, uint64_t size,
                               VkDevice device, VkDeviceMemory memory)
    : mSize(size), mBlobMapping(blobMapping), mDevice(device), mMemory(memory) {
    mHeap = u_mmInit(0, mSize);
    mBaseAddr = blobMapping->asRawPtr();
}

#if DETECT_OS_ANDROID
CoherentMemory::CoherentMemory(GoldfishAddressSpaceBlockPtr block, uint64_t gpuAddr, uint64_t size,
                               VkDevice device, VkDeviceMemory memory)
    : mSize(size), mBlock(block), mDevice(device), mMemory(memory) {
    mHeap = u_mmInit(0, mSize);
    mBaseAddr = (uint8_t*)block->mmap(gpuAddr);
}
#endif  // DETECT_OS_ANDROID

CoherentMemory::~CoherentMemory() {
    ResourceTracker::getThreadLocalEncoder()->vkFreeMemorySyncGOOGLE(mDevice, mMemory, nullptr,
                                                                     false);
    u_mmDestroy(mHeap);
}

VkDeviceMemory CoherentMemory::getDeviceMemory() const { return mMemory; }

bool CoherentMemory::subAllocate(uint64_t size, uint8_t** ptr, uint64_t& offset) {
    // 2^12 = 4096 (page size)
    auto block = u_mmAllocMem(mHeap, size, 12, 0);
    if (!block) return false;

    offset = block->ofs;
    *ptr = mBaseAddr + block->ofs;
    return true;
}

bool CoherentMemory::release(uint64_t offset) {
    auto block = u_mmFindBlock(mHeap, offset);
    if (block) {
        u_mmFreeMem(block);
        return true;
    } else {
        mesa_loge("unable to find block");
    }

    return false;
}

}  // namespace vk
}  // namespace gfxstream
