/*
 * Copyright © 2022 Collabora Ltd. and Red Hat Inc.
 * SPDX-License-Identifier: MIT
 */
#include "nvk_heap.h"

#include "nvk_device.h"
#include "nvk_physical_device.h"
#include "nvk_queue.h"

#include "util/macros.h"

#include "nv_push.h"
#include "nv_push_cl90b5.h"

VkResult
nvk_heap_init(struct nvk_device *dev, struct nvk_heap *heap,
              enum nvkmd_mem_flags mem_flags,
              enum nvkmd_mem_map_flags map_flags,
              uint32_t overalloc, bool contiguous)
{
   VkResult result = nvk_mem_arena_init(dev, &heap->arena, mem_flags, map_flags,
                                        contiguous, NVK_MEM_ARENA_MAX_SIZE);
   if (result != VK_SUCCESS)
      return result;

   assert(overalloc < NVK_MEM_ARENA_MIN_SIZE);
   heap->overalloc = overalloc;
   util_vma_heap_init(&heap->heap, 0, 0);

   return VK_SUCCESS;
}

void
nvk_heap_finish(struct nvk_device *dev, struct nvk_heap *heap)
{
   util_vma_heap_finish(&heap->heap);
   nvk_mem_arena_finish(dev, &heap->arena);
}

static VkResult
nvk_heap_grow_locked(struct nvk_device *dev, struct nvk_heap *heap)
{
   const bool is_first_grow = nvk_mem_arena_size_B(&heap->arena) == 0;
   VkResult result;

   uint64_t addr, mem_size_B;
   result = nvk_mem_arena_grow_locked(dev, &heap->arena, &addr, &mem_size_B);
   if (result != VK_SUCCESS)
      return result;

   if (nvk_mem_arena_is_contiguous(&heap->arena) && !is_first_grow) {
      util_vma_heap_free(&heap->heap, addr - heap->overalloc, mem_size_B);
   } else {
      util_vma_heap_free(&heap->heap, addr, mem_size_B - heap->overalloc);
   }

   return VK_SUCCESS;
}

static VkResult
nvk_heap_alloc_locked(struct nvk_device *dev, struct nvk_heap *heap,
                      uint64_t size, uint32_t alignment,
                      uint64_t *addr_out, void **map_out)
{
   /* Make sure we follow the restrictions in nvk_mem_arena_map(). */
   if (map_out != NULL && nvk_mem_arena_is_mapped(&heap->arena)) {
      assert(size <= NVK_MEM_ARENA_MIN_SIZE);
      alignment = MAX2(alignment, util_next_power_of_two(size));
   }

   while (1) {
      uint64_t addr = util_vma_heap_alloc(&heap->heap, size, alignment);
      if (addr != 0) {
         *addr_out = addr;

         if (map_out != NULL) {
            if (nvk_mem_arena_is_mapped(&heap->arena))
               *map_out = nvk_mem_arena_map(&heap->arena, addr, size);
            else
               *map_out = NULL;
         }

         return VK_SUCCESS;
      }

      VkResult result = nvk_heap_grow_locked(dev, heap);
      if (result != VK_SUCCESS)
         return result;
   }
}

static void
nvk_heap_free_locked(struct nvk_device *dev, struct nvk_heap *heap,
                     uint64_t addr, uint64_t size)
{
   assert(addr + size > addr);
   util_vma_heap_free(&heap->heap, addr, size);
}

VkResult
nvk_heap_alloc(struct nvk_device *dev, struct nvk_heap *heap,
               uint64_t size, uint32_t alignment,
               uint64_t *addr_out, void **map_out)
{
   simple_mtx_lock(&heap->arena.mutex);
   VkResult result = nvk_heap_alloc_locked(dev, heap, size, alignment,
                                           addr_out, map_out);
   simple_mtx_unlock(&heap->arena.mutex);

   return result;
}

VkResult
nvk_heap_upload(struct nvk_device *dev, struct nvk_heap *heap,
                const void *data, size_t size, uint32_t alignment,
                uint64_t *addr_out)
{
   VkResult result;

   result = nvk_heap_alloc(dev, heap, size, alignment, addr_out, NULL);
   if (result != VK_SUCCESS)
      return result;

   if (heap->arena.map_flags & NVKMD_MEM_MAP_WR) {
      nvk_mem_arena_copy_to_gpu(&heap->arena, *addr_out, data, size);
   } else {
      /* Otherwise, kick off an upload with the upload queue.
       *
       * This is a queued operation that the driver ensures happens before any
       * more client work via semaphores.  Because this is asynchronous and
       * heap allocations are synchronous we have to be a bit careful here.
       * The heap only ever tracks the current known CPU state of everything
       * while the upload queue makes that state valid at some point in the
       * future.
       *
       * This can be especially tricky for very fast upload/free cycles such
       * as if the client compiles a shader, throws it away without using it,
       * and then compiles another shader that ends up at the same address.
       * What makes this all correct is the fact that the everything on the
       * upload queue happens in a well-defined device-wide order.  In this
       * case the first shader will get uploaded and then the second will get
       * uploaded over top of it.  As long as we don't free the memory out
       * from under the upload queue, everything will end up in the correct
       * state by the time the client's shaders actually execute.
       */
      result = nvk_upload_queue_upload(dev, &dev->upload, *addr_out, data, size);
      if (result != VK_SUCCESS) {
         nvk_heap_free(dev, heap, *addr_out, size);
         return result;
      }
   }

   return VK_SUCCESS;
}

void
nvk_heap_free(struct nvk_device *dev, struct nvk_heap *heap,
              uint64_t addr, uint64_t size)
{
   simple_mtx_lock(&heap->arena.mutex);
   nvk_heap_free_locked(dev, heap, addr, size);
   simple_mtx_unlock(&heap->arena.mutex);
}
