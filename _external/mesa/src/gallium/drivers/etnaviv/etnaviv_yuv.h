/*
 * Copyright 2024 Igalia, S.L.
 * SPDX-License-Identifier: MIT
 */

#ifndef H_ETNA_YUV
#define H_ETNA_YUV

#include <stdbool.h>
#include <stdint.h>

#include "etnaviv_context.h"

struct etna_yuv_config {
   struct etna_resource *planes[3];
   struct etna_resource *dst;
   uint32_t width;
   uint32_t height;
   uint32_t format;
};

static inline bool
etna_format_needs_yuv_tiler(enum pipe_format format)
{
   return format == PIPE_FORMAT_NV12;
}

void
etna_yuv_emit_plane(struct etna_context *ctx, struct etna_resource *plane,
                    enum etna_resource_status status, uint32_t base, uint32_t stride);

bool
etna_try_yuv_blit(struct pipe_context *pctx,
                  const struct pipe_blit_info *blit_info);

#endif
