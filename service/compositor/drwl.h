/*
 * drwl - https://codeberg.org/sewn/drwl
 *
 * Copyright (c) 2023-2025 sewn <sewn@disroot.org>
 * Copyright (c) 2024 notchoc <notchoc@disroot.org>
 * 
 * Permission is hereby granted, free of charge, to any person obtaining
 * a copy of this software and associated documentation files (the
 * "Software"), to deal in the Software without restriction, including
 * without limitation the rights to use, copy, modify, merge, publish,
 * distribute, sublicense, and/or sell copies of the Software, and to
 * permit persons to whom the Software is furnished to do so, subject to
 * the following conditions:
 * 
 * The above copyright notice and this permission notice shall be
 * included in all copies or substantial portions of the Software.
 * 
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
 * EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
 * MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
 * NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
 * LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
 * OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
 * WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 *
 * The UTF-8 Decoder included is from Bjoern Hoehrmann:
 * Copyright (c) 2008-2010 Bjoern Hoehrmann <bjoern@hoehrmann.de>
 * See http://bjoern.hoehrmann.de/utf-8/decoder/dfa/ for details.
 */
#pragma once

#include <stdlib.h>
#include <fcft/fcft.h>
#include <pixman-1/pixman.h>

enum { ColFg, ColBg, ColBorder }; /* colorscheme index */

typedef struct fcft_font Fnt;
typedef pixman_image_t Img;

typedef struct {
	Img *image;
	Fnt *font;
	uint32_t *scheme;
} Drwl;

#define UTF8_ACCEPT 0
#define UTF8_REJECT 12
#define UTF8_INVALID 0xFFFD

static const uint8_t utf8d[] = {
	 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,  0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,
	 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,  0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,
	 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,  0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,
	 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,  0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,
	 1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,  9,9,9,9,9,9,9,9,9,9,9,9,9,9,9,9,
	 7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,  7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,
	 8,8,2,2,2,2,2,2,2,2,2,2,2,2,2,2,  2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,
	10,3,3,3,3,3,3,3,3,3,3,3,3,4,3,3, 11,6,6,6,5,8,8,8,8,8,8,8,8,8,8,8,

	 0,12,24,36,60,96,84,12,12,12,48,72, 12,12,12,12,12,12,12,12,12,12,12,12,
	12, 0,12,12,12,12,12, 0,12, 0,12,12, 12,24,12,12,12,12,12,24,12,24,12,12,
	12,12,12,12,12,12,12,24,12,12,12,12, 12,24,12,12,12,12,12,12,12,24,12,12,
	12,12,12,12,12,12,12,36,12,36,12,12, 12,36,12,12,12,12,12,36,12,36,12,12,
	12,36,12,12,12,12,12,12,12,12,12,12,
};

static inline uint32_t
utf8decode(uint32_t *state, uint32_t *codep, uint8_t byte)
{
	uint32_t type = utf8d[byte];

	*codep = (*state != UTF8_ACCEPT) ?
		(byte & 0x3fu) | (*codep << 6) :
		(0xff >> type) & (byte);

	*state = utf8d[256 + *state + type];
	return *state;
}

static int
drwl_init(void)
{
	fcft_set_scaling_filter(FCFT_SCALING_FILTER_LANCZOS3);
	return fcft_init(FCFT_LOG_COLORIZE_AUTO, 0, FCFT_LOG_CLASS_ERROR);
}

static Drwl *
drwl_create(void)
{
	Drwl *drwl;
	
	if (!(drwl = calloc(1, sizeof(Drwl))))
		return NULL;

	return drwl;
}

static void
drwl_setfont(Drwl *drwl, Fnt *font)
{
	if (drwl)
		drwl->font = font;
}

static void
drwl_setimage(Drwl *drwl, Img *image)
{
	if (drwl)
		drwl->image = image;
}

static Fnt *
drwl_font_create(Drwl *drwl, size_t count,
		const char *names[static count], const char *attributes)
{
	Fnt *font = fcft_from_name(count, names, attributes);
	if (drwl)
		drwl_setfont(drwl, font);
	return font;
}

static void
drwl_font_destroy(Fnt *font)
{
	fcft_destroy(font);
}

static inline pixman_color_t
convert_color(uint32_t clr)
{
	return (pixman_color_t){
		((clr >> 24) & 0xFF) * 0x101 * (clr & 0xFF) / 0xFF,
		((clr >> 16) & 0xFF) * 0x101 * (clr & 0xFF) / 0xFF,
		((clr >> 8) & 0xFF) * 0x101 * (clr & 0xFF) / 0xFF,
		(clr & 0xFF) * 0x101
	};
}

static void
drwl_setscheme(Drwl *drwl, uint32_t *scm)
{
	if (drwl)
		drwl->scheme = scm;
}

static Img *
drwl_image_create(Drwl *drwl, unsigned int w, unsigned int h, uint32_t *bits)
{
	Img *image;
	pixman_region32_t clip;

	image = pixman_image_create_bits_no_clear(
		PIXMAN_a8r8g8b8, w, h, bits, w * 4);
	if (!image)
		return NULL;
	pixman_region32_init_rect(&clip, 0, 0, w, h);
	pixman_image_set_clip_region32(image, &clip);
	pixman_region32_fini(&clip);

	if (drwl)
		drwl_setimage(drwl, image);
	return image;
}

