/*
 * Copyright 2006-2012 Red Hat, Inc.
 * Copyright 2018-2021 Cisco Systems, Inc. and/or its affiliates. All rights reserved.
 *
 * Originally imported from edid-decode.
 */

#include <math.h>

#include <libdisplay-info/cvt.h>

/**
 * The size of the top and bottom overscan margin as a percentage of the active
 * vertical image.
 */
#define MARGIN_PERC 1.8
#define MIN_VSYNC_BP 550.0
#define MIN_V_PORCH 3
/**
 * Minimum vertical backporch for CVT and CVT RBv1.
 */
#define MIN_V_BPORCH 7
/**
 * Fixed vertical backporch for CVT RBv2 and RBv3.
 */
#define FIXED_V_BPORCH 6
#define C_PRIME 30.0
#define M_PRIME 300.0
/**
 * Minimum VBlank period (in µs) for RB timings.
 */
#define RB_MIN_VBLANK 460.0

void
di_cvt_compute(struct di_cvt_timing *t, const struct di_cvt_options *options)
{
	enum di_cvt_reduced_blanking_version rb = options->red_blank_ver;
	double cell_gran, h_pixels_rnd, v_lines_rnd, hor_margin, vert_margin,
	       interlace, total_active_pixels, v_field_rate_rqd, clock_step,
	       h_blank, rb_v_fporch, refresh_multiplier, rb_min_vblank, h_sync,
	       v_sync, pixel_freq, v_blank, v_sync_bp, additional_hblank,
	       h_period_est, ideal_duty_cycle, total_pixels, vbi_lines,
	       rb_v_bporch, rb_min_vbi, total_v_lines, freq, h_front_porch,
	       v_back_porch, act_h_freq;

	cell_gran = rb == DI_CVT_REDUCED_BLANKING_V2 ? 1 : 8;
	h_pixels_rnd = floor(options->h_pixels / cell_gran) * cell_gran;
	v_lines_rnd = options->int_rqd ? floor(options->v_lines / 2.0) : options->v_lines;
	hor_margin = options->margins_rqd
		     ? floor((h_pixels_rnd * MARGIN_PERC / 100.0) / cell_gran) * cell_gran
		     : 0;
	vert_margin = options->margins_rqd
		      ? floor(MARGIN_PERC / 100.0 * v_lines_rnd)
		      : 0;
	interlace = options->int_rqd ? 0.5 : 0;
	total_active_pixels = h_pixels_rnd + hor_margin * 2;
	v_field_rate_rqd = options->int_rqd
			   ? options->ip_freq_rqd * 2
			   : options->ip_freq_rqd;
	clock_step = rb >= DI_CVT_REDUCED_BLANKING_V2 ? 0.001 : 0.25;
	h_blank = rb == DI_CVT_REDUCED_BLANKING_V1 ? 160 : 80;
	rb_v_fporch = rb == DI_CVT_REDUCED_BLANKING_V1 ? 3 : 1;
	refresh_multiplier = (rb == DI_CVT_REDUCED_BLANKING_V2 && options->video_opt) ? 1000.0 / 1001.0 : 1;
	rb_min_vblank = rb == DI_CVT_REDUCED_BLANKING_V3 ? options->vblank : RB_MIN_VBLANK;
	if (rb_min_vblank < RB_MIN_VBLANK)
		rb_min_vblank = RB_MIN_VBLANK;
	h_sync = 32;

	if (rb == DI_CVT_REDUCED_BLANKING_V3) {
		additional_hblank = options->additional_hblank;
		if (additional_hblank < 0)
			additional_hblank = 0;
		else if (additional_hblank > 120)
			additional_hblank = 120;
		h_blank += additional_hblank;
	}

	/* Determine VSync Width from aspect ratio */
	if ((options->v_lines * 4 / 3) == options->h_pixels)
		v_sync = 4;
	else if ((options->v_lines * 16 / 9) == options->h_pixels)
		v_sync = 5;
	else if ((options->v_lines * 16 / 10) == options->h_pixels)
		v_sync = 6;
	else if (!(options->v_lines % 4) && ((options->v_lines * 5 / 4) == options->h_pixels))
		v_sync = 7;
	else if ((options->v_lines * 15 / 9) == options->h_pixels)
		v_sync = 7;
	else /* Custom */
		v_sync = 10;

	if (rb >= DI_CVT_REDUCED_BLANKING_V2)
		v_sync = 8;

	if (rb == DI_CVT_REDUCED_BLANKING_NONE) {
		h_period_est = (1.0 / v_field_rate_rqd - MIN_VSYNC_BP / 1000000.0) /
			(v_lines_rnd + vert_margin * 2 + MIN_V_PORCH + interlace) * 1000000.0;
		v_sync_bp = floor(MIN_VSYNC_BP / h_period_est) + 1;
		if (v_sync_bp < v_sync + MIN_V_BPORCH)
			v_sync_bp = v_sync + MIN_V_BPORCH;
		v_blank = v_sync_bp + MIN_V_PORCH;
		total_v_lines = v_lines_rnd + vert_margin * 2 + v_sync_bp +
				interlace + MIN_V_PORCH;
		ideal_duty_cycle = C_PRIME - M_PRIME * h_period_est / 1000.0;
		if (ideal_duty_cycle < 20)
			ideal_duty_cycle = 20;
		h_blank = floor(total_active_pixels * ideal_duty_cycle /
				(100.0 - ideal_duty_cycle) /
				(2 * cell_gran)) * 2 * cell_gran;
		total_pixels = total_active_pixels + h_blank;
		h_sync = floor(total_pixels * 0.08 / cell_gran) * cell_gran;
		pixel_freq = floor(total_pixels / h_period_est / clock_step) * clock_step;
	} else {
		h_period_est = (1000000.0 / v_field_rate_rqd - rb_min_vblank) /
					(v_lines_rnd + vert_margin * 2);
		vbi_lines = floor(rb_min_vblank / h_period_est) + 1;
		rb_v_bporch = rb == DI_CVT_REDUCED_BLANKING_V1 ? MIN_V_BPORCH : FIXED_V_BPORCH;
		rb_min_vbi = rb_v_fporch + v_sync + rb_v_bporch;
		v_blank = vbi_lines < rb_min_vbi ? rb_min_vbi : vbi_lines;
		total_v_lines = v_blank + v_lines_rnd + vert_margin * 2 + interlace;
		if (rb == DI_CVT_REDUCED_BLANKING_V3 && options->early_vsync_rqd)
			rb_v_bporch = floor(vbi_lines / 2.0);
		if (rb == DI_CVT_REDUCED_BLANKING_V1)
			v_sync_bp = v_blank - rb_v_fporch;
		else
			v_sync_bp = v_sync + rb_v_bporch;
		total_pixels = h_blank + total_active_pixels;
		freq = v_field_rate_rqd * total_v_lines * total_pixels * refresh_multiplier;
		if (rb == DI_CVT_REDUCED_BLANKING_V3)
			pixel_freq = ceil(freq / 1000000.0 / clock_step) * clock_step;
		else
			pixel_freq = floor(freq / 1000000.0 / clock_step) * clock_step;
	}

	if (rb >= DI_CVT_REDUCED_BLANKING_V2)
		h_front_porch = 8;
	else
		h_front_porch = (h_blank / 2.0) - h_sync;

	v_back_porch = v_sync_bp - v_sync;
	act_h_freq = 1000 * pixel_freq / total_pixels;

	*t = (struct di_cvt_timing){
		.act_pixel_freq = pixel_freq,
		.total_active_pixels = total_active_pixels,
		.v_lines_rnd = v_lines_rnd,
		.h_front_porch = h_front_porch,
		.h_sync = h_sync,
		.h_back_porch = h_blank - h_front_porch - h_sync,
		.v_front_porch = v_blank - v_back_porch - v_sync,
		.v_back_porch = v_back_porch,
		.v_sync = v_sync,
		.act_frame_rate = 1000 * act_h_freq / total_v_lines,
	};
}
