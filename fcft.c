#include "fcft/fcft.h"

#include <stdlib.h>
#include <stdint.h>
#include <stdbool.h>
#include <math.h>
#include <assert.h>
#include <threads.h>
#include <locale.h>

#include <wchar.h>  /* TODO: remove */

#include <pthread.h>

#include <ft2build.h>
#include FT_FREETYPE_H
#include FT_MODULE_H
#include FT_LCD_FILTER_H
#include FT_TRUETYPE_TABLES_H
#include FT_SYNTHESIS_H
#include <fontconfig/fontconfig.h>

#if defined(FCFT_HAVE_HARFBUZZ)
 #include <harfbuzz/hb.h>
 #include <harfbuzz/hb-ft.h>
#endif
#if defined(FCFT_HAVE_UTF8PROC)
 #include <utf8proc.h>
#endif

#include <tllist.h>

#define LOG_MODULE "fcft"
#define LOG_ENABLE_DBG 0
#include "log.h"
#include "fcft/stride.h"

#include "emoji-data.h"
#include "unicode-compose-table.h"
#include "version.h"

#if defined(FCFT_ENABLE_SVG_NANOSVG)
 #include "svg-backend-nanosvg.h"
#endif
#if defined(FCFT_ENABLE_SVG_LIBRSVG)
 #include "3rd-party/freetype-rsvg/rsvg-port.h"
#endif

#define min(x, y) ((x) < (y) ? (x) : (y))
#define max(x, y) ((x) > (y) ? (x) : (y))
#define ALEN(v) (sizeof(v) / sizeof((v)[0]))

static FT_Library ft_lib = NULL;
static mtx_t ft_lock;
static bool can_set_lcd_filter = false;

/* fcft_set_scaling_filter() is deprecated; -1 means use the per-font
   scaling filter */
static enum fcft_scaling_filter scaling_filter = -1;

static const size_t glyph_cache_initial_size = 256;
#if defined(FCFT_HAVE_HARFBUZZ)
static const size_t grapheme_cache_initial_size = 256;
#endif

#if defined(_DEBUG)
static size_t glyph_cache_lookups = 0;
static size_t glyph_cache_collisions = 0;

#if defined(FCFT_HAVE_HARFBUZZ)
static size_t grapheme_cache_lookups = 0;
static size_t grapheme_cache_collisions = 0;
#endif
#endif

static int cpwidth(uint32_t cp);

void fcft_log_init(enum fcft_log_colorize _colorize, bool _do_syslog,
                   enum fcft_log_class _log_level);

struct glyph_priv {
    struct fcft_glyph public;
    enum fcft_subpixel subpixel;
    bool valid;
};

struct grapheme_priv {
    struct fcft_grapheme public;

    size_t len;
    uint32_t *cluster;  /* ‘len’ characters */

    enum fcft_subpixel subpixel;
    bool valid;
};

struct instance {
    char *name;
    char *path;
    FT_Face face;
    int load_flags;

#if defined(FCFT_HAVE_HARFBUZZ)
    hb_font_t *hb_font;
    hb_buffer_t *hb_buf;
    hb_feature_t hb_feats[32];
    size_t hb_feats_count;
#endif

    bool antialias;
    bool embolden;
    bool is_color;
    int render_flags_normal;
    int render_flags_subpixel;

    FT_LcdFilter lcd_filter;

    double pixel_size_fixup; /* Scale factor - should only be used with ARGB32 glyphs */
    bool pixel_fixup_estimated;
    bool bgr;  /* True for FC_RGBA_BGR and FC_RGBA_VBGR */

    struct fcft_font metrics;
};

struct fallback {
    FcPattern *pattern;
    FcCharSet *charset;
    FcLangSet *langset;
    struct instance *font;

    /* User-requested size(s) - i.e. sizes from *base* pattern */
    double req_pt_size;
    double req_px_size;
};

struct font_priv {
    /* Must be first */
    struct fcft_font public;

    mtx_t lock;
    pthread_rwlock_t glyph_cache_lock;
    struct {
        struct glyph_priv **table;
        size_t size;
        size_t count;
    } glyph_cache;

#if defined(FCFT_HAVE_HARFBUZZ)
    pthread_rwlock_t grapheme_cache_lock;
    struct {
        struct grapheme_priv **table;
        size_t size;
        size_t count;
    } grapheme_cache;
#endif

    tll(struct fallback) fallbacks;

    struct fcft_font_options options;
    size_t ref_counter;
};

/* Global font cache */
struct fcft_font_cache_entry {
    uint64_t hash;

    char **names;
    char *attributes;
    size_t name_count;
    struct fcft_font_options options;

    struct font_priv *font;

    int waiters;
    cnd_t cond;
};
static tll(struct fcft_font_cache_entry) font_cache = tll_init();
static mtx_t font_cache_lock;

FCFT_EXPORT enum fcft_capabilities
fcft_capabilities(void)
{
    enum fcft_capabilities ret = 0;

#if defined(FCFT_HAVE_HARFBUZZ)
    ret |= FCFT_CAPABILITY_GRAPHEME_SHAPING;
#endif
#if defined(FCFT_HAVE_HARFBUZZ) && defined(FCFT_HAVE_UTF8PROC)
    ret |= FCFT_CAPABILITY_TEXT_RUN_SHAPING;
#endif
#if defined(FCFT_ENABLE_SVG_LIBRSVG) || defined(FCFT_ENABLE_SVG_NANOSVG)
    ret |= FCFT_CAPABILITY_SVG;
#endif

    return ret;
}

static const char *
ft_error_string(FT_Error err)
{
    #undef FTERRORS_H_
    #undef __FTERRORS_H__
    #define FT_ERRORDEF( e, v, s )  case e: return s;
    #define FT_ERROR_START_LIST     switch (err) {
    #define FT_ERROR_END_LIST       }
    #include FT_ERRORS_H
    return "unknown error";
}

FCFT_EXPORT bool
fcft_init(enum fcft_log_colorize colorize, bool do_syslog,
          enum fcft_log_class log_level)
{
    fcft_log_init(colorize, do_syslog, log_level);

    FT_Error ft_err = FT_Init_FreeType(&ft_lib);
    if (ft_err != FT_Err_Ok) {
        LOG_ERR("failed to initialize FreeType: %s",
                ft_error_string(ft_err));
        return false;
    }

#if defined(FCFT_ENABLE_SVG_NANOSVG)
    FT_Property_Set(ft_lib, "ot-svg", "svg-hooks", &nanosvg_hooks);
#elif defined(FCFT_ENABLE_SVG_LIBRSVG)
    FT_Property_Set(ft_lib, "ot-svg", "svg-hooks", &rsvg_hooks);
#endif

    FcInit();

    /*
     * Some FreeType builds use the older ClearType-style subpixel
     * rendering. This mode requires an LCD filter to be set, or it
     * will produce *severe* color fringes.
     *
     * Which subpixel rendering mode to use depends on a FreeType
     * build-time #define, FT_CONFIG_OPTION_SUBPIXEL_RENDERING. By
     * default, it is *unset*, meaning FreeType will use the newer
     * 'Harmony' subpixel rendering mode (i.e. disabling
     * FT_CONFIG_OPTION_SUBPIXEL_RENDERING does *not* disable subpixel
     * rendering).
     *
     * If defined, ClearType-style subpixel rendering is instead enabled.
     *
     * The FT_Library_SetLcdFilter() configures a library instance
     * global LCD filter. This call will *fail* if
     * FT_CONFIG_OPTION_SUBPIXEL_RENDERING has not been set.
     *
     * In theory, different fonts can have different LCD filters, and
     * for this reason we need to set the filter *each* time we need
     * to render a glyph. Since FT_Library_SetLcdFilter() is per
     * library instance, this means taking the library global lock.
     *
     * For performance reasons, we want to skip this altogether if we
     * know that the call will fail (i.e. when FreeType is using
     * Harmony). So, detect here in init(), whether we can set an LCD
     * filter or not.
     *
     * When rendering a glyph, we will only configure the LCD filter
     * (and thus take the library global lock) if we know we *can* set
     * the filter, and if we need to (the rendering mode includes
     * FT_RENDER_MODE_LCD{,_V}).
     */

    FT_Error err = FT_Library_SetLcdFilter(ft_lib, FT_LCD_FILTER_DEFAULT);
    can_set_lcd_filter = err == 0;
    LOG_DBG("can set LCD filter: %s (%d)", ft_error_string(err), err);

    if (can_set_lcd_filter)
        FT_Library_SetLcdFilter(ft_lib, FT_LCD_FILTER_NONE);

#if defined(FCFT_HAVE_HARFBUZZ)
    /* Not thread safe first time called */
    hb_language_get_default();
#endif

    mtx_init(&ft_lock, mtx_plain);
    mtx_init(&font_cache_lock, mtx_plain);
    return true;
}

FCFT_EXPORT void
fcft_fini(void)
{
    while (tll_length(font_cache) > 0) {
        struct fcft_font_cache_entry *e = &tll_front(font_cache);

        if (e->font == NULL) {
            for (size_t i = 0; i < e->name_count; i++)
                free(e->names[i]);
            free(e->names);
            free(e->attributes);
            tll_pop_front(font_cache);
        } else
            fcft_destroy(&e->font->public);
    }

    assert(tll_length(font_cache) == 0);

    mtx_destroy(&font_cache_lock);
    mtx_destroy(&ft_lock);

    FT_Done_FreeType(ft_lib);
    FcFini();

    LOG_DBG("glyph cache: lookups=%zu, collisions=%zu",
            glyph_cache_lookups, glyph_cache_collisions);

#if defined(FCFT_HAVE_HARFBUZZ)
    LOG_DBG("grapheme cache: lookups=%zu, collisions=%zu",
            grapheme_cache_lookups, grapheme_cache_collisions);
#endif
}

static bool
is_assertions_enabled(void)
{
#if defined(NDEBUG)
    return false;
#else
    return true;
#endif
}

static void
log_version_information(void)
{
    static bool has_already_logged = false;

    mtx_lock(&font_cache_lock);
    if (has_already_logged) {
        mtx_unlock(&font_cache_lock);
        return;
    }
    has_already_logged = true;
    mtx_unlock(&font_cache_lock);

    enum fcft_capabilities caps = fcft_capabilities();

    char caps_str[256];
    snprintf(
        caps_str, sizeof(caps_str),
        "%cgraphemes %cruns %csvg%s %cassertions",
        caps & FCFT_CAPABILITY_GRAPHEME_SHAPING ? '+' : '-',
        caps & FCFT_CAPABILITY_TEXT_RUN_SHAPING ? '+' : '-',
        caps & FCFT_CAPABILITY_SVG ? '+' : '-',
#if defined(FCFT_ENABLE_SVG_LIBRSVG)
        "(librsvg)",
#elif defined(FCFT_ENABLE_SVG_NANOSVG)
        "(nanosvg)",
#else
        "",
#endif
        is_assertions_enabled() ? '+' : '-');

    LOG_INFO("fcft: %s %s", FCFT_VERSION, caps_str);

    char deps_str[128];
    int deps_idx = 0;

    {
        int raw_version = FcGetVersion();

        /* See FC_VERSION in <fontconfig/fontconfig.h> */
        const int major = raw_version / 10000; raw_version %= 10000;
        const int minor = raw_version / 100; raw_version %= 100;
        const int patch = raw_version;

        deps_idx += snprintf(
            &deps_str[deps_idx], sizeof(deps_str) - deps_idx,
            "fontconfig: %d.%d.%d", major, minor, patch);
    }

    {
        int major, minor, patch;
        FT_Library_Version(ft_lib, &major, &minor, &patch);
        deps_idx += snprintf(
            &deps_str[deps_idx], sizeof(deps_str) - deps_idx,
            ", freetype: %d.%d.%d", major, minor, patch);
    }

#if defined(FCFT_HAVE_HARFBUZZ)
    deps_idx += snprintf(
        &deps_str[deps_idx], sizeof(deps_str) - deps_idx,
        ", harfbuzz: %s", hb_version_string());
#endif

#if defined(FCFT_HAVE_UTF8PROC)
    deps_idx += snprintf(
        &deps_str[deps_idx], sizeof(deps_str) - deps_idx,
        ", utf8proc: %s (Unicode %s)",
        utf8proc_version(), utf8proc_unicode_version());
#endif

    LOG_INFO("%.*s", deps_idx, deps_str);
}

