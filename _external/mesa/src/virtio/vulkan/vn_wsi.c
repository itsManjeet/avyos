/*
 * Copyright 2019 Google LLC
 * SPDX-License-Identifier: MIT
 *
 * based in part on anv and radv which are:
 * Copyright © 2015 Intel Corporation
 * Copyright © 2016 Red Hat.
 * Copyright © 2016 Bas Nieuwenhuizen
 */

#include "vn_wsi.h"

#include "vk_enum_to_str.h"
#include "wsi_common_entrypoints.h"

#include "vn_device.h"
#include "vn_image.h"
#include "vn_instance.h"
#include "vn_physical_device.h"
#include "vn_queue.h"

/* The common WSI support makes some assumptions about the driver.
 *
 * In wsi_device_init, it assumes VK_EXT_pci_bus_info is available.  In
 * wsi_create_native_image and wsi_create_prime_image, it assumes
 * VK_KHR_external_memory_fd and VK_EXT_external_memory_dma_buf are enabled.
 *
 * In wsi_create_native_image, if wsi_device::supports_modifiers is set and
 * the window system supports modifiers, it assumes
 * VK_EXT_image_drm_format_modifier is enabled.  Otherwise, it assumes that
 * wsi_image_create_info can be chained to VkImageCreateInfo and
 * vkGetImageSubresourceLayout can be called even the tiling is
 * VK_IMAGE_TILING_OPTIMAL.
 *
 * Together, it knows how to share dma-bufs, with explicit or implicit
 * modifiers, to the window system.
 *
 * For venus, we use explicit modifiers when the renderer and the window
 * system support them.  Otherwise, we have to fall back to
 * VK_IMAGE_TILING_LINEAR (or trigger the prime blit path).  But the fallback
 * can be problematic when the memory is scanned out directly and special
 * requirements (e.g., alignments) must be met.
 *
 * The common WSI support makes other assumptions about the driver to support
 * implicit fencing.  In wsi_create_native_image and wsi_create_prime_image,
 * it assumes wsi_memory_allocate_info can be chained to VkMemoryAllocateInfo.
 * In wsi_common_queue_present, it assumes wsi_memory_signal_submit_info can
 * be chained to VkSubmitInfo.  Finally, in wsi_common_acquire_next_image2, it
 * calls wsi_device::signal_semaphore_for_memory, and
 * wsi_device::signal_fence_for_memory if the driver provides them.
 *
 * Some drivers use wsi_memory_allocate_info to set up implicit fencing.
 * Others use wsi_memory_signal_submit_info to set up implicit IN-fences and
 * use wsi_device::signal_*_for_memory to set up implicit OUT-fences.
 *
 * For venus, implicit fencing is broken (and there is no explicit fencing
 * support yet).  The kernel driver assumes everything is in the same fence
 * context and no synchronization is needed.  It should be fixed for
 * correctness, but it is still not ideal.  venus requires explicit fencing
 * (and renderer-side synchronization) to work well.
 */

/* cast a WSI object to a pointer for logging */
#define VN_WSI_PTR(obj) ((const void *)(uintptr_t)(obj))

static PFN_vkVoidFunction
vn_wsi_proc_addr(VkPhysicalDevice physicalDevice, const char *pName)
{
   struct vn_physical_device *physical_dev =
      vn_physical_device_from_handle(physicalDevice);
   return vk_instance_get_proc_addr_unchecked(
      &physical_dev->instance->base.vk, pName);
}

VkResult
vn_wsi_init(struct vn_physical_device *physical_dev)
{
   /* TODO Drop the workaround for NVIDIA_PROPRIETARY once hw prime buffer
    * blit path works there.
    */
   const bool use_sw_device =
      !physical_dev->base.vk.supported_extensions
          .EXT_external_memory_dma_buf ||
      physical_dev->renderer_driver_id == VK_DRIVER_ID_NVIDIA_PROPRIETARY;

   const VkAllocationCallbacks *alloc =
      &physical_dev->instance->base.vk.alloc;
   VkResult result = wsi_device_init(
      &physical_dev->wsi_device, vn_physical_device_to_handle(physical_dev),
      vn_wsi_proc_addr, alloc, -1, &physical_dev->instance->dri_options,
      &(struct wsi_device_options){
         .sw_device = use_sw_device,
         .extra_xwayland_image = true,
      });
   if (result != VK_SUCCESS)
      return result;

   physical_dev->wsi_device.supports_scanout = false;
   physical_dev->wsi_device.supports_modifiers =
      physical_dev->base.vk.supported_extensions.EXT_image_drm_format_modifier;
   physical_dev->base.vk.wsi_device = &physical_dev->wsi_device;

   return VK_SUCCESS;
}

