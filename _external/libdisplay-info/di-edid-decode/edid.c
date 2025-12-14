#include <assert.h>
#include <inttypes.h>
#include <math.h>
#include <stdio.h>
#include <stdlib.h>

#include <libdisplay-info/dmt.h>
#include <libdisplay-info/edid.h>
#include <libdisplay-info/gtf.h>
#include <libdisplay-info/cvt.h>

#include "di-edid-decode.h"

static size_t num_detailed_timing_defs = 0;

static const char *
standard_timing_aspect_ratio_name(enum di_edid_standard_timing_aspect_ratio aspect_ratio)
{
	switch (aspect_ratio) {
	case DI_EDID_STANDARD_TIMING_16_10:
		return "16:10";
	case DI_EDID_STANDARD_TIMING_4_3:
		return " 4:3 ";
	case DI_EDID_STANDARD_TIMING_5_4:
		return " 5:4 ";
	case DI_EDID_STANDARD_TIMING_16_9:
		return "16:9 ";
	}
	abort();
}

static void
print_standard_timing(const struct di_edid_standard_timing *t)
{
	int32_t vert_video;
	const struct di_dmt_timing *dmt;
	int hbl, vbl, horiz_total, vert_total;
	double refresh, horiz_freq_hz, pixel_clock_mhz, pixel_clock_khz;
	struct di_gtf_options gtf_options;
	struct di_gtf_timing gtf;

	vert_video = di_edid_standard_timing_get_vert_video(t);
	dmt = di_edid_standard_timing_get_dmt(t);

	printf("    ");
	if (dmt) {
		hbl = dmt->horiz_blank - 2 * dmt->horiz_border;
		vbl = dmt->vert_blank - 2 * dmt->vert_border;
		horiz_total = dmt->horiz_video + hbl;
		vert_total = dmt->vert_video + vbl;
		refresh = (double) dmt->pixel_clock_hz / (horiz_total * vert_total);
		horiz_freq_hz = (double) dmt->pixel_clock_hz / horiz_total;
		pixel_clock_mhz = (double) dmt->pixel_clock_hz / (1000 * 1000);

		printf("DMT 0x%02x", dmt ? dmt->dmt_id : 0);
	} else {
		/* TODO: CVT timings */

		gtf_options = (struct di_gtf_options) {
			.h_pixels = t->horiz_video,
			.v_lines = vert_video,
			.ip_param = DI_GTF_IP_PARAM_V_FRAME_RATE,
			.ip_freq_rqd = t->refresh_rate_hz,
			.m = DI_GTF_DEFAULT_M,
			.c = DI_GTF_DEFAULT_C,
			.k = DI_GTF_DEFAULT_K,
			.j = DI_GTF_DEFAULT_J,
		};
		di_gtf_compute(&gtf, &gtf_options);

		hbl = gtf.h_front_porch + gtf.h_sync + gtf.h_back_porch + 2 * gtf.h_border;
		vbl = gtf.v_front_porch + gtf.v_sync + gtf.v_back_porch + 2 * gtf.v_border;
		horiz_total = gtf.h_pixels + hbl;
		vert_total = gtf.v_lines + vbl;
		/* Upstream edid-decode rounds the pixel clock to kHz... */
		pixel_clock_khz = round(gtf.pixel_freq_mhz * 1000);
		refresh = (pixel_clock_khz * 1000) / (horiz_total * vert_total);
		horiz_freq_hz = (pixel_clock_khz * 1000) / horiz_total;
		pixel_clock_mhz = pixel_clock_khz / 1000;

		printf("GTF     ");
	}

	printf(":");
	printf(" %5dx%-5d", t->horiz_video, vert_video);
	printf(" %10.6f Hz", refresh);
	printf("  %s ", standard_timing_aspect_ratio_name(t->aspect_ratio));
	printf(" %8.3f kHz %13.6f MHz", horiz_freq_hz / 1000, pixel_clock_mhz);
	if (dmt != NULL && dmt->reduced_blanking)
		printf(" (RB)");
	printf("\n");
}

/**
 * Join a list of strings into a comma-separated string.
 *
 * The list must be NULL-terminated.
 */
static char *
join_str(const char *l[])
{
	char *out = NULL;
	size_t out_size = 0, i;
	FILE *f;

	f = open_memstream(&out, &out_size);
	if (!f) {
		return NULL;
	}

	for (i = 0; l[i] != NULL; i++) {
		if (i > 0) {
			fprintf(f, ", ");
		}
		fprintf(f, "%s", l[i]);
	}

	fclose(f);
	return out;
}

static bool
has_established_timings_i_ii(const struct di_edid_established_timings_i_ii *timings)
{
	return timings->has_720x400_70hz || timings->has_720x400_88hz
	       || timings->has_640x480_60hz || timings->has_640x480_67hz
	       || timings->has_640x480_72hz || timings->has_640x480_75hz
	       || timings->has_800x600_56hz || timings->has_800x600_60hz
	       || timings->has_800x600_72hz || timings->has_800x600_75hz
	       || timings->has_832x624_75hz || timings->has_1024x768_87hz_interlaced
	       || timings->has_1024x768_60hz || timings->has_1024x768_70hz
	       || timings->has_1024x768_75hz || timings->has_1280x1024_75hz
	       || timings->has_1152x870_75hz;
}

