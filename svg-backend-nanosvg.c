#include <math.h>
#include <float.h>
#include <assert.h>

#include <ft2build.h>
#include FT_OTSVG_H

#define LOG_MODULE "fcft/svg"
#define LOG_ENABLE_DBG 0
#include "log.h"
#include <nanosvg/nanosvg.h>
#include <nanosvg/nanosvgrast.h>

#define min(x, y) ((x) < (y) ? (x) : (y))
#define max(x, y) ((x) > (y) ? (x) : (y))

struct state {
    uint32_t cookie;  /* For debugging, to ensure the ‘generic’ field
                       * is ours */
    NSVGimage *svg;
    float scale;
    unsigned short glyph_id_start;
    unsigned short glyph_id_end;
    float x_ofs;
    float y_ofs;
    FT_Error error;
};

#define COOKIE 0xfcf77fcf

static void
slot_state_finalizer(void *object)
{
    FT_GlyphSlot slot = object;
    struct state *state = slot->generic.data;

    assert(state == NULL || state->cookie == COOKIE);

    free(state);
    slot->generic.data = NULL;
    slot->generic.finalizer = NULL;
}

static FT_Error
fcft_svg_init(FT_Pointer *state)
{
    *state = NULL;
    return FT_Err_Ok;
}

static void
fcft_svg_free(FT_Pointer *state)
{
}

static FT_Error
fcft_svg_render(FT_GlyphSlot slot, FT_Pointer *_state)
{
    assert(*_state == NULL);

    struct state *state = (struct state *)slot->generic.data;
    assert(state->cookie == COOKIE);

    if (state->error != FT_Err_Ok)
        return state->error;

    FT_Bitmap *bitmap = &slot->bitmap;

    /* TODO: implement this (note: we’re erroring out in preset_slot()
     * if this is not true) */
    assert(state->glyph_id_start == state->glyph_id_end);

    /* TODO: fix logging - svg->{width,height} is not the width/height we use */
    LOG_DBG("rendering to a %dx%d bitmap (svg size: %.2fx%.2f -> %.2fx%.2f)",
            bitmap->width, bitmap->rows,
            state->svg->width, state->svg->height,
            state->svg->width * state->scale,
            state->svg->height * state->scale);

    NSVGrasterizer *rast = nsvgCreateRasterizer();
    nsvgRasterize(
        rast, state->svg,
        state->x_ofs * state->scale, state->y_ofs * state->scale,
        state->scale, bitmap->buffer, bitmap->width, bitmap->rows,
        bitmap->pitch);
    nsvgDeleteRasterizer(rast);
    nsvgDelete(state->svg);

    bitmap->pixel_mode = FT_PIXEL_MODE_BGRA;
    bitmap->num_grays  = 256;
    slot->format = FT_GLYPH_FORMAT_BITMAP;

    /* Nanosvg produces non-premultiplied RGBA, while FreeType expects
     * premultiplied BGRA */
    for (size_t r = 0; r < bitmap->rows; r++) {
        for (size_t c  = 0; c < bitmap->pitch; c += 4) {
            uint8_t *pixel = &bitmap->buffer[r * bitmap->pitch + c];
            uint8_t red = pixel[0];
            uint8_t green = pixel[1];
            uint8_t blue = pixel[2];
            uint8_t alpha = pixel[3];

            if (alpha == 0x00)
                blue = green = red = 0x00;
            else {
                blue = blue * alpha / 0xff;
                green = green * alpha / 0xff;
                red = red * alpha / 0xff;
            }

            pixel[0] = blue;
            pixel[1] = green;
            pixel[2] = red;
            pixel[3] = alpha;
        }
    }

    /* Render slot boundaries */
#if 0
    for (size_t c = 0; c < bitmap->pitch; c += 4) {
            uint8_t *pixel = &bitmap->buffer[0 * bitmap->pitch + c];

            uint8_t red = 0xff;
            uint8_t green = 0;
            uint8_t blue = 0;
            uint8_t alpha = 0xff;
            if (alpha == 0x00)
                blue = green = red = 0x00;
            else {
                blue = blue * alpha / 0xff;
                green = green * alpha / 0xff;
                red = red * alpha / 0xff;
            }

            pixel[0] = blue;
            pixel[1] = green;
            pixel[2] = red;
            pixel[3] = alpha;
    }

    for (size_t c = 0; c < bitmap->pitch; c += 4) {
        uint8_t *pixel = &bitmap->buffer[(bitmap->rows - 1) * bitmap->pitch + c];

            uint8_t red = 0xff;
            uint8_t green = 0;
            uint8_t blue = 0;
            uint8_t alpha = 0xff;
            if (alpha == 0x00)
                blue = green = red = 0x00;
            else {
                blue = blue * alpha / 0xff;
                green = green * alpha / 0xff;
                red = red * alpha / 0xff;
            }

            pixel[0] = blue;
            pixel[1] = green;
            pixel[2] = red;
            pixel[3] = alpha;
    }

    for (size_t r = 0; r < bitmap->rows; r++) {
        uint8_t *pixel = &bitmap->buffer[r * bitmap->pitch + 0];

            uint8_t red = 0xff;
            uint8_t green = 0;
            uint8_t blue = 0;
            uint8_t alpha = 0xff;
            if (alpha == 0x00)
                blue = green = red = 0x00;
            else {
                blue = blue * alpha / 0xff;
                green = green * alpha / 0xff;
                red = red * alpha / 0xff;
            }

            pixel[0] = blue;
            pixel[1] = green;
            pixel[2] = red;
            pixel[3] = alpha;
    }

    for (size_t r = 0; r < bitmap->rows; r++) {
        uint8_t *pixel = &bitmap->buffer[r * bitmap->pitch + (bitmap->width - 1) * 4];

            uint8_t red = 0xff;
            uint8_t green = 0;
            uint8_t blue = 0;
            uint8_t alpha = 0xff;
            if (alpha == 0x00)
                blue = green = red = 0x00;
            else {
                blue = blue * alpha / 0xff;
                green = green * alpha / 0xff;
                red = red * alpha / 0xff;
            }

            pixel[0] = blue;
            pixel[1] = green;
            pixel[2] = red;
            pixel[3] = alpha;
    }
#endif
    return FT_Err_Ok;
}

