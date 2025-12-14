/*
 * Copyright © 2023 Imagination Technologies Ltd.
 *
 * based in part on anv driver which is:
 * Copyright © 2015 Intel Corporation
 *
 * based in part on radv driver which is:
 * Copyright © 2016 Red Hat.
 * Copyright © 2016 Bas Nieuwenhuizen
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

#ifndef PVR_COMMON_H
#define PVR_COMMON_H

#include <stdbool.h>
#include <stdint.h>
#include <vulkan/vulkan.h>

/* FIXME: Rename this, and ensure it only contains what's
 * relevant for the driver/compiler interface (no Vulkan types).
 */

#include "hwdef/rogue_hw_defs.h"
#include "pvr_limits.h"
#include "pvr_types.h"
#include "util/list.h"
#include "util/macros.h"
#include "util/vma.h"
#include "vk_descriptor_set_layout.h"
#include "vk_object.h"
#include "vk_sampler.h"
#include "vk_sync.h"

#define VK_VENDOR_ID_IMAGINATION 0x1010

#define PVR_WORKGROUP_DIMENSIONS 3U

#define PVR_SAMPLER_DESCRIPTOR_SIZE 4U
#define PVR_IMAGE_DESCRIPTOR_SIZE 4U

#define PVR_STATE_PBE_DWORDS 2U

#define PVR_PIPELINE_LAYOUT_SUPPORTED_DESCRIPTOR_TYPE_COUNT \
   (uint32_t)(VK_DESCRIPTOR_TYPE_INPUT_ATTACHMENT + 1U)

#define PVR_TRANSFER_MAX_LAYERS 1U
#define PVR_TRANSFER_MAX_LOADS 4U
#define PVR_TRANSFER_MAX_IMAGES \
   (PVR_TRANSFER_MAX_LAYERS * PVR_TRANSFER_MAX_LOADS)

/* TODO: move into a common surface library? */
enum pvr_memlayout {
   PVR_MEMLAYOUT_UNDEFINED = 0, /* explicitly treat 0 as undefined */
   PVR_MEMLAYOUT_LINEAR,
   PVR_MEMLAYOUT_TWIDDLED,
   PVR_MEMLAYOUT_3DTWIDDLED,
};

enum pvr_texture_state {
   PVR_TEXTURE_STATE_SAMPLE,
   PVR_TEXTURE_STATE_STORAGE,
   PVR_TEXTURE_STATE_ATTACHMENT,
   PVR_TEXTURE_STATE_MAX_ENUM,
};

enum pvr_sub_cmd_type {
   PVR_SUB_CMD_TYPE_INVALID = 0, /* explicitly treat 0 as invalid */
   PVR_SUB_CMD_TYPE_GRAPHICS,
   PVR_SUB_CMD_TYPE_COMPUTE,
   PVR_SUB_CMD_TYPE_TRANSFER,
   PVR_SUB_CMD_TYPE_OCCLUSION_QUERY,
   PVR_SUB_CMD_TYPE_EVENT,
};

enum pvr_event_type {
   PVR_EVENT_TYPE_SET,
   PVR_EVENT_TYPE_RESET,
   PVR_EVENT_TYPE_WAIT,
   PVR_EVENT_TYPE_BARRIER,
};

enum pvr_depth_stencil_usage {
   PVR_DEPTH_STENCIL_USAGE_UNDEFINED = 0, /* explicitly treat 0 as undefined */
   PVR_DEPTH_STENCIL_USAGE_NEEDED,
   PVR_DEPTH_STENCIL_USAGE_NEVER,
};

enum pvr_job_type {
   PVR_JOB_TYPE_GEOM,
   PVR_JOB_TYPE_FRAG,
   PVR_JOB_TYPE_COMPUTE,
   PVR_JOB_TYPE_TRANSFER,
   PVR_JOB_TYPE_OCCLUSION_QUERY,
   PVR_JOB_TYPE_MAX
};

enum pvr_pipeline_type {
   PVR_PIPELINE_TYPE_INVALID = 0, /* explicitly treat 0 as undefined */
   PVR_PIPELINE_TYPE_GRAPHICS,
   PVR_PIPELINE_TYPE_COMPUTE,
};

