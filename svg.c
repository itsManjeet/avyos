#include "svg.h"
#include <math.h>
#include <stdlib.h>
#include <stdio.h>

#include <sys/mman.h>

#include <nanosvg/nanosvgrast.h>

#define LOG_MODULE "svg"
#define LOG_ENABLE_DBG 0
#include "log.h"
#include "stride.h"

struct NSVGimage *svg_image = NULL;
struct NSVGrasterizer *rast = NULL;

bool
svg_load(FILE *fp, const char *path)
{
    if (rast != NULL) {
        nsvgDeleteRasterizer(rast);
        rast = NULL;
    }

    if (svg_image != NULL) {
        nsvgDelete(svg_image);
        svg_image = NULL;
    }

    if (fseek(fp, 0, SEEK_END) < 0) {
        LOG_ERRNO("%s: failed to seek to end of file", path);
        return false;
    }

    long int image_size = ftell(fp);

    if (fseek(fp, 0, SEEK_SET) < 0) {
        LOG_ERRNO("%s: failed to seek to beginning of file", path);
        return false;
    }

    /*
     * nanosvg requires input to be NULL terminated, so can't mmap
     * file directly :(
     */
    char *image_data = mmap(NULL, image_size + 1, PROT_READ | PROT_WRITE, MAP_PRIVATE | MAP_ANONYMOUS, -1, 0);
    if (image_data == MAP_FAILED) {
        LOG_ERRNO("%s: failed to allocate image buffer", path);
        return false;
    }

    clearerr(fp);
    if (fread(image_data, 1, image_size, fp) != image_size) {
        LOG_ERRNO_P("%s: failed to read", ferror(fp), path);
        munmap(image_data, image_size + 1);
        return false;
    }

    image_data[image_size] = '\0';

    svg_image = nsvgParse(image_data, "px", 96);
    munmap(image_data, image_size + 1);

    if (svg_image == NULL)
        return false;

    if (svg_image->width == 0 || svg_image->height == 0) {
        LOG_DBG("%s: width and/or heigth is zero, not a SVG?", path);
        nsvgDelete(svg_image);
        svg_image = NULL;
        return false;
    }

    rast = nsvgCreateRasterizer();

    if (rast == NULL)
        return false;

    return true;
}

pixman_image_t *
svg_render(const int width, const int height, bool stretch)
{
    pixman_image_t *pix = NULL;
    uint8_t *data = MAP_FAILED;
    int stride = stride_for_format_and_width(PIXMAN_a8b8g8r8, width);

    data = mmap(NULL, height * stride, PROT_READ | PROT_WRITE, MAP_PRIVATE | MAP_ANONYMOUS, -1, 0);
    if (data == MAP_FAILED)
        return NULL;

    double sx = width / (double)svg_image->width;
    double sy = height / (double)svg_image->height;
    double s = stretch ? fmax(sx, sy) : fmin(sx, sy);

    float tx = (width - svg_image->width * s) / 2;
    float ty = (height - svg_image->height * s) / 2;

    nsvgRasterize(rast, svg_image, tx, ty, s, data, width, height, stride);

    pix = pixman_image_create_bits_no_clear(PIXMAN_a8b8g8r8, width,
        height, (uint32_t *)data, stride);
    if (pix == NULL) {
        LOG_ERR("failed to render svg image");
        munmap(data, height * stride);
        return NULL;
    }

    /* Copied from fuzzel */
    /* Nanosvg produces non-premultiplied ABGR, while pixman expects
     * premultiplied */
    for (uint32_t *abgr = (uint32_t *)data;
         abgr < (uint32_t *)(data + (size_t)width * (size_t)height * 4);
         abgr++) {
        uint8_t alpha = (*abgr >> 24) & 0xff;
        uint8_t blue = (*abgr >> 16) & 0xff;
        uint8_t green = (*abgr >> 8) & 0xff;
        uint8_t red = (*abgr >> 0) & 0xff;

        if (alpha == 0xff)
            continue;

        if (alpha == 0x00)
            blue = green = red = 0x00;
        else {
            blue = blue * alpha / 0xff;
            green = green * alpha / 0xff;
            red = red * alpha / 0xff;
        }

        *abgr = (uint32_t)alpha << 24 | blue << 16 | green << 8 | red;
    }

    return pix;
}

void
svg_free(void)
{
    if (rast != NULL) {
        nsvgDeleteRasterizer(rast);
        rast = NULL;
    }

    if (svg_image != NULL) {
        nsvgDelete(svg_image);
        svg_image = NULL;
    }
}