FCFT_EXPORT bool
fcft_set_scaling_filter(enum fcft_scaling_filter filter)
{
    switch (filter) {
    case FCFT_SCALING_FILTER_NONE:
    case FCFT_SCALING_FILTER_NEAREST:
    case FCFT_SCALING_FILTER_BILINEAR:
    case FCFT_SCALING_FILTER_IMPULSE:
    case FCFT_SCALING_FILTER_BOX:
    case FCFT_SCALING_FILTER_LINEAR:
    case FCFT_SCALING_FILTER_CUBIC:
    case FCFT_SCALING_FILTER_GAUSSIAN:
    case FCFT_SCALING_FILTER_LANCZOS2:
    case FCFT_SCALING_FILTER_LANCZOS3:
    case FCFT_SCALING_FILTER_LANCZOS3_STRETCHED:
        scaling_filter = filter;
        return true;
    }

    return false;
}

static void
glyph_destroy_private(struct glyph_priv *glyph)
{
    if (glyph->valid) {
        void *image = pixman_image_get_data(glyph->public.pix);
        pixman_image_unref(glyph->public.pix);
        free(image);
    }

    free(glyph);
}

static void
glyph_destroy(const struct fcft_glyph *glyph)
{
    glyph_destroy_private((struct glyph_priv *)glyph);
}

static void
instance_destroy(struct instance *inst)
{
    if (inst == NULL)
        return;

#if defined(FCFT_HAVE_HARFBUZZ)
    hb_font_destroy(inst->hb_font);
    hb_buffer_destroy(inst->hb_buf);
#endif

    mtx_lock(&ft_lock);
    FT_Done_Face(inst->face);
    mtx_unlock(&ft_lock);

    free(inst->path);
    free(inst->name);
    free(inst);
}

static void
fallback_destroy(struct fallback *fallback)
{
    FcPatternDestroy(fallback->pattern);
    FcCharSetDestroy(fallback->charset);
    if (fallback->langset != NULL)
        FcLangSetDestroy(fallback->langset);
    instance_destroy(fallback->font);
}

static void
underline_strikeout_metrics(FT_Face ft_face, struct fcft_font *font)
{
    double y_scale = ft_face->size->metrics.y_scale / 65536.;
    double ascent = ft_face->size->metrics.ascender / 64.;
    double descent = ft_face->size->metrics.descender / 64.;

    double underline_position = ft_face->underline_position * y_scale / 64.;
    double underline_thickness = ft_face->underline_thickness * y_scale / 64.;

    if (underline_position == 0.) {
        LOG_DBG("estimating underline position and thickness");
        underline_thickness = fabs(descent / 5.);

        //underline_position = descent / 2.;           /* Alacritty's algorithm */
        underline_position = -2 * underline_thickness; /* My own */
    }

    /*
     * Position refers to the line's center, thus we need to take the
     * thickness into account to determine the line top.
     *
     * Since a *negative* number means the position is *under* the
     * baseline, we need to *add* half the width to adjust the
     * position "upwards".
     *
     * When rounding the thickness, take care not go below 1.0 as that
     * would make it invisible.
     */
    font->underline.position = trunc(underline_position + underline_thickness / 2.);
    font->underline.thickness = round(max(1., underline_thickness));

    LOG_DBG("underline: pos=%f, thick=%f -> pos=%f, pos=%d, thick=%d",
            underline_position, underline_thickness,
            underline_position + underline_thickness / 2.,
            font->underline.position, font->underline.thickness);

    double strikeout_position = 0., strikeout_thickness = 0.;
    TT_OS2 *os2 = FT_Get_Sfnt_Table(ft_face, ft_sfnt_os2);
    if (os2 != NULL) {
        strikeout_position = os2->yStrikeoutPosition * y_scale / 64.;
        strikeout_thickness = os2->yStrikeoutSize * y_scale / 64.;
    }

    if (strikeout_position == 0.) {
        LOG_DBG("estimating strikeout position and thickness");
        strikeout_thickness = underline_thickness;

        //strikeout_position = height / 2. + descent;                     /* Alacritty's algorithm */
        strikeout_position = 3. * ascent / 8. - underline_thickness / 2.; /* xterm's algorithm */
    }

    font->strikeout.position = trunc(strikeout_position + strikeout_thickness / 2.);
    font->strikeout.thickness = round(max(1., strikeout_thickness));

    LOG_DBG("strikeout: pos=%f, thick=%f -> pos=%f, pos=%d, thick=%d",
            strikeout_position, strikeout_thickness,
            strikeout_position + strikeout_thickness / 2.,
            font->strikeout.position, font->strikeout.thickness);
}

static FcPattern *
base_pattern_from_name(const char *name, FcFontSet **set)
{
    /* Fontconfig fails to parse floating point values unless locale
     * (LC_NUMERIC) is e.g C, or en_US.UTF-8 */

    locale_t c_locale = newlocale(LC_NUMERIC_MASK, "C", (locale_t)0);
    if (c_locale == (locale_t)0) {
        LOG_ERRNO("%s: failed to create temporary locale", name);
        return NULL;
    }

    locale_t old_locale = uselocale(c_locale);
    if (old_locale == (locale_t)0) {
        LOG_ERRNO("%s: failed to set temporary C locale", name);
        freelocale(c_locale);
        return NULL;
    }

    FcPattern *pattern = FcNameParse((const unsigned char *)name);

    /* Restore locale */
    uselocale(old_locale);
    freelocale(c_locale);

    if (pattern == NULL) {
        LOG_ERR("%s: failed to lookup font", name);
        return NULL;
    }

    if (!FcConfigSubstitute(NULL, pattern, FcMatchPattern)) {
        LOG_ERR("%s: failed to do config substitution", name);
        FcPatternDestroy(pattern);
        return NULL;
    }

    FcDefaultSubstitute(pattern);

    FcResult result;
    *set = FcFontSort(NULL, pattern, FcTrue, NULL, &result);
    if (result != FcResultMatch) {
        LOG_ERR("%s: failed to match font", name);
        FcPatternDestroy(pattern);
        return NULL;
    }

    return pattern;
}

static FcPattern *
pattern_from_font_set(FcPattern *base_pattern, FcFontSet *set, size_t idx)
{
    FcPattern *pattern = FcFontRenderPrepare(NULL, base_pattern, set->fonts[idx]);
    if (pattern == NULL) {
        LOG_ERR("failed to prepare 'final' pattern");
        return NULL;
    }

    return pattern;
}

static bool
instantiate_pattern(FcPattern *pattern, double req_pt_size, double req_px_size,
                    struct instance *font)
{
    FcChar8 *face_file = NULL;
    if (FcPatternGetString(pattern, FC_FT_FACE, 0, &face_file) != FcResultMatch &&
        FcPatternGetString(pattern, FC_FILE, 0, &face_file) != FcResultMatch)
    {
        LOG_ERR("no face file path in pattern");
        return false;
    }

    double dpi;
    if (FcPatternGetDouble(pattern, FC_DPI, 0, &dpi) != FcResultMatch)
        dpi = 75;

    double size;
    if (FcPatternGetDouble(pattern, FC_SIZE, 0, &size) != FcResultMatch)
        LOG_WARN("%s: failed to get size", face_file);

    double pixel_size;
    if (FcPatternGetDouble(pattern, FC_PIXEL_SIZE, 0, &pixel_size) != FcResultMatch) {
        LOG_ERR("%s: failed to get pixel size", face_file);
        return false;
    }

    int face_index;
    if (FcPatternGetInteger(pattern, FC_INDEX, 0, &face_index) != FcResultMatch) {
        LOG_WARN("%s: failed to get face index", face_file);
        face_index = 0;
    }

    FcChar8 *full_name = NULL;
    if (FcPatternGetString(pattern, FC_FULLNAME, 0, &full_name) != FcResultMatch)
        LOG_WARN("failed to get full font name");

    mtx_lock(&ft_lock);
    FT_Face ft_face;
    FT_Error ft_err = FT_New_Face(ft_lib, (const char *)face_file, face_index, &ft_face);
    mtx_unlock(&ft_lock);
    if (ft_err != FT_Err_Ok) {
        LOG_ERR("%s: failed to create FreeType face; %s",
                face_file, ft_error_string(ft_err));
        return false;
    }

    /*
     * Using FT_Set_Char_Size() instead of FT_Set_Pixel_Sizes() allows
     * us to use "fractional" pixel values. This _does_ make a
     * difference.
     *
     * Since FT_Set_Char_Size() expects a size in *points*, not
     * pixels, we use a DPI of 72. This means a pixel have the same
     * size as a point.
     */
    if ((ft_err = FT_Set_Char_Size(ft_face, pixel_size * 64., 0, 72, 72))) {
        LOG_ERR("%s: failed to set character size: %s", face_file, ft_error_string(ft_err));
        goto err_done_face;
    }

    FcBool scalable;
    if (FcPatternGetBool(pattern, FC_SCALABLE, 0, &scalable) != FcResultMatch)
        scalable = FcTrue;

    FcBool outline;
    if (FcPatternGetBool(pattern, FC_OUTLINE, 0, &outline) != FcResultMatch)
        outline = FcTrue;

    FcBool is_color;
    if (FcPatternGetBool(pattern, FC_COLOR, 0, &is_color) != FcResultMatch)
        is_color = FcFalse;

    double pixel_fixup = 1.;
    bool fixup_estimated = false;
    if (FcPatternGetDouble(pattern, "pixelsizefixupfactor", 0, &pixel_fixup) != FcResultMatch) {
        /*
         * Force a fixup factor on scalable bitmap fonts (typically
         * emoji fonts). The fixup factor is
         *   requested-pixel-size / actual-pixels-size
         */
        if (scalable && !outline) {
            if (req_px_size < 0.)
                req_px_size = req_pt_size * dpi / 72.;

            pixel_fixup = req_px_size / ft_face->size->metrics.y_ppem;
            fixup_estimated = true;
            LOG_DBG("estimated pixel fixup factor to %f (from pixel size: %f)",
                    pixel_fixup, req_px_size);
        } else
            pixel_fixup = 1.;
    }

#if 0
    LOG_DBG("FIXED SIZES: %d", ft_face->num_fixed_sizes);
    for (int i = 0; i < ft_face->num_fixed_sizes; i++)
        LOG_DBG("  #%d: height=%d, y_ppem=%f", i, ft_face->available_sizes[i].height, ft_face->available_sizes[i].y_ppem / 64.);
#endif

    FcBool fc_hinting;
    if (FcPatternGetBool(pattern, FC_HINTING,0,  &fc_hinting) != FcResultMatch)
        fc_hinting = FcTrue;

    FcBool fc_antialias;
    if (FcPatternGetBool(pattern, FC_ANTIALIAS, 0, &fc_antialias) != FcResultMatch)
        fc_antialias = FcTrue;

    int fc_hintstyle;
    if (FcPatternGetInteger(pattern, FC_HINT_STYLE, 0, &fc_hintstyle) != FcResultMatch)
        fc_hintstyle = FC_HINT_SLIGHT;

    int fc_rgba;
    if (FcPatternGetInteger(pattern, FC_RGBA, 0, &fc_rgba) != FcResultMatch)
        fc_rgba = FC_RGBA_UNKNOWN;

    int load_flags = FT_LOAD_DEFAULT;
    int load_target = FT_LOAD_TARGET_NORMAL;

    if (!fc_antialias) {
        if (!fc_hinting || fc_hintstyle == FC_HINT_NONE)
            load_flags |= FT_LOAD_NO_HINTING;
        else
            load_target = FT_LOAD_TARGET_MONO;

        load_flags |= FT_LOAD_MONOCHROME;
    }

    else {
        if (!fc_hinting || fc_hintstyle == FC_HINT_NONE)
            load_flags |= FT_LOAD_NO_HINTING;

        else if (fc_hintstyle == FC_HINT_SLIGHT)
            load_target = FT_LOAD_TARGET_LIGHT;

        else if (fc_hintstyle == FC_HINT_MEDIUM)
            ;

        else if (fc_rgba == FC_RGBA_RGB || fc_rgba == FC_RGBA_BGR)
            load_target = FT_LOAD_TARGET_LCD;

        else if (fc_rgba == FC_RGBA_VRGB || fc_rgba == FC_RGBA_VBGR)
            load_target = FT_LOAD_TARGET_LCD_V;
    }

    FcBool fc_embeddedbitmap;
    if (FcPatternGetBool(pattern, FC_EMBEDDED_BITMAP, 0, &fc_embeddedbitmap) != FcResultMatch)
        fc_embeddedbitmap = FcTrue;

    if (!fc_embeddedbitmap && outline)
        load_flags |= FT_LOAD_NO_BITMAP;

    FcBool fc_autohint;
    if (FcPatternGetBool(pattern, FC_AUTOHINT, 0, &fc_autohint) != FcResultMatch)
        fc_autohint = FcFalse;

    if (fc_autohint)
        load_flags |= FT_LOAD_FORCE_AUTOHINT;

    int render_flags_normal, render_flags_subpixel;
    if (!fc_antialias)
        render_flags_normal = render_flags_subpixel = FT_RENDER_MODE_MONO;