void
vn_wsi_fini(struct vn_physical_device *physical_dev)
{
   const VkAllocationCallbacks *alloc =
      &physical_dev->instance->base.vk.alloc;
   physical_dev->base.vk.wsi_device = NULL;
   wsi_device_finish(&physical_dev->wsi_device, alloc);
}

VkResult
vn_wsi_create_image(struct vn_device *dev,
                    const VkImageCreateInfo *create_info,
                    const struct wsi_image_create_info *wsi_info,
                    const VkAllocationCallbacks *alloc,
                    struct vn_image **out_img)
{
   VkImageCreateInfo local_create_info;
   if (dev->physical_device->renderer_driver_id ==
          VK_DRIVER_ID_INTEL_OPEN_SOURCE_MESA &&
       (create_info->flags & VK_IMAGE_CREATE_ALIAS_BIT)) {
      /* See explanation in vn_GetPhysicalDeviceImageFormatProperties2() */
      local_create_info = *create_info;
      local_create_info.flags &= ~VK_IMAGE_CREATE_ALIAS_BIT;
      create_info = &local_create_info;
   }

   /* Gamescope relies on legacy scanout support when explicit modifier isn't
    * available and it chains the mesa wsi hint requesting such. Venus doesn't
    * support legacy scanout with optimal tiling on its own, so venus disables
    * legacy scanout in favor of prime buffer blit for optimal performance. As
    * a workaround here, venus can once again force linear tiling when legacy
    * scanout is requested outside of common wsi.
    */
   if (wsi_info->scanout) {
      if (create_info != &local_create_info) {
         local_create_info = *create_info;
         local_create_info.tiling = VK_IMAGE_TILING_LINEAR;
         create_info = &local_create_info;
      } else {
         local_create_info.tiling = VK_IMAGE_TILING_LINEAR;
      }
   }

   struct vn_image *img;
   VkResult result = vn_image_create(dev, create_info, alloc, &img);
   if (result != VK_SUCCESS)
      return result;

   img->wsi.is_wsi = true;
   img->wsi.is_prime_blit_src = wsi_info->blit_src;

   *out_img = img;
   return VK_SUCCESS;
}

/* swapchain commands */

VkResult
vn_AcquireNextImage2KHR(VkDevice device,
                        const VkAcquireNextImageInfoKHR *pAcquireInfo,
                        uint32_t *pImageIndex)
{
   VN_TRACE_FUNC();
   struct vn_device *dev = vn_device_from_handle(device);

   VkResult result = wsi_common_acquire_next_image2(
      &dev->physical_device->wsi_device, device, pAcquireInfo, pImageIndex);
   if (VN_DEBUG(WSI) && result != VK_SUCCESS) {
      const int idx = result >= VK_SUCCESS ? *pImageIndex : -1;
      vn_log(dev->instance, "swapchain %p: acquired image %d: %s",
             VN_WSI_PTR(pAcquireInfo->swapchain), idx,
             vk_Result_to_str(result));
   }

   if (result != VK_SUCCESS && result != VK_SUBOPTIMAL_KHR)
      return vn_error(dev->instance, result);

   /* XXX this relies on renderer side doing implicit fencing */
   if (pAcquireInfo->semaphore != VK_NULL_HANDLE) {
      const VkImportSemaphoreFdInfoKHR info = {
         .sType = VK_STRUCTURE_TYPE_IMPORT_SEMAPHORE_FD_INFO_KHR,
         .semaphore = pAcquireInfo->semaphore,
         .flags = VK_SEMAPHORE_IMPORT_TEMPORARY_BIT,
         .handleType = VK_EXTERNAL_SEMAPHORE_HANDLE_TYPE_SYNC_FD_BIT,
         .fd = -1,
      };
      result = vn_ImportSemaphoreFdKHR(device, &info);
   }

   if (result == VK_SUCCESS && pAcquireInfo->fence != VK_NULL_HANDLE) {
      const VkImportFenceFdInfoKHR info = {
         .sType = VK_STRUCTURE_TYPE_IMPORT_FENCE_FD_INFO_KHR,
         .fence = pAcquireInfo->fence,
         .flags = VK_FENCE_IMPORT_TEMPORARY_BIT,
         .handleType = VK_EXTERNAL_FENCE_HANDLE_TYPE_SYNC_FD_BIT,
         .fd = -1,
      };
      result = vn_ImportFenceFdKHR(device, &info);
   }

   return vn_result(dev->instance, result);
}