enum pvr_pipeline_stage_bits {
   PVR_PIPELINE_STAGE_GEOM_BIT = BITFIELD_BIT(PVR_JOB_TYPE_GEOM),
   PVR_PIPELINE_STAGE_FRAG_BIT = BITFIELD_BIT(PVR_JOB_TYPE_FRAG),
   PVR_PIPELINE_STAGE_COMPUTE_BIT = BITFIELD_BIT(PVR_JOB_TYPE_COMPUTE),
   PVR_PIPELINE_STAGE_TRANSFER_BIT = BITFIELD_BIT(PVR_JOB_TYPE_TRANSFER),
   /* Note that this doesn't map to VkPipelineStageFlagBits so be careful with
    * this.
    */
   PVR_PIPELINE_STAGE_OCCLUSION_QUERY_BIT =
      BITFIELD_BIT(PVR_JOB_TYPE_OCCLUSION_QUERY),
};

#define PVR_PIPELINE_STAGE_ALL_GRAPHICS_BITS \
   (PVR_PIPELINE_STAGE_GEOM_BIT | PVR_PIPELINE_STAGE_FRAG_BIT)

#define PVR_PIPELINE_STAGE_ALL_BITS                                         \
   (PVR_PIPELINE_STAGE_ALL_GRAPHICS_BITS | PVR_PIPELINE_STAGE_COMPUTE_BIT | \
    PVR_PIPELINE_STAGE_TRANSFER_BIT)

#define PVR_NUM_SYNC_PIPELINE_STAGES 4U

/* Warning: Do not define an invalid stage as 0 since other code relies on 0
 * being the first shader stage. This allows for stages to be split or added
 * in the future. Defining 0 as invalid will very likely cause problems.
 */
enum pvr_stage_allocation {
   PVR_STAGE_ALLOCATION_VERTEX_GEOMETRY,
   PVR_STAGE_ALLOCATION_FRAGMENT,
   PVR_STAGE_ALLOCATION_COMPUTE,
   PVR_STAGE_ALLOCATION_COUNT
};

enum pvr_filter {
   PVR_FILTER_DONTCARE, /* Any filtering mode is acceptable. */
   PVR_FILTER_POINT,
   PVR_FILTER_LINEAR,
   PVR_FILTER_BICUBIC,
};

enum pvr_resolve_op {
   PVR_RESOLVE_BLEND,
   PVR_RESOLVE_MIN,
   PVR_RESOLVE_MAX,
   PVR_RESOLVE_SAMPLE0,
   PVR_RESOLVE_SAMPLE1,
   PVR_RESOLVE_SAMPLE2,
   PVR_RESOLVE_SAMPLE3,
   PVR_RESOLVE_SAMPLE4,
   PVR_RESOLVE_SAMPLE5,
   PVR_RESOLVE_SAMPLE6,
   PVR_RESOLVE_SAMPLE7,
};

enum pvr_event_state {
   PVR_EVENT_STATE_SET_BY_HOST,
   PVR_EVENT_STATE_RESET_BY_HOST,
   PVR_EVENT_STATE_SET_BY_DEVICE,
   PVR_EVENT_STATE_RESET_BY_DEVICE
};

enum pvr_deferred_cs_command_type {
   PVR_DEFERRED_CS_COMMAND_TYPE_DBSC,
   PVR_DEFERRED_CS_COMMAND_TYPE_DBSC2,
};

enum pvr_query_type {
   PVR_QUERY_TYPE_AVAILABILITY_WRITE,
   PVR_QUERY_TYPE_RESET_QUERY_POOL,
   PVR_QUERY_TYPE_COPY_QUERY_RESULTS,
};

struct pvr_buffer_descriptor {
   uint64_t addr;
} PACKED;
static_assert(sizeof(struct pvr_buffer_descriptor) == 2 * sizeof(uint32_t),
              "pvr_buffer_descriptor size is invalid.");

struct pvr_sampler_descriptor {
   uint64_t words[ROGUE_NUM_TEXSTATE_SAMPLER_WORDS];
} PACKED;
static_assert(sizeof(struct pvr_sampler_descriptor) ==
                 ROGUE_NUM_TEXSTATE_SAMPLER_WORDS * sizeof(uint64_t),
              "pvr_sampler_descriptor size is invalid.");

