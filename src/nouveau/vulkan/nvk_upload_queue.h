/*
 * Copyright © 2024 Collabora Ltd. and Red Hat Inc.
 * SPDX-License-Identifier: MIT
 */
#ifndef NVK_UPLOAD_QUEUE_H
#define NVK_UPLOAD_QUEUE_H 1

#include "nvk_mem_stream.h"

#include "nv_push.h"
#include "util/simple_mtx.h"

struct nvk_device;
struct nvkmd_ctx;

struct nvk_upload_queue {
   simple_mtx_t mutex;

   struct nvkmd_ctx *ctx;
   struct nvk_mem_stream stream;
   uint64_t last_time_point;

   uint32_t push_data[4096];
   struct nv_push push;
};

VkResult nvk_upload_queue_init(struct nvk_device *dev,
                               struct nvk_upload_queue *queue);

void nvk_upload_queue_finish(struct nvk_device *dev,
                             struct nvk_upload_queue *queue);

VkResult nvk_upload_queue_flush(struct nvk_device *dev,
                                struct nvk_upload_queue *queue,
                                uint64_t *time_point_out);

VkResult nvk_upload_queue_sync(struct nvk_device *dev,
                               struct nvk_upload_queue *queue);

VkResult nvk_upload_queue_upload(struct nvk_device *dev,
                                 struct nvk_upload_queue *queue,
                                 uint64_t dst_addr,
                                 const void *src, size_t size);

VkResult nvk_upload_queue_fill(struct nvk_device *dev,
                               struct nvk_upload_queue *queue,
                               uint64_t dst_addr, uint32_t data, size_t size);

#endif /* NVK_UPLOAD_QUEUE_H */