    else {
        if (fc_rgba == FC_RGBA_RGB || fc_rgba == FC_RGBA_BGR) {
            render_flags_subpixel = FT_RENDER_MODE_LCD;
            render_flags_normal = FT_RENDER_MODE_NORMAL;
        }

        else if (fc_rgba == FC_RGBA_VRGB || fc_rgba == FC_RGBA_VBGR) {
            render_flags_subpixel = FT_RENDER_MODE_LCD_V;
            render_flags_normal = FT_RENDER_MODE_NORMAL;
        }

        else
            render_flags_normal = render_flags_subpixel = FT_RENDER_MODE_NORMAL;
    }

    int fc_lcdfilter;
    if (FcPatternGetInteger(pattern, FC_LCD_FILTER, 0, &fc_lcdfilter) != FcResultMatch)
        fc_lcdfilter = FC_LCD_DEFAULT;

    font->lcd_filter = FT_LCD_FILTER_DEFAULT;

    switch (fc_lcdfilter) {
    case FC_LCD_NONE:    font->lcd_filter = FT_LCD_FILTER_NONE; break;
    case FC_LCD_DEFAULT: font->lcd_filter = FT_LCD_FILTER_DEFAULT; break;
    case FC_LCD_LIGHT:   font->lcd_filter = FT_LCD_FILTER_LIGHT; break;
    case FC_LCD_LEGACY:  font->lcd_filter = FT_LCD_FILTER_LEGACY; break;
    }

    FcBool fc_embolden;
    if (FcPatternGetBool(pattern, FC_EMBOLDEN, 0, &fc_embolden) != FcResultMatch)
        fc_embolden = FcFalse;

    FcMatrix *fc_matrix;
    if (FcPatternGetMatrix(pattern, FC_MATRIX, 0, &fc_matrix) == FcResultMatch) {
        FT_Matrix m = {
            .xx = fc_matrix->xx * 0x10000,
            .xy = fc_matrix->xy * 0x10000,
            .yx = fc_matrix->yx * 0x10000,
            .yy = fc_matrix->yy * 0x10000,
        };
        FT_Set_Transform(ft_face, &m, NULL);
    }

    font->name = full_name != NULL ? strdup((char *)full_name) : NULL;
    font->path = strdup((char *)face_file);
    if (font->path == NULL) {
        free(font->name);
        free(font->path);
        goto err_done_face;
    }

    font->face = ft_face;
    font->load_flags = load_target | load_flags | FT_LOAD_COLOR;
    font->antialias = fc_antialias;
    font->embolden = fc_embolden;
    font->is_color = is_color;
    font->render_flags_normal = render_flags_normal;
    font->render_flags_subpixel = render_flags_subpixel;
    font->pixel_size_fixup = pixel_fixup;
    font->pixel_fixup_estimated = fixup_estimated;
    font->bgr = fc_rgba == FC_RGBA_BGR || fc_rgba == FC_RGBA_VBGR;

    /* For logging: e.g. "+ss01 -dlig" */
    char features[256] = {0};

#if defined(FCFT_HAVE_HARFBUZZ)
    font->hb_feats_count = 0;

    font->hb_font = hb_ft_font_create_referenced(ft_face);
    if (font->hb_font == NULL) {
        LOG_ERR("%s: failed to instantiate harfbuzz font", face_file);
        goto err_free_path;
    }

    font->hb_buf = hb_buffer_create();
    if (font->hb_buf == NULL) {
        LOG_ERR("%s: failed to instantiate harfbuzz buffer", face_file);
        goto err_hb_font_destroy;
    }

    for (size_t feat_no = 0; feat_no < ALEN(font->hb_feats); feat_no++) {
        FcChar8 *fc_feat;
        if (FcPatternGetString(
            pattern, FC_FONT_FEATURES, feat_no, &fc_feat) != FcResultMatch)
        {
            break;
        }

        hb_feature_t *feat = &font->hb_feats[font->hb_feats_count];

        if (!hb_feature_from_string((const char *)fc_feat, -1, feat))
            continue;

        const char tag[4] = {
            (feat->tag >> 24) & 0xff,
            (feat->tag >> 16) & 0xff,
            (feat->tag >> 8) & 0xff,
            (feat->tag >> 0) & 0xff,
        };

        size_t ofs = font->hb_feats_count * 6;
        snprintf(&features[ofs], sizeof(features) - ofs,
                 " %c%.4s", feat->value ? '+' : '-', tag);

        LOG_DBG("feature: %.4s=%s", tag, feat->value ? "on" : "off");
        font->hb_feats_count++;
    }
#endif

    double max_x_advance = ft_face->size->metrics.max_advance / 64.;
    double max_y_advance = ft_face->size->metrics.height / 64.;
    double height = ft_face->size->metrics.height / 64.;
    double descent = ft_face->size->metrics.descender / 64.;
    double ascent = ft_face->size->metrics.ascender / 64.;

    font->metrics.height = ceil(height * font->pixel_size_fixup);
    font->metrics.descent = ceil(-descent * font->pixel_size_fixup);
    font->metrics.ascent = ceil(ascent * font->pixel_size_fixup);
    font->metrics.max_advance.x = ceil(max_x_advance * font->pixel_size_fixup);
    font->metrics.max_advance.y = ceil(max_y_advance * font->pixel_size_fixup);
    font->metrics.antialias = fc_antialias;
    font->metrics.subpixel = !fc_antialias
        ? FCFT_SUBPIXEL_NONE
        : (fc_rgba == FC_RGBA_RGB ? FCFT_SUBPIXEL_HORIZONTAL_RGB :
           fc_rgba == FC_RGBA_BGR ? FCFT_SUBPIXEL_HORIZONTAL_BGR :
           fc_rgba == FC_RGBA_VRGB ? FCFT_SUBPIXEL_VERTICAL_RGB :
           fc_rgba == FC_RGBA_VBGR ? FCFT_SUBPIXEL_VERTICAL_BGR :
           FCFT_SUBPIXEL_NONE);
    font->metrics.name = font->name;

    underline_strikeout_metrics(ft_face, &font->metrics);

#if defined(LOG_ENABLE_DBG) && LOG_ENABLE_DBG
    LOG_DBG("%s: size=%.2fpt/%.2fpx, dpi=%.2f, fixup-factor: %.4f, "
            "line-height: %dpx, ascent: %dpx, descent: %dpx, "
            "x-advance (max): %dpx, features:%s",
            font->path, size, pixel_size, dpi, font->pixel_size_fixup,
            font->metrics.height, font->metrics.ascent, font->metrics.descent,
            font->metrics.max_advance.x, features);
#else
    LOG_INFO("%s: size=%.2fpt/%.2fpx, dpi=%.2f%s",
             font->path, size, pixel_size, dpi, features);
#endif

    return true;

#if defined(FCFT_HAVE_HARFBUZZ)
err_hb_font_destroy:
    hb_font_destroy(font->hb_font);
err_free_path:
    free(font->path);
    free(font->name);
#endif

err_done_face:
    mtx_lock(&ft_lock);
    FT_Done_Face(ft_face);
    mtx_unlock(&ft_lock);
    return false;
}

static uint64_t
sdbm_hash(const char *s)
{
    uint64_t hash = 0;

    for (; *s != '\0'; s++) {
        int c = *s;
        hash = c + (hash << 6) + (hash << 16) - hash;
    }

    return hash;
}

static uint64_t
font_hash(size_t count, const char *names[static count], const char *attributes,
          const struct fcft_font_options *options)
{
    uint64_t hash = 0;
    for (size_t i = 0; i < count; i++)
        hash ^= sdbm_hash(names[i]);

    if (attributes != NULL)
        hash ^= sdbm_hash(attributes);

    _Static_assert(sizeof(*options) % 8 == 0, "struct fcft_font_options not a multiple of 8 bytes");

    for (size_t i = 0; i < sizeof(*options) / 8; i++) {
        const uint64_t v = ((const uint16_t *)options)[i];
        hash ^= v;
    }

    return hash;
}

static const struct fcft_font_options default_font_options = {
    .scaling_filter = FCFT_SCALING_FILTER_CUBIC,
    .emoji_presentation = FCFT_EMOJI_PRESENTATION_DEFAULT,
    .color_glyphs = {
        .srgb_decode = false,
        .format = PIXMAN_a8r8g8b8,
    },
};

FCFT_EXPORT struct fcft_font_options *
fcft_font_options_create(void)
{
    struct fcft_font_options *ret = calloc(1, sizeof(*ret));
    *ret = default_font_options;
    return ret;
}

FCFT_EXPORT void
fcft_font_options_destroy(struct fcft_font_options *options)
{
    free(options);
}

FCFT_EXPORT struct fcft_font *
fcft_from_name(size_t count, const char *names[static count],
               const char *attributes)
{
    return fcft_from_name2(count, names, attributes, NULL);
}


