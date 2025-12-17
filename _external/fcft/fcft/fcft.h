#pragma once

#include <stdbool.h>
#include <stdint.h>
#include <stddef.h>

#include <pixman.h>

/*
 * Logging
 *
 * By default, fcft does not log anything at all. This can be changed
 * by calling fcft_log_init() (typically at program start).
 *
 * Note that fcft_log_init() does *not* call openlog(3), even when
 * do_syslog==true.
 */
enum fcft_log_colorize {
    FCFT_LOG_COLORIZE_NEVER,
    FCFT_LOG_COLORIZE_ALWAYS,
    FCFT_LOG_COLORIZE_AUTO
};

/* Which log messages to show. If you enable e.g. FCFT_LOG_CLASS_INFO,
 * then WARNINGs and ERRORs will also be shown. */
enum fcft_log_class {
    FCFT_LOG_CLASS_NONE,
    FCFT_LOG_CLASS_ERROR,
    FCFT_LOG_CLASS_WARNING,
    FCFT_LOG_CLASS_INFO,
    FCFT_LOG_CLASS_DEBUG
};

/* Must be called before instantiating fonts */
bool fcft_init(enum fcft_log_colorize colorize, bool do_syslog,
               enum fcft_log_class log_level);

/* Optional, but needed for clean valgrind runs */
void fcft_fini(void);

/*
 * Defines the subpixel order to use.
 *
 * Note that this is *ignored* if antialiasing has been disabled.
 */
enum fcft_subpixel {
    FCFT_SUBPIXEL_DEFAULT,          /* Use subpixel order from FontConfig */
    FCFT_SUBPIXEL_NONE,             /* Disable subpixel antialiasing (use grayscale antialiasing) */
    FCFT_SUBPIXEL_HORIZONTAL_RGB,
    FCFT_SUBPIXEL_HORIZONTAL_BGR,
    FCFT_SUBPIXEL_VERTICAL_RGB,
    FCFT_SUBPIXEL_VERTICAL_BGR,
};

struct fcft_font {
    const char *name;  /* Primary font name. Note: may be NULL */

    /* font extents */
    int height;
    int descent;
    int ascent;

    /* Width/height of font's widest glyph */
    struct {
        int x;
        int y;
    } max_advance;

    struct {
        int position;
        int thickness;
    } underline;

    struct {
        int position;
        int thickness;
    } strikeout;

    bool antialias;

    /* Mode used if a) antialias==true, and b) rasterize is called
     * with FCFT_SUBPIXEL_DEFAULT */
    enum fcft_subpixel subpixel;
};

/* Bitmask of optional capabilities */
enum fcft_capabilities {
    FCFT_CAPABILITY_GRAPHEME_SHAPING = 0x1,  /* Since 2.3.0 */
    FCFT_CAPABILITY_TEXT_RUN_SHAPING = 0x2,  /* Since 2.4.0 */
    FCFT_CAPABILITY_SVG = 0x4,               /* Since 3.1.0 */
};

enum fcft_capabilities fcft_capabilities(void);

/* First entry is the main/primary font, the remaining (if any) are
 * custom fallback fonts */
struct fcft_font *fcft_from_name(
    size_t count, const char *names[static count], const char *attributes);
struct fcft_font *fcft_clone(const struct fcft_font *font);
void fcft_destroy(struct fcft_font *font);

struct fcft_glyph {
    uint32_t cp;
    int cols;              /* wcwidth(cp) */

    const char *font_name;  /* Note: may be NULL. Always NULL in text-runs */
    pixman_image_t *pix;

    int x;
    int y;
    int width;
    int height;

    struct {
        int x;
        int y;
    } advance;

    bool is_color_glyph;  /* To be blended, instead of used as mask */
};

/* Rasterize the Unicode codepoint 'cp' using 'font'. Use the defined
 * subpixel mode *if* antialiasing is enabled for this font */
