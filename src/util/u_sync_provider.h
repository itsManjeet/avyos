/*
 * Copyright © 2024 Google, Inc.
 * SPDX-License-Identifier: MIT
 */

#include <stdint.h>

#ifndef U_SYNC_PROVIDER_H
#define U_SYNC_PROVIDER_H

#ifdef __cplusplus
extern "C" {
#endif

struct util_sync_provider {
   int (*create)(struct util_sync_provider *p, uint32_t flags, uint32_t *handle);
   int (*destroy)(struct util_sync_provider *p, uint32_t handle);
   int (*handle_to_fd)(struct util_sync_provider *p, uint32_t handle, int *out_obj_fd);
   int (*fd_to_handle)(struct util_sync_provider *p, int obj_fd, uint32_t *handle);
   int (*import_sync_file)(struct util_sync_provider *p, uint32_t handle, int sync_file_fd);
   int (*export_sync_file)(struct util_sync_provider *p, uint32_t handle, int *out_sync_file_fd);
   int (*wait)(struct util_sync_provider *p, uint32_t *handles, unsigned num_handles,
               int64_t timeout_nsec, unsigned flags, uint32_t *first_signaled);
   int (*reset)(struct util_sync_provider *p, const uint32_t *handles, uint32_t handle_count);
   int (*signal)(struct util_sync_provider *p, const uint32_t *handles, uint32_t handle_count);
   int (*timeline_signal)(struct util_sync_provider *p, const uint32_t *handles,
                          uint64_t *points, uint32_t handle_count);
   int (*timeline_wait)(struct util_sync_provider *p, uint32_t *handles, uint64_t *points,
                        unsigned num_handles, int64_t timeout_nsec, unsigned flags,
                        uint32_t *first_signaled);
   int (*query)(struct util_sync_provider *p, uint32_t *handles, uint64_t *points,
                uint32_t handle_count, uint32_t flags);
   int (*transfer)(struct util_sync_provider *p, uint32_t dst_handle, uint64_t dst_point,
                   uint32_t src_handle, uint64_t src_point, uint32_t flags);

   void (*finalize)(struct util_sync_provider *p);
   struct util_sync_provider * (*clone)(struct util_sync_provider *p);
};

#if HAVE_LIBDRM
struct util_sync_provider * util_sync_provider_drm(int drm_fd);
#else
static inline struct util_sync_provider *
util_sync_provider_drm(int drm_fd)
{
   return NULL;
}
#endif

#ifdef __cplusplus
}
#endif

#endif /* U_SYNC_PROVIDER_H */
