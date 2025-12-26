#include "jpg.h"
#include <stdlib.h>
#include <stdio.h>
#include <assert.h>
#include <setjmp.h>

#include <sys/mman.h>

#include <jpeglib.h>

#define LOG_MODULE "jpg"
#define LOG_ENABLE_DBG 0
#include "log.h"
#include "stride.h"

struct my_error_mgr {
    struct jpeg_error_mgr mgr;
    jmp_buf setjmp_buffer;
};

static void
error_exit(j_common_ptr cinfo)
{
    struct my_error_mgr *err_handler = (struct my_error_mgr *)cinfo->err;
    longjmp(err_handler->setjmp_buffer, 1);
}

pixman_image_t *
jpg_load(FILE *fp, const char *path)
{
    struct jpeg_decompress_struct cinfo = {0};
    struct my_error_mgr err_handler;

    uint8_t *image_data = MAP_FAILED;
    pixman_image_t *pix = NULL;

    if (fseek(fp, 0, SEEK_SET) < 0) {
        LOG_ERRNO("%s: failed to seek to beginning of file", path);
        return NULL;
    }

    cinfo.err = jpeg_std_error(&err_handler.mgr);
    err_handler.mgr.error_exit = &error_exit;
    if (setjmp(err_handler.setjmp_buffer)) {
        char err_string[JMSG_LENGTH_MAX];
        cinfo.err->format_message((j_common_ptr)&cinfo, err_string);
        LOG_DBG("%s: %s", path, err_string);
        goto err;
    }

    jpeg_create_decompress(&cinfo);
    jpeg_stdio_src(&cinfo, fp);
    jpeg_read_header(&cinfo, true);
    jpeg_calc_output_dimensions(&cinfo);

    if (cinfo.output_components != 1 && cinfo.output_components != 3) {
        LOG_ERR("%s: unsupported number of color components: %d",
                path, cinfo.output_components);
        goto err;
    }

    pixman_format_code_t format = PIXMAN_b8g8r8;
    int width = cinfo.output_width;
    int height = cinfo.output_height;
    int stride = stride_for_format_and_width(format, width);

    LOG_DBG("width=%d, height=%d, stride=%d, components=%d",
            width, height, stride, cinfo.output_components);

    image_data = mmap(NULL, height * stride, PROT_READ | PROT_WRITE, MAP_PRIVATE | MAP_ANONYMOUS, -1, 0);
    if (image_data == MAP_FAILED)
        goto err;

    jpeg_start_decompress(&cinfo);

    for (int row_no = 0; cinfo.output_scanline < height; row_no++) {
        uint8_t *row = &image_data[row_no * stride];

        if (cinfo.output_components == 3) {
            /* Read directly info our to-be pixman image buffer */
            jpeg_read_scanlines(&cinfo, (JSAMPARRAY)&row, 1);
        }

        else {
            /* Read into temporary buffer, then expand to rgb */
            assert(cinfo.output_components == 1);

            uint8_t buf[cinfo.output_width];
            uint8_t *scanline = &buf[0];
            jpeg_read_scanlines(&cinfo, (JSAMPARRAY)&scanline, 1);

            for (size_t i = 0; i < cinfo.output_width; i++) {
                row[i * 3 + 0] = buf[i];
                row[i * 3 + 1] = buf[i];
                row[i * 3 + 2] = buf[i];
            }
        }
    }

    jpeg_finish_decompress(&cinfo);
    jpeg_destroy_decompress(&cinfo);

    pix = pixman_image_create_bits_no_clear(
        format, width, height, (uint32_t *)image_data, stride);

    if (pix == NULL) {
        LOG_ERR("%s: failed to instantiate pixman image", path);
        goto err;
    }

    return pix;

err:
    if (pix != NULL)
        pixman_image_unref(pix);
    if (image_data != MAP_FAILED)
        munmap(image_data, height * stride);
    jpeg_destroy_decompress(&cinfo);
    return NULL;
}
