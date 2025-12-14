/*
 * Test program, which tests negative strides in compositing with fence_malloc
 * to check for out-of-bounds memory access.
 */
#include "utils.h"

#if FENCE_MALLOC_ACTIVE

#include <stdio.h>
#include <stdlib.h>
#include <sys/mman.h>

typedef struct
{
    int                     width;
    int                     height;
    int                     stride;
    pixman_format_code_t    format;
} image_info_t;

typedef struct
{
    pixman_op_t     op;

    image_info_t    src;
    image_info_t    dest;

    int             src_x;
    int             src_y;
    int             dest_x;
    int             dest_y;
    int             width;
    int             height;
} composite_info_t;

const composite_info_t info =
{
    PIXMAN_OP_SRC,
    { 16, 1000, -16, PIXMAN_a8r8g8b8 },
    { 16, 1000, -16, PIXMAN_a8r8g8b8 },
    0, 0,
    0, 0,
    16, 1000
};

static pixman_image_t *
make_image (const image_info_t *info, char **buf)
{
    int size = info->height * abs (info->stride);
    char *data = fence_malloc (size);
    int i;

    for (i = 0; i < size; ++i)
        data[i] = (i % 255) ^ (((i % 16) << 4) | (i & 0xf0));

    *buf = data;
    if (info->stride < 0)
        /* Move to the start of the last scanline. */
        data += size + info->stride;

    return pixman_image_create_bits (info->format,
                                     info->width,
                                     info->height,
                                     (uint32_t *)data,
                                     info->stride);
}

static void
test_composite (const composite_info_t *info)
{
    char *src_buf;
    char *dest_buf;
    pixman_image_t *src = make_image (&info->src, &src_buf);
    pixman_image_t *dest = make_image (&info->dest, &dest_buf);

    pixman_image_composite (PIXMAN_OP_SRC, src, NULL, dest,
                            info->src_x, info->src_y,
                            0, 0,
                            info->dest_x, info->dest_y,
                            info->width, info->height);

    fence_free (src_buf);
    fence_free (dest_buf);
    pixman_image_unref (src);
    pixman_image_unref (dest);
}

int
main (int argc, char **argv)
{
    test_composite (&info);

    return 0;
}

#else /* FENCE_MALLOC_ACTIVE */

int
main (int argc, char **argv)
{
    /* Automake return code for test SKIP. */
    return 77;
}

#endif /* FENCE_MALLOC_ACTIVE */
