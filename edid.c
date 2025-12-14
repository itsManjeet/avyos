#include <assert.h>
#include <errno.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

#include "bits.h"
#include "dmt.h"
#include "edid.h"
#include "log.h"

/**
 * The size of an EDID block, defined in section 2.2.
 */
#define EDID_BLOCK_SIZE 128
/**
 * The size of an EDID standard timing, defined in section 3.9.
 */
#define EDID_STANDARD_TIMING_SIZE 2
/**
 * The size of an EDID CVT timing code, defined in section 3.10.3.8.
 */
#define EDID_CVT_TIMING_CODE_SIZE 3

/**
 * Fixed EDID header, defined in section 3.1.
 */
static const uint8_t header[] = { 0x00, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00 };

static void
add_failure(struct di_edid *edid, const char fmt[], ...)
{
	va_list args;

	va_start(args, fmt);
	_di_logger_va_add_failure(edid->logger, fmt, args);
	va_end(args);
}

static void
add_failure_until(struct di_edid *edid, int revision, const char fmt[], ...)
{
	va_list args;

	if (edid->revision > revision) {
		return;
	}

	va_start(args, fmt);
	_di_logger_va_add_failure(edid->logger, fmt, args);
	va_end(args);
}

static void
parse_version_revision(const uint8_t data[static EDID_BLOCK_SIZE],
		       int *version, int *revision)
{
	*version = (int) data[0x12];
	*revision = (int) data[0x13];
}

static size_t
parse_ext_count(const uint8_t data[static EDID_BLOCK_SIZE])
{
	return data[0x7E];
}

static bool
validate_block_checksum(const uint8_t data[static EDID_BLOCK_SIZE])
{
	uint8_t sum = 0;
	size_t i;

	for (i = 0; i < EDID_BLOCK_SIZE; i++) {
		sum += data[i];
	}

	return sum == 0;
}

static void
parse_vendor_product(struct di_edid *edid,
		     const uint8_t data[static EDID_BLOCK_SIZE])
{
	struct di_edid_vendor_product *out = &edid->vendor_product;
	uint16_t man, raw_week, raw_year;
	int year = 0;

	/* The ASCII 3-letter manufacturer code is encoded in 5-bit codes. */
	man = (uint16_t) ((data[0x08] << 8) | data[0x09]);
	out->manufacturer[0] = ((man >> 10) & 0x1F) + '@';
	out->manufacturer[1] = ((man >> 5) & 0x1F) + '@';
	out->manufacturer[2] = ((man >> 0) & 0x1F) + '@';

	out->product = (uint16_t) (data[0x0A] | (data[0x0B] << 8));
	out->serial = (uint32_t) (data[0x0C] |
				  (data[0x0D] << 8) |
				  (data[0x0E] << 16) |
				  (data[0x0F] << 24));

	raw_week = data[0x10];
	raw_year = data[0x11];

	if (raw_year >= 0x10 || edid->revision < 4) {
		year = data[0x11] + 1990;
	} else if (edid->revision == 4) {
		add_failure(edid, "Year set to reserved value.");
	}

	if (raw_week == 0xFF) {
		/* Special flag for model year */
		out->model_year = year;
	} else {
		out->manufacture_year = year;
		if (raw_week > 54) {
			add_failure_until(edid, 4,
					  "Invalid week %u of manufacture.",
					  raw_week);
		} else if (raw_week > 0) {
			out->manufacture_week = raw_week;
		}
	}
}

static void
parse_video_input_digital(struct di_edid *edid, uint8_t video_input)
{
	uint8_t color_bit_depth, interface;
	struct di_edid_video_input_digital *digital = &edid->video_input_digital;

	if (edid->revision < 2) {
		if (get_bit_range(video_input, 6, 0) != 0)
			add_failure(edid, "Digital Video Interface Standard set to reserved value 0x%02x.",
				    video_input);
		return;
	}
	if (edid->revision < 4) {
		if (get_bit_range(video_input, 6, 1) != 0)
			add_failure(edid, "Digital Video Interface Standard set to reserved value 0x%02x.",
				    video_input);
		digital->dfp1 = has_bit(video_input, 0);
		return;
	}

	color_bit_depth = get_bit_range(video_input, 6, 4);
	if (color_bit_depth == 0x07) {
		/* Reserved */
		add_failure_until(edid, 4, "Color Bit Depth set to reserved value.");
	} else if (color_bit_depth != 0) {
		digital->color_bit_depth = 2 * color_bit_depth + 4;
	}

	interface = get_bit_range(video_input, 3, 0);
	switch (interface) {
	case DI_EDID_VIDEO_INPUT_DIGITAL_UNDEFINED:
	case DI_EDID_VIDEO_INPUT_DIGITAL_DVI:
	case DI_EDID_VIDEO_INPUT_DIGITAL_HDMI_A:
	case DI_EDID_VIDEO_INPUT_DIGITAL_HDMI_B:
	case DI_EDID_VIDEO_INPUT_DIGITAL_MDDI:
	case DI_EDID_VIDEO_INPUT_DIGITAL_DISPLAYPORT:
		digital->interface = interface;
		break;
	default:
		add_failure_until(edid, 4,
				  "Digital Video Interface Standard set to reserved value 0x%02x.",
				  interface);
		digital->interface = DI_EDID_VIDEO_INPUT_DIGITAL_UNDEFINED;
		break;
	}
}

static void
parse_video_input_analog(struct di_edid *edid, uint8_t video_input)
{
	struct di_edid_video_input_analog *analog = &edid->video_input_analog;

	analog->signal_level_std = get_bit_range(video_input, 6, 5);
	analog->video_setup = has_bit(video_input, 4);
	analog->sync_separate = has_bit(video_input, 3);
	analog->sync_composite = has_bit(video_input, 2);
	analog->sync_on_green = has_bit(video_input, 1);
	analog->sync_serrations = has_bit(video_input, 0);
}

static void
parse_basic_params_features(struct di_edid *edid,
			    const uint8_t data[static EDID_BLOCK_SIZE])
{
	uint8_t video_input, width, height, features;
	struct di_edid_screen_size *screen_size = &edid->screen_size;

	video_input = data[0x14];
	edid->is_digital = has_bit(video_input, 7);

	if (edid->is_digital) {
		parse_video_input_digital(edid, video_input);
	} else {
		parse_video_input_analog(edid, video_input);
	}

	/* v1.3 says screen size is undefined if either byte is zero, v1.4 says
	 * screen size and aspect ratio are undefined if both bytes are zero and
	 * encodes the aspect ratio if either byte is zero. */
	width = data[0x15];
	height = data[0x16];
	if (width > 0 && height > 0) {
		screen_size->width_cm = width;
		screen_size->height_cm = height;
	} else if (edid->revision >= 4) {
		if (width > 0) {
			screen_size->landscape_aspect_ratio = ((float) width + 99) / 100;
		} else if (height > 0) {
			screen_size->portait_aspect_ratio = ((float) height + 99) / 100;
		}
	}

	if (data[0x17] != 0xFF) {
		edid->gamma = ((float) data[0x17] + 100) / 100;
	} else {
		edid->gamma = 0;
	}

	features = data[0x18];

	edid->dpms.standby = has_bit(features, 7);
	edid->dpms.suspend = has_bit(features, 6);
	edid->dpms.off = has_bit(features, 5);

	if (edid->is_digital && edid->revision >= 4) {
		edid->color_encoding_formats.rgb444 = true;
		edid->color_encoding_formats.ycrcb444 = has_bit(features, 3);
		edid->color_encoding_formats.ycrcb422 = has_bit(features, 4);
		edid->display_color_type = DI_EDID_DISPLAY_COLOR_UNDEFINED;
	} else {
		edid->display_color_type = get_bit_range(features, 4, 3);
	}

	if (edid->revision >= 4) {
		edid->misc_features.has_preferred_timing = true;
		edid->misc_features.continuous_freq = has_bit(features, 0);
		edid->misc_features.preferred_timing_is_native = has_bit(features, 1);
	} else {
		edid->misc_features.default_gtf = has_bit(features, 0);
		edid->misc_features.has_preferred_timing = has_bit(features, 1);
	}
	edid->misc_features.srgb_is_primary = has_bit(features, 2);
}