static const char *
detailed_timing_def_stereo_name(enum di_edid_detailed_timing_def_stereo stereo)
{
	switch (stereo) {
	case DI_EDID_DETAILED_TIMING_DEF_STEREO_NONE:
		return "none";
	case DI_EDID_DETAILED_TIMING_DEF_STEREO_FIELD_SEQ_RIGHT:
		return "field sequential L/R";
	case DI_EDID_DETAILED_TIMING_DEF_STEREO_FIELD_SEQ_LEFT:
		return "field sequential R/L";
	case DI_EDID_DETAILED_TIMING_DEF_STEREO_2_WAY_INTERLEAVED_RIGHT:
		return "interleaved right even";
	case DI_EDID_DETAILED_TIMING_DEF_STEREO_2_WAY_INTERLEAVED_LEFT:
		return "interleaved left even";
	case DI_EDID_DETAILED_TIMING_DEF_STEREO_4_WAY_INTERLEAVED:
		return "four way interleaved";
	case DI_EDID_DETAILED_TIMING_DEF_STEREO_SIDE_BY_SIDE_INTERLEAVED:
		return "side by side interleaved";
	}
	abort();
}

static const char *
detailed_timing_def_signal_type_name(enum di_edid_detailed_timing_def_signal_type type)
{
	switch (type) {
	case DI_EDID_DETAILED_TIMING_DEF_SIGNAL_ANALOG_COMPOSITE:
		return "analog composite";
	case DI_EDID_DETAILED_TIMING_DEF_SIGNAL_BIPOLAR_ANALOG_COMPOSITE:
		return "bipolar analog composite";
	case DI_EDID_DETAILED_TIMING_DEF_SIGNAL_DIGITAL_COMPOSITE:
		return "digital composite";
	case DI_EDID_DETAILED_TIMING_DEF_SIGNAL_DIGITAL_SEPARATE:
		/* edid-decode doesn't print anything in this case */
		return NULL;
	}
	abort();
}

static bool
detailed_timing_def_sync_serrations(const struct di_edid_detailed_timing_def *def)
{
	switch (def->signal_type) {
	case DI_EDID_DETAILED_TIMING_DEF_SIGNAL_ANALOG_COMPOSITE:
		return def->analog_composite->sync_serrations;
	case DI_EDID_DETAILED_TIMING_DEF_SIGNAL_BIPOLAR_ANALOG_COMPOSITE:
		return def->bipolar_analog_composite->sync_serrations;
	case DI_EDID_DETAILED_TIMING_DEF_SIGNAL_DIGITAL_COMPOSITE:
		return def->digital_composite->sync_serrations;
	default:
		return false;
	}
}

static bool
detailed_timing_def_sync_on_green(const struct di_edid_detailed_timing_def *def)
{
	switch (def->signal_type) {
	case DI_EDID_DETAILED_TIMING_DEF_SIGNAL_ANALOG_COMPOSITE:
		return def->analog_composite->sync_on_green;
	case DI_EDID_DETAILED_TIMING_DEF_SIGNAL_BIPOLAR_ANALOG_COMPOSITE:
		return def->bipolar_analog_composite->sync_on_green;
	default:
		return false;
	}
}

static const char *
detailed_timing_def_sync_polarity_name(enum di_edid_detailed_timing_def_sync_polarity polarity)
{
	switch (polarity) {
	case DI_EDID_DETAILED_TIMING_DEF_SYNC_NEGATIVE:
		return "N";
	case DI_EDID_DETAILED_TIMING_DEF_SYNC_POSITIVE:
		return "P";
	}
	abort();
}

