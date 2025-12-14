/*
 * Copyright © 2024, Google Inc.
 *
 * Permission is hereby granted, free of charge, to any person obtaining a
 * copy of this software and associated documentation files (the "Software"),
 * to deal in the Software without restriction, including without limitation
 * the rights to use, copy, modify, merge, publish, distribute, sublicense,
 * and/or sell copies of the Software, and to permit persons to whom the
 * Software is furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice (including the next
 * paragraph) shall be included in all copies or substantial portions of the
 * Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL
 * THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
 * FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS
 * IN THE SOFTWARE.
 */

#include <hardware/gralloc.h>

#if ANDROID_API_LEVEL >= 26
#include <hardware/gralloc1.h>
#endif

#include <vulkan/vk_android_native_buffer.h>

#include "util/libsync.h"
#include "util/os_file.h"
#include "util/libsync.h"

#include "vk_fence.h"
#include "vk_semaphore.h"
#include "vk_android.h"

#include "lvp_private.h"

VKAPI_ATTR VkResult VKAPI_CALL
lvp_GetSwapchainGrallocUsageANDROID(VkDevice device_h,
                                   VkFormat format,
                                   VkImageUsageFlags imageUsage,
                                   int *grallocUsage)
{
   *grallocUsage = GRALLOC_USAGE_SW_WRITE_OFTEN | GRALLOC_USAGE_SW_READ_OFTEN;

   return VK_SUCCESS;
}

#if ANDROID_API_LEVEL >= 26
VKAPI_ATTR VkResult VKAPI_CALL
lvp_GetSwapchainGrallocUsage2ANDROID(VkDevice device_h,
                                    VkFormat format,
                                    VkImageUsageFlags imageUsage,
                                    VkSwapchainImageUsageFlagsANDROID swapchainImageUsage,
                                    uint64_t *grallocConsumerUsage,
                                    uint64_t *grallocProducerUsage)
{
   *grallocConsumerUsage = 0;
   *grallocProducerUsage = GRALLOC1_PRODUCER_USAGE_CPU_WRITE_OFTEN | GRALLOC1_PRODUCER_USAGE_CPU_READ_OFTEN;

   return VK_SUCCESS;
}
#endif

VKAPI_ATTR VkResult VKAPI_CALL
lvp_AcquireImageANDROID(VkDevice _device,
                        VkImage image,
                        int nativeFenceFd,
                        VkSemaphore semaphore,
                        VkFence fence)
{
   VK_FROM_HANDLE(vk_device, vk_device, _device);
   VkResult result = VK_SUCCESS;

   if(nativeFenceFd >= 0)
   {
      sync_wait(nativeFenceFd, -1);
      close(nativeFenceFd);
   }

   if(fence != VK_NULL_HANDLE)
   {
      VK_FROM_HANDLE(vk_fence, vk_fence, fence);
      result = vk_sync_signal(vk_device, &vk_fence->permanent, 0);
   }

   if(result == VK_SUCCESS && semaphore != VK_NULL_HANDLE)
   {
      VK_FROM_HANDLE(vk_semaphore, vk_semaphore, semaphore);
      result = vk_sync_signal(vk_device, &vk_semaphore->permanent, 0);
   }

   return result;
}

VKAPI_ATTR VkResult VKAPI_CALL
lvp_QueueSignalReleaseImageANDROID(VkQueue _queue,
                                   uint32_t waitSemaphoreCount,
                                   const VkSemaphore *pWaitSemaphores,
                                   VkImage image,
                                   int *pNativeFenceFd)
{
   VK_FROM_HANDLE(vk_queue, queue, _queue);
   struct vk_device *device = queue->base.device;

   device->dispatch_table.QueueWaitIdle(_queue);

   *pNativeFenceFd = -1;

   return VK_SUCCESS;
}

VkResult
lvp_import_ahb_memory(struct lvp_device *device, struct lvp_device_memory *mem,
                      const VkImportAndroidHardwareBufferInfoANDROID *info)
{
   const native_handle_t *handle = AHardwareBuffer_getNativeHandle(info->buffer);
   int dma_buf = (handle && handle->numFds) ? handle->data[0] : -1;
   if (dma_buf < 0)
      return VK_ERROR_INVALID_EXTERNAL_HANDLE;

   uint64_t size;
   int result = device->pscreen->import_memory_fd(device->pscreen, dma_buf, (struct pipe_memory_allocation**)&mem->pmem, &size, true);
   if (!result)
      return VK_ERROR_INVALID_EXTERNAL_HANDLE;

   AHardwareBuffer_acquire(info->buffer);
   mem->android_hardware_buffer = info->buffer;
   mem->size = size;
   mem->map = device->pscreen->map_memory(device->pscreen, mem->pmem);
   mem->memory_type = LVP_DEVICE_MEMORY_TYPE_DMA_BUF;

   return VK_SUCCESS;
}

VkResult
lvp_create_ahb_memory(struct lvp_device *device, struct lvp_device_memory *mem,
                      const VkMemoryAllocateInfo *pAllocateInfo)
{
   mem->android_hardware_buffer = vk_alloc_ahardware_buffer(pAllocateInfo);
   if (mem->android_hardware_buffer == NULL)
      return VK_ERROR_OUT_OF_HOST_MEMORY;

   const struct VkImportAndroidHardwareBufferInfoANDROID import_info = {
      .buffer = mem->android_hardware_buffer,
   };

   VkResult result = lvp_import_ahb_memory(device, mem, &import_info);

   /* Release a reference to avoid leak for AHB allocation. */
   AHardwareBuffer_release(mem->android_hardware_buffer);

   return result;
}

#if ANDROID_API_LEVEL >= 26
VkResult
lvp_GetMemoryAndroidHardwareBufferANDROID(
   VkDevice device,
   const VkMemoryGetAndroidHardwareBufferInfoANDROID *pInfo,
   struct AHardwareBuffer **pBuffer)
{
   LVP_FROM_HANDLE(lvp_device_memory, mem, pInfo->memory);

   if (mem->android_hardware_buffer) {
      *pBuffer = mem->android_hardware_buffer;
      /* Increase refcount. */
      AHardwareBuffer_acquire(*pBuffer);
      return VK_SUCCESS;
   }

   return VK_ERROR_INVALID_EXTERNAL_HANDLE;
}
#endif
