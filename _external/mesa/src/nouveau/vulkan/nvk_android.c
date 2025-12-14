/*
 * Mesa 3-D graphics library
 *
 * Copyright © 2017, Google Inc.
 *
 * SPDX-License-Identifier: MIT
 */

#include "nvk_private.h"

#include "nvk_android.h"
#include "nvk_entrypoints.h"
#include "vk_android.h"

#include "util/libsync.h"
#include "util/os_file.h"
#include "vk_device.h"
#include "vk_fence.h"
#include "vk_queue.h"
#include "vk_semaphore.h"

VKAPI_ATTR VkResult VKAPI_CALL
nvk_AcquireImageANDROID(VkDevice _device,
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
nvk_QueueSignalReleaseImageANDROID(VkQueue _queue,
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