static float
decode_chromaticity_coord(uint8_t hi, uint8_t lo)
{
	uint16_t raw; /* only 10 bits are used */

	raw = (uint16_t) ((hi << 2) | lo);
	return (float) raw / 1024;
}

static void
parse_chromaticity_coords(struct di_edid *edid,
			  const uint8_t data[static EDID_BLOCK_SIZE])
{
	uint8_t lo;
	bool all_set, any_set;
	struct di_edid_chromaticity_coords *coords;

	coords = &edid->chromaticity_coords;

	lo = data[0x19];
	coords->red_x = decode_chromaticity_coord(data[0x1B], get_bit_range(lo, 7, 6));
	coords->red_y = decode_chromaticity_coord(data[0x1C], get_bit_range(lo, 5, 4));
	coords->green_x = decode_chromaticity_coord(data[0x1D], get_bit_range(lo, 3, 2));
	coords->green_y = decode_chromaticity_coord(data[0x1E], get_bit_range(lo, 1, 0));

	lo = data[0x1A];
	coords->blue_x = decode_chromaticity_coord(data[0x1F], get_bit_range(lo, 7, 6));
	coords->blue_y = decode_chromaticity_coord(data[0x20], get_bit_range(lo, 5, 4));
	coords->white_x = decode_chromaticity_coord(data[0x21], get_bit_range(lo, 3, 2));
	coords->white_y = decode_chromaticity_coord(data[0x22], get_bit_range(lo, 1, 0));

	/* Either all primaries coords must be set, either none must be set */
	any_set = coords->red_x != 0 || coords->red_y != 0
		  || coords->green_x != 0 || coords->green_y != 0
		  || coords->blue_x != 0 || coords->blue_y != 0;
	all_set = coords->red_x != 0 && coords->red_y != 0
		  && coords->green_x != 0 && coords->green_y != 0
		  && coords->blue_x != 0 && coords->blue_y != 0;
	if (any_set && !all_set) {
		add_failure(edid, "Some but not all primaries coordinates are unset.");
	}

	/* Both white-point coords must be set */
	if (coords->white_x == 0 || coords->white_y == 0) {
		add_failure(edid, "White-point coordinates are unset.");
	}
}

static void
parse_established_timings_i_ii(struct di_edid *edid,
			       const uint8_t data[static EDID_BLOCK_SIZE])
{
	struct di_edid_established_timings_i_ii *timings = &edid->established_timings_i_ii;

	timings->has_720x400_70hz = has_bit(data[0x23], 7);
	timings->has_720x400_88hz = has_bit(data[0x23], 6);
	timings->has_640x480_60hz = has_bit(data[0x23], 5);
	timings->has_640x480_67hz = has_bit(data[0x23], 4);
	timings->has_640x480_72hz = has_bit(data[0x23], 3);
	timings->has_640x480_75hz = has_bit(data[0x23], 2);
	timings->has_800x600_56hz = has_bit(data[0x23], 1);
	timings->has_800x600_60hz = has_bit(data[0x23], 0);

	/* Established timings II */
	timings->has_800x600_72hz = has_bit(data[0x24], 7);
	timings->has_800x600_75hz = has_bit(data[0x24], 6);
	timings->has_832x624_75hz = has_bit(data[0x24], 5);
	timings->has_1024x768_87hz_interlaced = has_bit(data[0x24], 4);
	timings->has_1024x768_60hz = has_bit(data[0x24], 3);
	timings->has_1024x768_70hz = has_bit(data[0x24], 2);
	timings->has_1024x768_75hz = has_bit(data[0x24], 1);
	timings->has_1280x1024_75hz = has_bit(data[0x24], 0);

	timings->has_1152x870_75hz = has_bit(data[0x25], 7);
	/* TODO: manufacturer specified timings in bits 6:0 */
}

static bool
parse_standard_timing(struct di_edid *edid,
		      const uint8_t data[static EDID_STANDARD_TIMING_SIZE],
		      struct di_edid_standard_timing **out)
{
	struct di_edid_standard_timing *t;

	*out = NULL;

	if (data[0] == 0x01 && data[1] == 0x01) {
		/* Unused */
		return true;
	}
	if (data[0] == 0x00) {
		add_failure_until(edid, 4,
				  "Use 0x0101 as the invalid Standard Timings code, not 0x%02x%02x.",
				  data[0], data[1]);
		return true;
	}

	t = calloc(1, sizeof(*t));
	if (!t) {
		return false;
	}

	t->horiz_video = ((int32_t) data[0] + 31) * 8;
	t->aspect_ratio = get_bit_range(data[1], 7, 6);
	t->refresh_rate_hz = (int32_t) get_bit_range(data[1], 5, 0) + 60;

	*out = t;
	return true;
}

