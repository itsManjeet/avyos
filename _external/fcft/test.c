#include <stdlib.h>
#include <stdio.h>
#include <getopt.h>

#include <check.h>
#include <fcft/fcft.h>

#define ALEN(v) (sizeof(v) / sizeof((v)[0]))

#if !defined(__STDC_UTF_32__) || !__STDC_UTF_32__
 #error "uint32_t does not use UTF-32"
#endif

static struct fcft_font_options *font_options = NULL;
static struct fcft_font *font = NULL;

static void
core_setup(void)
{
    font_options = fcft_font_options_create();
    ck_assert_ptr_nonnull(font_options);

    font = fcft_from_name2(
        1, (const char *[]){"Serif"}, NULL, font_options);
    ck_assert_ptr_nonnull(font);

    fcft_font_options_destroy(font_options);
    font_options = NULL;
}

static void
core_teardown(void)
{
    fcft_destroy(font);
    font = NULL;
}

START_TEST(test_capabilities)
{
    enum fcft_capabilities caps = fcft_capabilities();

#if defined(FCFT_HAVE_HARFBUZZ)
    ck_assert(caps & FCFT_CAPABILITY_GRAPHEME_SHAPING);
    caps &= ~FCFT_CAPABILITY_GRAPHEME_SHAPING;
#endif
#if defined(FCFT_HAVE_HARFBUZZ) && defined(FCFT_HAVE_UTF8PROC)
    ck_assert(caps & FCFT_CAPABILITY_TEXT_RUN_SHAPING);
    caps &= ~FCFT_CAPABILITY_TEXT_RUN_SHAPING;
#endif
#if defined(FCFT_ENABLE_SVG_LIBRSVG) || defined(FCFT_ENABLE_SVG_NANOSVG)
    ck_assert(caps & FCFT_CAPABILITY_SVG);
    caps &= ~FCFT_CAPABILITY_SVG;
#endif

    ck_assert_int_eq(caps, 0);
}
END_TEST

START_TEST(test_from_name)
{
    ck_assert_int_gt(font->height, 0);
    ck_assert_int_gt(font->max_advance.x, 0);
    ck_assert_int_gt(font->underline.thickness, 0);
    ck_assert_int_gt(font->strikeout.thickness, 0);
}
END_TEST

START_TEST(test_glyph_rasterize)
{
    const struct fcft_glyph *glyph = fcft_rasterize_char_utf32(
        font, U'A', FCFT_SUBPIXEL_NONE);
    ck_assert_ptr_nonnull(glyph);
    ck_assert_ptr_nonnull(glyph->pix);
    ck_assert_int_eq(glyph->cp, U'A');
    ck_assert_int_eq(glyph->cols, 1);
    ck_assert_int_gt(glyph->width, 0);
    ck_assert_int_gt(glyph->height, 0);
    ck_assert_int_gt(glyph->advance.x, 0);
}
END_TEST

START_TEST(test_precompose)
{
    uint32_t ret = fcft_precompose(font, U'a', U'\U00000301', NULL, NULL, NULL);

    /* All western fonts _should_ have this pre-composed character */
    ck_assert_int_eq(ret, U'á');

    /* Can't verify *_is_from_primary since we don't know which font
     * we're using */

    ret = fcft_precompose(font, U'X', U'Y', NULL, NULL, NULL);
    ck_assert_int_eq(ret, (uint32_t)-1);
}
END_TEST

#if defined(FCFT_HAVE_HARFBUZZ)

static struct fcft_font *emoji_font = NULL;

static void
text_shaping_setup(void)
{
    core_setup();
    emoji_font = fcft_from_name(1, (const char *[]){"emoji"}, NULL);
    ck_assert_ptr_nonnull(emoji_font);
}

static void
text_shaping_teardown(void)
{
    core_teardown();
    fcft_destroy(emoji_font);
    emoji_font = NULL;
}


START_TEST(test_emoji_zwj)
{
    const uint32_t emoji[] = U"🤚🏿";
    const struct fcft_grapheme *grapheme = fcft_rasterize_grapheme_utf32(
        emoji_font, ALEN(emoji) - 1, emoji, FCFT_SUBPIXEL_DEFAULT);
    ck_assert_ptr_nonnull(grapheme);
    ck_assert_int_eq(grapheme->count, 1);

    /* Verify grapheme was cached */
    const struct fcft_grapheme *grapheme2 = fcft_rasterize_grapheme_utf32(
        emoji_font, ALEN(emoji) - 1, emoji, FCFT_SUBPIXEL_DEFAULT);
    ck_assert_ptr_eq(grapheme, grapheme2);
}
END_TEST
#endif

Suite *
fcft_suite(bool run_text_shaping_tests)
{
    Suite *suite = suite_create("fcft");

    TCase *core = tcase_create("core");
    tcase_add_checked_fixture(core, &core_setup, &core_teardown);

    /* Slow systems, like the Pinebook Pro, with a *lot* of fonts, *will* be slow */
    tcase_set_timeout(core, 60);

    tcase_add_test(core, test_capabilities);
    tcase_add_test(core, test_from_name);
    tcase_add_test(core, test_glyph_rasterize);
    tcase_add_test(core, test_precompose);
    suite_add_tcase(suite, core);

#if defined(FCFT_HAVE_HARFBUZZ)
    if (run_text_shaping_tests) {
        TCase *text_shaping = tcase_create("text-shaping");
        tcase_set_timeout(text_shaping, 60);
        tcase_add_checked_fixture(
            text_shaping, &text_shaping_setup, &text_shaping_teardown);
        tcase_add_test(text_shaping, test_emoji_zwj);
        suite_add_tcase(suite, text_shaping);
    }
#endif

    suite_add_tcase(suite, core);
    return suite;
}

static void
print_usage(const char *prog_name)
{
    printf(
        "Usage: %s [OPTIONS...]\n"
        "\n"
        "Options:\n"
#if defined(FCFT_HAVE_HARFBUZZ)
        "  -s,--text-shaping                  run text shaping tests (requires an emoji font to be installed)\n"
#endif
        ,
        prog_name);
}

int
main(int argc, char *const *argv)
{
    const char *const prog_name = argv[0];

    static const struct option longopts[] =  {
#if defined(FCFT_HAVE_HARFBUZZ)
        {"text-shaping", no_argument, NULL, 's'},
#endif
        {NULL,           no_argument, NULL,   0},
    };

    bool run_text_shaping_tests = false;

    while (true) {
        int c = getopt_long(argc, argv, "sh", longopts, NULL);
        if (c == -1)
            break;

        switch (c) {
        case 's':
            run_text_shaping_tests = true;
            break;

        case 'h':
            print_usage(prog_name);
            return EXIT_SUCCESS;

        case '?':
            return EXIT_FAILURE;
        }
    }

    if (!fcft_init(FCFT_LOG_COLORIZE_AUTO, false, FCFT_LOG_CLASS_DEBUG))
        return 1;
    atexit(&fcft_fini);

    Suite *suite = fcft_suite(run_text_shaping_tests);
    SRunner *runner = srunner_create(suite);

    srunner_run_all(runner, CK_NORMAL);
    int failed = srunner_ntests_failed(runner);

    srunner_free(runner);
    return failed;
}