void
print_detailed_timing_def(const struct di_edid_detailed_timing_def *def)
{
	int hbl, vbl, horiz_total, vert_total;
	int horiz_back_porch, vert_back_porch;
	int horiz_ratio, vert_ratio;
	double refresh, horiz_freq_hz;
	const char *flags[32] = {0};
	const char *signal_type_name;
	char size_mm[64];
	size_t flags_len = 0;
	enum di_edid_detailed_timing_def_sync_polarity polarity;

	hbl = def->horiz_blank - 2 * def->horiz_border;
	vbl = def->vert_blank - 2 * def->vert_border;
	horiz_total = def->horiz_video + hbl;
	vert_total = def->vert_video + vbl;
	refresh = (double) def->pixel_clock_hz / (horiz_total * vert_total);
	horiz_freq_hz = (double) def->pixel_clock_hz / horiz_total;

	compute_aspect_ratio(def->horiz_video, def->vert_video,
			     &horiz_ratio, &vert_ratio);

	signal_type_name = detailed_timing_def_signal_type_name(def->signal_type);
	if (signal_type_name != NULL) {
		flags[flags_len++] = signal_type_name;
	}
	if (detailed_timing_def_sync_serrations(def)) {
		flags[flags_len++] = "serrate";
	}
	if (detailed_timing_def_sync_on_green(def)) {
		flags[flags_len++] = "sync-on-green";
	}
	if (def->stereo != DI_EDID_DETAILED_TIMING_DEF_STEREO_NONE) {
		flags[flags_len++] = detailed_timing_def_stereo_name(def->stereo);
	}
	if (def->horiz_image_mm != 0 || def->vert_image_mm != 0) {
		snprintf(size_mm, sizeof(size_mm), "%d mm x %d mm",
			 def->horiz_image_mm, def->vert_image_mm);
		flags[flags_len++] = size_mm;
	}
	assert(flags_len < sizeof(flags) / sizeof(flags[0]));

	printf("    DTD %zu:", ++num_detailed_timing_defs);
	printf(" %5dx%-5d", def->horiz_video, def->vert_video);
	if (def->interlaced) {
		printf("i");
	}
	printf(" %10.6f Hz", refresh);
	printf(" %3u:%-3u", horiz_ratio, vert_ratio);
	printf(" %8.3f kHz %13.6f MHz", horiz_freq_hz / 1000,
	       (double) def->pixel_clock_hz / (1000 * 1000));
	if (flags_len > 0) {
		char *flags_str = join_str(flags);
		printf(" (%s)", flags_str);
		free(flags_str);
	}
	printf("\n");

	horiz_back_porch = hbl - def->horiz_sync_pulse - def->horiz_front_porch;
	printf("                 Hfront %4d Hsync %3d Hback %4d",
	       def->horiz_front_porch, def->horiz_sync_pulse, horiz_back_porch);
	if (def->horiz_border != 0) {
		printf(" Hborder %d", def->horiz_border);
	}
	if (def->signal_type == DI_EDID_DETAILED_TIMING_DEF_SIGNAL_DIGITAL_COMPOSITE) {
		polarity = def->digital_composite->sync_horiz_polarity;
		printf(" Hpol %s", detailed_timing_def_sync_polarity_name(polarity));
	} else if (def->signal_type == DI_EDID_DETAILED_TIMING_DEF_SIGNAL_DIGITAL_SEPARATE) {
		polarity = def->digital_separate->sync_horiz_polarity;
		printf(" Hpol %s", detailed_timing_def_sync_polarity_name(polarity));
	}
	printf("\n");

	vert_back_porch = vbl - def->vert_sync_pulse - def->vert_front_porch;
	printf("                 Vfront %4u Vsync %3u Vback %4d",
	       def->vert_front_porch, def->vert_sync_pulse, vert_back_porch);
	if (def->vert_border != 0) {
		printf(" Vborder %d", def->vert_border);
	}
	if (def->signal_type == DI_EDID_DETAILED_TIMING_DEF_SIGNAL_DIGITAL_SEPARATE) {
		polarity = def->digital_separate->sync_vert_polarity;
		printf(" Vpol %s", detailed_timing_def_sync_polarity_name(polarity));
	}
	printf("\n");
}

static const char *
display_desc_tag_name(enum di_edid_display_descriptor_tag tag)
{
	switch (tag) {
	case DI_EDID_DISPLAY_DESCRIPTOR_PRODUCT_SERIAL:
		return "Display Product Serial Number";
	case DI_EDID_DISPLAY_DESCRIPTOR_DATA_STRING:
		return "Alphanumeric Data String";
	case DI_EDID_DISPLAY_DESCRIPTOR_RANGE_LIMITS:
		return "Display Range Limits";
	case DI_EDID_DISPLAY_DESCRIPTOR_PRODUCT_NAME:
		return "Display Product Name";
	case DI_EDID_DISPLAY_DESCRIPTOR_COLOR_POINT:
		return "Color Point Data";
	case DI_EDID_DISPLAY_DESCRIPTOR_STD_TIMING_IDS:
		return "Standard Timing Identifications";
	case DI_EDID_DISPLAY_DESCRIPTOR_DCM_DATA:
		return "Display Color Management Data";
	case DI_EDID_DISPLAY_DESCRIPTOR_CVT_TIMING_CODES:
		return "CVT 3 Byte Timing Codes";
	case DI_EDID_DISPLAY_DESCRIPTOR_ESTABLISHED_TIMINGS_III:
		return "Established timings III";
	case DI_EDID_DISPLAY_DESCRIPTOR_DUMMY:
		return "Dummy Descriptor";
	}
	abort();
}

static const char *
display_range_limits_type_name(enum di_edid_display_range_limits_type type)
{
	switch (type) {
	case DI_EDID_DISPLAY_RANGE_LIMITS_BARE:
		return "Range Limits Only";
	case DI_EDID_DISPLAY_RANGE_LIMITS_DEFAULT_GTF:
		return "GTF";
	case DI_EDID_DISPLAY_RANGE_LIMITS_SECONDARY_GTF:
		return "Secondary GTF";
	case DI_EDID_DISPLAY_RANGE_LIMITS_CVT:
		return "CVT";
	}
	abort();
}

static const char *
cvt_aspect_ratio_name(enum di_edid_cvt_aspect_ratio aspect_ratio)
{
	switch (aspect_ratio) {
	case DI_EDID_CVT_ASPECT_RATIO_4_3:
		return "4:3";
	case DI_EDID_CVT_ASPECT_RATIO_16_9:
		return "16:9";
	case DI_EDID_CVT_ASPECT_RATIO_16_10:
		return "16:10";
	case DI_EDID_CVT_ASPECT_RATIO_5_4:
		return "5:4";
	case DI_EDID_CVT_ASPECT_RATIO_15_9:
		return "15:9";
	}
	abort();
}

static float
truncate_chromaticity_coord(float coord)
{
	return floorf(coord * 10000) / 10000;
}

static void
print_color_point(const struct di_edid_color_point *c)
{
	printf("Index: %u White: %.4f, %.4f ",
	       c->index,
	       truncate_chromaticity_coord(c->white_x),
	       truncate_chromaticity_coord(c->white_y));

	if (c->gamma != 0) {
		printf("Gamma: %.2f\n", c->gamma);
	} else {
		printf("Gamma: is defined in an extension block\n");
	}
}