struct di_edid_detailed_timing_def_priv *
_di_edid_parse_detailed_timing_def(const uint8_t data[static EDID_BYTE_DESCRIPTOR_SIZE])
{
	struct di_edid_detailed_timing_def_priv *priv;
	struct di_edid_detailed_timing_def *def;
	struct di_edid_detailed_timing_analog_composite *analog_composite;
	struct di_edid_detailed_timing_bipolar_analog_composite *bipolar_analog_composite;
	struct di_edid_detailed_timing_digital_composite *digital_composite;
	struct di_edid_detailed_timing_digital_separate *digital_separate;
	int raw;
	uint8_t flags, stereo_hi, stereo_lo;

	priv = calloc(1, sizeof(*priv));
	if (!priv) {
		return NULL;
	}

	def = &priv->base;

	raw = (data[1] << 8) | data[0];
	def->pixel_clock_hz = raw * 10 * 1000;

	def->horiz_video = (get_bit_range(data[4], 7, 4) << 8) | data[2];
	def->horiz_blank = (get_bit_range(data[4], 3, 0) << 8) | data[3];

	def->vert_video = (get_bit_range(data[7], 7, 4) << 8) | data[5];
	def->vert_blank = (get_bit_range(data[7], 3, 0) << 8) | data[6];

	def->horiz_front_porch = (get_bit_range(data[11], 7, 6) << 8) | data[8];
	def->horiz_sync_pulse = (get_bit_range(data[11], 5, 4) << 8) | data[9];
	def->vert_front_porch = (get_bit_range(data[11], 3, 2) << 4)
				| get_bit_range(data[10], 7, 4);
	def->vert_sync_pulse = (get_bit_range(data[11], 1, 0) << 4)
			       | get_bit_range(data[10], 3, 0);

	def->horiz_image_mm = (get_bit_range(data[14], 7, 4) << 8) | data[12];
	def->vert_image_mm = (get_bit_range(data[14], 3, 0) << 8) | data[13];
	if ((def->horiz_image_mm == 16 && def->vert_image_mm == 9)
	    || (def->horiz_image_mm == 4 && def->vert_image_mm == 3)) {
		/* Table 3.21 note 18.2: these are special cases and define the
		 * aspect ratio rather than the size in mm.
		 * TODO: expose these values */
		def->horiz_image_mm = def->vert_image_mm = 0;
	}

	def->horiz_border = data[15];
	def->vert_border = data[16];

	flags = data[17];

	def->interlaced = has_bit(flags, 7);

	stereo_hi = get_bit_range(flags, 6, 5);
	stereo_lo = get_bit_range(flags, 0, 0);
	if (stereo_hi == 0) {
		def->stereo = DI_EDID_DETAILED_TIMING_DEF_STEREO_NONE;
	} else {
		switch ((stereo_hi << 1) | stereo_lo) {
		case (1 << 1) | 0:
			def->stereo = DI_EDID_DETAILED_TIMING_DEF_STEREO_FIELD_SEQ_RIGHT;
			break;
		case (2 << 1) | 0:
			def->stereo = DI_EDID_DETAILED_TIMING_DEF_STEREO_FIELD_SEQ_LEFT;
			break;
		case (1 << 1) | 1:
			def->stereo = DI_EDID_DETAILED_TIMING_DEF_STEREO_2_WAY_INTERLEAVED_RIGHT;
			break;
		case (2 << 1) | 1:
			def->stereo = DI_EDID_DETAILED_TIMING_DEF_STEREO_2_WAY_INTERLEAVED_LEFT;
			break;
		case (3 << 1) | 0:
			def->stereo = DI_EDID_DETAILED_TIMING_DEF_STEREO_4_WAY_INTERLEAVED;
			break;
		case (3 << 1) | 1:
			def->stereo = DI_EDID_DETAILED_TIMING_DEF_STEREO_SIDE_BY_SIDE_INTERLEAVED;
			break;
		default:
			abort(); /* unreachable */
		}
	}

	def->signal_type = get_bit_range(flags, 4, 3);

	switch (def->signal_type) {
	case DI_EDID_DETAILED_TIMING_DEF_SIGNAL_ANALOG_COMPOSITE:
		analog_composite = &priv->analog_composite;
		analog_composite->sync_serrations = has_bit(flags, 2);
		analog_composite->sync_on_green = !has_bit(flags, 1);
		def->analog_composite = analog_composite;
		break;
	case DI_EDID_DETAILED_TIMING_DEF_SIGNAL_BIPOLAR_ANALOG_COMPOSITE:
		bipolar_analog_composite = &priv->bipolar_analog_composite;
		bipolar_analog_composite->sync_serrations = has_bit(flags, 2);
		bipolar_analog_composite->sync_on_green = !has_bit(flags, 1);
		def->bipolar_analog_composite = bipolar_analog_composite;
		break;
	case DI_EDID_DETAILED_TIMING_DEF_SIGNAL_DIGITAL_COMPOSITE:
		digital_composite = &priv->digital_composite;
		digital_composite->sync_serrations = has_bit(flags, 2);
		digital_composite->sync_horiz_polarity = has_bit(flags, 1);
		def->digital_composite = digital_composite;
		break;
	case DI_EDID_DETAILED_TIMING_DEF_SIGNAL_DIGITAL_SEPARATE:
		digital_separate = &priv->digital_separate;
		digital_separate->sync_vert_polarity = has_bit(flags, 2);
		digital_separate->sync_horiz_polarity = has_bit(flags, 1);
		def->digital_separate = digital_separate;
		break;
	}

	return priv;
}

static bool
decode_display_range_limits_offset(struct di_edid *edid, uint8_t flags,
				  int *max_offset, int *min_offset)
{
	switch (flags) {
	case 0x00:
		/* No offset */
		break;
	case 0x02:
		*max_offset = 255;
		break;
	case 0x03:
		*max_offset = 255;
		*min_offset = 255;
		break;
	default:
		add_failure_until(edid, 4,
				  "Range offset flags set to reserved value 0x%02x.",
				  flags);
		return false;
	}

	return true;
}

static bool
parse_display_range_limits(struct di_edid *edid,
			   const uint8_t data[static EDID_BYTE_DESCRIPTOR_SIZE],
			   struct di_edid_display_range_limits_priv *priv)
{
	uint8_t offset_flags, vert_offset_flags, horiz_offset_flags;
	uint8_t support_flags, preferred_aspect_ratio;
	int max_vert_offset = 0, min_vert_offset = 0;
	int max_horiz_offset = 0, min_horiz_offset = 0;
	size_t i;
	struct di_edid_display_range_limits *base;
	struct di_edid_display_range_limits_secondary_gtf *secondary_gtf;
	struct di_edid_display_range_limits_cvt *cvt;

	base = &priv->base;

	offset_flags = data[4];
	if (edid->revision >= 4) {
		vert_offset_flags = get_bit_range(offset_flags, 1, 0);
		horiz_offset_flags = get_bit_range(offset_flags, 3, 2);

		if (!decode_display_range_limits_offset(edid,
							vert_offset_flags,
							&max_vert_offset,
							&min_vert_offset)) {
			return false;
		}
		if (!decode_display_range_limits_offset(edid,
							horiz_offset_flags,
							&max_horiz_offset,
							&min_horiz_offset)) {
			return false;
		}

		if (edid->revision <= 4 &&
		    get_bit_range(offset_flags, 7, 4) != 0) {
			add_failure(edid, "Display Range Limits: Bits 7:4 of the range offset flags are reserved.");
		}
	} else if (offset_flags != 0) {
		add_failure(edid, "Display Range Limits: Range offset flags are unsupported in EDID 1.3.");
	}

	if (edid->revision <= 4 && (data[5] == 0 || data[6] == 0 ||
				    data[7] == 0 || data[8] == 0)) {
		add_failure(edid, "Display Range Limits: Range limits set to reserved values.");
		return false;
	}

	base->min_vert_rate_hz = data[5] + min_vert_offset;
	base->max_vert_rate_hz = data[6] + max_vert_offset;
	base->min_horiz_rate_hz = (data[7] + min_horiz_offset) * 1000;
	base->max_horiz_rate_hz = (data[8] + max_horiz_offset) * 1000;

	if (base->min_vert_rate_hz > base->max_vert_rate_hz) {
		add_failure(edid, "Display Range Limits: Min vertical rate > max vertical rate.");
		return false;
	}
	if (base->min_horiz_rate_hz > base->max_horiz_rate_hz) {
		add_failure(edid, "Display Range Limits: Min horizontal freq > max horizontal freq.");
		return false;
	}

	base->max_pixel_clock_hz = (int64_t) data[9] * 10 * 1000 * 1000;
	if (edid->revision == 4 && base->max_pixel_clock_hz == 0) {
		add_failure(edid, "Display Range Limits: EDID 1.4 block does not set max dotclock.");
	}

