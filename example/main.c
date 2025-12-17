#include <stdlib.h>
#include <stdio.h>
#include <stdbool.h>
#include <string.h>
#include <ctype.h>
#include <locale.h>
#include <poll.h>
#include <signal.h>
#include <uchar.h>
#include <assert.h>
#include <errno.h>
#include <unistd.h>
#include <getopt.h>

#include <utf8proc.h>

#include <wayland-client.h>
#include <xdg-shell.h>
#include <xdg-decoration-unstable-v1.h>

#include <tllist.h>
#include <fcft/fcft.h>

#include "shm.h"

#define ALEN(v) (sizeof(v) / sizeof((v)[0]))

#if !defined(__STDC_UTF_32__) || !__STDC_UTF_32__
 #error "char32_t does not use UTF-32"
#endif

static struct wl_display *display;
static struct wl_registry *registry;
static struct wl_compositor *compositor;
static struct wl_shm *shm;
static struct wl_surface *surf;
static struct xdg_wm_base *shell;
static struct xdg_surface *xdg;
static struct xdg_toplevel *toplevel;
static struct zxdg_decoration_manager_v1 *deco_mgr;
static struct zxdg_toplevel_decoration_v1 *deco;

static bool have_argb8888 = false;

static char32_t *text;
static size_t text_len;

struct grapheme {
    size_t begin;
    size_t len;
};
static struct grapheme *graphemes;
static size_t grapheme_count = 0;

static struct fcft_font *font = NULL;
static enum fcft_subpixel subpixel_mode = FCFT_SUBPIXEL_DEFAULT;

static pixman_color_t fg = {0x0000, 0x0000, 0x0000, 0xffff};
static pixman_color_t bg = {0xffff, 0xffff, 0xffff, 0xffff};
static enum fcft_scaling_filter scaling_filter = FCFT_SCALING_FILTER_CUBIC;
static enum fcft_emoji_presentation emoji_presentation = FCFT_EMOJI_PRESENTATION_DEFAULT;

static int width;
static int height;

static volatile sig_atomic_t aborted = false;

static void
sig_handler(int signo)
{
    aborted = true;
}

static void
shm_format(void *data, struct wl_shm *wl_shm, uint32_t format)
{
    if (format == WL_SHM_FORMAT_ARGB8888)
        have_argb8888 = true;
}

static const struct wl_shm_listener shm_listener = {
    .format = &shm_format,
};

static void
xdg_wm_base_ping(void *data, struct xdg_wm_base *shell, uint32_t serial)
{
    xdg_wm_base_pong(shell, serial);
}

static const struct xdg_wm_base_listener xdg_wm_base_listener = {
    .ping = &xdg_wm_base_ping,
};

static void
xdg_toplevel_configure(void *data, struct xdg_toplevel *xdg_toplevel,
                       int32_t _width, int32_t _height, struct wl_array *states)
{
    width = _width;
    height = _height;
}

static void
xdg_toplevel_close(void *data, struct xdg_toplevel *xdg_toplevel)
{
    aborted = true;
}

static const struct xdg_toplevel_listener xdg_toplevel_listener = {
    .configure = &xdg_toplevel_configure,
    .close = &xdg_toplevel_close,
};

static void
render_glyphs(struct buffer *buf, int *x, const int *y, pixman_image_t *color,
              size_t count, const struct fcft_glyph *glyphs[static count],
              long *kern)
{
    for (size_t i = 0; i < count; i++) {
        const struct fcft_glyph *g = glyphs[i];
        if (g == NULL)
            continue;

        if (kern != NULL)
            *x += kern[i];

        if (g->is_color_glyph) {
            pixman_image_composite32(
                PIXMAN_OP_OVER, g->pix, NULL, buf->pix, 0, 0, 0, 0,
                *x + g->x, *y + font->ascent - g->y, g->width, g->height);
        } else {
            pixman_image_composite32(
                PIXMAN_OP_OVER, color, g->pix, buf->pix, 0, 0, 0, 0,
                *x + g->x, *y + font->ascent - g->y, g->width, g->height);
        }

        *x += g->advance.x;
    }
}

