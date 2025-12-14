/*
 * Copyright © 2021 Collabora Ltd.
 * SPDX-License-Identifier: MIT
 */

#include "panvk_buffer.h"
#include "panvk_device.h"
#include "panvk_device_memory.h"
#include "panvk_entrypoints.h"

#include "pan_props.h"

#include "vk_log.h"

#define PANVK_MAX_BUFFER_SIZE (1 << 30)

VKAPI_ATTR uint64_t VKAPI_CALL
panvk_GetBufferOpaqueCaptureAddress(VkDevice _device,
                                    const VkBufferDeviceAddressInfo *pInfo)
{
   VK_FROM_HANDLE(panvk_buffer, buffer, pInfo->buffer);

   return buffer->vk.device_address;
}

VKAPI_ATTR void VKAPI_CALL
panvk_GetDeviceBufferMemoryRequirements(VkDevice device,
                                        const VkDeviceBufferMemoryRequirements *pInfo,
                                        VkMemoryRequirements2 *pMemoryRequirements)
{
   const uint64_t align = 64;
   const uint64_t size = align64(pInfo->pCreateInfo->size, align);

   pMemoryRequirements->memoryRequirements.memoryTypeBits = 1;
   pMemoryRequirements->memoryRequirements.alignment = align;
   pMemoryRequirements->memoryRequirements.size = size;

   vk_foreach_struct_const(ext, pMemoryRequirements->pNext) {
      switch (ext->sType) {
      case VK_STRUCTURE_TYPE_MEMORY_DEDICATED_REQUIREMENTS: {
         VkMemoryDedicatedRequirements *dedicated = (void *)ext;
         dedicated->requiresDedicatedAllocation = false;
         dedicated->prefersDedicatedAllocation = dedicated->requiresDedicatedAllocation;
         break;
      }
      default:
         vk_debug_ignored_stype(ext->sType);
         break;
      }
   }
}

VKAPI_ATTR VkResult VKAPI_CALL
panvk_BindBufferMemory2(VkDevice _device, uint32_t bindInfoCount,
                        const VkBindBufferMemoryInfo *pBindInfos)
{
   VK_FROM_HANDLE(panvk_device, device, _device);
   const struct panvk_physical_device *phys_dev =
      to_panvk_physical_device(device->vk.physical);
   const unsigned arch = pan_arch(phys_dev->kmod.props.gpu_id);
   VkResult result = VK_SUCCESS;

   for (uint32_t i = 0; i < bindInfoCount; i++) {
      VK_FROM_HANDLE(panvk_device_memory, mem, pBindInfos[i].memory);
      VK_FROM_HANDLE(panvk_buffer, buffer, pBindInfos[i].buffer);
      const VkBindMemoryStatus *bind_status =
         vk_find_struct_const(&pBindInfos[i], BIND_MEMORY_STATUS);

      if (bind_status)
         *bind_status->pResult = VK_SUCCESS;

      assert(mem != NULL);
      assert(buffer->vk.device_address == 0);

      buffer->vk.device_address = mem->addr.dev + pBindInfos[i].memoryOffset;

      /* FIXME: Only host map for index buffers so we can do the min/max
       * index retrieval on the CPU. This is all broken anyway and the
       * min/max search should be done with a compute shader that also
       * patches the job descriptor accordingly (basically an indirect draw).
       *
       * Make sure this goes away as soon as we fixed indirect draws.
       */
      if (arch < 9 && (buffer->vk.usage & VK_BUFFER_USAGE_INDEX_BUFFER_BIT)) {
         VkDeviceSize offset = pBindInfos[i].memoryOffset;
         VkDeviceSize pgsize = getpagesize();
         off_t map_start = offset & ~(pgsize - 1);
         off_t map_end = offset + buffer->vk.size;
         void *map_addr =
            pan_kmod_bo_mmap(mem->bo, map_start, map_end - map_start,
                             PROT_WRITE, MAP_SHARED, NULL);

         if (map_addr == MAP_FAILED) {
            result = panvk_errorf(device, VK_ERROR_OUT_OF_HOST_MEMORY,
                                  "Failed to CPU map index buffer");
            if (bind_status)
               *bind_status->pResult = result;
            continue;
         }

         buffer->host_ptr = map_addr + (offset & pgsize);
      }
   }
   return result;
}

VKAPI_ATTR VkResult VKAPI_CALL
panvk_CreateBuffer(VkDevice _device, const VkBufferCreateInfo *pCreateInfo,
                   const VkAllocationCallbacks *pAllocator, VkBuffer *pBuffer)
{
   VK_FROM_HANDLE(panvk_device, device, _device);
   struct panvk_buffer *buffer;

   assert(pCreateInfo->sType == VK_STRUCTURE_TYPE_BUFFER_CREATE_INFO);

   if (pCreateInfo->size > PANVK_MAX_BUFFER_SIZE)
      return panvk_error(device, VK_ERROR_OUT_OF_DEVICE_MEMORY);

   buffer =
      vk_buffer_create(&device->vk, pCreateInfo, pAllocator, sizeof(*buffer));
   if (buffer == NULL)
      return panvk_error(device, VK_ERROR_OUT_OF_HOST_MEMORY);

   *pBuffer = panvk_buffer_to_handle(buffer);

   return VK_SUCCESS;
}

VKAPI_ATTR void VKAPI_CALL
panvk_DestroyBuffer(VkDevice _device, VkBuffer _buffer,
                    const VkAllocationCallbacks *pAllocator)
{
   VK_FROM_HANDLE(panvk_device, device, _device);
   VK_FROM_HANDLE(panvk_buffer, buffer, _buffer);

   if (!buffer)
      return;

   if (buffer->host_ptr) {
      VkDeviceSize pgsize = getpagesize();
      uintptr_t map_start = (uintptr_t)buffer->host_ptr & ~(pgsize - 1);
      uintptr_t map_end =
         ALIGN_POT((uintptr_t)buffer->host_ptr + buffer->vk.size, pgsize);
      ASSERTED int ret = os_munmap((void *)map_start, map_end - map_start);

      assert(!ret);
      buffer->host_ptr = NULL;
   }

   vk_buffer_destroy(&device->vk, pAllocator, &buffer->vk);
}