static void
print_dmt_timing(const struct di_dmt_timing *t)
{
	int hbl, vbl, horiz_total, vert_total, horiz_ratio, vert_ratio;
	double refresh, horiz_freq_hz, pixel_clock_mhz;

	compute_aspect_ratio(t->horiz_video, t->vert_video,
			     &horiz_ratio, &vert_ratio);

	hbl = t->horiz_blank - 2 * t->horiz_border;
	vbl = t->vert_blank - 2 * t->vert_border;
	horiz_total = t->horiz_video + hbl;
	vert_total = t->vert_video + vbl;
	refresh = (double) t->pixel_clock_hz / (horiz_total * vert_total);
	horiz_freq_hz = (double) t->pixel_clock_hz / horiz_total;
	pixel_clock_mhz = (double) t->pixel_clock_hz / (1000 * 1000);

	printf("      DMT 0x%02x:", t->dmt_id);
	printf(" %5dx%-5d", t->horiz_video, t->vert_video);
	printf(" %10.6f Hz", refresh);
	printf(" %3u:%-3u", horiz_ratio, vert_ratio);
	printf(" %8.3f kHz %13.6f MHz", horiz_freq_hz / 1000, pixel_clock_mhz);
	if (t->reduced_blanking)
		printf(" (RB)");
	printf("\n");
}

static void
print_cvt_timing(struct di_cvt_timing *t, struct di_cvt_options *options,
		 int hratio, int vratio, bool preferred, bool rb)
{
	double hbl, htotal;

	hbl = t->h_front_porch + t->h_sync + t->h_back_porch;
	htotal = t->total_active_pixels + hbl;

	printf("      CVT: %5dx%-5d", (int)options->h_pixels, (int)options->v_lines);
	printf(" %10.6f Hz", t->act_frame_rate);
	printf(" %3u:%-3u", hratio, vratio);
	printf(" %8.3f kHz %13.6f MHz", t->act_pixel_freq * 1000 / htotal,
	       (double) t->act_pixel_freq);

	if (preferred || rb) {
		printf(" (%s%s%s)", rb ? "RB" : "",
				    (preferred && rb) ? ", " : "",
				    preferred ? "preferred vertical rate" : "");
	}

	printf("\n");
}

static void
print_cvt_timing_code(const struct di_edid_cvt_timing_code *t)
{
	struct di_cvt_timing timing;
	struct di_cvt_options options;
	enum di_edid_cvt_timing_code_preferred_vrate pref = t->preferred_vertical_rate;
	int hratio, vratio;

	options.int_rqd = false;
	options.margins_rqd = false;
	options.v_lines = t->addressable_lines_per_field;

	switch (t->aspect_ratio) {
	case DI_EDID_CVT_TIMING_CODE_4_3:
		hratio = 4;
		vratio = 3;
		break;
	case DI_EDID_CVT_TIMING_CODE_16_9:
		hratio = 16;
		vratio = 9;
		break;
	case DI_EDID_CVT_TIMING_CODE_16_10:
		hratio = 16;
		vratio = 10;
		break;
	case DI_EDID_CVT_TIMING_CODE_15_9:
		hratio = 15;
		vratio = 9;
		break;
	}

	options.h_pixels = 8 * (((options.v_lines * hratio) / vratio) / 8);

	if (t->supports_50hz_sb) {
		options.ip_freq_rqd = 50;
		options.red_blank_ver = DI_CVT_REDUCED_BLANKING_NONE;

		di_cvt_compute(&timing, &options);
		print_cvt_timing(&timing, &options, hratio, vratio,
				 pref == DI_EDID_CVT_TIMING_CODE_PREFERRED_VRATE_50HZ,
				 false);
	}
	if (t->supports_60hz_sb) {
		options.ip_freq_rqd = 60;
		options.red_blank_ver = DI_CVT_REDUCED_BLANKING_NONE;

		di_cvt_compute(&timing, &options);
		print_cvt_timing(&timing, &options, hratio, vratio,
				 pref == DI_EDID_CVT_TIMING_CODE_PREFERRED_VRATE_60HZ &&
				 !t->supports_60hz_rb,
				 false);
	}
	if (t->supports_75hz_sb) {
		options.ip_freq_rqd = 75;
		options.red_blank_ver = DI_CVT_REDUCED_BLANKING_NONE;

		di_cvt_compute(&timing, &options);
		print_cvt_timing(&timing, &options, hratio, vratio,
				 pref == DI_EDID_CVT_TIMING_CODE_PREFERRED_VRATE_75HZ,
				 false);
	}
	if (t->supports_85hz_sb) {
		options.ip_freq_rqd = 85;
		options.red_blank_ver = DI_CVT_REDUCED_BLANKING_NONE;

		di_cvt_compute(&timing, &options);
		print_cvt_timing(&timing, &options, hratio, vratio,
				 pref == DI_EDID_CVT_TIMING_CODE_PREFERRED_VRATE_85HZ,
				 false);
	}
	if (t->supports_60hz_rb) {
		options.ip_freq_rqd = 60;
		options.red_blank_ver = DI_CVT_REDUCED_BLANKING_V1;

		di_cvt_compute(&timing, &options);
		print_cvt_timing(&timing, &options, hratio, vratio,
				 pref == DI_EDID_CVT_TIMING_CODE_PREFERRED_VRATE_60HZ,
				 true);
	}
}

