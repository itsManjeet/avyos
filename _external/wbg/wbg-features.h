#pragma once
#include <stdbool.h>

static inline bool feature_png(void)
{
#if defined(WBG_HAVE_PNG) && WBG_HAVE_PNG
    return true;
#else
    return false;
#endif
}

static inline bool feature_jpg(void)
{
#if defined(WBG_HAVE_JPG) && WBG_HAVE_JPG
    return true;
#else
    return false;
#endif
}

static inline bool feature_webp(void)
{
#if defined(WBG_HAVE_WEBP) && WBG_HAVE_WEBP
    return true;
#else
    return false;
#endif
}

static inline bool feature_svg(void)
{
#if defined(WBG_HAVE_SVG) && WBG_HAVE_SVG
    return true;
#else
    return false;
#endif
}

static inline bool feature_jxl(void)
{
#if defined(WBG_HAVE_JXL) && WBG_HAVE_JXL
    return true;
#else
    return false;
#endif
}
