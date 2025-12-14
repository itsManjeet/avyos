/*
 * Copyright 2024 Igalia, S.L.
 * SPDX-License-Identifier: MIT
 */

#include "etnaviv_context.h"
#include "etnaviv_emit.h"
#include "etnaviv_yuv.h"

void
etna_yuv_emit_plane(struct etna_context *ctx, struct etna_resource *plane,
                    enum etna_resource_status status, uint32_t base, uint32_t stride)
{
   if (!plane)
      return;

   uint32_t flags = (status == ETNA_PENDING_WRITE) ? ETNA_RELOC_WRITE : ETNA_RELOC_READ;

   etna_resource_used(ctx, &plane->base, status);
   etna_set_state_reloc(ctx->stream, base, &(struct etna_reloc) {
      .bo = plane->bo,
      .offset = plane->levels[0].offset,
      .flags = flags,
   });
   etna_set_state(ctx->stream, stride, plane->levels[0].stride);
}

bool
etna_try_yuv_blit(struct pipe_context *pctx,
                  const struct pipe_blit_info *blit_info)
{
   struct etna_context *ctx = etna_context(pctx);
   struct etna_cmd_stream *stream = ctx->stream;
   struct pipe_resource *src = blit_info->src.resource;
   struct etna_yuv_config config = { 0 };
   ASSERTED unsigned num_planes;
   int idx = 0;

   assert(util_format_is_yuv(blit_info->src.format));
   assert(blit_info->dst.format == PIPE_FORMAT_YUYV);
   assert(blit_info->src.level == 0);
   assert(blit_info->dst.level == 0);

   config.dst = etna_resource(blit_info->dst.resource);
   config.height = blit_info->dst.box.height;
   config.width = blit_info->dst.box.width;

   switch (blit_info->src.format) {
   case PIPE_FORMAT_NV12:
      config.format = 0x1;
      num_planes = 2;
      break;
   default:
      return false;
   }

   while (src) {
      config.planes[idx++] = etna_resource(src);
      src = src->next;
   }

   assert(idx == num_planes);

   etna_set_state(stream, VIVS_GL_FLUSH_CACHE,
                  VIVS_GL_FLUSH_CACHE_COLOR | VIVS_GL_FLUSH_CACHE_DEPTH);
   etna_stall(stream, SYNC_RECIPIENT_RA, SYNC_RECIPIENT_PE);

   etna_set_state(stream, VIVS_TS_FLUSH_CACHE, VIVS_TS_FLUSH_CACHE_FLUSH);
   etna_set_state(stream, VIVS_TS_MEM_CONFIG, 0);

   ctx->emit_yuv_tiler_state(ctx, &config);

   ctx->dirty |= ETNA_DIRTY_TS;

   return true;
}