static void
render_chars(const char32_t *text, size_t text_len,
             struct buffer *buf, int y, pixman_image_t *color)
{
    const struct fcft_glyph *glyphs[text_len];
    long kern[text_len];
    int text_width = 0;

    for (size_t i = 0; i < text_len; i++) {
        glyphs[i] = fcft_rasterize_char_utf32(font, text[i], subpixel_mode);
        if (glyphs[i] == NULL)
            continue;

        kern[i] = 0;
        if (i > 0) {
            long x_kern;
            if (fcft_kerning(font, text[i - 1], text[i], &x_kern, NULL))
                kern[i] = x_kern;
        }

        text_width += kern[i] + glyphs[i]->advance.x;
    }

    int x = (buf->width - text_width) / 2;
    render_glyphs(buf, &x, &y, color, text_len, glyphs, kern);
}

static void
render_graphemes(struct buffer *buf, int y, pixman_image_t *color)
{
    if (!(fcft_capabilities() & FCFT_CAPABILITY_GRAPHEME_SHAPING)) {
        static const char32_t unsupported[] =
            U"fcft compiled without grapheme shaping support";
        render_chars(unsupported, ALEN(unsupported) - 1, buf, y, color);
        return;
    }

    const struct fcft_grapheme *graphs[grapheme_count];
    int text_width = 0;

    for (size_t i = 0; i < grapheme_count; i++) {
        graphs[i] = fcft_rasterize_grapheme_utf32(
            font, graphemes[i].len, &text[graphemes[i].begin], subpixel_mode);

        if (graphs[i] == NULL)
            continue;

        for (size_t j = 0; j < graphs[i]->count; j++)
            text_width += graphs[i]->glyphs[j]->advance.x;
    }

    int x = (buf->width - text_width) / 2;

    for (size_t i = 0; i < grapheme_count; i++) {
        if (graphs[i] == NULL)
            continue;

        render_glyphs(
            buf, &x, &y, color, graphs[i]->count, graphs[i]->glyphs, NULL);
    }
}

static void
render_shaped(struct buffer *buf, int y, pixman_image_t *color)
{
    if (!(fcft_capabilities() & FCFT_CAPABILITY_TEXT_RUN_SHAPING)) {
        static const char32_t unsupported[] =
            U"fcft compiled without text-run shaping support";
        render_chars(unsupported, ALEN(unsupported) - 1, buf, y, color);
        return;
    }

    struct fcft_text_run *run = fcft_rasterize_text_run_utf32(
        font, text_len, text, subpixel_mode);

    if (run == NULL)
        return;

    int text_width = 0;
    for (size_t i = 0; i < run->count; i++)
        text_width += run->glyphs[i]->advance.x;
    int x = (buf->width - text_width) / 2;
    render_glyphs(buf, &x, &y, color, run->count, run->glyphs, NULL);
    fcft_text_run_destroy(run);
}

static void
xdg_surface_configure(void *data, struct xdg_surface *xdg_surface,
                      uint32_t serial)
{
    xdg_surface_ack_configure(xdg, serial);

    static int last_width = -1;
    static int last_height = -1;

    if (last_width == width && last_height == height) {
        wl_surface_commit(surf);
        return;
    }

    int w = width == 0 ? 100 : width;
    int h = height == 0 ? 100 : height;

    last_width = w;
    last_height = h;

    struct buffer *buf = shm_get_buffer(shm, w, h, 0xdeadbeef);
    assert(buf != NULL);

    /*
     * Set clip region to the entire surface size. This allows us to
     * render without considering wether things are outside the buffer
     * or not.
     */

    pixman_region32_t clip;
    pixman_region32_init_rect(&clip, 0, 0, buf->width, buf->height);
    pixman_image_set_clip_region32(buf->pix, &clip);
    pixman_region32_fini(&clip);

    /* Background */
    pixman_image_fill_rectangles(
        PIXMAN_OP_SRC, buf->pix, &bg, 1,
        (pixman_rectangle16_t []){{0, 0, w, h}});

    /* Source “image” (foreground color) to composite with */
    pixman_image_t *clr_pix = pixman_image_create_solid_fill(&fg);

    /* Center 2 lines, using a 1.5x line height */
    int y = (h - 2 * (3 * font->height / 2)) / 2;
    render_chars(text, text_len, buf, y, clr_pix);

    /* 1.5x line height */
    y += 3 * font->height / 2;
    render_graphemes(buf, y, clr_pix);

    /* 1.5x line height */
    y += 3 * font->height / 2;
    render_shaped(buf, y, clr_pix);

    pixman_image_unref(clr_pix);

    wl_surface_attach(surf, buf->wl_buf, 0, 0);
    wl_surface_damage_buffer(surf, 0, 0, w, h);
    wl_surface_commit(surf);
    shm_buffer_destroy(buf);
}

