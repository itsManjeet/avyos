#ifndef DI_CVT_H
#define DI_CVT_H

/**
 * Low-level API for VESA Coordinated Video Timings (CVT) version 2.0.
 */

#include <stdbool.h>
#include <stdint.h>

enum di_cvt_reduced_blanking_version {
	DI_CVT_REDUCED_BLANKING_NONE,
	DI_CVT_REDUCED_BLANKING_V1,
	DI_CVT_REDUCED_BLANKING_V2,
	DI_CVT_REDUCED_BLANKING_V3,
};

/**
 * Input parameters, defined in table 3-1.
 */
struct di_cvt_options {
	/* Version of the reduced blanking timing formula to be used */
	enum di_cvt_reduced_blanking_version red_blank_ver;
	/* Desired active (visible) horizontal pixels and vertical lines per
	 * frame */
	int32_t h_pixels, v_lines;
	/* Target vertical refresh rate (in Hz) */
	double ip_freq_rqd;
	/* Whether to generate a "video-optimized" timing variant (RBv2 only) */
	bool video_opt;
	/* Desired VBlank time (in µs, RBv3 only, must be greater than 460) */
	double vblank;
	/* Desired additional number of pixels to add to the base HBlank
	 * duration (RBv3 only, must be a multiple of 8 between 0 and 120) */
	int32_t additional_hblank;
	/* Indicates whether the VSync location is early (RBv3 only) */
	bool early_vsync_rqd;
	/* Whether interlaced is required (no RB and RBv1 only) */
	bool int_rqd;
	/* Whether margins are required (no RB and RBv1 only) */
	bool margins_rqd;
};

/**
 * Output parameters, defined in table 3-4.
 */
struct di_cvt_timing {
	/* Pixel clock frequency (in MHz) */
	double act_pixel_freq;
	/* Total number of active (visible) pixels per line */
	double total_active_pixels;
	/* Total number of active (visible) vertical lines per frame */
	double v_lines_rnd;
	/* Number of pixels in the horizontal front porch period */
	double h_front_porch;
	/* Number of pixels in the HSync period */
	double h_sync;
	/* Number of pixels in the horizontal back porch period */
	double h_back_porch;
	/* Number of lines in the vertical front porch period */
	double v_front_porch;
	/* Number of lines in the VSync period */
	double v_sync;
	/* Number of lines in the vertical back porch period */
	double v_back_porch;
	/* Frame rate (in Hz) */
	double act_frame_rate;
};

/**
 * Compute a timing via the CVT formula.
 */
void
di_cvt_compute(struct di_cvt_timing *t, const struct di_cvt_options *options);

#endif