	support_flags = data[10];
	switch (support_flags) {
	case 0x00:
		/* For EDID 1.4 and later, always indicates support for default
		 * GTF. For EDID 1.3 and earlier, a misc features bit indicates
		 * support for default GTF. */
		if (edid->revision >= 4 || edid->misc_features.default_gtf) {
			base->type = DI_EDID_DISPLAY_RANGE_LIMITS_DEFAULT_GTF;
		} else {
			base->type = DI_EDID_DISPLAY_RANGE_LIMITS_BARE;
		}
		break;
	case 0x01:
		if (edid->revision < 4) {
			/* Reserved */
			add_failure(edid, "Display Range Limits: 'Bare Limits' is not allowed for EDID < 1.4.");
			return false;
		}
		base->type = DI_EDID_DISPLAY_RANGE_LIMITS_BARE;
		break;
	case 0x02:
		base->type = DI_EDID_DISPLAY_RANGE_LIMITS_SECONDARY_GTF;
		break;
	case 0x04:
		if (edid->revision < 4) {
			/* Reserved */
			add_failure(edid, "Display Range Limits: 'CVT' is not allowed for EDID < 1.4.");
			return false;
		}
		base->type = DI_EDID_DISPLAY_RANGE_LIMITS_CVT;
		break;
	default:
		/* Reserved */
		if (edid->revision <= 4) {
			add_failure(edid,
				    "Display Range Limits: Unknown range class (0x%02x).",
				    support_flags);
			return false;
		}
		base->type = DI_EDID_DISPLAY_RANGE_LIMITS_BARE;
		break;
	}

	/* Some types require the display to support continuous frequencies, but
	 * this flag is only set for EDID 1.4 and later */
	if (edid->revision >= 4 && !edid->misc_features.continuous_freq) {
		switch (base->type) {
		case DI_EDID_DISPLAY_RANGE_LIMITS_DEFAULT_GTF:
			add_failure(edid, "Display Range Limits: GTF is supported, but the display does not support continuous frequencies.");
			return false;
		case DI_EDID_DISPLAY_RANGE_LIMITS_SECONDARY_GTF:
			add_failure(edid, "Display Range Limits: Secondary GTF is supported, but the display does not support continuous frequencies.");
			return false;
		case DI_EDID_DISPLAY_RANGE_LIMITS_CVT:
			add_failure(edid, "Display Range Limits: CVT is supported, but the display does not support continuous frequencies.");
			return false;
		default:
			break;
		}
	}

	switch (base->type) {
	case DI_EDID_DISPLAY_RANGE_LIMITS_SECONDARY_GTF:
		secondary_gtf = &priv->secondary_gtf;

		if (data[11] != 0)
			add_failure(edid,
				    "Display Range Limits: Byte 11 is 0x%02x instead of 0x00.",
				    data[11]);

		secondary_gtf->start_freq_hz = data[12] * 2 * 1000;
		secondary_gtf->c = (float) data[13] / 2;
		secondary_gtf->m = (float) ((data[15] << 8) | data[14]);
		secondary_gtf->k = (float) data[16];
		secondary_gtf->j = (float) data[17] / 2;

		base->secondary_gtf = secondary_gtf;
		break;
	case DI_EDID_DISPLAY_RANGE_LIMITS_CVT:
		cvt = &priv->cvt;

		cvt->version = get_bit_range(data[11], 7, 4);
		cvt->revision = get_bit_range(data[11], 3, 0);

		base->max_pixel_clock_hz -= get_bit_range(data[12], 7, 2) * 250 * 1000;
		cvt->max_horiz_px = 8 * ((get_bit_range(data[12], 1, 0) << 8) | data[13]);

		cvt->supported_aspect_ratio = data[14];
		if (get_bit_range(data[14], 2, 0) != 0)
			add_failure_until(edid, 4,
					  "Display Range Limits: Reserved bits of byte 14 are non-zero.");

		preferred_aspect_ratio = get_bit_range(data[15], 7, 5);
		switch (preferred_aspect_ratio) {
		case 0:
			cvt->preferred_aspect_ratio = DI_EDID_CVT_ASPECT_RATIO_4_3;
			break;
		case 1:
			cvt->preferred_aspect_ratio = DI_EDID_CVT_ASPECT_RATIO_16_9;
			break;
		case 2:
			cvt->preferred_aspect_ratio = DI_EDID_CVT_ASPECT_RATIO_16_10;
			break;
		case 3:
			cvt->preferred_aspect_ratio = DI_EDID_CVT_ASPECT_RATIO_5_4;
			break;
		case 4:
			cvt->preferred_aspect_ratio = DI_EDID_CVT_ASPECT_RATIO_15_9;
			break;
		default:
			/* Reserved */
			add_failure_until(edid, 4,
					  "Display Range Limits: Invalid preferred aspect ratio 0x%02x.",
					  preferred_aspect_ratio);
			return false;
		}

		cvt->standard_blanking = has_bit(data[15], 3);
		cvt->reduced_blanking = has_bit(data[15], 4);

		if (get_bit_range(data[15], 2, 0) != 0)
			add_failure_until(edid, 4,
					  "Display Range Limits: Reserved bits of byte 15 are non-zero.");

		cvt->supported_scaling = data[16];
		if (get_bit_range(data[16], 3, 0) != 0)
			add_failure_until(edid, 4,
					  "Display Range Limits: Reserved bits of byte 16 are non-zero.");

		cvt->preferred_vert_refresh_hz = data[17];
		if (cvt->preferred_vert_refresh_hz == 0) {
			add_failure_until(edid, 4,
					  "Display Range Limits: Preferred vertical refresh rate must be specified.");
			return false;
		}

		base->cvt = cvt;
		break;
	case DI_EDID_DISPLAY_RANGE_LIMITS_BARE:
	case DI_EDID_DISPLAY_RANGE_LIMITS_DEFAULT_GTF:
		if (data[11] != 0x0A)
			add_failure(edid,
				    "Display Range Limits: Byte 11 is 0x%02x instead of 0x0a.",
				    data[11]);
		for (i = 12; i < EDID_BYTE_DESCRIPTOR_SIZE; i++) {
			if (data[i] != 0x20) {
				add_failure(edid,
					    "Display Range Limits: Bytes 12-17 must be 0x20.");
				break;
			}
		}
		break;
	}

	return true;
}

static bool
parse_standard_timings_descriptor(struct di_edid *edid,
				  const uint8_t data[static EDID_BYTE_DESCRIPTOR_SIZE],
				  struct di_edid_display_descriptor *desc)
{
	struct di_edid_standard_timing *t;
	size_t i;
	const uint8_t *timing_data;

	for (i = 0; i < EDID_MAX_DESCRIPTOR_STANDARD_TIMING_COUNT; i++) {
		timing_data = &data[5 + i * EDID_STANDARD_TIMING_SIZE];
		if (!parse_standard_timing(edid, timing_data, &t))
			return false;
		if (t) {
			assert(desc->standard_timings_len < EDID_MAX_DESCRIPTOR_STANDARD_TIMING_COUNT);
			desc->standard_timings[desc->standard_timings_len++] = t;
		}
	}

	if (data[17] != 0x0A)
		add_failure_until(edid, 4,
				  "Standard Timing Identifications: Last byte must be a line feed.");

	return true;
}

/**
 * Mapping table for established timings III.
 *
 * Contains one entry per bit, with the value set to the DMT ID.
 */