static void
drwl_rect(Drwl *drwl,
		int x, int y, unsigned int w, unsigned int h,
		int filled, int invert)
{
	pixman_color_t clr;
	if (!drwl || !drwl->scheme || !drwl->image)
		return;

	clr = convert_color(drwl->scheme[invert ? ColBg : ColFg]);
	if (filled)
		pixman_image_fill_rectangles(PIXMAN_OP_SRC, drwl->image, &clr, 1,
			&(pixman_rectangle16_t){x, y, w, h});
	else
		pixman_image_fill_rectangles(PIXMAN_OP_SRC, drwl->image, &clr, 4,
			(pixman_rectangle16_t[4]){
				{ x,         y,         w, 1 },
				{ x,         y + h - 1, w, 1 },
				{ x,         y,         1, h },
				{ x + w - 1, y,         1, h }});
}

static int
drwl_text(Drwl *drwl,
		int x, int y, unsigned int w, unsigned int h,
		unsigned int lpad, const char *text, int invert)
{
	int ty;
	int render = x || y || w || h;
	long x_kern;
	uint32_t cp = 0, last_cp = 0, state;
	pixman_color_t clr;
	pixman_image_t *fg_pix = NULL;
	int noellipsis = 0;
	const struct fcft_glyph *glyph, *eg = NULL;
	int fcft_subpixel_mode = FCFT_SUBPIXEL_DEFAULT;

	if (!drwl || (render && (!drwl->scheme || !w || !drwl->image)) || !text || !drwl->font)
		return 0;

	if (!render) {
		w = invert ? invert : ~invert;
	} else {
		clr = convert_color(drwl->scheme[invert ? ColBg : ColFg]);
		fg_pix = pixman_image_create_solid_fill(&clr);

		drwl_rect(drwl, x, y, w, h, 1, !invert);

		x += lpad;
		w -= lpad;
	}

	if (render && (drwl->scheme[ColBg] & 0xFF) != 0xFF)
		fcft_subpixel_mode = FCFT_SUBPIXEL_NONE;

	if (render)
		eg = fcft_rasterize_char_utf32(drwl->font, 0x2026 /* … */, fcft_subpixel_mode);

	for (const char *p = text, *pp; pp = p, *p; p++) {
		for (state = UTF8_ACCEPT; *p &&
		     utf8decode(&state, &cp, *p) > UTF8_REJECT; p++)
			;
		if (!*p || state == UTF8_REJECT) {
			cp = UTF8_INVALID;
			if (p > pp)
				p--;
		}

		glyph = fcft_rasterize_char_utf32(drwl->font, cp, fcft_subpixel_mode);
		if (!glyph)
			continue;

		x_kern = 0;
		if (last_cp)
			fcft_kerning(drwl->font, last_cp, cp, &x_kern, NULL);
		last_cp = cp;

		ty = y + (h - drwl->font->height) / 2 + drwl->font->ascent;

		if (render && !noellipsis && x_kern + glyph->advance.x + eg->advance.x > w &&
		    *(p + 1) != '\0') {
			/* cannot fit ellipsis after current codepoint */
			if (drwl_text(drwl, 0, 0, 0, 0, 0, pp, 0) + x_kern <= w) {
				noellipsis = 1;
			} else {
				w -= eg->advance.x;
				pixman_image_composite32(
					PIXMAN_OP_OVER, fg_pix, eg->pix, drwl->image, 0, 0, 0, 0,
					x + eg->x, ty - eg->y, eg->width, eg->height);
			}
		}

		if ((x_kern + glyph->advance.x) > w)
			break;

		x += x_kern;

		if (render && pixman_image_get_format(glyph->pix) == PIXMAN_a8r8g8b8)
			/* pre-rendered glyphs (eg. emoji) */
			pixman_image_composite32(
				PIXMAN_OP_OVER, glyph->pix, NULL, drwl->image, 0, 0, 0, 0,
				x + glyph->x, ty - glyph->y, glyph->width, glyph->height);
		else if (render)
			pixman_image_composite32(
				PIXMAN_OP_OVER, fg_pix, glyph->pix, drwl->image, 0, 0, 0, 0,
				x + glyph->x, ty - glyph->y, glyph->width, glyph->height);

		x += glyph->advance.x;
		w -= glyph->advance.x;
	}

	if (render)
		pixman_image_unref(fg_pix);

	return x + (render ? w : 0);
}

static unsigned int
drwl_font_getwidth(Drwl *drwl, const char *text)
{
	if (!drwl || !drwl->font || !text)
		return 0;
	return drwl_text(drwl, 0, 0, 0, 0, 0, text, 0);
}

static void
drwl_image_destroy(Img *image)
{
	pixman_image_unref(image);
}

static void
drwl_destroy(Drwl *drwl)
{
	if (drwl->font)
		drwl_font_destroy(drwl->font);
	if (drwl->image)
		drwl_image_destroy(drwl->image);
	free(drwl);
}

static void
drwl_fini(void)
{
	fcft_fini();
}
