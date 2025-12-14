/**************************************************************************
 *
 * Copyright 2008 VMware, Inc.
 * All Rights Reserved.
 *
 * Permission is hereby granted, free of charge, to any person obtaining a
 * copy of this software and associated documentation files (the
 * "Software"), to deal in the Software without restriction, including
 * without limitation the rights to use, copy, modify, merge, publish,
 * distribute, sub license, and/or sell copies of the Software, and to
 * permit persons to whom the Software is furnished to do so, subject to
 * the following conditions:
 *
 * The above copyright notice and this permission notice (including the
 * next paragraph) shall be included in all copies or substantial portions
 * of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS
 * OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
 * MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NON-INFRINGEMENT.
 * IN NO EVENT SHALL VMWARE AND/OR ITS SUPPLIERS BE LIABLE FOR
 * ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,
 * TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
 * SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 *
 **************************************************************************/

#include "util/u_inlines.h"
#include "util/u_hash_table.h"
#include "util/u_memory.h"

#include "tr_screen.h"
#include "tr_context.h"
#include "tr_texture.h"

struct pipe_transfer *
trace_transfer_create(struct trace_context *tr_ctx,
		      struct pipe_resource *res,
		      struct pipe_transfer *transfer)
{
   struct trace_transfer *tr_trans;

   if (!transfer)
      goto error;

   tr_trans = CALLOC_STRUCT(trace_transfer);
   if (!tr_trans)
      goto error;

   memcpy(&tr_trans->base, transfer, tr_ctx->threaded ? sizeof(struct threaded_transfer) : sizeof(struct pipe_transfer));

   tr_trans->base.b.resource = NULL;
   tr_trans->transfer = transfer;

   pipe_resource_reference(&tr_trans->base.b.resource, res);
   assert(tr_trans->base.b.resource == res);

   return &tr_trans->base.b;

error:
   if (res->target == PIPE_BUFFER)
      tr_ctx->pipe->buffer_unmap(tr_ctx->pipe, transfer);
   else
      tr_ctx->pipe->texture_unmap(tr_ctx->pipe, transfer);
   return NULL;
}


void
trace_transfer_destroy(struct trace_context *tr_context,
                       struct trace_transfer *tr_trans)
{
   pipe_resource_reference(&tr_trans->base.b.resource, NULL);
   FREE(tr_trans);
}

struct pipe_sampler_view *
trace_sampler_view_create(struct trace_context *tr_ctx,
                  struct pipe_resource *tr_res,
                  struct pipe_sampler_view *view)
{
   assert(tr_res == view->texture);
   struct trace_sampler_view *tr_view = CALLOC_STRUCT(trace_sampler_view);
   memcpy(&tr_view->base, view, sizeof(struct pipe_sampler_view));
   tr_view->base.reference.count = 1;
   tr_view->base.texture = NULL;
   pipe_resource_reference(&tr_view->base.texture, tr_res);
   tr_view->base.context = &tr_ctx->base;
   tr_view->sampler_view = view;
   return &tr_view->base;
}

void
trace_sampler_view_destroy(struct trace_sampler_view *tr_view)
{
   pipe_resource_reference(&tr_view->base.texture, NULL);
   FREE(tr_view);
}

struct pipe_sampler_view *
trace_sampler_view_unwrap(struct trace_sampler_view *tr_view)
{
   if (!tr_view)
      return NULL;
   return tr_view->sampler_view;
}

#undef SAMPLER_VIEW_PRIVATE_REFCOUNT
