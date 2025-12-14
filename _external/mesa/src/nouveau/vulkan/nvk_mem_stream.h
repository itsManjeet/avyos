/*
 * Copyright © 2025 Collabora Ltd. and Red Hat Inc.
 * SPDX-License-Identifier: MIT
 */
#ifndef NVK_MEM_STREAM_H
#define NVK_MEM_STREAM_H 1

#include "nvk_private.h"

#include "util/list.h"

struct nvk_device;
struct nvk_mem_stream_chunk;
struct nvkmd_ctx;
struct vk_sync;
struct vk_sync_wait;

#define NVK_MEM_STREAM_MAX_ALLOC_SIZE (64*1024)

/** A streaming memory allocator */
struct nvk_mem_stream {
   struct vk_sync *sync;
   uint64_t next_time_point;
   uint64_t time_point_passed;

   bool needs_flush;

   struct nvk_mem_stream_chunk *chunk;
   uint32_t chunk_alloc_B;

   /* list of nvk_mem_stream_chunk */
   struct list_head recycle;
};

VkResult nvk_mem_stream_init(struct nvk_device *dev,
                             struct nvk_mem_stream *stream);

/* The caller must ensure the stream is not in use.  This can be done by
 * calling hvk_mem_stream_sync().
 */
void nvk_mem_stream_finish(struct nvk_device *dev,
                           struct nvk_mem_stream *stream);

VkResult nvk_mem_stream_alloc(struct nvk_device *dev,
                              struct nvk_mem_stream *stream,
                              uint32_t size_B, uint32_t align_B,
                              uint64_t *addr_out, void **map_out);

/** Flushes the stream.
 *
 * Any memory allocated by nvk_mem_stream_alloc() prior to this call are now
 * owned by the GPU and may no longer be accessed on the CPU.  The memory will
 * be automatically recycled once the GPU is done with it.
 *
 * If non-NULL, time_point_out will be populated with a time point which some
 * other context can use to wait on this stream with stream->sync.  If the
 * flush fails, time_point_out is not written.
 */
VkResult nvk_mem_stream_flush(struct nvk_device *dev,
                              struct nvk_mem_stream *stream,
                              struct nvkmd_ctx *ctx,
                              uint64_t *time_point_out);

/* An alloc, memcpy(), push, and flush, all wrapped up into one */
VkResult nvk_mem_stream_push(struct nvk_device *dev,
                             struct nvk_mem_stream *stream,
                             struct nvkmd_ctx *ctx,
                             const uint32_t *push_data,
                             uint32_t push_dw_count,
                             uint64_t *time_point_out);

VkResult nvk_mem_stream_sync(struct nvk_device *dev,
                             struct nvk_mem_stream *stream,
                             struct nvkmd_ctx *ctx);

#endif /* NVK_STREAM_H */