static const uint8_t established_timings_iii[] = {
	/* 0x06 */
	0x01, /* 640 x 350 @ 85 Hz */
	0x02, /* 640 x 400 @ 85 Hz */
	0x03, /* 720 x 400 @ 85 Hz */
	0x07, /* 640 x 480 @ 85 Hz */
	0x0e, /* 848 x 480 @ 60 Hz */
	0x0c, /* 800 x 600 @ 85 Hz */
	0x13, /* 1024 x 768 @ 85 Hz */
	0x15, /* 1152 x 864 @ 75 Hz */
	/* 0x07 */
	0x16, /* 1280 x 768 @ 60 Hz (RB) */
	0x17, /* 1280 x 768 @ 60 Hz */
	0x18, /* 1280 x 768 @ 75 Hz */
	0x19, /* 1280 x 768 @ 85 Hz */
	0x20, /* 1280 x 960 @ 60 Hz */
	0x21, /* 1280 x 960 @ 85 Hz */
	0x23, /* 1280 x 1024 @ 60 Hz */
	0x25, /* 1280 x 1024 @ 85 Hz */
	/* 0x08 */
	0x27, /* 1360 x 768 @ 60 Hz */
	0x2e, /* 1440 x 900 @ 60 Hz (RB) */
	0x2f, /* 1440 x 900 @ 60 Hz */
	0x30, /* 1440 x 900 @ 75 Hz */
	0x31, /* 1440 x 900 @ 85 Hz */
	0x29, /* 1400 x 1050 @ 60 Hz (RB) */
	0x2a, /* 1400 x 1050 @ 60 Hz */
	0x2b, /* 1400 x 1050 @ 75 Hz */
	/* 0x09 */
	0x2c, /* 1400 x 1050 @ 85 Hz */
	0x39, /* 1680 x 1050 @ 60 Hz (RB) */
	0x3a, /* 1680 x 1050 @ 60 Hz */
	0x3b, /* 1680 x 1050 @ 75 Hz */
	0x3c, /* 1680 x 1050 @ 85 Hz */
	0x33, /* 1600 x 1200 @ 60 Hz */
	0x34, /* 1600 x 1200 @ 65 Hz */
	0x35, /* 1600 x 1200 @ 70 Hz */
	/* 0x0a */
	0x36, /* 1600 x 1200 @ 75 Hz */
	0x37, /* 1600 x 1200 @ 85 Hz */
	0x3e, /* 1792 x 1344 @ 60 Hz */
	0x3f, /* 1792 x 1344 @ 75 Hz */
	0x41, /* 1856 x 1392 @ 60 Hz */
	0x42, /* 1856 x 1392 @ 75 Hz */
	0x44, /* 1920 x 1200 @ 60 Hz (RB) */
	0x45, /* 1920 x 1200 @ 60 Hz */
	/* 0x0b */
	0x46, /* 1920 x 1200 @ 75 Hz */
	0x47, /* 1920 x 1200 @ 85 Hz */
	0x49, /* 1920 x 1440 @ 60 Hz */
	0x4a, /* 1920 x 1440 @ 75 Hz */
};
static_assert(EDID_MAX_DESCRIPTOR_ESTABLISHED_TIMING_III_COUNT
	      == sizeof(established_timings_iii) / sizeof(established_timings_iii[0]),
	      "Invalid number of established timings III in table");

static const struct di_dmt_timing *
get_dmt_timing(uint8_t dmt_id)
{
	size_t i;
	const struct di_dmt_timing *t;

	for (i = 0; i < _di_dmt_timings_len; i++) {
		t = &_di_dmt_timings[i];
		if (t->dmt_id == dmt_id)
			return t;
	}

	return NULL;
}

static void
parse_established_timings_iii_descriptor(struct di_edid *edid,
					 const uint8_t data[static EDID_BYTE_DESCRIPTOR_SIZE],
					 struct di_edid_display_descriptor *desc)
{
	size_t i, offset, bit;
	uint8_t dmt_id;
	const struct di_dmt_timing *t;
	bool has_zeroes;

	if (edid->revision < 4)
		add_failure(edid, "Established timings III: Not allowed for EDID < 1.4.");

	for (i = 0; i < EDID_MAX_DESCRIPTOR_ESTABLISHED_TIMING_III_COUNT; i++) {
		dmt_id = established_timings_iii[i];
		offset = 0x06 + i / 8;
		bit = 7 - i % 8;
		assert(offset < EDID_BYTE_DESCRIPTOR_SIZE);
		if (has_bit(data[offset], bit)) {
			t = get_dmt_timing(dmt_id);
			assert(t != NULL);
			desc->established_timings_iii[desc->established_timings_iii_len++] = t;
		}
	}

	has_zeroes = get_bit_range(data[11], 3, 0) == 0;
	for (i = 12; i < EDID_BYTE_DESCRIPTOR_SIZE; i++) {
		has_zeroes = has_zeroes && data[i] == 0;
	}
	if (!has_zeroes) {
		add_failure_until(edid, 4,
				  "Established timings III: Reserved bits must be set to zero.");
	}
}

static bool
parse_color_point_descriptor(struct di_edid *edid,
			     const uint8_t data[static EDID_BYTE_DESCRIPTOR_SIZE],
			     struct di_edid_display_descriptor *desc)
{
	struct di_edid_color_point *c;

	if (data[5] == 0) {
		add_failure(edid, "White Point Index Number set to reserved value 0");
	}

	c = calloc(1, sizeof(*c));
	if (!c) {
		return false;
	}

	c->index = data[5];
	c->white_x = decode_chromaticity_coord(data[7], get_bit_range(data[6], 3, 2));
	c->white_y = decode_chromaticity_coord(data[8], get_bit_range(data[6], 1, 0));

	if (data[9] != 0xFF) {
		c->gamma = ((float) data[9] + 100) / 100;
	}

	desc->color_points[desc->color_points_len++] = c;
	if (data[10] == 0)  {
		return true;
	}

	c = calloc(1, sizeof(*c));
	if (!c) {
		return false;
	}

	c->index = data[10];
	c->white_x = decode_chromaticity_coord(data[12], get_bit_range(data[11], 3, 2));
	c->white_y = decode_chromaticity_coord(data[13], get_bit_range(data[11], 1, 0));

	if (data[14] != 0xFF) {
		c->gamma = ((float) data[14] + 100) / 100;
	}

	desc->color_points[desc->color_points_len++] = c;
	return true;
}

static void
parse_color_management_data_descriptor(struct di_edid *edid,
				       const uint8_t data[static EDID_BYTE_DESCRIPTOR_SIZE],
				       struct di_edid_display_descriptor *desc)
{
	desc->dcm_data.version = data[5];

	desc->dcm_data.red_a3 = (uint16_t)(data[6] | (data[7] << 8)) / 100.0f;
	desc->dcm_data.red_a2 = (uint16_t)(data[8] | (data[9] << 8)) / 100.0f;
	desc->dcm_data.green_a3 = (uint16_t)(data[10] | (data[11] << 8)) / 100.0f;
	desc->dcm_data.green_a2 = (uint16_t)(data[12] | (data[13] << 8)) / 100.0f;
	desc->dcm_data.blue_a3 = (uint16_t)(data[14] | (data[15] << 8)) / 100.0f;
	desc->dcm_data.blue_a2 = (uint16_t)(data[16] | (data[17] << 8)) / 100.0f;

	if (desc->dcm_data.version != 3) {
		add_failure_until(edid, 4,
				  "Color Management Data version must be 3");
	}
}

static bool
is_cvt_timing_code_preferred_vrate_supported(const struct di_edid_cvt_timing_code *t)
{
	switch (t->preferred_vertical_rate) {
	case DI_EDID_CVT_TIMING_CODE_PREFERRED_VRATE_50HZ:
		return t->supports_50hz_sb;
	case DI_EDID_CVT_TIMING_CODE_PREFERRED_VRATE_60HZ:
		return t->supports_60hz_sb || t->supports_60hz_rb;
	case DI_EDID_CVT_TIMING_CODE_PREFERRED_VRATE_75HZ:
		return t->supports_75hz_sb;
	case DI_EDID_CVT_TIMING_CODE_PREFERRED_VRATE_85HZ:
		return t->supports_85hz_sb;
	}
	abort(); /* unreachable */
}

