/*
 * Copyright © 2024 Google, Inc.
 * SPDX-License-Identifier: MIT
 */

#include "util/u_memory.h"
#include "util/u_sync_provider.h"
#include <xf86drm.h>

struct util_sync_provider_drm {
   struct util_sync_provider base;
   int drm_fd;
};

static int
drm_fd(struct util_sync_provider *p)
{
   struct util_sync_provider_drm *d = (struct util_sync_provider_drm *)p;
   return d->drm_fd;
}

static int
drm_syncobj_create(struct util_sync_provider *p, uint32_t flags, uint32_t *handle)
{
   return drmSyncobjCreate(drm_fd(p), flags, handle);
}

static int
drm_syncobj_destroy(struct util_sync_provider *p, uint32_t handle)
{
   return drmSyncobjDestroy(drm_fd(p), handle);
}

static int
drm_syncobj_handle_to_fd(struct util_sync_provider *p, uint32_t handle, int *out_obj_fd)
{
   return drmSyncobjHandleToFD(drm_fd(p), handle, out_obj_fd);
}

static int
drm_syncobj_fd_to_handle(struct util_sync_provider *p, int obj_fd, uint32_t *handle)
{
   return drmSyncobjFDToHandle(drm_fd(p), obj_fd, handle);
}

static int
drm_syncobj_import_sync_file(struct util_sync_provider *p, uint32_t handle, int sync_file_fd)
{
   return drmSyncobjImportSyncFile(drm_fd(p), handle, sync_file_fd);
}

static int
drm_syncobj_export_sync_file(struct util_sync_provider *p, uint32_t handle, int *out_sync_file_fd)
{
   return drmSyncobjExportSyncFile(drm_fd(p), handle, out_sync_file_fd);
}

static int
drm_syncobj_wait(struct util_sync_provider *p, uint32_t *handles, unsigned num_handles,
                 int64_t timeout_nsec, unsigned flags, uint32_t *first_signaled)
{
   return drmSyncobjWait(drm_fd(p), handles, num_handles, timeout_nsec, flags, first_signaled);
}

static int
drm_syncobj_reset(struct util_sync_provider *p, const uint32_t *handles, uint32_t handle_count)
{
   return drmSyncobjReset(drm_fd(p), handles, handle_count);
}

static int
drm_syncobj_signal(struct util_sync_provider *p, const uint32_t *handles, uint32_t handle_count)
{
   return drmSyncobjSignal(drm_fd(p), handles, handle_count);
}

static int
drm_syncobj_timeline_signal(struct util_sync_provider *p, const uint32_t *handles,
                            uint64_t *points, uint32_t handle_count)
{
   return drmSyncobjTimelineSignal(drm_fd(p), handles, points, handle_count);
}

static int
drm_syncobj_timeline_wait(struct util_sync_provider *p, uint32_t *handles, uint64_t *points,
                          unsigned num_handles, int64_t timeout_nsec, unsigned flags,
                          uint32_t *first_signaled)
{
   return drmSyncobjTimelineWait(drm_fd(p), handles, points, num_handles, timeout_nsec, flags, first_signaled);
}

static int
drm_syncobj_query(struct util_sync_provider *p, uint32_t *handles, uint64_t *points,
                  uint32_t handle_count, uint32_t flags)
{
   return drmSyncobjQuery2(drm_fd(p), handles, points, handle_count, flags);
}

static int
drm_syncobj_transfer(struct util_sync_provider *p, uint32_t dst_handle, uint64_t dst_point,
                     uint32_t src_handle, uint64_t src_point, uint32_t flags)
{
   return drmSyncobjTransfer(drm_fd(p), dst_handle, dst_point, src_handle, src_point, flags);
}

static void
drm_syncobj_finalize(struct util_sync_provider *p)
{
   free(p);
}

static struct util_sync_provider *
drm_sync_provider_clone(struct util_sync_provider *p)
{
   return util_sync_provider_drm(drm_fd(p));
}


struct util_sync_provider *
util_sync_provider_drm(int drm_fd)
{
   struct util_sync_provider_drm *d = CALLOC_STRUCT(util_sync_provider_drm);

   d->drm_fd = drm_fd;
   d->base = (struct util_sync_provider) {
      .create = drm_syncobj_create,
      .destroy = drm_syncobj_destroy,
      .handle_to_fd = drm_syncobj_handle_to_fd,
      .fd_to_handle = drm_syncobj_fd_to_handle,
      .import_sync_file = drm_syncobj_import_sync_file,
      .export_sync_file = drm_syncobj_export_sync_file,
      .wait = drm_syncobj_wait,
      .reset = drm_syncobj_reset,
      .signal = drm_syncobj_signal,
      .query = drm_syncobj_query,
      .transfer = drm_syncobj_transfer,
      .finalize = drm_syncobj_finalize,
      .clone = drm_sync_provider_clone,
   };

   uint64_t cap;
   int err = drmGetCap(drm_fd, DRM_CAP_SYNCOBJ_TIMELINE, &cap);
   if (err == 0 && cap != 0) {
      d->base.timeline_signal = drm_syncobj_timeline_signal;
      d->base.timeline_wait = drm_syncobj_timeline_wait;
   }

   return &d->base;
}