static void
print_display_desc(const struct di_edid *edid,
		   const struct di_edid_display_descriptor *desc)
{
	enum di_edid_display_descriptor_tag tag;
	const char *tag_name, *str;
	const struct di_edid_display_range_limits *range_limits;
	enum di_edid_display_range_limits_type range_limits_type;
	const struct di_edid_standard_timing *const *standard_timings;
	const struct di_edid_color_point *const *color_points;
	const struct di_dmt_timing *const *established_timings_iii;
	const struct di_edid_color_management_data *color_management_data;
	const struct di_edid_cvt_timing_code *const *cvt_timings;
	size_t i;

	tag = di_edid_display_descriptor_get_tag(desc);
	tag_name = display_desc_tag_name(tag);

	printf("    %s:", tag_name);

	switch (tag) {
	case DI_EDID_DISPLAY_DESCRIPTOR_PRODUCT_SERIAL:
	case DI_EDID_DISPLAY_DESCRIPTOR_DATA_STRING:
	case DI_EDID_DISPLAY_DESCRIPTOR_PRODUCT_NAME:
		str = di_edid_display_descriptor_get_string(desc);
		printf(" '%s'\n", str);
		break;
	case DI_EDID_DISPLAY_DESCRIPTOR_RANGE_LIMITS:
		range_limits = di_edid_display_descriptor_get_range_limits(desc);

		range_limits_type = range_limits->type;
		if (di_edid_get_revision(edid) < 4
		    && range_limits_type == DI_EDID_DISPLAY_RANGE_LIMITS_BARE) {
			/* edid-decode always prints "GTF" for EDID 1.3 and
			 * earlier even if the display doesn't support it */
			range_limits_type = DI_EDID_DISPLAY_RANGE_LIMITS_DEFAULT_GTF;
		}

		printf("\n      Monitor ranges (%s): %d-%d Hz V, %d-%d kHz H",
		       display_range_limits_type_name(range_limits_type),
		       range_limits->min_vert_rate_hz,
		       range_limits->max_vert_rate_hz,
		       range_limits->min_horiz_rate_hz / 1000,
		       range_limits->max_horiz_rate_hz / 1000);
		if (range_limits->max_pixel_clock_hz != 0) {
			printf(", max dotclock %"PRIi64" MHz",
			       range_limits->max_pixel_clock_hz / (1000 * 1000));
		}
		printf("\n");

		switch (range_limits_type) {
		case DI_EDID_DISPLAY_RANGE_LIMITS_SECONDARY_GTF:
			printf("      GTF Secondary Curve Block:\n");
			printf("        Start frequency: %u kHz\n",
			       range_limits->secondary_gtf->start_freq_hz / 1000);
			printf("        C: %.1f%%\n", range_limits->secondary_gtf->c);
			printf("        M: %u%%/kHz\n", (int) range_limits->secondary_gtf->m);
			printf("        K: %u\n", (int) range_limits->secondary_gtf->k);
			printf("        J: %.1f%%\n", range_limits->secondary_gtf->j);
			break;
		case DI_EDID_DISPLAY_RANGE_LIMITS_CVT:
			printf("      CVT version %d.%d\n",
			       range_limits->cvt->version,
			       range_limits->cvt->revision);

			if (range_limits->cvt->max_horiz_px != 0)
				printf("      Max active pixels per line: %d\n",
				       range_limits->cvt->max_horiz_px);

			printf("      Supported aspect ratios:");
			if (range_limits->cvt->supported_aspect_ratio & DI_EDID_CVT_ASPECT_RATIO_4_3)
				printf(" 4:3");
			if (range_limits->cvt->supported_aspect_ratio & DI_EDID_CVT_ASPECT_RATIO_16_9)
				printf(" 16:9");
			if (range_limits->cvt->supported_aspect_ratio & DI_EDID_CVT_ASPECT_RATIO_16_10)
				printf(" 16:10");
			if (range_limits->cvt->supported_aspect_ratio & DI_EDID_CVT_ASPECT_RATIO_5_4)
				printf(" 5:4");
			if (range_limits->cvt->supported_aspect_ratio & DI_EDID_CVT_ASPECT_RATIO_15_9)
				printf(" 15:9");
			printf("\n");

			printf("      Preferred aspect ratio: %s\n",
			       cvt_aspect_ratio_name(range_limits->cvt->preferred_aspect_ratio));

			if (range_limits->cvt->standard_blanking)
				printf("      Supports CVT standard blanking\n");
			if (range_limits->cvt->reduced_blanking)
				printf("      Supports CVT reduced blanking\n");

			if (range_limits->cvt->supported_scaling != 0) {
				printf("      Supported display scaling:\n");
				if (range_limits->cvt->supported_scaling & DI_EDID_CVT_SCALING_HORIZ_SHRINK)
					printf("        Horizontal shrink\n");
				if (range_limits->cvt->supported_scaling & DI_EDID_CVT_SCALING_HORIZ_STRETCH)
					printf("        Horizontal stretch\n");
				if (range_limits->cvt->supported_scaling & DI_EDID_CVT_SCALING_VERT_SHRINK)
					printf("        Vertical shrink\n");
				if (range_limits->cvt->supported_scaling & DI_EDID_CVT_SCALING_VERT_STRETCH)
					printf("        Vertical stretch\n");
			}

			printf("      Preferred vertical refresh: %d Hz\n",
			       range_limits->cvt->preferred_vert_refresh_hz);
			break;
		default:
			break;
		}
		break;
	case DI_EDID_DISPLAY_DESCRIPTOR_STD_TIMING_IDS:
		standard_timings = di_edid_display_descriptor_get_standard_timings(desc);

		printf("\n");
		for (i = 0; standard_timings[i] != NULL; i++) {
			printf("  ");
			print_standard_timing(standard_timings[i]);
		}
		break;
	case DI_EDID_DISPLAY_DESCRIPTOR_COLOR_POINT:
		color_points = di_edid_display_descriptor_get_color_points(desc);

		for (i = 0; color_points[i] != NULL; i++) {
			printf("      ");
			print_color_point(color_points[i]);
		}

		uncommon_features.color_point_descriptor = true;
		break;
	case DI_EDID_DISPLAY_DESCRIPTOR_ESTABLISHED_TIMINGS_III:
		established_timings_iii = di_edid_display_descriptor_get_established_timings_iii(desc);

		printf("\n");
		for (i = 0; established_timings_iii[i] != NULL; i++) {
			print_dmt_timing(established_timings_iii[i]);
		}
		break;
	case DI_EDID_DISPLAY_DESCRIPTOR_DCM_DATA:
		color_management_data = di_edid_display_descriptor_get_color_management_data(desc);

		printf("      Version : %d\n", color_management_data->version);
		printf("      Red a3  : %.2f\n", color_management_data->red_a3);
		printf("      Red a2  : %.2f\n", color_management_data->red_a2);
		printf("      Green a3: %.2f\n", color_management_data->green_a3);
		printf("      Green a2: %.2f\n", color_management_data->green_a2);
		printf("      Blue a3 : %.2f\n", color_management_data->blue_a3);
		printf("      Blue a2 : %.2f\n", color_management_data->blue_a2);

		uncommon_features.color_management_data = true;
		break;
	case DI_EDID_DISPLAY_DESCRIPTOR_CVT_TIMING_CODES:
		cvt_timings = di_edid_display_descriptor_get_cvt_timing_codes(desc);

		printf("\n");
		for (i = 0; cvt_timings[i] != NULL; i++)
			print_cvt_timing_code(cvt_timings[i]);
		break;
	default:
		printf("\n");
		break; /* TODO: print other tags */
	}
}