const struct fcft_glyph *fcft_rasterize_char_utf32(
    struct fcft_font *font, uint32_t cp, enum fcft_subpixel subpixel);

struct fcft_grapheme {
    int cols;  /* wcswidth(grapheme) */

    size_t count;
    const struct fcft_glyph **glyphs;
};

const struct fcft_grapheme *fcft_rasterize_grapheme_utf32(
    struct fcft_font *font,
    size_t len, const uint32_t grapheme_cluster[static len],
    enum fcft_subpixel subpixel);

struct fcft_text_run {
    const struct fcft_glyph **glyphs;
    int *cluster;
    size_t count;
};

struct fcft_text_run *fcft_rasterize_text_run_utf32(
    struct fcft_font *font, size_t len, const uint32_t text[static len],
    enum fcft_subpixel subpixel);

void fcft_text_run_destroy(struct fcft_text_run *run);

bool fcft_kerning(
    struct fcft_font *font, uint32_t left, uint32_t right,
    long *restrict x, long *restrict y);

uint32_t fcft_precompose(const struct fcft_font *font,
                         uint32_t base, uint32_t comb,
                         bool *base_is_from_primary,
                         bool *comb_is_from_primary,
                         bool *composed_is_from_primary);

#if defined(__has_attribute)
 #define FCFT_DEPRECATED __attribute__((deprecated))
#else
 #define FCFT_DEPRECATED
#endif

enum fcft_scaling_filter {
    FCFT_SCALING_FILTER_NONE,
    FCFT_SCALING_FILTER_NEAREST,
    FCFT_SCALING_FILTER_BILINEAR,

    /*
     * Separable convolution filters
     */

    FCFT_SCALING_FILTER_CUBIC,
    FCFT_SCALING_FILTER_LANCZOS3,

    /* Added in 3.3.0 */
    FCFT_SCALING_FILTER_IMPULSE,
    FCFT_SCALING_FILTER_BOX,
    FCFT_SCALING_FILTER_LINEAR,
    FCFT_SCALING_FILTER_GAUSSIAN,
    FCFT_SCALING_FILTER_LANCZOS2,
    FCFT_SCALING_FILTER_LANCZOS3_STRETCHED,
};

/* Note: this function does not clear any caches - call *before*
 * rasterizing any glyphs! */
bool fcft_set_scaling_filter(enum fcft_scaling_filter filter) FCFT_DEPRECATED;

/*
 * Emoji presentation
 *
 * This API allows you to configure which emoji presentation to use
 * for emojis that have both a “text” and an “emoji” presentation,
 * when no explicit presentation selector is present.
 *
 * This setting does *not* affect emojis that does not have multiple
 * presentation forms. Nor does it affect emoji codepoints followed by
 * an explicit presentation selector (0xfe0e or 0xfe0f).
 *
 * Note that this setting is always applied (for emojis with multiple
 * presentation forms, that is) in fcft_glyph_rasterize(), since it
 * only sees a single codepoint.
 *
 * Note: this function does *not* clear the glyph or grapheme caches -
 * call *before* rasterizing any glyphs!
 */
enum fcft_emoji_presentation {
    FCFT_EMOJI_PRESENTATION_DEFAULT,
    FCFT_EMOJI_PRESENTATION_TEXT,
    FCFT_EMOJI_PRESENTATION_EMOJI,
};


/* TODO: remove */
void fcft_set_emoji_presentation(
    struct fcft_font *font,
    enum fcft_emoji_presentation presentation) FCFT_DEPRECATED;


struct fcft_font_options {
    enum fcft_emoji_presentation emoji_presentation;

    struct {
        bool srgb_decode;
        pixman_format_code_t format;
    } color_glyphs;

    enum fcft_scaling_filter scaling_filter;
};

struct fcft_font_options *fcft_font_options_create(void);
void fcft_font_options_destroy(struct fcft_font_options *options);

struct fcft_font *fcft_from_name2(
    size_t count, const char *names[static count], const char *attributes,
    const struct fcft_font_options *options);