static bool
parse_cvt_timing_code(struct di_edid *edid,
		      const uint8_t data[static EDID_CVT_TIMING_CODE_SIZE],
		      struct di_edid_cvt_timing_code **out,
		      bool first)
{
	struct di_edid_cvt_timing_code *t;
	int32_t raw;

	*out = NULL;

	if (!first && data[0] == 0 && data[1] == 0 && data[2] == 0) {
		/* Unused */
		return true;
	}
	if (data[0] == 0) {
		add_failure(edid,
			    "CVT byte 0 is 0, which is a reserved value.");
	}

	t = calloc(1, sizeof(*t));
	if (!t) {
		return false;
	}

	raw = (int32_t)(data[0] | (get_bit_range(data[1], 7, 4) << 8));
	t->addressable_lines_per_field = (raw + 1) * 2;
	t->aspect_ratio = get_bit_range(data[1], 3, 2);

	if (get_bit_range(data[1], 1, 0) != 0) {
		add_failure(edid,
			    "Reserved bits of CVT byte 1 are non-zero.");
	}

	t->supports_50hz_sb = has_bit(data[2], 4);
	t->supports_60hz_sb = has_bit(data[2], 3);
	t->supports_75hz_sb = has_bit(data[2], 2);
	t->supports_85hz_sb = has_bit(data[2], 1);
	t->supports_60hz_rb = has_bit(data[2], 0);

	if (get_bit_range(data[2], 4, 0) == 0) {
		add_failure(edid,
			    "CVT byte 2 does not support any vertical rates.");
	}

	t->preferred_vertical_rate = get_bit_range(data[2], 6, 5);

	if (has_bit(data[2], 7) != 0) {
		add_failure(edid,
			    "Reserved bit of CVT byte 2 is non-zero.");
	}

	if (!is_cvt_timing_code_preferred_vrate_supported(t))
		add_failure(edid, "The preferred CVT Vertical Rate is not supported.");

	*out = t;
	return true;
}

static bool
parse_cvt_timing_codes_descriptor(struct di_edid *edid,
				  const uint8_t data[static EDID_BYTE_DESCRIPTOR_SIZE],
				  struct di_edid_display_descriptor *desc)
{
	struct di_edid_cvt_timing_code *t;
	size_t i;
	const uint8_t *timing_data;

	if (data[5] != 1) {
		add_failure_until(edid, 4, "Invalid version number %u.",
				  data[5]);
	}

	for (i = 0; i < EDID_MAX_DESCRIPTOR_CVT_TIMING_CODES_COUNT; i++) {
		timing_data = &data[6 + i * EDID_CVT_TIMING_CODE_SIZE];
		if (!parse_cvt_timing_code(edid, timing_data, &t, !i))
			return false;
		if (t) {
			assert(desc->cvt_timing_codes_len < EDID_MAX_DESCRIPTOR_CVT_TIMING_CODES_COUNT);
			desc->cvt_timing_codes[desc->cvt_timing_codes_len++] = t;
		}
	}

	return true;
}

static bool
parse_byte_descriptor(struct di_edid *edid,
		      const uint8_t data[static EDID_BYTE_DESCRIPTOR_SIZE])
{
	struct di_edid_display_descriptor *desc;
	struct di_edid_detailed_timing_def_priv *detailed_timing_def;
	uint8_t tag;
	char *newline;

	if (data[0] || data[1]) {
		if (edid->display_descriptors_len > 0) {
			/* A detailed timing descriptor is not allowed after a
			 * display descriptor per note 3 of table 3.20. */
			add_failure(edid, "Invalid detailed timing descriptor ordering.");
		}

		detailed_timing_def = _di_edid_parse_detailed_timing_def(data);
		if (!detailed_timing_def) {
			return false;
		}

		assert(edid->detailed_timing_defs_len < EDID_BYTE_DESCRIPTOR_COUNT);
		edid->detailed_timing_defs[edid->detailed_timing_defs_len++] = detailed_timing_def;
		return true;
	}

	if (edid->revision >= 3 && edid->revision <= 4 &&
	    edid->detailed_timing_defs_len == 0) {
		/* Per section 3.10.1 */
		add_failure(edid,
			    "The first byte descriptor must contain the preferred timing.");
	}

	desc = calloc(1, sizeof(*desc));
	if (!desc) {
		return false;
	}

	tag = data[3];
	switch (tag) {
	case DI_EDID_DISPLAY_DESCRIPTOR_PRODUCT_SERIAL:
	case DI_EDID_DISPLAY_DESCRIPTOR_DATA_STRING:
	case DI_EDID_DISPLAY_DESCRIPTOR_PRODUCT_NAME:
		memcpy(desc->str, &data[5], 13);

		/* A newline (if any) indicates the end of the string. */
		newline = strchr(desc->str, '\n');
		if (newline) {
			newline[0] = '\0';
		}
		break;
	case DI_EDID_DISPLAY_DESCRIPTOR_RANGE_LIMITS:
		if (!parse_display_range_limits(edid, data, &desc->range_limits)) {
			free(desc);
			return true;
		}
		break;
	case DI_EDID_DISPLAY_DESCRIPTOR_STD_TIMING_IDS:
		if (!parse_standard_timings_descriptor(edid, data, desc)) {
			free(desc);
			return false;
		}
		break;
	case DI_EDID_DISPLAY_DESCRIPTOR_ESTABLISHED_TIMINGS_III:
		parse_established_timings_iii_descriptor(edid, data, desc);
		break;
	case DI_EDID_DISPLAY_DESCRIPTOR_COLOR_POINT:
		if (!parse_color_point_descriptor(edid, data, desc)) {
			free(desc);
			return false;
		}
		break;
	case DI_EDID_DISPLAY_DESCRIPTOR_DCM_DATA:
		parse_color_management_data_descriptor(edid, data, desc);
		break;
	case DI_EDID_DISPLAY_DESCRIPTOR_CVT_TIMING_CODES:
		if (!parse_cvt_timing_codes_descriptor(edid, data, desc)) {
			free(desc);
			return false;
		}
		break;
	case DI_EDID_DISPLAY_DESCRIPTOR_DUMMY:
		break; /* Ignore */
	default:
		free(desc);
		if (tag <= 0x0F) {
			/* Manufacturer-specific */
		} else {
			add_failure_until(edid, 4, "Unknown Type 0x%02hhx.", tag);
		}
		return true;
	}

	desc->tag = tag;
	assert(edid->display_descriptors_len < EDID_BYTE_DESCRIPTOR_COUNT);
	edid->display_descriptors[edid->display_descriptors_len++] = desc;
	return true;
}

static const char *
ext_tag_name(enum di_edid_ext_tag tag)
{
	switch (tag) {
	case DI_EDID_EXT_CEA:
		return "CTA-861 Extension Block";
	case DI_EDID_EXT_VTB:
		return "Video Timing Extension Block";
	case DI_EDID_EXT_DI:
		return "Display Information Extension Block";
	case DI_EDID_EXT_LS:
		return "Localized String Extension Block";
	case DI_EDID_EXT_DPVL:
		return "Digital Packet Video Link Extension";
	case DI_EDID_EXT_BLOCK_MAP:
		return "Block Map Extension Block";
	case DI_EDID_EXT_VENDOR:
		return "Manufacturer-Specific Extension Block";
	case DI_EDID_EXT_DISPLAYID:
		return "DisplayID Extension Block";
	}
	return "Unknown Extension Block";
}