FCFT_EXPORT struct fcft_font *
fcft_from_name2(
    size_t count, const char *names[static count], const char *attributes,
    const struct fcft_font_options *options)
{
    if (ft_lib == NULL) {
        LOG_ERR("fcft_init() not called");
        return NULL;
    }

    log_version_information();

    if (count == 0)
        return NULL;

    if (options == NULL)
        options = &default_font_options;

#if 0
    if (options->color_glyphs.format != PIXMAN_a8r8g8b8 &&
        options->color_glyphs.format != PIXMAN_a2r10g10b10 &&
        options->color_glyphs.format != PIXMAN_a2b10g10r10)
    {
        /* Unsupported format */
        return NULL;
    }
#endif
    uint64_t hash = font_hash(count, names, attributes, options);
    struct fcft_font_cache_entry *cache_entry = NULL;

    mtx_lock(&font_cache_lock);
    tll_foreach(font_cache, it) {
        if (it->item.hash != hash)
            continue;

        if (memcmp(&it->item.options, options, sizeof(*options)) != 0)
            continue;

        if (it->item.name_count != count)
            continue;

        if ((attributes == NULL && it->item.attributes != NULL) ||
            (attributes != NULL && it->item.attributes == NULL))
        {
            continue;
        }

        if (attributes != NULL &&
            strcmp(it->item.attributes, attributes) != 0)
        {
            continue;
        }

        for (size_t i = 0; i < count; i++) {
            if (strcmp(it->item.names[i], names[i]) != 0)
                continue;
        }

        struct fcft_font_cache_entry *e = &it->item;

        if (e->font != (void *)(uintptr_t)-1) {
            /* Font has already been fully initialized */

            if (e->font != NULL) {
                mtx_lock(&e->font->lock);
                e->font->ref_counter++;
                mtx_unlock(&e->font->lock);
            }

            mtx_unlock(&font_cache_lock);
            return e->font != NULL ? &e->font->public : NULL;
        }

        /*
         * Cache entry is only a reservation - font hasn't yet
         * been initialized.
         */

        /* Let instantiating thread know (yet) another thread
         * wants a reference to it */
        e->waiters++;

        while (e->font == (void *)(uintptr_t)-1)
            cnd_wait(&e->cond, &font_cache_lock);
        mtx_unlock(&font_cache_lock);
        return e->font == NULL ? NULL : &e->font->public;
    }

    /* Pre-allocate entry */
    tll_push_back(
        font_cache,
        ((struct fcft_font_cache_entry){.hash = hash, .font = (void *)(uintptr_t)-1}));

    cache_entry = &tll_back(font_cache);

    cache_entry->name_count = count;
    cache_entry->names = malloc(count * sizeof(char *));
    cache_entry->attributes = attributes != NULL ? strdup(attributes) : NULL;
    memcpy(&cache_entry->options, options, sizeof(*options));

    for (size_t i = 0; i < count; i++)
        cache_entry->names[i] = strdup(names[i]);

    if (cnd_init(&cache_entry->cond) != thrd_success) {
        LOG_ERR("%s: failed to instantiate font cache condition variable", names[0]);
        tll_pop_back(font_cache);
        mtx_unlock(&font_cache_lock);
        return NULL;
    }
    mtx_unlock(&font_cache_lock);

    struct font_priv *font = NULL;

    bool have_attrs = attributes != NULL && strlen(attributes) > 0;
    size_t attr_len = have_attrs ? strlen(attributes) + 1 : 0;

    tll(struct fallback) fc_fallbacks = tll_init();

    bool first = true;
    for (size_t i = 0; i < count; i++) {
        const char *base_name = names[i];

        char name[strlen(base_name) + attr_len + 1];
        strcpy(name, base_name);
        if (have_attrs) {
            strcat(name, ":");
            strcat(name, attributes);
        }

        FcFontSet *set;
        FcPattern *base_pattern = base_pattern_from_name(name, &set);
        if (base_pattern == NULL)
            break;

        FcPattern *pattern = pattern_from_font_set(base_pattern, set, 0);
        if (pattern == NULL)
            break;

        FcCharSet *charset;
        if (FcPatternGetCharSet(base_pattern, FC_CHARSET, 0, &charset) != FcResultMatch &&
            FcPatternGetCharSet(pattern, FC_CHARSET, 0, &charset) != FcResultMatch)
        {
            LOG_ERR("%s: failed to get charset", name);
            FcPatternDestroy(pattern);
            FcPatternDestroy(base_pattern);
            FcFontSetDestroy(set);
            break;
        }

        FcLangSet *langset;
        if (FcPatternGetLangSet(pattern, FC_LANG, 0, &langset) != FcResultMatch)
            langset = NULL;

        double req_px_size = -1., req_pt_size = -1.;
        FcPatternGetDouble(base_pattern, FC_PIXEL_SIZE, 0, &req_px_size);
        FcPatternGetDouble(base_pattern, FC_SIZE, 0, &req_pt_size);

        if (first) {
            first = false;

            bool lock_failed = true;
            bool glyph_cache_lock_failed = true;
            bool grapheme_cache_lock_failed = true;
            bool pattern_failed = true;

            font = calloc(1, sizeof(*font));
            if (font == NULL)
                goto fc_destroy;

            if (mtx_init(&font->lock, mtx_plain) != thrd_success)
                LOG_WARN("%s: failed to instantiate mutex", name);
            else
                lock_failed = false;

            if (pthread_rwlock_init(&font->glyph_cache_lock, NULL) != 0)
                LOG_WARN("%s: failed to instantiate glyph cache rwlock", name);
            else
                glyph_cache_lock_failed = false;

            struct instance *primary = malloc(sizeof(*primary));
            if (primary == NULL ||
                !instantiate_pattern(pattern, req_pt_size, req_px_size, primary))
            {
                ;
            } else
                pattern_failed = false;

            struct glyph_priv **glyph_cache_table = calloc(
                glyph_cache_initial_size, sizeof(glyph_cache_table[0]));

#if defined(FCFT_HAVE_HARFBUZZ)
            if (pthread_rwlock_init(&font->grapheme_cache_lock, NULL) != 0)
                LOG_WARN("%s: failed to instantiate grapheme cache rwlock", name);
            else
                grapheme_cache_lock_failed = false;

            struct grapheme_priv **grapheme_cache_table = calloc(
                grapheme_cache_initial_size, sizeof(grapheme_cache_table[0]));
#else
            struct grapheme_priv **grapheme_cache_table = NULL;
            grapheme_cache_lock_failed = false;
#endif

            /* Handle failure(s) */
            if (lock_failed || glyph_cache_lock_failed ||
                grapheme_cache_lock_failed || pattern_failed ||
                glyph_cache_table == NULL
#if defined(FCFT_HAVE_HARFBUZZ)
                || grapheme_cache_table == NULL
#endif
                )
            {
                if (!lock_failed)
                    mtx_destroy(&font->lock);
                if (!glyph_cache_lock_failed)
                    pthread_rwlock_destroy(&font->glyph_cache_lock);
#if defined(FCFT_HAVE_HARFBUZZ)
                if (!grapheme_cache_lock_failed)
                    pthread_rwlock_destroy(&font->grapheme_cache_lock);
#endif
                free(primary);
                free(font);
                font = NULL;
                free(glyph_cache_table);
                free(grapheme_cache_table);
fc_destroy:
                FcPatternDestroy(pattern);
                FcPatternDestroy(base_pattern);
                FcFontSetDestroy(set);
                break;
            }

            font->ref_counter = 1;
            font->glyph_cache.size = glyph_cache_initial_size;
            font->glyph_cache.count = 0;
            font->glyph_cache.table = glyph_cache_table;
            font->options = *options;
            font->public = primary->metrics;

#if defined(FCFT_HAVE_HARFBUZZ)
            font->grapheme_cache.size = grapheme_cache_initial_size;
            font->grapheme_cache.count = 0;
            font->grapheme_cache.table = grapheme_cache_table;
#endif

            tll_push_back(font->fallbacks, ((struct fallback){
                        .pattern = pattern,
                        .charset = FcCharSetCopy(charset),
                        .langset = langset != NULL ? FcLangSetCopy(langset) : NULL,
                        .font = primary,
                        .req_px_size = req_px_size,
                        .req_pt_size = req_pt_size}));

            for (size_t i = 1; i < set->nfont; i++) {
                FcPattern *fallback_pattern = pattern_from_font_set(base_pattern, set, i);
                if (fallback_pattern == NULL)
                    continue;

                FcCharSet *fallback_charset;
                if (FcPatternGetCharSet(base_pattern, FC_CHARSET, 0, &fallback_charset) != FcResultMatch &&
                    FcPatternGetCharSet(fallback_pattern, FC_CHARSET, 0, &fallback_charset) != FcResultMatch)
                {
                    LOG_ERR("%s: failed to get charset", name);
                    FcPatternDestroy(fallback_pattern);
                    continue;
                }

                FcLangSet *fallback_langset;
                if (FcPatternGetLangSet(fallback_pattern, FC_LANG, 0, &fallback_langset) != FcResultMatch)
                    fallback_langset = NULL;

                FcPatternGetDouble(base_pattern, FC_PIXEL_SIZE, 0, &req_px_size);
                FcPatternGetDouble(base_pattern, FC_SIZE, 0, &req_pt_size);

                tll_push_back(fc_fallbacks, ((struct fallback){
                            .pattern = fallback_pattern,
                            .charset = FcCharSetCopy(fallback_charset),
                            .langset = fallback_langset != NULL ? FcLangSetCopy(fallback_langset) : NULL,
                            .req_px_size = req_px_size,
                            .req_pt_size = req_pt_size}));
            }

        } else {
            assert(font != NULL);
            tll_push_back(font->fallbacks, ((struct fallback){
                        .pattern = pattern,
                        .charset = FcCharSetCopy(charset),
                        .langset = langset != NULL ? FcLangSetCopy(langset) : NULL,
                        .req_px_size = req_px_size,
                        .req_pt_size = req_pt_size}));
        }

        FcFontSetDestroy(set);
        FcPatternDestroy(base_pattern);
    }

    /* Append fontconfig-based fallbacks at the end of our fallback list */
    if (font != NULL) {
        tll_foreach(fc_fallbacks, it)
            tll_push_back(font->fallbacks, it->item);
        tll_free(fc_fallbacks);
    }

    mtx_lock(&font_cache_lock);
    cache_entry->font = font;
    if (cache_entry->font != NULL)
        cache_entry->font->ref_counter += cache_entry->waiters;
    cnd_broadcast(&cache_entry->cond);
    mtx_unlock(&font_cache_lock);

    assert(font == NULL || (void *)&font->public == (void *)font);
    return font != NULL ? &font->public : NULL;
}

FCFT_EXPORT struct fcft_font *
fcft_clone(const struct fcft_font *_font)
{
    if (_font == NULL)
        return NULL;

    struct font_priv *font = (struct font_priv *)_font;

    mtx_lock(&font->lock);
    {
        assert(font->ref_counter >= 1);
        font->ref_counter++;
    }
    mtx_unlock(&font->lock);

    return &font->public;
}

static bool
glyph_for_index(const struct instance *inst, uint32_t index,
                enum fcft_subpixel subpixel,
                const struct fcft_font_options *options,
                struct glyph_priv *glyph)
{
    glyph->valid = false;
    glyph->subpixel = subpixel;

    pixman_image_t *pix = NULL;
    uint8_t *data = NULL;

    FT_Error err;
    if ((err = FT_Load_Glyph(inst->face, index, inst->load_flags)) != FT_Err_Ok) {
        LOG_ERR("%s: failed to load glyph #%d: %s",
                inst->path, index, ft_error_string(err));
        goto err;
    }

    if (inst->embolden && inst->face->glyph->format == FT_GLYPH_FORMAT_OUTLINE)
        FT_GlyphSlot_Embolden(inst->face->glyph);

    int render_flags;
    bool bgr;

    if (inst->antialias) {
        switch (subpixel) {
        case FCFT_SUBPIXEL_NONE:
            render_flags = inst->render_flags_normal;
            bgr = false;
            break;

        case FCFT_SUBPIXEL_HORIZONTAL_RGB:
        case FCFT_SUBPIXEL_HORIZONTAL_BGR:
            render_flags = FT_RENDER_MODE_LCD;
            bgr = subpixel == FCFT_SUBPIXEL_HORIZONTAL_BGR;
            break;

        case FCFT_SUBPIXEL_VERTICAL_RGB:
        case FCFT_SUBPIXEL_VERTICAL_BGR:
            render_flags = FT_RENDER_MODE_LCD_V;
            bgr = subpixel == FCFT_SUBPIXEL_VERTICAL_BGR;
            break;

        case FCFT_SUBPIXEL_DEFAULT:
        default:
            render_flags = inst->render_flags_subpixel;
            bgr = inst->bgr;
            break;
        }
    } else {
        render_flags = inst->render_flags_normal;
        bgr = false;
    }

#if FREETYPE_MAJOR >= 3 || (FREETYPE_MAJOR >= 2 && FREETYPE_MINOR >= 12)
    if (inst->face->glyph->format == FT_GLYPH_FORMAT_SVG) {
        /* Up to, and include, 2.12.1, FreeType rejects everything
         * else with “bad argument” */
        render_flags = FT_RENDER_MODE_NORMAL;
    }
#endif

    /*
     * LCD filter is per library instance, hence we need to re-set it
     * every time. But only if we need it, and only if we _can_, to
     * avoid locking a global mutex. See init().
     */
    bool unlock_ft_lock = false;
    if (can_set_lcd_filter &&
        ((render_flags & FT_RENDER_MODE_LCD) ||
         (render_flags & FT_RENDER_MODE_LCD_V)))
    {
        mtx_lock(&ft_lock);
        unlock_ft_lock = true;

        FT_Error err = FT_Library_SetLcdFilter(ft_lib, inst->lcd_filter);

        if (err != FT_Err_Ok) {
            LOG_WARN("failed to set LCD filter (%d): %s",
                     inst->lcd_filter, ft_error_string(err));
        }
    }

    if (inst->face->glyph->format != FT_GLYPH_FORMAT_BITMAP) {
        if ((err = FT_Render_Glyph(inst->face->glyph, render_flags)) != FT_Err_Ok) {
            LOG_ERR("%s: failed to render glyph: %s",
                    inst->path, ft_error_string(err));
            if (unlock_ft_lock)
                mtx_unlock(&ft_lock);
            goto err;
        }
    }

    if (unlock_ft_lock)
        mtx_unlock(&ft_lock);

    if (inst->face->glyph->format != FT_GLYPH_FORMAT_BITMAP) {
        LOG_ERR("%s: rasterized glyph is not a bitmap", inst->path);
        goto err;
    }

    const FT_Bitmap *bitmap = &inst->face->glyph->bitmap;
    pixman_format_code_t pix_format;
    int width;
    int rows;

    switch (bitmap->pixel_mode) {
    case FT_PIXEL_MODE_MONO:
        pix_format = PIXMAN_a1;
        width = bitmap->width;
        rows = bitmap->rows;
        break;

    case FT_PIXEL_MODE_GRAY:
        pix_format = PIXMAN_a8;
        width = bitmap->width;
        rows = bitmap->rows;
        break;

    case FT_PIXEL_MODE_LCD:
        pix_format = PIXMAN_x8r8g8b8;
        width = bitmap->width / 3;
        rows = bitmap->rows;
        break;

    case FT_PIXEL_MODE_LCD_V:
        pix_format = PIXMAN_x8r8g8b8;
        width = bitmap->width;
        rows = bitmap->rows / 3;
        break;

    case FT_PIXEL_MODE_BGRA:
        pix_format = options->color_glyphs.srgb_decode
            ? PIXMAN_a8r8g8b8_sRGB
            : PIXMAN_a8r8g8b8;
        width = bitmap->width;
        rows = bitmap->rows;
        break;

    default:
        LOG_ERR("unimplemented: FT pixel mode: %d", bitmap->pixel_mode);
        goto err;
        break;
    }

    int stride = stride_for_format_and_width(pix_format, width);
    assert(stride >= bitmap->pitch);

    assert(bitmap->buffer != NULL || rows * stride == 0);
    data = malloc(rows * stride);
    if (data == NULL)
        goto err;