static const char *
analog_signal_level_std_name(enum di_edid_video_input_analog_signal_level_std std)
{
	switch (std) {
	case DI_EDID_VIDEO_INPUT_ANALOG_SIGNAL_LEVEL_0:
		return "0.700 : 0.300 : 1.000 V p-p";
	case DI_EDID_VIDEO_INPUT_ANALOG_SIGNAL_LEVEL_1:
		return "0.714 : 0.286 : 1.000 V p-p";
	case DI_EDID_VIDEO_INPUT_ANALOG_SIGNAL_LEVEL_2:
		return "1.000 : 0.400 : 1.400 V p-p";
	case DI_EDID_VIDEO_INPUT_ANALOG_SIGNAL_LEVEL_3:
		return "0.700 : 0.000 : 0.700 V p-p";
	}
	abort();
}

static const char *
digital_interface_name(enum di_edid_video_input_digital_interface interface)
{
	switch (interface) {
	case DI_EDID_VIDEO_INPUT_DIGITAL_UNDEFINED:
		return "Digital interface is not defined";
	case DI_EDID_VIDEO_INPUT_DIGITAL_DVI:
		return "DVI interface";
	case DI_EDID_VIDEO_INPUT_DIGITAL_HDMI_A:
		return "HDMI-a interface";
	case DI_EDID_VIDEO_INPUT_DIGITAL_HDMI_B:
		return "HDMI-b interface";
	case DI_EDID_VIDEO_INPUT_DIGITAL_MDDI:
		return "MDDI interface";
	case DI_EDID_VIDEO_INPUT_DIGITAL_DISPLAYPORT:
		return "DisplayPort interface";
	}
	abort();
}

static const char *
display_color_type_name(enum di_edid_display_color_type type)
{
	switch (type) {
	case DI_EDID_DISPLAY_COLOR_MONOCHROME:
		return "Monochrome or grayscale display";
	case DI_EDID_DISPLAY_COLOR_RGB:
		return "RGB color display";
	case DI_EDID_DISPLAY_COLOR_NON_RGB:
		return "Non-RGB color display";
	case DI_EDID_DISPLAY_COLOR_UNDEFINED:
		return "Undefined display color type";
	}
	abort();
}