static bool
parse_ext(struct di_edid *edid, const uint8_t data[static EDID_BLOCK_SIZE])
{
	struct di_edid_ext *ext;
	struct di_logger logger;
	char section_name[64];
	const uint8_t *ext_data;
	size_t ext_size;
	bool ok;

	ext = calloc(1, sizeof(*ext));
	if (!ext) {
		return false;
	}
	ext->tag = data[0x00];

	snprintf(section_name, sizeof(section_name), "Block %zu, %s",
		 edid->exts_len + 1, ext_tag_name(ext->tag));
	logger = (struct di_logger) {
		.f = edid->logger->f,
		.section = section_name,
	};

	switch (ext->tag) {
	case DI_EDID_EXT_CEA:
		if (!_di_edid_cta_parse(&ext->cta, data, EDID_BLOCK_SIZE, &logger)) {
			free(ext);
			return errno == EINVAL;
		}
		break;
	case DI_EDID_EXT_VTB:
	case DI_EDID_EXT_DI:
	case DI_EDID_EXT_LS:
	case DI_EDID_EXT_DPVL:
	case DI_EDID_EXT_BLOCK_MAP:
	case DI_EDID_EXT_VENDOR:
		/* Supported */
		break;
	case DI_EDID_EXT_DISPLAYID:
		ext_data = &data[1];
		ext_size = EDID_BLOCK_SIZE - 2;

		ext->displayid_version = _di_displayid_parse_version(ext_data, ext_size);
		switch (ext->displayid_version) {
		case 1:
			ok = _di_displayid_parse(&ext->displayid, ext_data,
						 ext_size, &logger);
			break;
		case 2:
			ok = _di_displayid2_parse(&ext->displayid2, ext_data,
						  ext_size, &logger);
			break;
		default:
			/* Unsupported */
			free(ext);
			return true;
		}
		if (!ok) {
			free(ext);
			return errno == ENOTSUP || errno == EINVAL;
		}
		break;
	default:
		/* Unsupported */
		free(ext);
		add_failure_until(edid, 4, "Unknown Extension Block.");
		return true;
	}

	if (!validate_block_checksum(data))
		_di_logger_add_failure(&logger, "Invalid extension checksum.");

	assert(edid->exts_len < EDID_MAX_BLOCK_COUNT - 1);
	edid->exts[edid->exts_len++] = ext;
	return true;
}

struct di_edid *
_di_edid_parse(const void *data, size_t size, FILE *failure_msg_file)
{
	struct di_edid *edid;
	struct di_logger logger;
	int version, revision;
	size_t exts_len, parsed_ext_len, i;
	const uint8_t *standard_timing_data, *byte_desc_data, *ext_data;
	struct di_edid_standard_timing *standard_timing;

	if (size < EDID_BLOCK_SIZE) {
		errno = EINVAL;
		return NULL;
	}

	if (memcmp(data, header, sizeof(header)) != 0) {
		errno = EINVAL;
		return NULL;
	}

	parse_version_revision(data, &version, &revision);
	if (version != 1) {
		/* Only EDID version 1 is supported -- as per section 2.1.7
		 * subsequent versions break the structure */
		errno = ENOTSUP;
		return NULL;
	}

	if (!validate_block_checksum(data)) {
		errno = EINVAL;
		return NULL;
	}

	edid = calloc(1, sizeof(*edid));
	if (!edid) {
		return NULL;
	}

	logger = (struct di_logger) {
		.f = failure_msg_file,
		.section = "Block 0, Base EDID",
	};
	edid->logger = &logger;

	edid->version = version;
	edid->revision = revision;

	if (size % EDID_BLOCK_SIZE != 0)
		add_failure(edid, "The data is not a multiple of the block size.");
	if (size > EDID_MAX_BLOCK_COUNT * EDID_BLOCK_SIZE)
		add_failure(edid, "The data is exceeding the maximum block count.");

	exts_len = (size / EDID_BLOCK_SIZE) - 1;
	parsed_ext_len = parse_ext_count(data);

	if (exts_len != parsed_ext_len)
		add_failure(edid, "The data size does not match the encoded block count.");

	if (parsed_ext_len < exts_len)
		exts_len = parsed_ext_len;

	assert(exts_len < EDID_MAX_BLOCK_COUNT);

	parse_vendor_product(edid, data);
	parse_basic_params_features(edid, data);
	parse_chromaticity_coords(edid, data);

	parse_established_timings_i_ii(edid, data);

	for (i = 0; i < EDID_MAX_STANDARD_TIMING_COUNT; i++) {
		standard_timing_data = (const uint8_t *) data
				       + 0x26 + i * EDID_STANDARD_TIMING_SIZE;
		if (!parse_standard_timing(edid, standard_timing_data,
					   &standard_timing)) {
			_di_edid_destroy(edid);
			return NULL;
		}
		if (standard_timing) {
			assert(edid->standard_timings_len < EDID_MAX_STANDARD_TIMING_COUNT);
			edid->standard_timings[edid->standard_timings_len++] = standard_timing;
		}
	}

	for (i = 0; i < EDID_BYTE_DESCRIPTOR_COUNT; i++) {
		byte_desc_data = (const uint8_t *) data
			       + 0x36 + i * EDID_BYTE_DESCRIPTOR_SIZE;
		if (!parse_byte_descriptor(edid, byte_desc_data)) {
			_di_edid_destroy(edid);
			return NULL;
		}
	}

	for (i = 0; i < exts_len; i++) {
		ext_data = (const uint8_t *) data + (i + 1) * EDID_BLOCK_SIZE;
		if (!parse_ext(edid, ext_data)) {
			_di_edid_destroy(edid);
			return NULL;
		}
	}

	edid->logger = NULL;
	return edid;
}

static void
destroy_display_descriptor(struct di_edid_display_descriptor *desc)
{
	size_t i;

	switch (desc->tag) {
	case DI_EDID_DISPLAY_DESCRIPTOR_STD_TIMING_IDS:
		for (i = 0; i < desc->standard_timings_len; i++) {
			free(desc->standard_timings[i]);
		}
		break;
	case DI_EDID_DISPLAY_DESCRIPTOR_COLOR_POINT:
		for (i = 0; i < desc->color_points_len; i++) {
			free(desc->color_points[i]);
		}
		break;
	case DI_EDID_DISPLAY_DESCRIPTOR_CVT_TIMING_CODES:
		for (i = 0; i < desc->cvt_timing_codes_len; i++) {
			free(desc->cvt_timing_codes[i]);
		}
		break;
	default:
		break; /* Nothing to do */
	}
	free(desc);
}

void
_di_edid_destroy(struct di_edid *edid)
{
	size_t i;
	struct di_edid_ext *ext;

	for (i = 0; i < edid->standard_timings_len; i++) {
		free(edid->standard_timings[i]);
	}

	for (i = 0; i < edid->detailed_timing_defs_len; i++) {
		free(edid->detailed_timing_defs[i]);
	}

	for (i = 0; i < edid->display_descriptors_len; i++) {
		destroy_display_descriptor(edid->display_descriptors[i]);
	}

	for (i = 0; edid->exts[i] != NULL; i++) {
		ext = edid->exts[i];
		switch (ext->tag) {
		case DI_EDID_EXT_CEA:
			_di_edid_cta_finish(&ext->cta);
			break;
		case DI_EDID_EXT_DISPLAYID:
			_di_displayid_finish(&ext->displayid);
			_di_displayid2_finish(&ext->displayid2);
			break;
		default:
			break; /* Nothing to do */
		}
		free(ext);
	}

	free(edid);
}

int
di_edid_get_version(const struct di_edid *edid)
{
	return edid->version;
}