    /* Convert FT bitmap to pixman image */
    switch (bitmap->pixel_mode) {
    case FT_PIXEL_MODE_MONO:  /* PIXMAN_a1 */
        /*
         * FreeType: left-most pixel is stored in MSB  ABCDEFGH IJKLMNOP
         * Pixman: LE: left-most pixel in LSB          HGFEDCBA PONMLKJI
         *         BE: left-most pixel in MSB          ABCDEFGH IJKLMNOP
         *
         * Thus, we need to reverse each byte on little-endian systems.
         */
        for (size_t r = 0; r < bitmap->rows; r++) {
            for (size_t c = 0; c < (bitmap->width + 7) / 8; c++) {
                uint8_t v = bitmap->buffer[r * bitmap->pitch + c];
#if __BYTE_ORDER__ == __ORDER_LITTLE_ENDIAN__
                uint8_t reversed = 0;
                for (size_t i = 0; i < min(8, bitmap->width - c * 8); i++)
                    reversed |= ((v >> (7 - i)) & 1) << i;

                data[r * stride + c] = reversed;
#else
                data[r * stride + c] = v;
#endif
            }
        }
        break;

    case FT_PIXEL_MODE_GRAY: /* PIXMAN_a8 */
        /*
         * One pixel, one byte. No endianness to worry about
         */
        if (stride == bitmap->pitch) {
            if (bitmap->buffer != NULL)
                memcpy(data, bitmap->buffer, rows * stride);
        } else {
            for (size_t r = 0; r < bitmap->rows; r++) {
                for (size_t c = 0; c < bitmap->width; c++)
                    data[r * stride + c] = bitmap->buffer[r * bitmap->pitch + c];
            }
        }
        break;

    case FT_PIXEL_MODE_BGRA: /* PIXMAN_a8r8g8b8 by default */
        /*
         * FreeType: blue comes *first* in memory
         * Pixman: LE: blue comes *first* in memory
         *         BE: alpha comes *first* in memory
         *
         * Pixman is ARGB *when loaded into a register*, assuming
         * machine native 32-bit loads. Thus, it’s in-memory layout
         * depends on the host’s endianness.
         */
        assert(stride == bitmap->pitch);
        for (size_t r = 0; r < bitmap->rows; r++) {
            for (size_t c = 0; c < bitmap->width * 4; c += 4) {
                unsigned char _b = bitmap->buffer[r * bitmap->pitch + c + 0];
                unsigned char _g = bitmap->buffer[r * bitmap->pitch + c + 1];
                unsigned char _r = bitmap->buffer[r * bitmap->pitch + c + 2];
                unsigned char _a = bitmap->buffer[r * bitmap->pitch + c + 3];

                uint32_t *p = (uint32_t *)&data[r * stride + c];
                *p = (uint32_t)_a << 24 | _r << 16 | _g << 8 | _b;
            }
        }

        break;

    case FT_PIXEL_MODE_LCD: /* PIXMAN_x8r8g8b8 */
        /*
         * FreeType: red comes *first* in memory
         * Pixman: LE: blue comes *first* in memory
         *         BE: x comes *first* in memory
         *
         * Same as above, except that the FreeType data is now RGBx
         * instead of BGRA.
         */
        for (size_t r = 0; r < bitmap->rows; r++) {
            for (size_t c = 0; c < bitmap->width; c += 3) {
                unsigned char _r = bitmap->buffer[r * bitmap->pitch + c + (bgr ? 2 : 0)];
                unsigned char _g = bitmap->buffer[r * bitmap->pitch + c + 1];
                unsigned char _b = bitmap->buffer[r * bitmap->pitch + c + (bgr ? 0 : 2)];

                uint32_t *p = (uint32_t *)&data[r * stride + 4 * (c / 3)];
                *p = _r << 16 | _g << 8 | _b;
            }
        }
        break;

    case FT_PIXEL_MODE_LCD_V: /* PIXMAN_x8r8g8b8 */
        /*
         * FreeType: red comes *first* in memory
         * Pixman: LE: blue comes *first* in memory
         *         BE: x comes *first* in memory
         *
         * Same as above, except that the FreeType data is now RGBx
         * instead of BGRA.
         */
        for (size_t r = 0; r < bitmap->rows; r += 3) {
            for (size_t c = 0; c < bitmap->width; c++) {
                unsigned char _r = bitmap->buffer[(r + (bgr ? 2 : 0)) * bitmap->pitch + c];
                unsigned char _g = bitmap->buffer[(r + 1) * bitmap->pitch + c];
                unsigned char _b = bitmap->buffer[(r + (bgr ? 0 : 2)) * bitmap->pitch + c];

                uint32_t *p = (uint32_t *)&data[r / 3 * stride + 4 * c];
                *p =  _r << 16 | _g << 8 | _b;
            }
        }
        break;

    default:
        abort();
        break;
    }

    if ((pix = pixman_image_create_bits_no_clear(
             pix_format, width, rows, (uint32_t *)data, stride)) == NULL)
        goto err;

    if (bitmap->pixel_mode == FT_PIXEL_MODE_BGRA &&
        options->color_glyphs.format != pix_format)
    {
        /*
         * Backing buffer is currently in sRGB, a8r8g8b8. Need to
         * decode and/or convert to requested output format.
         */
        assert(pix_format == PIXMAN_a8r8g8b8 ||
               pix_format == PIXMAN_a8r8g8b8_sRGB);

        /* Old pixman image + backing buffer will be replaced */
        pixman_image_t *a8r8g8b8 = pix;
        uint8_t *a8_data = data;

        /* New backing buffer */
        pix_format = options->color_glyphs.format;
        stride = stride_for_format_and_width(pix_format, width);
        data = malloc(rows * stride);

        if (data == NULL) {
            data = a8_data;
            pix = a8r8g8b8;
            goto err;
        }

        /* User requested output format */
        pix = pixman_image_create_bits_no_clear(
            pix_format, width, rows, (uint32_t *)data, stride);

        if (pix == NULL) {
            free(data);
            data = a8_data;
            pix = a8r8g8b8;
            goto err;
        }

        /* "copy" a8r8g8b8 source to user requested output
           format. This also decodes from sRGB, if requested */
        pixman_image_composite32(
            PIXMAN_OP_SRC, a8r8g8b8, NULL, pix, 0, 0, 0, 0, 0, 0, width, rows);

        pixman_image_unref(a8r8g8b8);
        free(a8_data);
    }

    pixman_image_set_component_alpha(
        pix,
        bitmap->pixel_mode == FT_PIXEL_MODE_LCD ||
        bitmap->pixel_mode == FT_PIXEL_MODE_LCD_V);

    int x = inst->face->glyph->bitmap_left;
    int y = inst->face->glyph->bitmap_top;

    if (inst->pixel_size_fixup == 0.)
        x = y = width = rows = 0;

    else if (inst->pixel_size_fixup != 1.) {
        struct pixman_f_transform scale;
        pixman_f_transform_init_scale(
            &scale,
            1. / inst->pixel_size_fixup,
            1. / inst->pixel_size_fixup);

        struct pixman_transform _scale;
        pixman_transform_from_pixman_f_transform(&_scale, &scale);
        pixman_image_set_transform(pix, &_scale);

        const enum fcft_scaling_filter filter_to_use = inst->is_color
            ? (scaling_filter != (enum fcft_scaling_filter)-1
                ? scaling_filter
                : options->scaling_filter)
            : FCFT_SCALING_FILTER_NONE;

        switch (filter_to_use) {
        case FCFT_SCALING_FILTER_NONE:
            break;

        case FCFT_SCALING_FILTER_NEAREST:
            pixman_image_set_filter(pix, PIXMAN_FILTER_NEAREST, NULL, 0);
            break;

        case FCFT_SCALING_FILTER_BILINEAR:
            pixman_image_set_filter(pix, PIXMAN_FILTER_BILINEAR, NULL, 0);
            break;

        case FCFT_SCALING_FILTER_IMPULSE:
        case FCFT_SCALING_FILTER_BOX:
        case FCFT_SCALING_FILTER_LINEAR:
        case FCFT_SCALING_FILTER_CUBIC:
        case FCFT_SCALING_FILTER_GAUSSIAN:
        case FCFT_SCALING_FILTER_LANCZOS2:
        case FCFT_SCALING_FILTER_LANCZOS3:
        case FCFT_SCALING_FILTER_LANCZOS3_STRETCHED: {
            /*
             * TODO:
             *   - find out how the subsample_bit_{x,y} parameters should be set
             */
            int param_count = 0;
            pixman_kernel_t kernel;

            switch (filter_to_use) {
            case FCFT_SCALING_FILTER_IMPULSE: kernel = PIXMAN_KERNEL_IMPULSE; break;
            case FCFT_SCALING_FILTER_BOX: kernel = PIXMAN_KERNEL_BOX; break;
            case FCFT_SCALING_FILTER_LINEAR: kernel = PIXMAN_KERNEL_LINEAR; break;
            case FCFT_SCALING_FILTER_CUBIC: kernel = PIXMAN_KERNEL_CUBIC; break;
            case FCFT_SCALING_FILTER_GAUSSIAN: kernel = PIXMAN_KERNEL_GAUSSIAN; break;
            case FCFT_SCALING_FILTER_LANCZOS2: kernel = PIXMAN_KERNEL_LANCZOS2; break;
            case FCFT_SCALING_FILTER_LANCZOS3: kernel = PIXMAN_KERNEL_LANCZOS3; break;
            case FCFT_SCALING_FILTER_LANCZOS3_STRETCHED:  kernel = PIXMAN_KERNEL_LANCZOS3_STRETCHED; break;

            default: assert(false); kernel = PIXMAN_KERNEL_CUBIC; break;
            }

            pixman_fixed_t *params = pixman_filter_create_separable_convolution(
                &param_count,
                pixman_double_to_fixed(1. / inst->pixel_size_fixup),
                pixman_double_to_fixed(1. / inst->pixel_size_fixup),
                kernel, kernel,
                kernel, kernel,
                pixman_int_to_fixed(1),
                pixman_int_to_fixed(1));

            pixman_image_set_filter(
                pix, PIXMAN_FILTER_SEPARABLE_CONVOLUTION,
                params, param_count);
            free(params);
            break;
        }
        }

        int scaled_width = width / (1. / inst->pixel_size_fixup);
        int scaled_rows = rows / (1. / inst->pixel_size_fixup);
        int scaled_stride = stride_for_format_and_width(pix_format, scaled_width);

        if (bitmap->pixel_mode == FT_PIXEL_MODE_BGRA) {
            uint8_t *scaled_data = malloc(scaled_rows * scaled_stride);
            if (scaled_data == NULL)
                goto err;

            pixman_image_t *scaled_pix = pixman_image_create_bits_no_clear(
                pix_format, scaled_width, scaled_rows,
                (uint32_t *)scaled_data, scaled_stride);

            if (scaled_pix == NULL) {
                free(scaled_data);
                goto err;
            }

            pixman_image_composite32(
                PIXMAN_OP_SRC, pix, NULL, scaled_pix, 0, 0, 0, 0,
                0, 0, scaled_width, scaled_rows);

            pixman_image_unref(pix);
            free(data);

            data = scaled_data;
            pix = scaled_pix;
        }

        rows = scaled_rows;
        width = scaled_width;
        stride = scaled_stride;

        x *= inst->pixel_size_fixup;
        y *= inst->pixel_size_fixup;
    }

    *glyph = (struct glyph_priv){
        .public = {
            .font_name = inst->name,
            .pix = pix,
            .x = x,
            .y = y,
            .advance = {
                .x = (inst->face->glyph->advance.x / 64. *
                      (inst->pixel_fixup_estimated ? inst->pixel_size_fixup : 1.)),
                .y = (inst->face->glyph->advance.y / 64. *
                      (inst->pixel_fixup_estimated ? inst->pixel_size_fixup : 1.)),
            },
            .width = width,
            .height = rows,
            .is_color_glyph = bitmap->pixel_mode == FT_PIXEL_MODE_BGRA,
        },
        .subpixel = subpixel,
        .valid = true,
    };

    return true;

err:
    if (pix != NULL)
        pixman_image_unref(pix);
    free(data);
    assert(!glyph->valid);
    return false;
}

static bool
glyph_for_codepoint(const struct instance *inst, uint32_t cp,
                    enum fcft_subpixel subpixel,
                    const struct fcft_font_options *options,
                    struct glyph_priv *glyph)
{
    FT_UInt idx = -1;

#if defined(FCFT_HAVE_HARFBUZZ)
    /*
     * Use HarfBuzz if we have any font features. If we don’t, there’s
     * no point in using HarfBuzz since it only slows things down, and
     * we can just lookup the glyph index directly using FreeType’s
     * API.x
     */
    if (inst->hb_feats_count > 0) {
        hb_buffer_add_utf32(inst->hb_buf, &cp, 1, 0, 1);
        hb_buffer_guess_segment_properties(inst->hb_buf);
        hb_shape(inst->hb_font, inst->hb_buf, inst->hb_feats, inst->hb_feats_count);

        unsigned count = hb_buffer_get_length(inst->hb_buf);
        if (count == 1) {
            const hb_glyph_info_t *info = hb_buffer_get_glyph_infos(inst->hb_buf, NULL);
            idx = info[0].codepoint;
        }

        hb_buffer_clear_contents(inst->hb_buf);
    }
#endif

