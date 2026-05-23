#pragma once
#include <stdint.h>

/* ─── Event types ─────────────────────────────────────────── */
#define GEVT_NONE      0
#define GEVT_KEYDOWN   1
#define GEVT_KEYUP     2
#define GEVT_RUNE      3
#define GEVT_MOUSEDOWN 4
#define GEVT_MOUSEUP   5
#define GEVT_MOUSEMOVE 6
#define GEVT_SCROLL    7
#define GEVT_RESIZE    8
#define GEVT_CLOSE     9
#define GEVT_FOCUS     10
#define GEVT_BLUR      11

/* Modifier flags (same bit positions as event.Modifiers) */
#define GMOD_SHIFT  1
#define GMOD_CTRL   2
#define GMOD_ALT    4
#define GMOD_SUPER  8

/* ─── Key code values ─────────────────────────────────────────────────────────
 * These MUST stay in sync with the event.KeyCode iota in event/event.go.
 * KeyUnknown = 0, KeyA = 1 … KeyZ = 26, Key0 = 27 … Key9 = 36,
 * arrows = 37-40, home/end/pgup/pgdn = 41-44,
 * backspace=45 delete=46 insert=47 tab=48 enter=49 escape=50 space=51,
 * F1-F12 = 52-63, shift=64 ctrl=65 alt=66 super=67,
 * minus=68 equal=69 [ =70 ] =71 ; =72 '=73 `=74 \=75 ,=76 .=77 /=78
 * ──────────────────────────────────────────────────────────────────────────── */
#define GKEY_UNKNOWN        0

#define GKEY_A              1
#define GKEY_B              2
#define GKEY_C              3
#define GKEY_D              4
#define GKEY_E              5
#define GKEY_F              6
#define GKEY_G              7
#define GKEY_H              8
#define GKEY_I              9
#define GKEY_J              10
#define GKEY_K              11
#define GKEY_L              12
#define GKEY_M              13
#define GKEY_N              14
#define GKEY_O              15
#define GKEY_P              16
#define GKEY_Q              17
#define GKEY_R              18
#define GKEY_S              19
#define GKEY_T              20
#define GKEY_U              21
#define GKEY_V              22
#define GKEY_W              23
#define GKEY_X              24
#define GKEY_Y              25
#define GKEY_Z              26

#define GKEY_0              27
#define GKEY_1              28
#define GKEY_2              29
#define GKEY_3              30
#define GKEY_4              31
#define GKEY_5              32
#define GKEY_6              33
#define GKEY_7              34
#define GKEY_8              35
#define GKEY_9              36

#define GKEY_ARROW_UP       37
#define GKEY_ARROW_DOWN     38
#define GKEY_ARROW_LEFT     39
#define GKEY_ARROW_RIGHT    40
#define GKEY_HOME           41
#define GKEY_END            42
#define GKEY_PAGE_UP        43
#define GKEY_PAGE_DOWN      44

#define GKEY_BACKSPACE      45
#define GKEY_DELETE         46
#define GKEY_INSERT         47
#define GKEY_TAB            48
#define GKEY_ENTER          49
#define GKEY_ESCAPE         50
#define GKEY_SPACE          51

#define GKEY_F1             52
#define GKEY_F2             53
#define GKEY_F3             54
#define GKEY_F4             55
#define GKEY_F5             56
#define GKEY_F6             57
#define GKEY_F7             58
#define GKEY_F8             59
#define GKEY_F9             60
#define GKEY_F10            61
#define GKEY_F11            62
#define GKEY_F12            63

#define GKEY_SHIFT          64
#define GKEY_CTRL           65
#define GKEY_ALT            66
#define GKEY_SUPER          67

#define GKEY_MINUS          68
#define GKEY_EQUAL          69
#define GKEY_LEFT_BRACKET   70
#define GKEY_RIGHT_BRACKET  71
#define GKEY_SEMICOLON      72
#define GKEY_APOSTROPHE     73
#define GKEY_GRAVE          74
#define GKEY_BACKSLASH      75
#define GKEY_COMMA          76
#define GKEY_PERIOD         77
#define GKEY_SLASH          78

typedef struct GEvent {
    int          type;
    int          keyCode;    /* GKEY_* value matching event.KeyCode */
    unsigned int rune;       /* Unicode code point for GEVT_RUNE */
    float        x, y;      /* mouse position in PHYSICAL pixels, Y-down */
    float        dx, dy;    /* scroll delta (logical points) */
    int          width, height; /* for GEVT_RESIZE (physical pixels) */
    int          button;    /* 1=left 2=middle 3=right */
    int          mods;      /* GMOD_* bitmask */
} GEvent;

/* ─── Lifecycle ───────────────────────────────────────────── */
void gfx_init(void);
uintptr_t gfx_create_window(int width, int height, const char *title);
void gfx_destroy_window(uintptr_t handle);

/* ─── Rendering ───────────────────────────────────────────── */
void gfx_present(uintptr_t handle, const unsigned char *rgba, int width, int height);

/* ─── Scale ───────────────────────────────────────────────── */
float gfx_scale(uintptr_t handle);
float gfx_main_screen_scale(void);

/* ─── Events ──────────────────────────────────────────────── */
int gfx_poll_event(uintptr_t handle, GEvent *out);
