/*
 * SGI FREE SOFTWARE LICENSE B (Version 2.0, Sept. 18, 2008)
 * Copyright (C) 1991-2000 Silicon Graphics, Inc. All Rights Reserved.
 *
 * SPDX-License-Identifier: SGI-B-2.0
 */

/*
 * (C) Copyright IBM Corporation 2005
 * All Rights Reserved.
 * 
 * Permission is hereby granted, free of charge, to any person obtaining a
 * copy of this software and associated documentation files (the "Software"),
 * to deal in the Software without restriction, including without limitation
 * the rights to use, copy, modify, merge, publish, distribute, sub license,
 * and/or sell copies of the Software, and to permit persons to whom the
 * Software is furnished to do so, subject to the following conditions:
 * 
 * The above copyright notice and this permission notice (including the next
 * paragraph) shall be included in all copies or substantial portions of the
 * Software.
 * 
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NON-INFRINGEMENT.  IN NO EVENT SHALL
 * IBM,
 * AND/OR THEIR SUPPLIERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
 * WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF
 * OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

#include "packrender.h"
#include "indirect.h"

/**
 * Send a large image to the server.  If necessary, a buffer is allocated
 * to hold the unpacked data that is copied from the clients memory.
 * 
 * \param gc        Current GLX context
 * \param compsize  Size, in bytes, of the image portion
 * \param dim       Number of dimensions of the image
 * \param width     Width of the image
 * \param height    Height of the image, must be 1 for 1D images
 * \param depth     Depth of the image, must be 1 for 1D or 2D images
 * \param format    Format of the image
 * \param type      Data type of the image
 * \param src       Pointer to the image data
 * \param pc        Pointer to end of the command header
 * \param modes     Pointer to the pixel unpack data
 *
 * \todo
 * Modify this function so that \c NULL images are sent using
 * \c __glXSendLargeChunk instead of __glXSendLargeCommand.  Doing this
 * will eliminate the need to allocate a buffer for that case.
 */
void
__glXSendLargeImage(struct glx_context * gc, GLint compsize, GLint dim,
                    GLint width, GLint height, GLint depth,
                    GLenum format, GLenum type, const GLvoid * src,
                    GLubyte * pc, GLubyte * modes)
{
    /* Allocate a temporary holding buffer */
    GLubyte *buf = malloc(compsize);
    if (!buf) {
   __glXSetError(gc, GL_OUT_OF_MEMORY);
   return;
    }

    /* Apply pixel store unpack modes to copy data into buf */
    if (src != NULL) {
   __glFillImage(gc, dim, width, height, depth, format, type,
                      src, buf, modes);
    }
    else {
   if (dim < 3) {
       (void) memcpy(modes, __glXDefaultPixelStore + 4, 20);
   }
   else {
       (void) memcpy(modes, __glXDefaultPixelStore + 0, 36);
   }
    }

    /* Send large command */
    __glXSendLargeCommand(gc, gc->pc, pc - gc->pc, buf, compsize);

    /* Free buffer */
    free((char *) buf);
}