    if (idx == (FT_UInt)-1)
        idx = FT_Get_Char_Index(inst->face, cp);

    bool ret = glyph_for_index(inst, idx, subpixel, options, glyph);
    glyph->public.cp = cp;
    glyph->public.cols = cpwidth(cp);
    return ret;
}

static size_t
hash_index_for_size(size_t size, size_t v)
{
    return (v * 2654435761) & (size - 1);
}

static size_t
glyph_hash_index(const struct font_priv *font, size_t v)
{
    return hash_index_for_size(font->glyph_cache.size, v);
}

static uint32_t
hash_value_for_cp(uint32_t cp, enum fcft_subpixel subpixel)
{
    return subpixel << 29 | cp;
}

static struct glyph_priv **
glyph_cache_lookup(struct font_priv *font, uint32_t cp,
                   enum fcft_subpixel subpixel)
{
    size_t idx = glyph_hash_index(font, hash_value_for_cp(cp, subpixel));
    struct glyph_priv **glyph = &font->glyph_cache.table[idx];

    while (*glyph != NULL && !((*glyph)->public.cp == cp &&
                               (*glyph)->subpixel == subpixel))
    {
        idx = (idx + 1) & (font->glyph_cache.size - 1);
        glyph = &font->glyph_cache.table[idx];

#if defined(_DEBUG)
        glyph_cache_collisions++;
#endif
    }

#if defined(_DEBUG)
    glyph_cache_lookups++;
#endif
    return glyph;
}

static bool
glyph_cache_resize(struct font_priv *font)
{
    if (font->glyph_cache.count * 100 / font->glyph_cache.size < 75)
        return false;

    size_t size = 2 * font->glyph_cache.size;
    assert(__builtin_popcount(size) == 1);

    struct glyph_priv **table = calloc(size, sizeof(table[0]));
    if (table == NULL)
        return false;

    for (size_t i = 0; i < font->glyph_cache.size; i++) {
        struct glyph_priv *entry = font->glyph_cache.table[i];

        if (entry == NULL)
            continue;

        size_t idx = hash_index_for_size(
            size, hash_value_for_cp(entry->public.cp, entry->subpixel));

        while (table[idx] != NULL) {
            assert(!(table[idx]->public.cp == entry->public.cp &&
                     table[idx]->subpixel == entry->subpixel));
            idx = (idx + 1) & (size - 1);
        }

        assert(table[idx] == NULL);
        table[idx] = entry;
    }

    pthread_rwlock_wrlock(&font->glyph_cache_lock);
    {
        free(font->glyph_cache.table);

        LOG_DBG("resized glyph cache from %zu to %zu", font->glyph_cache.size, size);
        font->glyph_cache.table = table;
        font->glyph_cache.size = size;
    }
    pthread_rwlock_unlock(&font->glyph_cache_lock);
    return true;
}

static int
emoji_compare(const void *_key, const void *_emoji)
{
    const uint32_t key = *(const uint32_t *)_key;
    const struct emoji *emoji = _emoji;

    if (key < emoji->cp)
        return -1;
    if (key >= emoji->cp + emoji->count)
        return 1;
    assert(key >= emoji->cp && key < emoji->cp + emoji->count);
    return 0;
}

static const struct emoji *
emoji_lookup(uint32_t cp)
{
    return bsearch(&cp, emojis, ALEN(emojis), sizeof(emojis[0]), &emoji_compare);
}

#if defined(_DEBUG)
static void __attribute__((constructor))
test_emoji_compare(void)
{
#if defined(FCFT_HAVE_HARFBUZZ)
    /* WHITE SMILING FACE */
    const struct emoji *e = emoji_lookup(0x263a);

    assert(e != NULL);
    assert(0x263a >= e->cp);
    assert(0x263a < e->cp + e->count);
    assert(!e->emoji_presentation);

    e = emoji_lookup(L'a');
    assert(e == NULL);
#else
    assert(ALEN(emojis) == 0);
#endif
}
#endif

FCFT_EXPORT const struct fcft_glyph *
fcft_rasterize_char_utf32(struct fcft_font *_font, uint32_t cp,
                          enum fcft_subpixel subpixel)
{
    struct font_priv *font = (struct font_priv *)_font;

    pthread_rwlock_rdlock(&font->glyph_cache_lock);
    struct glyph_priv **entry = glyph_cache_lookup(font, cp, subpixel);

    if (*entry != NULL) {
        const struct glyph_priv *glyph = *entry;
        pthread_rwlock_unlock(&font->glyph_cache_lock);
        return glyph->valid ? &glyph->public : NULL;
    }

    pthread_rwlock_unlock(&font->glyph_cache_lock);
    mtx_lock(&font->lock);

    /* Check again - another thread may have resized the cache, or
     * populated the entry while we acquired the write-lock */
    entry = glyph_cache_lookup(font, cp, subpixel);
    if (*entry != NULL) {
        const struct glyph_priv *glyph = *entry;
        mtx_unlock(&font->lock);
        return glyph->valid ? &glyph->public : NULL;
    }

    if (glyph_cache_resize(font)) {
        /* Entry pointer is invalid if the cache was resized */
        entry = glyph_cache_lookup(font, cp, subpixel);
    }

    struct glyph_priv *glyph = malloc(sizeof(*glyph));
    if (glyph == NULL) {
        mtx_unlock(&font->lock);
        return NULL;
    }

    glyph->public.cp = cp;
    glyph->valid = false;

    const struct emoji *emoji = emoji_lookup(cp);
    assert(emoji == NULL || (cp >= emoji->cp && cp < emoji->cp + emoji->count));

    bool force_text_presentation = false;
    bool force_emoji_presentation = false;
    bool enforce_presentation_style = emoji != NULL;

    if (emoji != NULL && !emoji->component) {
        switch (font->options.emoji_presentation) {
        case FCFT_EMOJI_PRESENTATION_TEXT:
            force_text_presentation = true;
            force_emoji_presentation = false;
            break;

        case FCFT_EMOJI_PRESENTATION_EMOJI:
            force_text_presentation = false;
            force_emoji_presentation = true;
            break;

        case FCFT_EMOJI_PRESENTATION_DEFAULT:
            force_text_presentation = !emoji->emoji_presentation;
            force_emoji_presentation = emoji->emoji_presentation;
            break;
        }
    }

    assert(tll_length(font->fallbacks) > 0);

    bool no_one = true;
    bool got_glyph = false;

search_fonts:

    tll_foreach(font->fallbacks, it) {
        if (!FcCharSetHasChar(it->item.charset, cp))
            continue;

        static const FcChar8 *const lang_emoji = (const FcChar8 *)"und-zsye";

        if (enforce_presentation_style && it->item.langset != NULL) {
            bool has_lang_emoji =
                FcLangSetHasLang(it->item.langset, lang_emoji) == FcLangEqual;

            if (force_text_presentation && has_lang_emoji)
                continue;
            if (force_emoji_presentation && !has_lang_emoji)
                continue;
        }

        if (it->item.font == NULL) {
            struct instance *inst = malloc(sizeof(*inst));
            if (inst == NULL)
                continue;

            if (!instantiate_pattern(
                    it->item.pattern,
                    it->item.req_pt_size, it->item.req_px_size,
                    inst))
            {
                /* Remove, so that we don't have to keep trying to
                 * instantiate it */
                free(inst);
                fallback_destroy(&it->item);
                tll_remove(font->fallbacks, it);
                continue;
            }

            it->item.font = inst;
        }

        assert(it->item.font != NULL);
        got_glyph = glyph_for_codepoint(it->item.font, cp, subpixel, &font->options, glyph);
        no_one = false;
        break;
    }

    if (no_one && enforce_presentation_style) {
        enforce_presentation_style = false;
        goto search_fonts;
    }

    if (no_one) {
        /*
         * No font claimed this glyph - use the primary font anyway.
         */
        assert(tll_length(font->fallbacks) > 0);
        struct instance *inst = tll_front(font->fallbacks).font;

        assert(inst != NULL);
        got_glyph = glyph_for_codepoint(inst, cp, subpixel, &font->options, glyph);
    }

    assert(*entry == NULL);
    *entry = glyph;
    font->glyph_cache.count++;

    mtx_unlock(&font->lock);
    return got_glyph ? &glyph->public : NULL;
}

#if defined(FCFT_HAVE_HARFBUZZ)

static size_t
grapheme_hash_index(const struct font_priv *font, size_t v)
{
    return hash_index_for_size(font->grapheme_cache.size, v);
}

static uint64_t
sdbm_hash_wide(const uint32_t *s, size_t len)
{
    uint64_t hash = 0;

    for (size_t i = 0; i < len; i++, s++)
        hash = (hash << 4) ^ *s;
    return hash;
}

static uint64_t
hash_value_for_grapheme(size_t len, const uint32_t grapheme[static len],
                        enum fcft_subpixel subpixel)
{
    uint64_t hash = sdbm_hash_wide(grapheme, len);
    hash &= (1ull << 29) - 1;
    return subpixel << 29 | hash;
}

static struct grapheme_priv **
grapheme_cache_lookup(struct font_priv *font,
                      size_t len, const uint32_t cluster[static len],
                      enum fcft_subpixel subpixel)
{
    size_t idx = grapheme_hash_index(
        font, hash_value_for_grapheme(len, cluster, subpixel));
    struct grapheme_priv **entry = &font->grapheme_cache.table[idx];

    while (*entry != NULL && !(
               (*entry)->len == len &&
               memcmp((*entry)->cluster, cluster, len * sizeof(cluster[0])) == 0 &&
               (*entry)->subpixel == subpixel))
    {
        idx = (idx + 1) & (font->grapheme_cache.size - 1);
        entry = &font->grapheme_cache.table[idx];

#if defined(_DEBUG)
        grapheme_cache_collisions++;
#endif
    }

#if defined(_DEBUG)
    grapheme_cache_lookups++;
#endif
    return entry;
}

static bool
grapheme_cache_resize(struct font_priv *font)
{
    if (font->grapheme_cache.count * 100 / font->grapheme_cache.size < 75)
        return false;

    size_t size = 2 * font->grapheme_cache.size;
    assert(__builtin_popcount(size) == 1);

    struct grapheme_priv **table = calloc(size, sizeof(table[0]));
    if (table == NULL)
        return false;

    for (size_t i = 0; i < font->grapheme_cache.size; i++) {
        struct grapheme_priv *entry = font->grapheme_cache.table[i];

        if (entry == NULL)
            continue;

        size_t idx = hash_index_for_size(
            size, hash_value_for_grapheme(
                entry->len, entry->cluster, entry->subpixel));

        while (table[idx] != NULL) {
            assert(
                !(table[idx]->len == entry->len &&
                  memcmp(table[idx]->cluster, entry->cluster,
                         entry->len * sizeof(entry->cluster[0])) == 0 &&
                  table[idx]->subpixel == entry->subpixel));
            idx = (idx + 1) & (size - 1);
        }

        assert(table[idx] == NULL);
        table[idx] = entry;
    }

    pthread_rwlock_wrlock(&font->grapheme_cache_lock);
    {
        free(font->grapheme_cache.table);

        LOG_DBG("resized grapheme cache from %zu to %zu (count: %zu)", font->grapheme_cache.size, size, font->grapheme_cache.count);
        font->grapheme_cache.table = table;
        font->grapheme_cache.size = size;
    }
    pthread_rwlock_unlock(&font->grapheme_cache_lock);
    return true;
}

/* Must only be called while font->lock is held */
static bool
font_for_grapheme(struct font_priv *font,
                  size_t len, const uint32_t cluster[static len],
                  struct instance **inst, bool enforce_presentation_style)
{
    static const FcChar8 *const lang_emoji = (const FcChar8 *)"und-zsye";