static const struct xdg_surface_listener xdg_surface_listener = {
    .configure = &xdg_surface_configure,
};

static void
xdg_toplevel_decoration_configure(
    void *data,
    struct zxdg_toplevel_decoration_v1 *zxdg_toplevel_decoration_v1,
    uint32_t mode)
{
    switch (mode) {
    case ZXDG_TOPLEVEL_DECORATION_V1_MODE_CLIENT_SIDE:
        fprintf(
            stderr,
            "warning: compositor refuses to use server side decorations\n");
        break;
    }
}

static const struct zxdg_toplevel_decoration_v1_listener xdg_toplevel_decoration_listener = {
    .configure = &xdg_toplevel_decoration_configure,
};

static bool
verify_iface_version(const char *iface, uint32_t version, uint32_t wanted)
{
    if (version >= wanted)
        return true;

    fprintf(
        stderr,
        "error: %s: "
        "need interface version %u, but compositor only implements %u\n",
        iface, wanted, version);
    return false;
}

static void
handle_global(void *data, struct wl_registry *registry,
              uint32_t name, const char *interface, uint32_t version)
{
    if (strcmp(interface, wl_compositor_interface.name) == 0) {
        const uint32_t required = 4;
        if (!verify_iface_version(interface, version, required))
            return;

        compositor = wl_registry_bind(
            registry, name, &wl_compositor_interface, required);
    }

    else if (strcmp(interface, wl_shm_interface.name) == 0) {
        const uint32_t required = 1;
        if (!verify_iface_version(interface, version, required))
            return;

        shm = wl_registry_bind(
            registry, name, &wl_shm_interface, required);
        wl_shm_add_listener(shm, &shm_listener, NULL);
    }

    else if (strcmp(interface, xdg_wm_base_interface.name) == 0) {
        const uint32_t required = 1;
        if (!verify_iface_version(interface, version, required))
            return;

        shell = wl_registry_bind(
            registry, name, &xdg_wm_base_interface, required);
        xdg_wm_base_add_listener(shell, &xdg_wm_base_listener, NULL);
    }

    else if (strcmp(interface, zxdg_decoration_manager_v1_interface.name) == 0) {
        const uint32_t required = 1;
        if (!verify_iface_version(interface, version, required))
            return;

        deco_mgr = wl_registry_bind(
            registry, name, &zxdg_decoration_manager_v1_interface, required);
    }
}

static void
handle_global_remove(void *data, struct wl_registry *registry, uint32_t name)
{
}

static const struct wl_registry_listener registry_listener = {
    .global = &handle_global,
    .global_remove = &handle_global_remove,
};

bool
locale_is_utf8(void)
{
    static const char u8[] = u8"ö";
    assert(strlen(u8) == 2);

    char32_t w;
    if (mbrtoc32(&w, u8, 2, &(mbstate_t){0}) != 2)
        return false;

    return w == U'ö';
}

static void
usage(const char *name)
{
    printf(
        "Usage: %s [OPTIONS...]\n"
        "\n"
        "Options:\n"
        "  -t,--text=TEXT            text string to render\n"
        "  -f,--font=FONTS               comma separated list of FontConfig formatted font specifications\n"
        "  -b,--background=RRGGBBAA      background color (e.g. ff000077 for semi-transparent red)\n"
        "  -c,--foreground=RRGGBBAA      foreground color (e.g. 00ff00ff for non-transparent green)\n"
        "  -s,--scaling-filter=FILT      pixman scaling filter to use when scaling color glyphs\n"
        "  -e,--emoji-presentation=PRES  default emoji presentation\n"
        "  -h,--help                 show usage\n",
        name);
}

