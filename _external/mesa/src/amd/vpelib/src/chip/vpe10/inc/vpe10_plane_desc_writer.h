/* Copyright 2024 Advanced Micro Devices, Inc.
 *
 * Permission is hereby granted, free of charge, to any person obtaining a
 * copy of this software and associated documentation files (the "Software"),
 * to deal in the Software without restriction, including without limitation
 * the rights to use, copy, modify, merge, publish, distribute, sublicense,
 * and/or sell copies of the Software, and to permit persons to whom the
 * Software is furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL
 * THE COPYRIGHT HOLDER(S) OR AUTHOR(S) BE LIABLE FOR ANY CLAIM, DAMAGES OR
 * OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
 * ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
 * OTHER DEALINGS IN THE SOFTWARE.
 *
 * Authors: AMD
 *
 */

#pragma once

#include "vpe_types.h"
#include "plane_desc_writer.h"

#ifdef __cplusplus
extern "C" {
#endif

struct vpe10_plane_desc_header {
    int32_t nps0;
    int32_t npd0;
    int32_t nps1;
    int32_t npd1;
    int32_t subop;
};

struct vpe10_plane_desc_src {
    uint8_t                      tmz;
    enum vpe_swizzle_mode_values swizzle;
    enum vpe_rotation_angle      rotation;
    uint32_t                     base_addr_lo;
    uint32_t                     base_addr_hi;
    uint16_t                     pitch;
    uint16_t                     viewport_x;
    uint16_t                     viewport_y;
    uint16_t                     viewport_w;
    uint16_t                     viewport_h;
    uint8_t                      elem_size;
};

struct vpe10_plane_desc_dst {
    uint8_t                      tmz;
    enum vpe_swizzle_mode_values swizzle;
    enum vpe_mirror              mirror;
    uint32_t                     base_addr_lo;
    uint32_t                     base_addr_hi;
    uint16_t                     pitch;
    uint16_t                     viewport_x;
    uint16_t                     viewport_y;
    uint16_t                     viewport_w;
    uint16_t                     viewport_h;
    uint8_t                      elem_size;
};

/** initialize the plane descriptor writer.
 * Calls right before building any plane descriptor
 *
 * /param   writer               writer instance
 * /param   buf                  points to the current buf,
 * /param   plane_desc_header    header
 */

void vpe10_plane_desc_writer_init(
    struct plane_desc_writer *writer, struct vpe_buf *buf, void *p_header);

/** fill the value to the embedded buffer. */
void vpe10_plane_desc_writer_add_source(
    struct plane_desc_writer *writer, void *p_source, bool is_plane0);

/** fill the value to the embedded buffer. */
void vpe10_plane_desc_writer_add_destination(
    struct plane_desc_writer *writer, void *p_destination, bool is_plane0);

void vpe10_construct_plane_desc_writer(struct plane_desc_writer *writer);

#ifdef __cplusplus
}
#endif