static FT_Error
fcft_svg_preset_slot(FT_GlyphSlot slot, FT_Bool cache, FT_Pointer *_state)
{
    assert(*_state == NULL);
    struct state *state = NULL;
    struct state state_dummy = {0};

    FT_SVG_Document  document = (FT_SVG_Document)slot->other;
    FT_Size_Metrics  metrics  = document->metrics;

    if (cache) {
        if (slot->generic.data == NULL) {
            slot->generic.data = calloc(1, sizeof(*state));
            slot->generic.finalizer = &slot_state_finalizer;
            ((struct state *)slot->generic.data)->cookie = COOKIE;
        }
        state = slot->generic.data;
        state->error = FT_Err_Ok;
        assert(state->cookie == COOKIE);
    } else
        state = &state_dummy;

    /* The nanosvg rasterizer does not support rasterizing specific
     * element IDs */
    if (document->start_glyph_id != document->end_glyph_id) {
        static bool have_warned = false;
        if (!have_warned) {
            LOG_ERR("multi-glyph rendering is unsupported");
            have_warned = true;
        }

        state->error = FT_Err_Unimplemented_Feature;
        return FT_Err_Unimplemented_Feature;
    }

    state->glyph_id_start = document->start_glyph_id;
    state->glyph_id_end = document->end_glyph_id;

    char *svg_copy = malloc(document->svg_document_length + 1);
    memcpy(svg_copy, document->svg_document, document->svg_document_length);
    svg_copy[document->svg_document_length] = '\0';
    //LOG_DBG("SVG document:\n%s", svg_copy);

    state->svg = nsvgParse(svg_copy, "px", 0.);
    free(svg_copy);

    if (state->svg == NULL) {
        LOG_ERR("failed to parse SVG document");
        state->error = FT_Err_Invalid_SVG_Document;
        return FT_Err_Invalid_SVG_Document;
    }

    /*
     * Not sure if bug in nanosvg, but for images with negative
     * bounds, the image size (svg->width, svg->height) is
     * wrong. Workaround by figuring out the bounds ourselves, and
     * calculating the size from that.
     */
    float min_x = FLT_MAX;
    float min_y = FLT_MAX;
    float max_x = FLT_MIN;
    float max_y = FLT_MIN;

    LOG_DBG("shapes' bounds:");
    for (const struct NSVGshape *shape = state->svg->shapes;
         shape != NULL;
         shape = shape->next)
    {
#if 0
        LOG_DBG("  %s: %.2f %.2f %.2f %.2f", shape->id,
                shape->bounds[0], shape->bounds[1], shape->bounds[2],
                shape->bounds[3]);
#endif
#if 0   /* Verify the shape’s paths’ bounds don’t exceed the shape’s bounds */
        for (const struct NSVGpath *path = shape->paths;
             path != NULL;
             path = path->next)
        {
            assert(path->bounds[0] >= shape->bounds[0]);
            assert(path->bounds[1] >= shape->bounds[1]);
            assert(path->bounds[2] <= shape->bounds[2]);
            assert(path->bounds[3] <= shape->bounds[3]);

            LOG_DBG("    path: %0.2f %0.2f %0.2f %0.2f",
                    path->bounds[0], path->bounds[1],
                    path->bounds[2], path->bounds[3]);
        }
#endif

        min_x = min(min_x, shape->bounds[0]);
        min_y = min(min_y, shape->bounds[1]);
        max_x = max(max_x, shape->bounds[2]);
        max_y = max(max_y, shape->bounds[3]);
    }

    LOG_DBG("image bounds: min: x=%.2f, y=%.2f, max: x=%.2f, y=%.2f ",
            min_x, min_y, max_x, max_y);
    LOG_DBG("image size: %.2fx%.2f (calculated), %.2fx%.2f (NSVGimage)",
            max_x - min_x, max_y - min_y, state->svg->width, state->svg->height);

    /* For the rasterizer */
    state->x_ofs = -min_x;
    state->y_ofs = -min_y;

    float svg_width = max_x - min_x;
    float svg_height = max_y - min_y;

    if (svg_width == 0 || svg_height == 0) {
        svg_width = document->units_per_EM;
        svg_height = document->units_per_EM;
    }

    float x_scale = (float)metrics.x_ppem / floorf(svg_width);
    float y_scale = (float)metrics.y_ppem / floorf(svg_height);
    state->scale = x_scale < y_scale ? x_scale : y_scale;

    float width = floorf(svg_width) * state->scale;
    float height = floorf(svg_height) * state->scale;

    LOG_DBG(
        "dimensions: x-ppem=%hu, y-ppem=%hu, "
        "target width=%.2f, target height=%.2f, scale=%f",
        metrics.x_ppem, metrics.y_ppem, width, height, state->scale);

    /*
     * We need to take into account any transformations applied.  The end
     * user who applied the transformation doesn't know the internal details
     * of the SVG document.  Thus, we expect that the end user should just
     * write the transformation as if the glyph is a traditional one.  We
     * then do some maths on this to get the equivalent transformation in
     * SVG coordinates.
     */
    float xx =  (float)document->transform.xx / ( 1 << 16 );
    float xy = -(float)document->transform.xy / ( 1 << 16 );
    float yx = -(float)document->transform.yx / ( 1 << 16 );
    float yy =  (float)document->transform.yy / ( 1 << 16 );

    float x0 =
        (float)document->delta.x / 64 * svg_width / metrics.x_ppem;
    float y0 =
        -(float)document->delta.y / 64 * svg_height / metrics.y_ppem;

    LOG_DBG("transform: xx=%.2f, yy=%.2f, xy=%.2f, yx=%.2f, x0=%.2f, y0=%.2f",
            xx, yy, xy, yx, x0, y0);

    /*
     * User transformations
     *
     * Normally, we don’t set any in fcft. There’s one exception -
     * when FontConfig has added an FC_MATRIX pattern. This is
     * typically done when simulating italic fonts.
     *
     * Preferably, we’d like to error out here, and simply skip the
     * glyph. However, it seems FreeType ignores errors thrown from
     * this hook. This leads to a crash in the render hook, since
     * we’ve free:d the NSVG image.
     *
     * Therefore, we log a warning, and then *ignore* the
     * transform. For the normal use case, where the transform is
     * intended to simulate italics, it’s probably *better* to ignore
     * it, since most SVG glyphs are emojis, which doesn’t really look
     * good when slanted.
     */
    if (xx != 1. || yy != 1. || xy != 0. || yx != 0. || x0 != 0. || y0 != 0.) {
        static bool have_warned = false;
        if (!have_warned) {
            LOG_WARN("user transformations not supported");
            have_warned = true;
        }

#if 0   /* Spams too much */
        LOG_WARN(
            "user transformations not supported (%s, glyph index %04x): "
            "xx=%.2f, yy=%.2f, xy=%.2f, yx=%.2f, x0=%.2f, y0=%.2f",
            slot->face->family_name, slot->glyph_index,
            xx, yy, xy, yx, x0, y0);
#endif

#if 0
        nsvgDelete(state->svg);
        state->error = FT_Err_Unimplemented_Feature;
        return FT_Err_Unimplemented_Feature;
#endif
    }

    float ascender __attribute__((unused)) = metrics.ascender / 64.;
    float descender = metrics.descender / 64.;
    float line_height = metrics.height / 64.;
    float baseline = line_height + descender;

    LOG_DBG("ascender = %.2f, descender = %.2f, min_x=%.2f, min_y=%.2f, "
            "baseline=%.2f, x_ppem=%u, y_ppem=%u, line-height=%.2f",
            ascender, descender, min_x, min_y, baseline,
            metrics.x_ppem, metrics.y_ppem, line_height);

    slot->bitmap.rows = ceilf(height);
    slot->bitmap.width = ceilf(width);

    /* Centered */
    slot->bitmap_left = metrics.x_ppem >= width
        ? ((float)metrics.x_ppem - width) / 2.
        : 0;

    /*
     * Try to center the glyph on the *line* (i.e. *not* around the
     * baseline).
     *
     * Remember that 'top' refers to the distance between the
     * baseline, and the glyphs first scanline. So, to center the
     * glyph on the line, first calculate the (line) center point, and
     * then subtract that from the baseline.
     */
    float line_center = (line_height - height) / 2.;
    slot->bitmap_top = baseline >= line_center && line_center >= 0
        ? baseline - (line_height - height) / 2.
        : 0;

    /* Original version, based on FreeType2's example code, using
     * librsvg as 'backend' */
    //slot->bitmap_left = min_x * state->scale +
    //(metrics.x_ppem - (int)slot->bitmap.width) / 2;
    //slot->bitmap_top = min_y != 0. ? -min_y * state->scale : ascender;

    slot->bitmap.pitch = slot->bitmap.width * 4;
    slot->bitmap.pixel_mode = FT_PIXEL_MODE_BGRA;

    LOG_DBG("bitmap: x=%d, y=%d, width=%d, height=%d, scale=%f",
            slot->bitmap_left, slot->bitmap_top,
            slot->bitmap.width, slot->bitmap.rows, state->scale);

    /* Everything below is from rsvg reference hooks */

    /* Compute all the bearings and set them correctly. The outline is */
    /* scaled already, we just need to use the bounding box. */
    float horiBearingX = 0.;
    float horiBearingY = -slot->bitmap_top;

    /* XXX parentheses correct? */
    float vertBearingX =
        slot->metrics.horiBearingX / 64.0f - slot->metrics.horiAdvance / 64.0f / 2;
    float vertBearingY =
        (slot->metrics.vertAdvance / 64.0f - slot->metrics.height / 64.0f) / 2;

    /* Do conversion in two steps to avoid 'bad function cast' warning. */
    slot->metrics.width  = roundf(width * 64);
    slot->metrics.height = roundf(height * 64);

    slot->metrics.horiBearingX = horiBearingX * 64; /* XXX rounding? */
    slot->metrics.horiBearingY = horiBearingY * 64;
    slot->metrics.vertBearingX = vertBearingX * 64;
    slot->metrics.vertBearingY = vertBearingY * 64;

    if (slot->metrics.vertAdvance == 0)
        slot->metrics.vertAdvance = height * 1.2f * 64;

    if (!cache) {
        assert(state == &state_dummy);
        nsvgDelete(state->svg);
    }
    return FT_Err_Ok;
}

SVG_RendererHooks nanosvg_hooks = {
    .init_svg = &fcft_svg_init,
    .free_svg = &fcft_svg_free,
    .render_svg = &fcft_svg_render,
    .preset_slot = &fcft_svg_preset_slot,
};