int
di_edid_get_revision(const struct di_edid *edid)
{
	return edid->revision;
}

const struct di_edid_vendor_product *
di_edid_get_vendor_product(const struct di_edid *edid)
{
	return &edid->vendor_product;
}

const struct di_edid_video_input_analog *
di_edid_get_video_input_analog(const struct di_edid *edid)
{
	return edid->is_digital ? NULL : &edid->video_input_analog;
}

const struct di_edid_video_input_digital *
di_edid_get_video_input_digital(const struct di_edid *edid)
{
	return edid->is_digital ? &edid->video_input_digital : NULL;
}

const struct di_edid_screen_size *
di_edid_get_screen_size(const struct di_edid *edid)
{
	return &edid->screen_size;
}

float
di_edid_get_basic_gamma(const struct di_edid *edid)
{
	return edid->gamma;
}

const struct di_edid_dpms *
di_edid_get_dpms(const struct di_edid *edid)
{
	return &edid->dpms;
}

enum di_edid_display_color_type
di_edid_get_display_color_type(const struct di_edid *edid)
{
	return edid->display_color_type;
}

const struct di_edid_color_encoding_formats *
di_edid_get_color_encoding_formats(const struct di_edid *edid)
{
	/* If color encoding formats are specified, RGB 4:4:4 is always
	 * supported. */
	return edid->color_encoding_formats.rgb444 ? &edid->color_encoding_formats : NULL;
}

const struct di_edid_misc_features *
di_edid_get_misc_features(const struct di_edid *edid)
{
	return &edid->misc_features;
}

const struct di_edid_chromaticity_coords *
di_edid_get_chromaticity_coords(const struct di_edid *edid)
{
	return &edid->chromaticity_coords;
}

const struct di_edid_established_timings_i_ii *
di_edid_get_established_timings_i_ii(const struct di_edid *edid)
{
	return &edid->established_timings_i_ii;
}

int32_t
di_edid_standard_timing_get_vert_video(const struct di_edid_standard_timing *t)
{
	switch (t->aspect_ratio) {
	case DI_EDID_STANDARD_TIMING_16_10:
		return t->horiz_video * 10 / 16;
	case DI_EDID_STANDARD_TIMING_4_3:
		return t->horiz_video * 3 / 4;
	case DI_EDID_STANDARD_TIMING_5_4:
		return t->horiz_video * 4 / 5;
	case DI_EDID_STANDARD_TIMING_16_9:
		return t->horiz_video * 9 / 16;
	}
	abort(); /* unreachable */
}

const struct di_dmt_timing *
di_edid_standard_timing_get_dmt(const struct di_edid_standard_timing *t)
{
	int32_t vert_video;
	size_t i;
	const struct di_dmt_timing *dmt;

	vert_video = di_edid_standard_timing_get_vert_video(t);

	for (i = 0; i < _di_dmt_timings_len; i++) {
		dmt = &_di_dmt_timings[i];

		if (dmt->horiz_video == t->horiz_video
		    && dmt->vert_video == vert_video
		    && dmt->refresh_rate_hz == (float)t->refresh_rate_hz
		    && dmt->edid_std_id != 0) {
			return dmt;
		}
	}

	return 0;
}

const struct di_edid_standard_timing *const *
di_edid_get_standard_timings(const struct di_edid *edid)
{
	return (const struct di_edid_standard_timing *const *) &edid->standard_timings;
}

const struct di_edid_detailed_timing_def *const *
di_edid_get_detailed_timing_defs(const struct di_edid *edid)
{
	return (const struct di_edid_detailed_timing_def *const *) &edid->detailed_timing_defs;
}

const struct di_edid_display_descriptor *const *
di_edid_get_display_descriptors(const struct di_edid *edid)
{
	return (const struct di_edid_display_descriptor *const *) &edid->display_descriptors;
}

enum di_edid_display_descriptor_tag
di_edid_display_descriptor_get_tag(const struct di_edid_display_descriptor *desc)
{
	return desc->tag;
}

const char *
di_edid_display_descriptor_get_string(const struct di_edid_display_descriptor *desc)
{
	switch (desc->tag) {
	case DI_EDID_DISPLAY_DESCRIPTOR_PRODUCT_SERIAL:
	case DI_EDID_DISPLAY_DESCRIPTOR_DATA_STRING:
	case DI_EDID_DISPLAY_DESCRIPTOR_PRODUCT_NAME:
		return desc->str;
	default:
		return NULL;
	}
}

const struct di_edid_display_range_limits *
di_edid_display_descriptor_get_range_limits(const struct di_edid_display_descriptor *desc)
{
	if (desc->tag != DI_EDID_DISPLAY_DESCRIPTOR_RANGE_LIMITS) {
		return NULL;
	}
	return &desc->range_limits.base;
}

const struct di_edid_standard_timing *const *
di_edid_display_descriptor_get_standard_timings(const struct di_edid_display_descriptor *desc)
{
	if (desc->tag != DI_EDID_DISPLAY_DESCRIPTOR_STD_TIMING_IDS) {
		return NULL;
	}
	return (const struct di_edid_standard_timing *const *) desc->standard_timings;
}

const struct di_edid_color_point *const *
di_edid_display_descriptor_get_color_points(const struct di_edid_display_descriptor *desc)
{
	if (desc->tag != DI_EDID_DISPLAY_DESCRIPTOR_COLOR_POINT) {
		return NULL;
	}
	return (const struct di_edid_color_point *const *) desc->color_points;
}

const struct di_dmt_timing *const *
di_edid_display_descriptor_get_established_timings_iii(const struct di_edid_display_descriptor *desc)
{
	if (desc->tag != DI_EDID_DISPLAY_DESCRIPTOR_ESTABLISHED_TIMINGS_III) {
		return NULL;
	}
	return desc->established_timings_iii;
}

const struct di_edid_color_management_data *
di_edid_display_descriptor_get_color_management_data(const struct di_edid_display_descriptor *desc)
{
	if (desc->tag != DI_EDID_DISPLAY_DESCRIPTOR_DCM_DATA) {
		return NULL;
	}
	return &desc->dcm_data;
}

const struct di_edid_cvt_timing_code *const *
di_edid_display_descriptor_get_cvt_timing_codes(const struct di_edid_display_descriptor *desc)
{
	if (desc->tag != DI_EDID_DISPLAY_DESCRIPTOR_CVT_TIMING_CODES) {
		return NULL;
	}
	return (const struct di_edid_cvt_timing_code *const *) desc->cvt_timing_codes;
}

const struct di_edid_ext *const *
di_edid_get_extensions(const struct di_edid *edid)
{
	return (const struct di_edid_ext *const *) edid->exts;
}

enum di_edid_ext_tag
di_edid_ext_get_tag(const struct di_edid_ext *ext)
{
	return ext->tag;
}

const struct di_edid_cta *
di_edid_ext_get_cta(const struct di_edid_ext *ext)
{
	if (ext->tag != DI_EDID_EXT_CEA) {
		return NULL;
	}
	return &ext->cta;
}

const struct di_displayid *
di_edid_ext_get_displayid(const struct di_edid_ext *ext)
{
	if (ext->tag != DI_EDID_EXT_DISPLAYID || ext->displayid_version != 1) {
		return NULL;
	}
	return &ext->displayid;
}

const struct di_displayid2 *
di_edid_ext_get_displayid2(const struct di_edid_ext *ext)
{
	if (ext->tag != DI_EDID_EXT_DISPLAYID || ext->displayid_version != 2) {
		return NULL;
	}
	return &ext->displayid2;
}
