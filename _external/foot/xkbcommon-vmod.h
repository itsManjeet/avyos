#pragma once

#include <xkbcommon/xkbcommon-names.h>

/* Added in libxkbcommon 1.8.0 */
#if !defined(XKB_VMOD_NAME_ALT)
/* Common *virtual* modifiers, encoded in xkeyboard-config in the compat and
 * symbols files. They have been stable since the beginning of the project and
 * are unlikely to ever change. */
#define XKB_VMOD_NAME_ALT       "Alt"
#define XKB_VMOD_NAME_HYPER     "Hyper"
#define XKB_VMOD_NAME_LEVEL3    "LevelThree"
#define XKB_VMOD_NAME_LEVEL5    "LevelFive"
#define XKB_VMOD_NAME_META      "Meta"
#define XKB_VMOD_NAME_NUM       "NumLock"
#define XKB_VMOD_NAME_SCROLL    "ScrollLock"
#define XKB_VMOD_NAME_SUPER     "Super"
#endif