void
print_edid(const struct di_edid *edid)
{
	const struct di_edid_vendor_product *vendor_product;
	const struct di_edid_video_input_analog *video_input_analog;
	const struct di_edid_video_input_digital *video_input_digital;
	const struct di_edid_screen_size *screen_size;
	float gamma;
	const struct di_edid_dpms *dpms;
	enum di_edid_display_color_type display_color_type;
	const struct di_edid_color_encoding_formats *color_encoding_formats;
	const struct di_edid_misc_features *misc_features;
	const struct di_edid_chromaticity_coords *chromaticity_coords;
	const struct di_edid_established_timings_i_ii *established_timings_i_ii;
	const struct di_edid_standard_timing *const *standard_timings;
	const struct di_edid_detailed_timing_def *const *detailed_timing_defs;
	const struct di_edid_display_descriptor *const *display_descs;
	size_t i;

	printf("Block 0, Base EDID:\n");
	printf("  EDID Structure Version & Revision: %d.%d\n",
	       di_edid_get_version(edid), di_edid_get_revision(edid));

	vendor_product = di_edid_get_vendor_product(edid);
	printf("  Vendor & Product Identification:\n");
	printf("    Manufacturer: %.3s\n", vendor_product->manufacturer);
	printf("    Model: %" PRIu16 "\n", vendor_product->product);
	if (vendor_product->serial != 0) {
		printf("    Serial Number: %" PRIu32 " (0x%08x)\n",
		       vendor_product->serial, vendor_product->serial);
	}
	if (vendor_product->model_year != 0) {
		printf("    Model year: %d\n", vendor_product->model_year);
	} else if (vendor_product->manufacture_week != 0) {
		printf("    Made in: week %d of %d\n",
		       vendor_product->manufacture_week,
		       vendor_product->manufacture_year);
	} else {
		printf("    Made in: %d\n", vendor_product->manufacture_year);
	}

	printf("  Basic Display Parameters & Features:\n");
	video_input_analog = di_edid_get_video_input_analog(edid);
	if (video_input_analog) {
		printf("    Analog display\n");
		printf("    Signal Level Standard: %s\n",
		       analog_signal_level_std_name(video_input_analog->signal_level_std));
		switch (video_input_analog->video_setup) {
		case DI_EDID_VIDEO_INPUT_ANALOG_BLANK_LEVEL_EQ_BLACK:
			printf("    Blank level equals black level\n");
			break;
		case DI_EDID_VIDEO_INPUT_ANALOG_BLANK_TO_BLACK_SETUP_PEDESTAL:
			printf("    Blank-to-black setup/pedestal\n");
			break;
		}
		printf("    Sync:");
		if (video_input_analog->sync_separate)
			printf(" Separate");
		if (video_input_analog->sync_composite)
			printf(" Composite");
		if (video_input_analog->sync_on_green)
			printf(" SyncOnGreen");
		if (video_input_analog->sync_serrations)
			printf(" Serration");
		printf("\n");
	}
	video_input_digital = di_edid_get_video_input_digital(edid);
	if (video_input_digital) {
		printf("    Digital display\n");
		if (di_edid_get_revision(edid) >= 4) {
			if (video_input_digital->color_bit_depth == 0) {
				printf("    Color depth is undefined\n");
			} else {
				printf("    Bits per primary color channel: %d\n",
				       video_input_digital->color_bit_depth);
			}
			printf("    %s\n",
			       digital_interface_name(video_input_digital->interface));
		}
		if (video_input_digital->dfp1)
			printf("    DFP 1.x compatible TMDS\n");
	}
	screen_size = di_edid_get_screen_size(edid);
	if (screen_size->width_cm > 0) {
		printf("    Maximum image size: %d cm x %d cm\n",
		       screen_size->width_cm, screen_size->height_cm);
	} else if (screen_size->landscape_aspect_ratio > 0) {
		printf("    Aspect ratio: %.2f (landscape)\n",
		       screen_size->landscape_aspect_ratio);
	} else if (screen_size->portait_aspect_ratio > 0) {
		printf("    Aspect ratio: %.2f (portrait)\n",
		       screen_size->portait_aspect_ratio);
	} else {
		printf("    Image size is variable\n");
	}

	gamma = di_edid_get_basic_gamma(edid);
	if (gamma != 0) {
		printf("    Gamma: %.2f\n", gamma);
	} else {
		printf("    Gamma is defined in an extension block\n");
	}

	dpms = di_edid_get_dpms(edid);
	if (dpms->standby || dpms->suspend || dpms->off) {
		printf("    DPMS levels:");
		if (dpms->standby) {
			printf(" Standby");
		}
		if (dpms->suspend) {
			printf(" Suspend");
		}
		if (dpms->off) {
			printf(" Off");
		}
		printf("\n");
	}

	if (!video_input_digital || di_edid_get_revision(edid) < 4) {
		display_color_type = di_edid_get_display_color_type(edid);
		printf("    %s\n", display_color_type_name(display_color_type));
	}

	color_encoding_formats = di_edid_get_color_encoding_formats(edid);
	if (color_encoding_formats) {
		assert(color_encoding_formats->rgb444);
		printf("    Supported color formats: RGB 4:4:4");
		if (color_encoding_formats->ycrcb444) {
			printf(", YCrCb 4:4:4");
		}
		if (color_encoding_formats->ycrcb422) {
			printf(", YCrCb 4:2:2");
		}
		printf("\n");
	}

	misc_features = di_edid_get_misc_features(edid);
	if (misc_features->srgb_is_primary) {
		printf("    Default (sRGB) color space is primary color space\n");
	}
	if (di_edid_get_revision(edid) >= 4) {
		assert(misc_features->has_preferred_timing);
		if (misc_features->preferred_timing_is_native) {
			printf("    First detailed timing includes the native "
			       "pixel format and preferred refresh rate\n");
		} else {
			printf("    First detailed timing does not include the "
			       "native pixel format and preferred refresh rate\n");
		}
	} else {
		if (misc_features->has_preferred_timing) {
			printf("    First detailed timing is the preferred timing\n");
		}
	}
	if (misc_features->continuous_freq) {
		printf("    Display supports continuous frequencies\n");
	}
	if (misc_features->default_gtf) {
		printf("    Supports GTF timings within operating range\n");
	}

	/* edid-decode truncates the result, but %f rounds it */
	chromaticity_coords = di_edid_get_chromaticity_coords(edid);
	printf("  Color Characteristics:\n");
	printf("    Red  : %.4f, %.4f\n",
	       truncate_chromaticity_coord(chromaticity_coords->red_x),
	       truncate_chromaticity_coord(chromaticity_coords->red_y));
	printf("    Green: %.4f, %.4f\n",
	       truncate_chromaticity_coord(chromaticity_coords->green_x),
	       truncate_chromaticity_coord(chromaticity_coords->green_y));
	printf("    Blue : %.4f, %.4f\n",
	       truncate_chromaticity_coord(chromaticity_coords->blue_x),
	       truncate_chromaticity_coord(chromaticity_coords->blue_y));
	printf("    White: %.4f, %.4f\n",
	       truncate_chromaticity_coord(chromaticity_coords->white_x),
	       truncate_chromaticity_coord(chromaticity_coords->white_y));

	printf("  Established Timings I & II:");
	established_timings_i_ii = di_edid_get_established_timings_i_ii(edid);
	if (!has_established_timings_i_ii(established_timings_i_ii)) {
		printf(" none");
	}
	printf("\n");
	if (established_timings_i_ii->has_720x400_70hz)
		printf("    IBM     :   720x400    70.081663 Hz   9:5     31.467 kHz     28.320000 MHz\n");
	if (established_timings_i_ii->has_720x400_88hz)
		printf("    IBM     :   720x400    87.849542 Hz   9:5     39.444 kHz     35.500000 MHz\n");
	if (established_timings_i_ii->has_640x480_60hz)
		printf("    DMT 0x04:   640x480    59.940476 Hz   4:3     31.469 kHz     25.175000 MHz\n");
	if (established_timings_i_ii->has_640x480_67hz)
		printf("    Apple   :   640x480    66.666667 Hz   4:3     35.000 kHz     30.240000 MHz\n");
	if (established_timings_i_ii->has_640x480_72hz)
		printf("    DMT 0x05:   640x480    72.808802 Hz   4:3     37.861 kHz     31.500000 MHz\n");
	if (established_timings_i_ii->has_640x480_75hz)
		printf("    DMT 0x06:   640x480    75.000000 Hz   4:3     37.500 kHz     31.500000 MHz\n");
	if (established_timings_i_ii->has_800x600_56hz)
		printf("    DMT 0x08:   800x600    56.250000 Hz   4:3     35.156 kHz     36.000000 MHz\n");
	if (established_timings_i_ii->has_800x600_60hz)
		printf("    DMT 0x09:   800x600    60.316541 Hz   4:3     37.879 kHz     40.000000 MHz\n");
	if (established_timings_i_ii->has_800x600_72hz)
		printf("    DMT 0x0a:   800x600    72.187572 Hz   4:3     48.077 kHz     50.000000 MHz\n");
	if (established_timings_i_ii->has_800x600_75hz)
		printf("    DMT 0x0b:   800x600    75.000000 Hz   4:3     46.875 kHz     49.500000 MHz\n");
	if (established_timings_i_ii->has_832x624_75hz)
		printf("    Apple   :   832x624    74.551266 Hz   4:3     49.726 kHz     57.284000 MHz\n");
	if (established_timings_i_ii->has_1024x768_87hz_interlaced)
		printf("    DMT 0x0f:  1024x768i   86.957532 Hz   4:3     35.522 kHz     44.900000 MHz\n");
	if (established_timings_i_ii->has_1024x768_60hz)
		printf("    DMT 0x10:  1024x768    60.003840 Hz   4:3     48.363 kHz     65.000000 MHz\n");
	if (established_timings_i_ii->has_1024x768_70hz)
		printf("    DMT 0x11:  1024x768    70.069359 Hz   4:3     56.476 kHz     75.000000 MHz\n");
	if (established_timings_i_ii->has_1024x768_75hz)
		printf("    DMT 0x12:  1024x768    75.028582 Hz   4:3     60.023 kHz     78.750000 MHz\n");
	if (established_timings_i_ii->has_1280x1024_75hz)
		printf("    DMT 0x24:  1280x1024   75.024675 Hz   5:4     79.976 kHz    135.000000 MHz\n");
	if (established_timings_i_ii->has_1152x870_75hz)
		printf("    Apple   :  1152x870    75.061550 Hz 192:145   68.681 kHz    100.000000 MHz\n");

	printf("  Standard Timings:");
	standard_timings = di_edid_get_standard_timings(edid);
	if (standard_timings[0] == NULL) {
		printf(" none");
	}
	printf("\n");
	for (i = 0; standard_timings[i] != NULL; i++) {
		print_standard_timing(standard_timings[i]);
	}

	printf("  Detailed Timing Descriptors:\n");
	detailed_timing_defs = di_edid_get_detailed_timing_defs(edid);
	for (i = 0; detailed_timing_defs[i] != NULL; i++) {
		print_detailed_timing_def(detailed_timing_defs[i]);
	}
	display_descs = di_edid_get_display_descriptors(edid);
	for (i = 0; display_descs[i] != NULL; i++) {
		print_display_desc(edid, display_descs[i]);
	}
}