    tll_foreach(font->fallbacks, it) {
        const bool has_lang_emoji = it->item.langset != NULL &&
            FcLangSetHasLang(it->item.langset, lang_emoji) == FcLangEqual;

        bool has_all_code_points = true;
        for (size_t i = 0; i < len && has_all_code_points; i++) {

            const struct emoji *emoji = emoji_lookup(cluster[i]);
            assert(emoji == NULL || (cluster[i] >= emoji->cp &&
                                     cluster[i] < emoji->cp + emoji->count));

            if (enforce_presentation_style &&
                emoji != NULL && !emoji->component &&
                (i + 1 >= len || (cluster[i + 1] != 0xfe0e &&
                                  cluster[i + 1] != 0xfe0f))) {
                /*
                 * We have an emoji, that is either the last codepoint
                 * in the grapheme, *or* is followed by a codepoint
                 * that is *not* a presentation selector.
                 */
                bool force_text_presentation = false;
                bool force_emoji_presentation = false;

                switch (font->options.emoji_presentation) {
                case FCFT_EMOJI_PRESENTATION_TEXT:
                    force_text_presentation = true;
                    force_emoji_presentation = false;
                    break;

                case FCFT_EMOJI_PRESENTATION_EMOJI:
                    force_text_presentation = false;
                    force_emoji_presentation = true;
                    break;

                case FCFT_EMOJI_PRESENTATION_DEFAULT:
                    force_text_presentation = !emoji->emoji_presentation;
                    force_emoji_presentation = emoji->emoji_presentation;
                    break;
                }

                if (force_text_presentation && has_lang_emoji) {
                    has_all_code_points = false;
                    continue;
                }

                if (force_emoji_presentation && !has_lang_emoji) {
                    has_all_code_points = false;
                    continue;
                }
            }

            if (cluster[i] == 0x200d) {
                /* ZWJ */
                continue;
            }

            if (cluster[i] == 0xfe0f) {
                /* Explicit emoji selector */

#if 0
                /* Require colored emoji? */
                if (!it->item.is_color) {
                    /* Skip font if it is not a colored font */
                    has_all_code_points = false;
                }
#endif
                if (!has_lang_emoji) {
                    /* Skip font if it isn't an emoji font */
                    has_all_code_points = false;
                }

                continue;
            }

            else if (cluster[i] == 0xfe0e) {
                /* Explicit text selector */

                if (has_lang_emoji) {
                    /* Skip font if it is an emoji font */
                    has_all_code_points = false;
                }

                continue;
            }

            if (!FcCharSetHasChar(it->item.charset, cluster[i])) {
                has_all_code_points = false;
                break;
            }
        }

        if (has_all_code_points) {
            if (it->item.font == NULL) {
                *inst = malloc(sizeof(**inst));
                if (*inst == NULL)
                    return false;

                if (!instantiate_pattern(
                        it->item.pattern,
                        it->item.req_pt_size, it->item.req_px_size,
                        *inst))
                {
                    /* Remove, so that we don't have to keep trying to
                     * instantiate it */
                    free(*inst);
                    fallback_destroy(&it->item);
                    tll_remove(font->fallbacks, it);
                    continue;
                }

                it->item.font = *inst;
            } else
                *inst = it->item.font;

            return true;
        }
    }

    if (enforce_presentation_style)
        return font_for_grapheme(font, len, cluster, inst, false);

    /* No font found, use primary font anyway */
    *inst = tll_front(font->fallbacks).font;
    return *inst != NULL;
}

FCFT_EXPORT const struct fcft_grapheme *
fcft_rasterize_grapheme_utf32(struct fcft_font *_font,
                              size_t len, const uint32_t cluster[static len],
                              enum fcft_subpixel subpixel)
{
    struct font_priv *font = (struct font_priv *)_font;
    struct instance *inst = NULL;

    pthread_rwlock_rdlock(&font->grapheme_cache_lock);
    struct grapheme_priv **entry = grapheme_cache_lookup(
        font, len, cluster, subpixel);

    if (*entry != NULL) {
        const struct grapheme_priv *grapheme = *entry;
        pthread_rwlock_unlock(&font->grapheme_cache_lock);
        return grapheme->valid ? &grapheme->public : NULL;
    }

    pthread_rwlock_unlock(&font->grapheme_cache_lock);
    mtx_lock(&font->lock);

    /* Check again - another thread may have resized the cache, or
     * populated the entry while we acquired the write-lock */
    entry = grapheme_cache_lookup(font, len, cluster, subpixel);
    if (*entry != NULL) {
        const struct grapheme_priv *grapheme = *entry;
        mtx_unlock(&font->lock);
        return grapheme->valid ? &grapheme->public : NULL;
    }

    if (grapheme_cache_resize(font)) {
        /* Entry pointer is invalid if the cache was resized */
        entry = grapheme_cache_lookup(font, len, cluster, subpixel);
    }

    struct grapheme_priv *grapheme = malloc(sizeof(*grapheme));
    uint32_t *cluster_copy = malloc(len * sizeof(cluster_copy[0]));
    if (grapheme == NULL || cluster_copy == NULL) {
        /* Can’t update cache entry since we can’t store the cluster */
        free(grapheme);
        free(cluster_copy);
        mtx_unlock(&font->lock);
        return NULL;
    }

    size_t glyph_idx = 0;
    memcpy(cluster_copy, cluster, len * sizeof(cluster[0]));
    grapheme->valid = false;
    grapheme->len = len;
    grapheme->cluster = cluster_copy;
    grapheme->subpixel = subpixel;
    grapheme->public.glyphs = NULL;
    grapheme->public.count = 0;

    /* Find a font that has all codepoints in the grapheme */
    if (!font_for_grapheme(font, len, cluster, &inst, true))
        goto err;

    assert(inst->hb_font != NULL);

    hb_buffer_add_utf32(inst->hb_buf, (const uint32_t *)cluster, len, 0, len);
    hb_buffer_guess_segment_properties(inst->hb_buf);

    hb_shape(inst->hb_font, inst->hb_buf, inst->hb_feats, inst->hb_feats_count);

    unsigned count = hb_buffer_get_length(inst->hb_buf);
    const hb_glyph_info_t *info = hb_buffer_get_glyph_infos(inst->hb_buf, NULL);
    const hb_glyph_position_t *pos = hb_buffer_get_glyph_positions(inst->hb_buf, NULL);

    int grapheme_width = 0;
    int min_grapheme_width = 0;
    for (size_t i = 0; i < len; i++) {
        if (cluster[i] == 0xfe0f)
            min_grapheme_width = 2;
        grapheme_width += cpwidth(cluster[i]);
    }

    LOG_DBG("length: %u", hb_buffer_get_length(inst->hb_buf));
    LOG_DBG("infos: %u", count);

    struct fcft_glyph **glyphs = calloc(count, sizeof(glyphs[0]));
    if (glyphs == NULL)
        goto err;

    grapheme->public.cols = max(grapheme_width, min_grapheme_width);
    grapheme->public.glyphs = (const struct fcft_glyph **)glyphs;

    const unsigned count_from_the_beginning = count;

    for (unsigned i = 0; i < count_from_the_beginning; i++) {
        LOG_DBG("code point: %04x, cluster: %u", info[i].codepoint, info[i].cluster);
        LOG_DBG("x-advance: %d, x-offset: %d, y-advance: %d, y-offset: %d",
                pos[i].x_advance, pos[i].x_offset,
                pos[i].y_advance, pos[i].y_offset);

        struct glyph_priv *glyph = malloc(sizeof(*glyph));
        if (glyph == NULL ||
            !glyph_for_index(inst, info[i].codepoint, subpixel, &font->options, glyph))
        {
            assert(glyph == NULL || !glyph->valid);
            free(glyph);
            goto err;
        }

        assert(glyph->valid);

        assert(info[i].cluster < len);
        glyph->public.cp = cluster[info[i].cluster];
        glyph->public.cols = cpwidth(glyph->public.cp);

#if 0
        LOG_DBG("grapheme: x: advance: %d -> %d, offset: %d -> %d",
                glyph->public.advance.x,
                (int)(pos[i].x_advance / 64. * inst->pixel_size_fixup),
                glyph->public.x,
                (int)(pos[i].x_offset / 64. * inst->pixel_size_fixup));
        LOG_DBG("grapheme: y: advance: %d -> %d, offset: %d -> %d",
                glyph->public.advance.y,
                (int)(pos[i].y_advance / 64. * inst->pixel_size_fixup),
                glyph->public.y,
                (int)(pos[i].y_offset / 64. * inst->pixel_size_fixup));
#endif

        glyph->public.x += pos[i].x_offset / 64. * inst->pixel_size_fixup;
        glyph->public.y += pos[i].y_offset / 64. * inst->pixel_size_fixup;
        glyph->public.advance.x = pos[i].x_advance / 64. * inst->pixel_size_fixup;
        glyph->public.advance.y = pos[i].y_advance / 64. * inst->pixel_size_fixup;

        grapheme->public.glyphs[glyph_idx++] = &glyph->public;
    }

#if defined(_DEBUG)
    assert(glyph_idx == count);
    for (size_t i = 0; i < count; i++) {
        const struct glyph_priv *g
            = (const struct glyph_priv *)grapheme->public.glyphs[i];

        assert(g != NULL);
        assert(g->valid);
    }
#endif

    hb_buffer_clear_contents(inst->hb_buf);

    assert(*entry == NULL);
    grapheme->public.count = glyph_idx;
    grapheme->valid = true;
    *entry = grapheme;
    font->grapheme_cache.count++;

    mtx_unlock(&font->lock);
    return &grapheme->public;

err:
    hb_buffer_clear_contents(inst->hb_buf);
    for (size_t i = 0; i < glyph_idx; i++)
        glyph_destroy(grapheme->public.glyphs[i]);
    free(grapheme->public.glyphs);

    assert(*entry == NULL);
    assert(!grapheme->valid);
    grapheme->public.count = 0;
    grapheme->public.glyphs = NULL;
    *entry = grapheme;
    font->grapheme_cache.count++;
    mtx_unlock(&font->lock);
    return NULL;
}
#else /* !FCFT_HAVE_HARFBUZZ */

FCFT_EXPORT const struct fcft_grapheme *
fcft_rasterize_grapheme_utf32(struct fcft_font *_font,
                              size_t len, const uint32_t cluster[static len],
                              enum fcft_subpixel subpixel)
{
    return NULL;
}

#endif

#if defined(FCFT_HAVE_HARFBUZZ) && defined(FCFT_HAVE_UTF8PROC)
struct text_run {
    struct fcft_text_run *public;
    size_t size;
};

static bool
rasterize_partial_run(struct text_run *run, const struct instance *inst,
                      const uint32_t *text, size_t len,
                      size_t run_start, size_t run_len,
                      enum fcft_subpixel subpixel,
                      const struct fcft_font_options *options)
{
    hb_buffer_add_utf32(inst->hb_buf, text, len, run_start, run_len);
    hb_buffer_guess_segment_properties(inst->hb_buf);

    hb_segment_properties_t props;
    hb_buffer_get_segment_properties(inst->hb_buf, &props);

    assert(props.direction == HB_DIRECTION_LTR ||
           props.direction == HB_DIRECTION_RTL);

    if (props.direction != HB_DIRECTION_LTR &&
        props.direction != HB_DIRECTION_RTL)
    {
        LOG_ERR("unimplemented: hb_direction=%d", props.direction);
        return false;
    }

    hb_shape(inst->hb_font, inst->hb_buf, inst->hb_feats, inst->hb_feats_count);

    unsigned count = hb_buffer_get_length(inst->hb_buf);
    const hb_glyph_info_t *infos = hb_buffer_get_glyph_infos(inst->hb_buf, NULL);
    const hb_glyph_position_t *poss = hb_buffer_get_glyph_positions(inst->hb_buf, NULL);

    for (int i = 0; i < count; i++) {
        const hb_glyph_info_t *info = &infos[i];
        const hb_glyph_position_t *pos = &poss[i];

        LOG_DBG("#%u: codepoint=%04x, cluster=%d", i, info->codepoint, info->cluster);

        struct glyph_priv *glyph = malloc(sizeof(*glyph));
        if (glyph == NULL)
            return false;

        if (!glyph_for_index(inst, info->codepoint, subpixel, options, glyph)) {
            free(glyph);
            continue;
        }

        assert(info->cluster < len);
        glyph->public.cp = text[info->cluster];
        glyph->public.cols = cpwidth(glyph->public.cp);

        /* TODO: can’t reference font data, since the font may be
         * free:d before the text-run (and thus all the text-run’s
         * glyphs) */
        glyph->public.font_name = NULL;
        glyph->public.x += pos->x_offset / 64. * inst->pixel_size_fixup;
        glyph->public.y += pos->y_offset / 64. * inst->pixel_size_fixup;
        glyph->public.advance.x = pos->x_advance / 64. * inst->pixel_size_fixup;
        glyph->public.advance.y = pos->y_advance / 64. * inst->pixel_size_fixup;

        if (run->public->count >= run->size) {
            size_t new_glyphs_size = run->size * 2;
            const struct fcft_glyph **new_glyphs = realloc(
                run->public->glyphs, new_glyphs_size * sizeof(new_glyphs[0]));
            int *new_cluster = realloc(
                run->public->cluster, new_glyphs_size * sizeof(new_cluster[0]));

            if (new_glyphs == NULL || new_cluster == NULL) {
                free(new_glyphs);
                free(new_cluster);
                return false;
            }

            run->public->glyphs = new_glyphs;
            run->public->cluster = new_cluster;
            run->size = new_glyphs_size;
        }

        assert(run->public->count < run->size);
        run->public->cluster[run->public->count] = info->cluster;
        run->public->glyphs[run->public->count] = &glyph->public;
        run->public->count++;
    }

    return true;
}

