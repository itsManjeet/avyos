/*
 * Copyright © 2022 Imagination Technologies Ltd.
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice (including the next
 * paragraph) shall be included in all copies or substantial portions of the
 * Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

#include <assert.h>
#include <limits.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <vulkan/vulkan.h>

#include "hwdef/rogue_hw_utils.h"
#include "pvr_bo.h"
#include "pvr_debug.h"
#include "pvr_private.h"
#include "pvr_types.h"
#include "util/compiler.h"
#include "util/list.h"
#include "util/log.h"
#include "util/macros.h"
#include "util/vma.h"
#include "vk_alloc.h"
#include "vk_descriptors.h"
#include "vk_descriptor_set_layout.h"
#include "vk_format.h"
#include "vk_log.h"
#include "vk_object.h"
#include "vk_util.h"
#include "vulkan/util/vk_enum_to_str.h"

static bool
binding_has_immutable_samplers(const VkDescriptorSetLayoutBinding *binding)
{
   switch (binding->descriptorType) {
   case VK_DESCRIPTOR_TYPE_SAMPLER:
   case VK_DESCRIPTOR_TYPE_COMBINED_IMAGE_SAMPLER:
      return binding->pImmutableSamplers != NULL;

   default:
      return false;
   }
}

static bool
binding_has_dynamic_buffers(const VkDescriptorSetLayoutBinding *binding)
{
   return vk_descriptor_type_is_dynamic(binding->descriptorType);
}

static bool
binding_has_combined_image_samplers(const VkDescriptorSetLayoutBinding *binding)
{
   return binding->descriptorType == VK_DESCRIPTOR_TYPE_COMBINED_IMAGE_SAMPLER;
}

static unsigned pvr_descriptor_size(VkDescriptorType type)
{
   switch (type) {
   case VK_DESCRIPTOR_TYPE_UNIFORM_BUFFER:
      return sizeof(struct pvr_buffer_descriptor);

   default:
      mesa_loge("Unsupported descriptor type %s.\n",
                vk_DescriptorType_to_str(type));
      unreachable("");
   }
}

VkResult pvr_CreateDescriptorSetLayout(
   VkDevice _device,
   const VkDescriptorSetLayoutCreateInfo *pCreateInfo,
   const VkAllocationCallbacks *pAllocator,
   VkDescriptorSetLayout *pSetLayout)
{
   PVR_FROM_HANDLE(pvr_device, device, _device);
   VkDescriptorSetLayoutBinding *bindings;
   uint32_t binding_count = 0;
   uint32_t immutable_sampler_count = 0;
   uint32_t dynamic_buffer_count = 0;
   uint32_t descriptor_count = 0;
   VkResult result = VK_SUCCESS;

   assert(pCreateInfo->sType ==
          VK_STRUCTURE_TYPE_DESCRIPTOR_SET_LAYOUT_CREATE_INFO);

   vk_foreach_struct_const (ext, pCreateInfo->pNext) {
      vk_debug_ignored_stype(ext->sType);
   }

   for (unsigned u = 0; u < pCreateInfo->bindingCount; ++u) {
      const VkDescriptorSetLayoutBinding *binding = &pCreateInfo->pBindings[u];

      descriptor_count += binding->descriptorCount;

      if (binding_has_immutable_samplers(binding))
         immutable_sampler_count += binding->descriptorCount;
      else if (binding_has_dynamic_buffers(binding))
         dynamic_buffer_count += binding->descriptorCount;
   }

   result = vk_create_sorted_bindings(pCreateInfo->pBindings,
                                      pCreateInfo->bindingCount,
                                      &bindings);

   if (result != VK_SUCCESS)
      return vk_error(device, result);

   if (bindings)
      binding_count = bindings[pCreateInfo->bindingCount - 1].binding + 1;

   VK_MULTIALLOC(ma);
   VK_MULTIALLOC_DECL(&ma, struct pvr_descriptor_set_layout, layout, 1);
   VK_MULTIALLOC_DECL(&ma,
                      struct pvr_descriptor_set_layout_binding,
                      layout_bindings,
                      binding_count);
   VK_MULTIALLOC_DECL(&ma,
                      struct pvr_sampler *,
                      immutable_samplers,
                      immutable_sampler_count);

   if (!vk_descriptor_set_layout_multizalloc(&device->vk, &ma, pCreateInfo)) {
      result = VK_ERROR_OUT_OF_HOST_MEMORY;
      goto err_free_bindings;
   }

   layout->flags = pCreateInfo->flags;

   layout->descriptor_count = descriptor_count;
   layout->dynamic_buffer_count = dynamic_buffer_count;

   layout->binding_count = binding_count;
   layout->bindings = layout_bindings;

   layout->immutable_sampler_count = immutable_sampler_count;
   layout->immutable_samplers = immutable_samplers;

   const VkDescriptorSetLayoutBindingFlagsCreateInfo *binding_flags_create_info =
      vk_find_struct_const(pCreateInfo->pNext,
                           DESCRIPTOR_SET_LAYOUT_BINDING_FLAGS_CREATE_INFO);

   if (binding_flags_create_info && !binding_flags_create_info->bindingCount)
      binding_flags_create_info = NULL;

   assert(!binding_flags_create_info ||
          binding_flags_create_info->bindingCount == binding_count);

   for (unsigned b = 0; b < pCreateInfo->bindingCount; ++b) {
      const VkDescriptorSetLayoutBinding *binding = &bindings[b];

      if (!binding->descriptorCount)
         continue;

      struct pvr_descriptor_set_layout_binding *layout_binding =
         &layout_bindings[binding->binding];

      layout_binding->offset = layout->size;
      layout_binding->stride = pvr_descriptor_size(binding->descriptorType);

      layout->size += binding->descriptorCount * layout_binding->stride;

      layout_binding->type = binding->descriptorType;

      layout_binding->flags = binding_flags_create_info
                                 ? binding_flags_create_info->pBindingFlags[b]
                                 : 0;

      layout_binding->descriptor_count = binding->descriptorCount;
      layout_binding->stage_flags = binding->stageFlags;
      layout->stage_flags |= binding->stageFlags;

      if (binding_has_immutable_samplers(binding)) {
         layout_binding->immutable_sampler_count = binding->descriptorCount;
         layout_binding->immutable_samplers = immutable_samplers;
         immutable_samplers += binding->descriptorCount;

         for (unsigned s = 0; s < layout_binding->immutable_sampler_count;
              ++s) {
            VK_FROM_HANDLE(pvr_sampler,
                           sampler,
                           binding->pImmutableSamplers[s]);
            layout_binding->immutable_samplers[s] = sampler;
         }
      }
   }

   free(bindings);

   *pSetLayout = pvr_descriptor_set_layout_to_handle(layout);

   return VK_SUCCESS;

err_free_bindings:
   free(bindings);

   return result;
}

VkResult pvr_CreateDescriptorPool(VkDevice _device,
                                  const VkDescriptorPoolCreateInfo *pCreateInfo,
                                  const VkAllocationCallbacks *pAllocator,
                                  VkDescriptorPool *pDescriptorPool)
{
   PVR_FROM_HANDLE(pvr_device, device, _device);
   const uint32_t cache_line_size =
      rogue_get_slc_cache_line_size(&device->pdevice->dev_info);
   struct pvr_descriptor_pool *pool;
   uint64_t bo_size = 0;
   VkResult result;

   assert(pCreateInfo->sType == VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO);

   pool = vk_object_alloc(&device->vk,
                          pAllocator,
                          sizeof(*pool),
                          VK_OBJECT_TYPE_DESCRIPTOR_POOL);
   if (!pool)
      return vk_error(device, VK_ERROR_OUT_OF_HOST_MEMORY);

   if (pAllocator)
      pool->alloc = *pAllocator;
   else
      pool->alloc = device->vk.alloc;

   pool->flags = pCreateInfo->flags;

   list_inithead(&pool->desc_sets);

   if (pCreateInfo->maxSets > 0) {
      for (uint32_t i = 0; i < pCreateInfo->poolSizeCount; i++) {
         const VkDescriptorType type = pCreateInfo->pPoolSizes[i].type;
         const uint32_t descriptor_count =
            pCreateInfo->pPoolSizes[i].descriptorCount;

         bo_size += descriptor_count * pvr_descriptor_size(type);
      }
   }

   result = pvr_bo_suballoc(&device->suballoc_general,
                            bo_size,
                            cache_line_size,
                            false,
                            &pool->pvr_bo);

   if (result != VK_SUCCESS)
      goto err_free_pool;

   pool->mapping = pvr_bo_suballoc_get_map_addr(pool->pvr_bo);
   assert(pool->mapping);

   util_vma_heap_init(&pool->heap, pool->pvr_bo->dev_addr.addr, bo_size);

   *pDescriptorPool = pvr_descriptor_pool_to_handle(pool);

   return VK_SUCCESS;

err_free_pool:
   vk_object_free(&device->vk, pAllocator, pool);

   return result;
}

static VkResult pvr_pool_alloc(struct pvr_descriptor_pool *pool,
                               unsigned size,
                               pvr_dev_addr_t *dev_addr,
                               void **mapping)
{
   uint64_t _dev_addr = util_vma_heap_alloc(&pool->heap, size, 1);
   if (!_dev_addr)
      return VK_ERROR_OUT_OF_POOL_MEMORY;

   *mapping =
      (uint8_t *)pool->mapping + (_dev_addr - pool->pvr_bo->dev_addr.addr);

   *dev_addr = PVR_DEV_ADDR(_dev_addr);

   return VK_SUCCESS;
}

static void pvr_pool_free(struct pvr_descriptor_pool *pool,
                          pvr_dev_addr_t *dev_addr,
                          unsigned size)
{
   util_vma_heap_free(&pool->heap, dev_addr->addr, size);
}

static void pvr_free_descriptor_set(struct pvr_device *device,
                                    struct pvr_descriptor_pool *pool,
                                    struct pvr_descriptor_set *set)
{
   list_del(&set->link);
   vk_descriptor_set_layout_unref(&device->vk, &set->layout->vk);
   if (set->size > 0)
      pvr_pool_free(pool, &set->dev_addr, set->size);
   vk_object_free(&device->vk, &pool->alloc, set);
}

void pvr_DestroyDescriptorPool(VkDevice _device,
                               VkDescriptorPool _pool,
                               const VkAllocationCallbacks *pAllocator)
{
   PVR_FROM_HANDLE(pvr_device, device, _device);
   PVR_FROM_HANDLE(pvr_descriptor_pool, pool, _pool);

   if (!pool)
      return;

   list_for_each_entry_safe (struct pvr_descriptor_set,
                             set,
                             &pool->desc_sets,
                             link) {
      pvr_free_descriptor_set(device, pool, set);
   }

   util_vma_heap_finish(&pool->heap);
   pvr_bo_suballoc_free(pool->pvr_bo);

   vk_object_free(&device->vk, pAllocator, pool);
}

VkResult pvr_ResetDescriptorPool(VkDevice _device,
                                 VkDescriptorPool descriptorPool,
                                 VkDescriptorPoolResetFlags flags)
{
   PVR_FROM_HANDLE(pvr_descriptor_pool, pool, descriptorPool);
   PVR_FROM_HANDLE(pvr_device, device, _device);

   list_for_each_entry_safe (struct pvr_descriptor_set,
                             set,
                             &pool->desc_sets,
                             link) {
      pvr_free_descriptor_set(device, pool, set);
   }

   return VK_SUCCESS;
}

static VkResult
pvr_descriptor_set_create(struct pvr_device *device,
                          struct pvr_descriptor_pool *pool,
                          struct pvr_descriptor_set_layout *layout,
                          struct pvr_descriptor_set **const descriptor_set_out)
{
   struct pvr_descriptor_set *set;
   VkResult result;

   *descriptor_set_out = NULL;

   set = vk_object_zalloc(&device->vk,
                          &pool->alloc,
                          sizeof(*set),
                          VK_OBJECT_TYPE_DESCRIPTOR_SET);
   if (!set)
      return vk_error(device, VK_ERROR_OUT_OF_HOST_MEMORY);

   set->size = layout->size;
   if (set->size > 0) {
      result = pvr_pool_alloc(pool, set->size, &set->dev_addr, &set->mapping);
      if (result != VK_SUCCESS)
         goto err_free_descriptor_set;
   }

   vk_descriptor_set_layout_ref(&layout->vk);
   set->layout = layout;
   set->pool = pool;

   list_addtail(&set->link, &pool->desc_sets);

   *descriptor_set_out = set;

   return VK_SUCCESS;

err_free_descriptor_set:
   vk_object_free(&device->vk, &pool->alloc, set);

   return result;
}

VkResult
pvr_AllocateDescriptorSets(VkDevice _device,
                           const VkDescriptorSetAllocateInfo *pAllocateInfo,
                           VkDescriptorSet *pDescriptorSets)
{
   PVR_FROM_HANDLE(pvr_descriptor_pool, pool, pAllocateInfo->descriptorPool);
   PVR_FROM_HANDLE(pvr_device, device, _device);
   VkResult result;
   uint32_t i;

   vk_foreach_struct_const (ext, pAllocateInfo->pNext) {
      vk_debug_ignored_stype(ext->sType);
   }

   for (i = 0; i < pAllocateInfo->descriptorSetCount; i++) {
      PVR_FROM_HANDLE(pvr_descriptor_set_layout,
                      layout,
                      pAllocateInfo->pSetLayouts[i]);
      struct pvr_descriptor_set *set;

      result = pvr_descriptor_set_create(device, pool, layout, &set);
      if (result != VK_SUCCESS)
         goto err_free_descriptor_sets;

      pDescriptorSets[i] = pvr_descriptor_set_to_handle(set);
   }

   return VK_SUCCESS;

err_free_descriptor_sets:
   pvr_FreeDescriptorSets(_device,
                          pAllocateInfo->descriptorPool,
                          i,
                          pDescriptorSets);

   for (i = 0; i < pAllocateInfo->descriptorSetCount; i++)
      pDescriptorSets[i] = VK_NULL_HANDLE;

   return result;
}

VkResult pvr_FreeDescriptorSets(VkDevice _device,
                                VkDescriptorPool descriptorPool,
                                uint32_t count,
                                const VkDescriptorSet *pDescriptorSets)
{
   PVR_FROM_HANDLE(pvr_descriptor_pool, pool, descriptorPool);
   PVR_FROM_HANDLE(pvr_device, device, _device);

   for (uint32_t i = 0; i < count; i++) {
      struct pvr_descriptor_set *set;

      if (!pDescriptorSets[i])
         continue;

      set = pvr_descriptor_set_from_handle(pDescriptorSets[i]);
      pvr_free_descriptor_set(device, pool, set);
   }

   return VK_SUCCESS;
}

static void
write_buffer(const struct pvr_descriptor_set *set,
             const VkDescriptorBufferInfo *buffer_info,
             const struct pvr_descriptor_set_layout_binding *binding,
             uint32_t elem)
{
   VK_FROM_HANDLE(pvr_buffer, buffer, buffer_info->buffer);
   const unsigned desc_offset = binding->offset + (elem * binding->stride);
   void *desc_mapping = (uint8_t *)set->mapping + desc_offset;

   const pvr_dev_addr_t buffer_addr =
      PVR_DEV_ADDR_OFFSET(buffer->dev_addr, buffer_info->offset);

   const struct pvr_buffer_descriptor buffer_desc = {
      .addr = buffer_addr.addr,
   };

   memcpy(desc_mapping, &buffer_desc, sizeof(buffer_desc));
}

void pvr_UpdateDescriptorSets(VkDevice _device,
                              uint32_t descriptorWriteCount,
                              const VkWriteDescriptorSet *pDescriptorWrites,
                              uint32_t descriptorCopyCount,
                              const VkCopyDescriptorSet *pDescriptorCopies)
{
   for (uint32_t i = 0; i < descriptorWriteCount; i++) {
      const VkWriteDescriptorSet *write = &pDescriptorWrites[i];
      PVR_FROM_HANDLE(pvr_descriptor_set, set, write->dstSet);
      const struct pvr_descriptor_set_layout *layout = set->layout;
      const struct pvr_descriptor_set_layout_binding *binding;

      assert(write->dstBinding < layout->binding_count);
      binding = &layout->bindings[write->dstBinding];

      vk_foreach_struct_const (ext, write->pNext) {
         vk_debug_ignored_stype(ext->sType);
      }

      if (!binding->stage_flags)
         continue;

      switch (write->descriptorType) {
      case VK_DESCRIPTOR_TYPE_UNIFORM_BUFFER:
         for (uint32_t j = 0; j < write->descriptorCount; j++) {
            write_buffer(set,
                         &write->pBufferInfo[j],
                         binding,
                         write->dstArrayElement + j);
         }
         break;

      default:
         unreachable("");
      }
   }

   assert(!descriptorCopyCount);
}
