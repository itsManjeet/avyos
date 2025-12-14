/*
 * Copyright 2006-2012 Red Hat, Inc.
 * Copyright 2018-2021 Cisco Systems, Inc. and/or its affiliates. All rights reserved.
 *
 * Originally imported from edid-decode.
 */

#include <math.h>

#include <libdisplay-info/gtf.h>

/**
 * The assumed character cell granularity of the graphics system, in pixels.
 */
#define CELL_GRAN 8.0
/**
 * The size of the top and bottom overscan margin as a percentage of the active
 * vertical image.
 */
#define MARGIN_PERC 1.8
/**
 * The minimum front porch in lines (vertical) and character cells (horizontal).
 */
#define MIN_PORCH 1.0
/**
 * The width of the V sync in lines.
 */
#define V_SYNC_RQD 3.0
/**
 * The width of the H sync as a percentage of the total line period.
 */
#define H_SYNC_PERC 8.0
/**
 * Minimum time of vertical sync + back porch interval (µs).
 */
#define MIN_VSYNC_BP 550.0

void di_gtf_compute(struct di_gtf_timing *t, const struct di_gtf_options *options)
{
	double c_prime, m_prime, h_pixels_rnd, v_lines_rnd, h_margin, v_margin, interlace,
	       total_active_pixels, v_field_rate_rqd, h_period_est, total_v_lines, v_field_rate_est,
	       h_period, ideal_duty_cycle, h_freq, ideal_h_period, v_back_porch, h_sync,
	       h_front_porch;
	double v_sync_bp = 0.0, h_blank_pixels = 0.0, total_pixels = 0.0, pixel_freq = 0.0;

	/* C' and M' are part of the Blanking Duty Cycle computation */
	c_prime = ((options->c - options->j) * options->k / 256.0) + options->j;
	m_prime = options->k / 256.0 * options->m;

	h_pixels_rnd = round(options->h_pixels / CELL_GRAN) * CELL_GRAN;
	v_lines_rnd = options->int_rqd ?
		      round(options->v_lines / 2.0) :
		      options->v_lines;
	h_margin = options->margins_rqd ?
		   round(h_pixels_rnd * MARGIN_PERC / 100.0 / CELL_GRAN) * CELL_GRAN :
		   0;
	v_margin = options->margins_rqd ?
		   round(MARGIN_PERC / 100.0 * v_lines_rnd) :
		   0;
	interlace = options->int_rqd ? 0.5 : 0;
	total_active_pixels = h_pixels_rnd + h_margin * 2;

	switch (options->ip_param) {
	case DI_GTF_IP_PARAM_V_FRAME_RATE:
		// vertical frame frequency (Hz)
		v_field_rate_rqd = options->int_rqd ?
				   options->ip_freq_rqd * 2 :
				   options->ip_freq_rqd;
		h_period_est = (1.0 / v_field_rate_rqd - MIN_VSYNC_BP / 1000000.0) /
			       (v_lines_rnd + v_margin * 2 + MIN_PORCH + interlace) * 1000000.0;
		v_sync_bp = round(MIN_VSYNC_BP / h_period_est);
		total_v_lines = v_lines_rnd + v_margin * 2 +
				v_sync_bp + interlace + MIN_PORCH;
		v_field_rate_est = 1.0 / h_period_est / total_v_lines * 1000000.0;
		h_period = h_period_est / (v_field_rate_rqd / v_field_rate_est);
		ideal_duty_cycle = c_prime - m_prime * h_period / 1000.0;
		h_blank_pixels = round(total_active_pixels * ideal_duty_cycle /
				       (100.0 - ideal_duty_cycle) /
				       (2 * CELL_GRAN)) * 2 * CELL_GRAN;
		total_pixels = total_active_pixels + h_blank_pixels;
		pixel_freq = total_pixels / h_period;
		break;
	case DI_GTF_IP_PARAM_H_FREQ:
		// horizontal frequency (kHz)
		h_freq = options->ip_freq_rqd;
		v_sync_bp = round(MIN_VSYNC_BP * h_freq / 1000.0);
		ideal_duty_cycle = c_prime - m_prime / h_freq;
		h_blank_pixels = round(total_active_pixels * ideal_duty_cycle /
				       (100.0 - ideal_duty_cycle) /
				       (2 * CELL_GRAN)) * 2 * CELL_GRAN;
		total_pixels = total_active_pixels + h_blank_pixels;
		pixel_freq = total_pixels * h_freq / 1000.0;
		break;
	case DI_GTF_IP_PARAM_H_PIXELS:
		// pixel clock rate (MHz)
		pixel_freq = options->ip_freq_rqd;
		ideal_h_period = (c_prime - 100.0 +
				  sqrt((100.0 - c_prime) * (100.0 - c_prime) +
				       0.4 * m_prime * (total_active_pixels + h_margin * 2) / pixel_freq))
				  / 2.0 / m_prime * 1000.0;
		ideal_duty_cycle = c_prime - m_prime * ideal_h_period / 1000.0;
		h_blank_pixels = round(total_active_pixels * ideal_duty_cycle /
				       (100.0 - ideal_duty_cycle) /
				       (2 * CELL_GRAN)) * 2 * CELL_GRAN;
		total_pixels = total_active_pixels + h_blank_pixels;
		h_freq = pixel_freq / total_pixels * 1000.0;
		v_sync_bp = round(MIN_VSYNC_BP * h_freq / 1000.0);
		break;
	}

	v_back_porch = v_sync_bp - V_SYNC_RQD;
	h_sync = round(H_SYNC_PERC / 100.0 * total_pixels / CELL_GRAN) * CELL_GRAN;
	h_front_porch = h_blank_pixels / 2.0 - h_sync;

	*t = (struct di_gtf_timing) {
		.h_pixels = (int) h_pixels_rnd,
		.v_lines = options->v_lines,
		.v_sync = V_SYNC_RQD,
		.h_sync = (int) h_sync,
		.v_front_porch = MIN_PORCH,
		.v_back_porch = (int) v_back_porch,
		.h_front_porch = (int) h_front_porch,
		.h_back_porch = (int) (h_front_porch + h_sync),
		.h_border = (int) h_margin,
		.v_border = (int) v_margin,
		.pixel_freq_mhz = pixel_freq,
	};
}