int
main(int argc, char *const *argv)
{
    int exit_code = EXIT_SUCCESS;

    const char *locale = setlocale(LC_CTYPE, "");
    if (!locale_is_utf8()) {
        /* Try to force an UTF-8 locale */
        if (setlocale(LC_CTYPE, "C.UTF-8") == NULL &&
            setlocale(LC_CTYPE, "en_US.UTF-8") == NULL)
        {
            fprintf(stderr, "error: locale '%s' is not UTF-8\n", locale);
            return 1;
        }
    }

    assert(locale_is_utf8());

    fcft_init(FCFT_LOG_COLORIZE_AUTO, false, FCFT_LOG_CLASS_DEBUG);
    atexit(&fcft_fini);

    const char *prog_name = argv[0];

    const struct option options[] = {
        {"text",       required_argument, NULL, 't'},
        {"font",       required_argument, NULL, 'f'},
        {"background", required_argument, NULL, 'b'},
        {"foreground", required_argument, NULL, 'c'},
        {"scaling-filter", required_argument, NULL, 's'},
        {"emoji-presentation", required_argument, NULL, 'e'},
        {NULL,         no_argument,       NULL, '\0'},
    };

    const char *user_text = u8"hello world | ligatures: fi | اَلْعَرَبِيَّةُ | עִבְרִית‎ | graphemes: 👨‍👩‍👧‍👦 🇸🇪";
    const char *font_list = "serif:size=24";

    while (true) {
        int c = getopt_long(argc, argv, "+t:f:b:c:s:e:h", options, NULL);
        if (c < 0)
            break;

        switch (c) {
        case 't':
            user_text = optarg;
            break;

        case 'f':
            font_list = optarg;
            break;

        case 'b':
        case 'c': {
            errno = 0;
            char *end;
            unsigned long color = strtoul(optarg, &end, 16);

            assert(*end == '\0');
            assert(errno == 0);

            uint8_t _alpha = color & 0xff;
            uint16_t alpha = (uint16_t)_alpha << 8 | _alpha;

            uint32_t r = (color >> 24) & 0xff;
            uint32_t g = (color >> 16) & 0xff;
            uint32_t b = (color >> 8) & 0xff;

            pixman_color_t *clr = c == 'b' ? &bg : &fg;
            *clr = (pixman_color_t){
                .red =   (r << 8 | r) * alpha / 0xffff,
                .green = (g << 8 | g) * alpha / 0xffff,
                .blue =  (b << 8 | b) * alpha / 0xffff,
                .alpha = alpha,
            };
            break;
        }

        case 's':
            if (strcmp(optarg, "none") == 0)
                scaling_filter = FCFT_SCALING_FILTER_NONE;
            else if (strcmp(optarg, "nearest") == 0)
                scaling_filter = FCFT_SCALING_FILTER_NEAREST;
            else if (strcmp(optarg, "bilinear") == 0)
                scaling_filter = FCFT_SCALING_FILTER_BILINEAR;
            else if (strcmp(optarg, "impulse") == 0)
                scaling_filter = FCFT_SCALING_FILTER_IMPULSE;
            else if (strcmp(optarg, "box") == 0)
                scaling_filter = FCFT_SCALING_FILTER_BOX;
            else if (strcmp(optarg, "linear") == 0)
                scaling_filter = FCFT_SCALING_FILTER_LINEAR;
            else if (strcmp(optarg, "cubic") == 0)
                scaling_filter = FCFT_SCALING_FILTER_CUBIC;
            else if (strcmp(optarg, "gaussian") == 0)
                scaling_filter = FCFT_SCALING_FILTER_GAUSSIAN;
            else if (strcmp(optarg, "lanczos2") == 0)
                scaling_filter = FCFT_SCALING_FILTER_LANCZOS2;
            else if (strcmp(optarg, "lanczos3") == 0)
                scaling_filter = FCFT_SCALING_FILTER_LANCZOS3;
            else if (strcmp(optarg, "lanczos3-stretched") == 0)
                scaling_filter = FCFT_SCALING_FILTER_LANCZOS3_STRETCHED;
            else {
                fprintf(
                    stderr,
                    "%s: invalid scaling-filter, not one of 'none', "
                    "'nearest', 'bilinear', 'impulse', 'box', 'linear', "
                    "'cubic', 'gaussian', 'lanczos2', 'lanczos3', or "
                    "'lanczos3-stretched'\n", optarg);
                return EXIT_FAILURE;
            }
            break;

        case 'e':
            if (strcmp(optarg, "default") == 0)
                emoji_presentation = FCFT_EMOJI_PRESENTATION_DEFAULT;
            else if (strcmp(optarg, "text") == 0)
                emoji_presentation = FCFT_EMOJI_PRESENTATION_TEXT;
            else if (strcmp(optarg, "emoji") == 0)
                emoji_presentation = FCFT_EMOJI_PRESENTATION_EMOJI;
            else {
                fprintf(
                    stderr,
                    "%s: invalid emoji-presentation, not one of 'default', "
                    "'text' or 'emoji'\n", optarg);
                return EXIT_FAILURE;
            }
            break;

        case 'h':
            usage(prog_name);
            return EXIT_SUCCESS;

        case '?':
            return EXIT_FAILURE;
        }
    }

    /* Convert text string to Unicode */
    text = calloc(strlen(user_text) + 1, sizeof(text[0]));
    assert(text != NULL);

    {
        mbstate_t ps = {0};
        const char *in = user_text;
        const char *const end = user_text + strlen(user_text) + 1;

        size_t ret;

        while ((ret = mbrtoc32(&text[text_len], in, end - in, &ps)) != 0) {
            switch (ret) {
            case (size_t)-1:
                break;

            case (size_t)-2:
                break;

            case (size_t)-3:
                break;
            }

            in += ret;
            text_len++;
        }
    }

    /* Grapheme segmentation */
    graphemes = malloc(text_len * sizeof(graphemes[0]));
    assert(graphemes != NULL);

    {
        graphemes[0].begin = 0;
        utf8proc_int32_t state = 0;

        for (size_t i = 1; i < text_len; i++) {
            if (text[i - 1] == 0x200d) {
                /* ZWJ */
                continue;
            }

            if (utf8proc_grapheme_break_stateful(text[i - 1], text[i], &state)) {
                state = 0;

                /* Terminate previous grapheme */
                struct grapheme *grapheme = &graphemes[grapheme_count];
                grapheme->len = i - grapheme->begin;

                /* And begin the next one */
                grapheme = &graphemes[++grapheme_count];
                assert(grapheme_count <= text_len);
                grapheme->begin = i;
            }
        }

        /* Terminate final grapheme */
        struct grapheme *grapheme = &graphemes[grapheme_count];
        grapheme->len = text_len - grapheme->begin;
        grapheme_count++;
    }

    /* Instantiate font, and fallbacks */
    {
        tll(const char *) font_names = tll_init();

        char *copy = strdup(font_list);
        for (char *name = strtok(copy, ",");
             name != NULL;
             name = strtok(NULL, ","))
        {
            while (isspace(*name))
                name++;

            size_t len = strlen(name);
            while (len > 0 && isspace(name[len - 1]))
                name[--len] = '\0';

            tll_push_back(font_names, name);
        }

        const char *names[tll_length(font_names)];
        size_t idx = 0;

        tll_foreach(font_names, it)
            names[idx++] = it->item;

        struct fcft_font_options *options = fcft_font_options_create();
        /* Optionally set custom options */

        options->scaling_filter = scaling_filter;
        options->emoji_presentation = emoji_presentation;

        font = fcft_from_name2(tll_length(font_names), names, NULL, options);
        fcft_font_options_destroy(options);
        tll_free(font_names);
        free(copy);

        if (font == NULL) {
            exit_code = EXIT_FAILURE;
            goto err;
        }
    }

    display = wl_display_connect(NULL);
    assert(display != NULL);

    registry = wl_display_get_registry(display);
    assert(registry != NULL);

    wl_registry_add_listener(registry, &registry_listener, NULL);

    /* Trigger handle_global() */
    wl_display_roundtrip(display);

    /* Trigger listeners registered in handle_global() */
    wl_display_roundtrip(display);

    assert(compositor != NULL);
    assert(shell != NULL);
    assert(have_argb8888);

    surf = wl_compositor_create_surface(compositor);
    assert(surf != NULL);

    struct wl_region *empty_region = wl_compositor_create_region(compositor);
    wl_surface_set_input_region(surf, empty_region);
    wl_region_destroy(empty_region);

    xdg = xdg_wm_base_get_xdg_surface(shell, surf);
    assert(xdg != NULL);
    xdg_surface_add_listener(xdg, &xdg_surface_listener, NULL);

    toplevel = xdg_surface_get_toplevel(xdg);
    assert(toplevel != NULL);
    xdg_toplevel_add_listener(toplevel, &xdg_toplevel_listener, NULL);
    xdg_toplevel_set_app_id(toplevel, "fcft-example");

    /* We don’t have client side decorations - try to enable server side */
    if (deco_mgr != NULL) {
        deco = zxdg_decoration_manager_v1_get_toplevel_decoration(
            deco_mgr, toplevel);
        assert(deco != NULL);

        zxdg_toplevel_decoration_v1_add_listener(
            deco, &xdg_toplevel_decoration_listener, NULL);

        zxdg_toplevel_decoration_v1_set_mode(
            deco, ZXDG_TOPLEVEL_DECORATION_V1_MODE_SERVER_SIDE);
    } else
        fprintf(
            stderr,
            "warning: compositor does not implement server side decorations\n");

    wl_surface_commit(surf);

    sigset_t mask;
    {
        sigset_t blocked;
        sigemptyset(&blocked);
        sigaddset(&blocked, SIGINT);
        sigaddset(&blocked, SIGQUIT);

        sigprocmask(SIG_BLOCK, &blocked, &mask);
    }

    const struct sigaction sig_action = {.sa_handler = &sig_handler};
    sigaction(SIGINT, &sig_action, NULL);
    sigaction(SIGQUIT, &sig_action, NULL);
    sigaction(SIGTERM, &sig_action, NULL);

    while (!aborted) {
        wl_display_flush(display);

        struct pollfd fds[] = {
            {.fd = wl_display_get_fd(display), .events = POLLIN},
        };
        int ret = ppoll(fds, sizeof(fds) / sizeof(fds[0]), NULL, &mask);

        if (ret < 0) {
            if (errno == EINTR) {
                if (aborted)
                    break;
                continue;
            }

            fprintf(stderr, "error: failed to poll: %s\n", strerror(errno));
            exit_code = EXIT_FAILURE;
            break;
        }

        if (fds[0].revents & POLLHUP) {
            fprintf(stderr, "warning: disconnected by compositor\n");
            exit_code = EXIT_FAILURE;
            break;
        }

        if (fds[0].revents & POLLIN)
            wl_display_dispatch(display);
    }

err:
    if (deco != NULL)
        zxdg_toplevel_decoration_v1_destroy(deco);
    if (toplevel != NULL)
        xdg_toplevel_destroy(toplevel);
    if (xdg != NULL)
        xdg_surface_destroy(xdg);
    if (shell != NULL)
        xdg_wm_base_destroy(shell);
    if (surf != NULL)
        wl_surface_destroy(surf);
    if (deco_mgr != NULL)
        zxdg_decoration_manager_v1_destroy(deco_mgr);
    if (shm != NULL)
        wl_shm_destroy(shm);
    if (compositor != NULL)
        wl_compositor_destroy(compositor);
    if (registry != NULL)
        wl_registry_destroy(registry);
    if (display != NULL)
        wl_display_disconnect(display);
    fcft_destroy(font);
    free(text);

    return exit_code;
}
