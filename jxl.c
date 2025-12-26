#include "jxl.h"
#include <stdlib.h>
#include <stdio.h>

#include <sys/mman.h>

#include <jxl/decode.h>
#include <jxl/codestream_header.h>
#if defined(WBG_HAVE_JXL_THREADS)
 #include <jxl/resizable_parallel_runner.h>
#endif

#define LOG_MODULE "jxl"
#define LOG_ENABLE_DBG 0
#include "log.h"
#include "stride.h"

pixman_image_t *
jxl_load(FILE *fp, const char *path)
{
    pixman_image_t *pix = NULL;
    pixman_format_code_t format = PIXMAN_x8b8g8r8;
    bool ok = false;
    uint8_t *file_data = MAP_FAILED;
    uint8_t *image = MAP_FAILED;
    size_t file_size, image_size = 0;
    int width = 0, height = 0, stride = 0;

#if defined(WBG_HAVE_JXL_THREADS)
    JxlParallelRunner *runner = NULL;
#endif
    JxlDecoder *decoder = NULL;
    JxlBasicInfo info = {0};
    JxlPixelFormat jxl_format = {
        .num_channels = 4,
        .data_type = JXL_TYPE_UINT8,
        .endianness = JXL_LITTLE_ENDIAN,
        .align = 0
    };

    if (fseek(fp, 0, SEEK_END) < 0) {
        LOG_ERRNO("%s: failed to seek to end of file", path);
        return NULL;
    }

    file_size = ftell(fp);
    if (fseek(fp, 0, SEEK_SET) < 0) {
        LOG_ERRNO("%s: failed to seek to beginning of file", path);
        return NULL;
    }

    file_data = mmap(NULL, file_size, PROT_READ, MAP_PRIVATE, fileno(fp), 0);
    if (file_data == MAP_FAILED)
        goto err;

    if (JxlSignatureCheck(file_data, file_size) == JXL_SIG_INVALID) {
        LOG_DBG("%s: not a jpegxl image", path);
        goto err;
    }

    if (!(decoder = JxlDecoderCreate(NULL)))
        goto err;
#if defined(WBG_HAVE_JXL_THREADS)
    if (!(runner = JxlResizableParallelRunnerCreate(NULL)))
        goto err;
    JxlDecoderSetParallelRunner(decoder, JxlResizableParallelRunner, runner);
#endif

    JxlDecoderSubscribeEvents(decoder, JXL_DEC_BASIC_INFO | JXL_DEC_FULL_IMAGE);

    JxlDecoderSetInput(decoder, file_data, file_size);
    JxlDecoderCloseInput(decoder);

    while (1) {
        JxlDecoderStatus status = JxlDecoderProcessInput(decoder);

        if (status == JXL_DEC_ERROR) {
            LOG_ERR("%s: decoder error", path);
            goto err;
        } else if (status == JXL_DEC_NEED_MORE_INPUT) {
            LOG_ERR("%s: decoder requires more input but already provided all input", path);
            goto err;
        } else if (status == JXL_DEC_BASIC_INFO) {
            if (JxlDecoderGetBasicInfo(decoder, &info)
                    != JXL_DEC_SUCCESS) {
                LOG_ERR("%s: failed to get basic info", path);
            }

            width = info.xsize;
            height = info.ysize;
            stride = stride_for_format_and_width(format, width);
            image_size = height * stride;

            LOG_DBG("%s: %dx%d@%hhubpp, %d channels, %d alpha bits", path, width, height,
                    info.bits_per_sample, info.num_color_channels, info.alpha_bits);

            image = mmap(NULL, image_size, PROT_READ | PROT_WRITE, MAP_PRIVATE | MAP_ANONYMOUS, -1, 0);
            if (image == MAP_FAILED)
                goto err;

#if defined(WBG_HAVE_JXL_THREADS)
            JxlResizableParallelRunnerSetThreads(runner,
                    JxlResizableParallelRunnerSuggestThreads(width, height));
#endif
        } else if (status == JXL_DEC_NEED_IMAGE_OUT_BUFFER) {
            size_t min_size = 0;
            if (JxlDecoderImageOutBufferSize(decoder, &jxl_format, &min_size)
                    != JXL_DEC_SUCCESS) {
                LOG_ERR("%s: failed to get the minimum size of the output buffer", path);
                goto err;
            }

            if (min_size > image_size) {
                LOG_ERR("minimum size [%zu] is greater than the expected size [%zu]",
                        min_size, image_size);
                goto err;
            } else if (min_size < image_size) {
                LOG_WARN("minimum size [%zu] is less than the expected size [%zu]",
                        min_size, image_size);
            }

            if (JxlDecoderSetImageOutBuffer(decoder, &jxl_format, image,
                    image_size) != JXL_DEC_SUCCESS) {
                LOG_ERR("%s: failed to set output buffer", path);
                goto err;
            }
        } else {
            break;
        }
    }

    for (uint32_t *abgr = (uint32_t *)image;
         abgr < (uint32_t *)(image + (size_t)width * (size_t)height * 4);
         abgr++) {
        uint8_t alpha = (*abgr >> 24) & 0xff;
        uint8_t red   = (*abgr >> 16) & 0xff;
        uint8_t green = (*abgr >> 8) & 0xff;
        uint8_t blue  = (*abgr >> 0) & 0xff;

        if (alpha == 0xff)
            continue;

        if (alpha == 0x00)
            blue = green = red = 0x00;
        else {
            blue = blue * alpha / 0xff;
            green = green * alpha / 0xff;
            red = red * alpha / 0xff;
        }

        *abgr = (uint32_t)alpha << 24 | red << 16 | green << 8 | blue;
    }

    pix = pixman_image_create_bits_no_clear(format, width, height,
            (uint32_t *)image, stride);
    ok = pix != NULL;

err:
    if (!ok) {
        if (image != MAP_FAILED)
            munmap(image, image_size);
    }

    if (file_data != MAP_FAILED)
        munmap(file_data, file_size);

#if defined(WBG_HAVE_JXL_THREADS)
    JxlResizableParallelRunnerDestroy(runner);
#endif
    JxlDecoderDestroy(decoder);

    return pix;
}
