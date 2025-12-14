/*
 * Copyright © 2025 Collabora, Ltd.
 *
 * Permission is hereby granted, free of charge, to any person obtaining a
 * copy of this software and associated documentation files (the "Software"),
 * to deal in the Software without restriction, including without limitation
 * the rights to use, copy, modify, merge, publish, distribute, sublicense,
 * and/or sell copies of the Software, and to permit persons to whom the
 * Software is furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice (including the next
 * paragraph) shall be included in all copies or substantial portions of the
 * Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL
 * THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
 * FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS
 * IN THE SOFTWARE.
 */
#include "x11_display.h"

#include "util/macros.h"

#include <stdio.h>
#include <X11/Xlibint.h>

bool
x11_xlib_display_is_thread_safe(Display *dpy)
{
   static bool warned = false;

   /* 'lock_fns' is the XLockDisplay function pointer of the X11 display 'dpy'.
    * It will be NULL if XInitThreads wasn't called.
    */
   if (likely(dpy->lock_fns != NULL))
      return true;

   if (unlikely(!warned)) {
      fprintf(stderr, "Xlib is not thread-safe.  This should never be the "
                      "case starting with XLib 1.8.  Either upgrade XLib "
                      "or call XInitThreads() from your app.\n");
      warned = true;
   }

   return false;
}