struct pvr_image_descriptor {
   uint64_t words[ROGUE_NUM_TEXSTATE_IMAGE_WORDS];
} PACKED;
static_assert(sizeof(struct pvr_image_descriptor) ==
                 ROGUE_NUM_TEXSTATE_IMAGE_WORDS * sizeof(uint64_t),
              "pvr_image_descriptor size is invalid.");

struct pvr_combined_image_sampler_descriptor {
   struct pvr_image_descriptor image;
   struct pvr_sampler_descriptor sampler;
} PACKED;
static_assert(sizeof(struct pvr_combined_image_sampler_descriptor) ==
                 (ROGUE_NUM_TEXSTATE_IMAGE_WORDS +
                  ROGUE_NUM_TEXSTATE_SAMPLER_WORDS) *
                    sizeof(uint64_t),
              "pvr_combined_image_sampler_descriptor size is invalid.");

struct pvr_sampler {
   struct vk_sampler vk;
   struct pvr_sampler_descriptor descriptor;
};

struct pvr_descriptor_set_layout_binding {
   VkDescriptorType type;
   VkDescriptorBindingFlags flags;

   uint32_t stage_flags; /** Which stages can use this binding. */

   uint32_t descriptor_count;
   uint32_t immutable_sampler_count;
   struct pvr_sampler **immutable_samplers;

   unsigned offset; /** Offset within the descriptor set. */
   unsigned stride; /** Stride of each descriptor in this binding. */
};

struct pvr_descriptor_set_layout {
   struct vk_descriptor_set_layout vk;
   VkDescriptorSetLayoutCreateFlagBits flags;

   uint32_t descriptor_count;
   uint32_t dynamic_buffer_count;

   uint32_t binding_count;
   struct pvr_descriptor_set_layout_binding *bindings;

   uint32_t immutable_sampler_count;
   struct pvr_sampler **immutable_samplers;

   uint32_t stage_flags; /** Which stages can use this binding. */

   unsigned size; /** Size in bytes. */
};

struct pvr_descriptor_pool {
   struct vk_object_base base;

   VkDescriptorType type;
   VkAllocationCallbacks alloc;
   VkDescriptorPoolCreateFlags flags;

   /** List of the descriptor sets created using this pool. */
   struct list_head desc_sets;

   struct pvr_suballoc_bo *pvr_bo; /** Pool buffer object. */
   void *mapping; /** Pool buffer CPU mapping. */
   struct util_vma_heap heap; /** Pool (sub)allocation heap. */
};

struct pvr_descriptor {
   VkDescriptorType type;

   union {
      struct {
         struct pvr_buffer_view *bview;
         pvr_dev_addr_t buffer_dev_addr;
         VkDeviceSize buffer_desc_range;
         VkDeviceSize buffer_whole_range;
      };

      struct {
         VkImageLayout layout;
         const struct pvr_image_view *iview;
         const struct pvr_sampler *sampler;
      };
   };
};

struct pvr_descriptor_set {
   struct vk_object_base base;

   struct pvr_descriptor_set_layout *layout;
   struct pvr_descriptor_pool *pool;

   unsigned size; /** Descriptor set size. */
   pvr_dev_addr_t dev_addr; /** Descriptor set device address. */
   void *mapping; /** Descriptor set CPU mapping. */

   struct list_head link; /** Link in pvr_descriptor_pool::desc_sets. */
};

struct pvr_event {
   struct vk_object_base base;

   enum pvr_event_state state;
   struct vk_sync *sync;
};

#define PVR_MAX_DYNAMIC_BUFFERS                      \
   (PVR_MAX_DESCRIPTOR_SET_UNIFORM_DYNAMIC_BUFFERS + \
    PVR_MAX_DESCRIPTOR_SET_STORAGE_DYNAMIC_BUFFERS)

struct pvr_descriptor_state {
   struct pvr_descriptor_set *sets[PVR_MAX_DESCRIPTOR_SETS];
   uint32_t dirty_sets;
};

#undef PVR_MAX_DYNAMIC_BUFFERS

#endif /* PVR_COMMON_H */
