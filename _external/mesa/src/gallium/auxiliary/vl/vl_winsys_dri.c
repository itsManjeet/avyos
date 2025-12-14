/**************************************************************************
 *
 * Copyright 2009 Younes Manton.
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

/* directly referenced from target Makefile, because of X dependencies */

#include <sys/types.h>
#include <sys/stat.h>

#include <X11/Xlib-xcb.h>
#include <xf86drm.h>
#include <errno.h>

#include "loader.h"

#include "pipe/p_screen.h"
#include "pipe/p_context.h"
#include "pipe/p_state.h"
#include "pipe-loader/pipe_loader.h"
#include "frontend/drm_driver.h"

#include "util/u_memory.h"
#include "util/crc32.h"
#include "util/u_hash_table.h"
#include "util/u_inlines.h"

#include "vl/vl_compositor.h"
#include "vl/vl_winsys.h"

#include "drm-uapi/drm_fourcc.h"

static xcb_visualtype_t *
get_xcb_visualtype_for_depth(struct vl_screen *vscreen, int depth)
{
   xcb_visualtype_iterator_t visual_iter;
   xcb_screen_t *screen = vscreen->xcb_screen;
   xcb_depth_iterator_t depth_iter;

   if (!screen)
      return NULL;

   depth_iter = xcb_screen_allowed_depths_iterator(screen);
   for (; depth_iter.rem; xcb_depth_next(&depth_iter)) {
      if (depth_iter.data->depth != depth)
         continue;

      visual_iter = xcb_depth_visuals_iterator(depth_iter.data);
      if (visual_iter.rem)
         return visual_iter.data;
   }

   return NULL;
}

static uint32_t
get_red_mask_for_depth(struct vl_screen *vscreen, int depth)
{
   xcb_visualtype_t *visual = get_xcb_visualtype_for_depth(vscreen, depth);

   if (visual) {
      return visual->red_mask;
   }

   return 0;
}

uint32_t
vl_dri2_format_for_depth(struct vl_screen *vscreen, int depth)
{
   switch (depth) {
   case 24:
      return PIPE_FORMAT_B8G8R8X8_UNORM;
   case 30:
      /* Different preferred formats for different hw */
      if (get_red_mask_for_depth(vscreen, 30) == 0x3ff)
         return PIPE_FORMAT_R10G10B10X2_UNORM;
      else
         return PIPE_FORMAT_B10G10R10X2_UNORM;
   default:
      return PIPE_FORMAT_NONE;
   }
}

xcb_screen_t *
vl_dri_get_screen_for_root(xcb_connection_t *conn, xcb_window_t root)
{
   xcb_screen_iterator_t screen_iter =
   xcb_setup_roots_iterator(xcb_get_setup(conn));

   for (; screen_iter.rem; xcb_screen_next (&screen_iter)) {
      if (screen_iter.data->root == root)
         return screen_iter.data;
   }

   return NULL;
}