FCFT_EXPORT struct fcft_text_run *
fcft_rasterize_text_run_utf32(
    struct fcft_font *_font, size_t len, const uint32_t text[static len],
    enum fcft_subpixel subpixel)
{
    struct font_priv *font = (struct font_priv *)_font;
    mtx_lock(&font->lock);

    LOG_DBG("rasterizing a %zu character text run", len);

    struct partial_run {
        size_t start;
        size_t len;
        struct instance *inst;
    };

    tll(struct partial_run) pruns = tll_init();

    struct text_run run = {
        .size = len,
        .public = malloc(sizeof(*run.public)),
    };

    if (run.public == NULL)
        goto err;

    run.public->glyphs = malloc(len * sizeof(run.public->glyphs[0]));
    run.public->cluster = malloc(len * sizeof(run.public->cluster[0]));
    run.public->count = 0;

    if (run.public->glyphs == NULL || run.public->cluster == NULL)
        goto err;


    tll_push_back(pruns, ((struct partial_run){.start = 0}));

    /* Split run into graphemes */
    utf8proc_int32_t state = 0;
    for (size_t i = 1; i < len; i++) {
        if (utf8proc_grapheme_break_stateful(text[i - 1], text[i], &state)) {
            state = 0;

            struct partial_run *prun = &tll_back(pruns);

            assert(i > prun->start);
            prun->len = i - prun->start;

            if (!font_for_grapheme(
                    font, prun->len, &text[prun->start], &prun->inst, true))
            {
                goto err;
            }

            tll_push_back(pruns, ((struct partial_run){.start = i}));
        }
    }

    /* “Close” that last run */
    {
        struct partial_run *prun = &tll_back(pruns);

        assert(prun->len == 0);
        prun->len = len - prun->start;

        if (!font_for_grapheme(
                font, prun->len, &text[prun->start], &prun->inst, true))
        {
            goto err;
        }
    }

#if defined(_DEBUG) && LOG_ENABLE_DBG
    LOG_DBG("%zu partial runs (before merge):", tll_length(pruns));
    tll_foreach(pruns, it) {
        const struct partial_run *prun = &it->item;
        LOG_DBG("  %.*ls (start=%zu, %zu chars), inst=%p",
                (int)prun->len, &text[prun->start], prun->start,
                prun->len, prun->inst);
    }
#endif

    /*
     * Merge consecutive graphemes if:
     *  - they belong to the same script (“language”)
     *  - they have the same font instance
     */
    {
        hb_buffer_t *hb_buf = hb_buffer_create();
        if (hb_buf == NULL)
            goto err;

        struct partial_run *prev = NULL;
        hb_script_t prev_script = HB_SCRIPT_INVALID;

        tll_foreach(pruns, it) {
            struct partial_run *prun = &it->item;

            /* Get script (“language”) of grapheme */
            hb_buffer_add_utf32(
                hb_buf, (const uint32_t *)text, len, prun->start, prun->len);
            hb_buffer_guess_segment_properties(hb_buf);

            hb_script_t script = hb_buffer_get_script(hb_buf);
            hb_buffer_clear_contents(hb_buf);

            if (prev == NULL) {
                prev = prun;
                prev_script = script;
                continue;
            }

            if (prev->inst == prun->inst && prev_script == script) {
                prev->len += prun->len;
                tll_remove(pruns, it);
            } else {
                prev = prun;
                prev_script = script;
            }
        }

        hb_buffer_destroy(hb_buf);
    }

#if defined(_DEBUG) && LOG_ENABLE_DBG
    LOG_DBG("%zu partial runs (after merge):", tll_length(pruns));
    tll_foreach(pruns, it) {
        const struct partial_run *prun = &it->item;
        LOG_DBG("  %.*ls (start=%zu, %zu chars), inst=%p",
                (int)prun->len, &text[prun->start], prun->start,
                prun->len, prun->inst);
    }
#endif

    /* Shape each partial run */
    tll_foreach(pruns, it) {
        const struct partial_run *prun = &it->item;

        bool ret = rasterize_partial_run(
            &run, prun->inst, (const uint32_t *)text, len,
            prun->start, prun->len, subpixel, &font->options);

        hb_buffer_clear_contents(prun->inst->hb_buf);
        if (!ret)
            goto err;
    }

    /* Re-alloc glyphs/cluster arrays */
    if (run.public->count > 0) {
        const struct fcft_glyph **final_glyphs = realloc(
            run.public->glyphs, run.public->count * sizeof(final_glyphs[0]));
        int *final_cluster = realloc(
            run.public->cluster, run.public->count * sizeof(final_cluster[0]));

        if ((final_glyphs == NULL || final_cluster == NULL) &&
            run.public->count > 0)
        {
            free(final_glyphs);
            free(final_cluster);
            goto err;
        }

        run.public->glyphs = final_glyphs;
        run.public->cluster = final_cluster;
    } else {
        free(run.public->glyphs);
        free(run.public->cluster);

        run.public->glyphs = NULL;
        run.public->cluster = NULL;
    }

    LOG_DBG("glyph count: %zu", run.public->count);

    tll_free(pruns);
    mtx_unlock(&font->lock);
    return run.public;

err:

    if (run.public != NULL) {
        for (size_t i = 0; i < run.public->count; i++) {
            assert(run.public->glyphs[i] != NULL);
            glyph_destroy(run.public->glyphs[i]);
        }

        free(run.public->glyphs);
        free(run.public->cluster);
        free(run.public);
    }

    tll_free(pruns);
    mtx_unlock(&font->lock);
    return NULL;
}

#else /* !FCFT_HAVE_HARFBUZZ || !FCFT_HAVE_UTF8PROC */

FCFT_EXPORT struct fcft_text_run *
fcft_rasterize_text_run_utf32(
    struct fcft_font *font, size_t len, const uint32_t text[static len],
    enum fcft_subpixel subpixel)
{
    return NULL;
}

#endif /* !FCFT_HAVE_HARFBUZZ */

FCFT_EXPORT void
fcft_text_run_destroy(struct fcft_text_run *run)
{
    if (run == NULL)
        return;

    for (size_t i = 0; i < run->count; i++) {
        assert(run->glyphs[i] != NULL);
        glyph_destroy(run->glyphs[i]);
    }

    free(run->glyphs);
    free(run->cluster);
    free(run);
}

FCFT_EXPORT void
fcft_destroy(struct fcft_font *_font)
{
    if (_font == NULL)
        return;

    struct font_priv *font = (struct font_priv *)_font;

    bool in_cache = false;
    mtx_lock(&font_cache_lock);
    tll_foreach(font_cache, it) {
        if (it->item.font == font) {

            in_cache = true;

            mtx_lock(&font->lock);
            if (--font->ref_counter > 0) {
                mtx_unlock(&font->lock);
                mtx_unlock(&font_cache_lock);
                return;
            }
            mtx_unlock(&font->lock);

            cnd_destroy(&it->item.cond);

            for (size_t i = 0; i < it->item.name_count; i++)
                free(it->item.names[i]);
            free(it->item.names);
            free(it->item.attributes);

            tll_remove(font_cache, it);
            break;
        }
    };
    mtx_unlock(&font_cache_lock);

    if (!in_cache) {
        mtx_lock(&font->lock);
        if (--font->ref_counter > 0) {
            mtx_unlock(&font->lock);
            return;
        }
        mtx_unlock(&font->lock);
    }

    tll_foreach(font->fallbacks, it)
        fallback_destroy(&it->item);

    tll_free(font->fallbacks);
    mtx_destroy(&font->lock);

    for (size_t i = 0;
         i < font->glyph_cache.size && font->glyph_cache.table != NULL;
         i++)
    {
        struct glyph_priv *entry = font->glyph_cache.table[i];

        if (entry == NULL)
            continue;

        glyph_destroy_private(entry);
    }
    free(font->glyph_cache.table);
    pthread_rwlock_destroy(&font->glyph_cache_lock);

#if defined(FCFT_HAVE_HARFBUZZ)
    for (size_t i = 0;
         i < font->grapheme_cache.size && font->grapheme_cache.table != NULL;
         i++)
    {
        struct grapheme_priv *entry = font->grapheme_cache.table[i];

        if (entry == NULL)
            continue;

        for (size_t j = 0; j < entry->public.count; j++) {
            assert(entry->public.glyphs[j] != NULL);
            glyph_destroy(entry->public.glyphs[j]);
        }

        free(entry->public.glyphs);
        free(entry->cluster);
        free(entry);
    }
    free(font->grapheme_cache.table);
    pthread_rwlock_destroy(&font->grapheme_cache_lock);
#endif

    free(font);
}

FCFT_EXPORT bool
fcft_kerning(struct fcft_font *_font, uint32_t left, uint32_t right,
             long *restrict x, long *restrict y)
{
    struct font_priv *font = (struct font_priv *)_font;

    if (x != NULL)
        *x = 0;
    if (y != NULL)
        *y = 0;

    mtx_lock(&font->lock);

    assert(tll_length(font->fallbacks) > 0);
    const struct instance *primary = tll_front(font->fallbacks).font;

    if (!FT_HAS_KERNING(primary->face))
        goto err;

    FT_UInt left_idx = FT_Get_Char_Index(primary->face, left);
    if (left_idx == 0)
        goto err;

    FT_UInt right_idx = FT_Get_Char_Index(primary->face, right);
    if (right_idx == 0)
        goto err;

    FT_Vector kerning;
    FT_Error err = FT_Get_Kerning(
        primary->face, left_idx, right_idx, FT_KERNING_DEFAULT, &kerning);

    if (err != FT_Err_Ok) {
        LOG_WARN("%s: failed to get kerning for %lc -> %lc: %s",
                 primary->path, (int)left, (int)right, ft_error_string(err));
        goto err;
    }

    if (x != NULL)
        *x = kerning.x / 64. * primary->pixel_size_fixup;
    if (y != NULL)
        *y = kerning.y / 64. * primary->pixel_size_fixup;

    LOG_DBG("%s: kerning: %lc -> %lc: x=%ld 26.6, y=%ld 26.6",
            primary->path, (int)left, (int)right,
            kerning.x, kerning.y);

    mtx_unlock(&font->lock);
    return true;

err:
    mtx_unlock(&font->lock);
    return false;
}

#if defined(_DEBUG)
static void __attribute__((constructor))
verify_precompose_table_is_sorted(void)
{
    uint32_t last_base = 0, last_comb = 0;

    for (size_t i = 0; i < ALEN(precompose_table); i++) {
        uint32_t base = precompose_table[i].base;
        uint32_t comb = precompose_table[i].comb;

        assert(base >= last_base);
        if (base != last_base)
            last_comb = 0;

        assert(comb >= last_comb);

        last_base = base;
        last_comb = comb;
    }
}
#endif /* _DEBUG */

FCFT_EXPORT uint32_t
fcft_precompose(const struct fcft_font *_font, uint32_t base, uint32_t comb,
                bool *base_is_from_primary,
                bool *comb_is_from_primary,
                bool *composed_is_from_primary)
{
    _Static_assert(2 * sizeof(uint32_t) <= sizeof(uint64_t),
                  "two wchars does not fit in an uint64_t");

    const struct font_priv *font = (const struct font_priv *)_font;

    assert(tll_length(font->fallbacks) > 0);
    const struct fallback *primary = &tll_front(font->fallbacks);

    if (font != NULL) {
        if (base_is_from_primary != NULL)
            *base_is_from_primary = FcCharSetHasChar(primary->charset, base);
        if (comb_is_from_primary != NULL)
            *comb_is_from_primary = FcCharSetHasChar(primary->charset, comb);
    }

    const uint64_t match = (uint64_t)base << 32 | comb;

    ssize_t start = 0;
    ssize_t end = (sizeof(precompose_table) / sizeof(precompose_table[0])) - 1;

    while (start <= end) {
        size_t middle = (start + end) / 2;

        const uint64_t maybe =
            (uint64_t)precompose_table[middle].base << 32 | precompose_table[middle].comb;

        if (maybe < match)
            start = middle + 1;
        else if (maybe > match)
            end = middle - 1;
        else {
            uint32_t composed = precompose_table[middle].replacement;
            if (font != NULL && composed_is_from_primary != NULL) {
                *composed_is_from_primary = FcCharSetHasChar(
                    primary->charset, composed);
            }
            return composed;
        }
    }

    if (composed_is_from_primary != NULL)
        *composed_is_from_primary = false;
    return (uint32_t)-1;
}

FCFT_EXPORT void
fcft_set_emoji_presentation(struct fcft_font *_font,
                            enum fcft_emoji_presentation presentation)
{
    struct font_priv *font = (struct font_priv *)_font;
    font->options.emoji_presentation = presentation;
}

static int
cpwidth(uint32_t cp)
{
#if defined(FCFT_HAVE_UTF8PROC)
    return utf8proc_charwidth((utf8proc_int32_t)cp);
#else
    return wcwidth((wchar_t)cp);
#endif
}
