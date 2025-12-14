#include <inttypes.h>
#include <math.h>
#include <stdio.h>
#include <stdlib.h>

#include <libdisplay-info/cta.h>

#include "bits.h"
#include "di-edid-decode.h"

static const char *
video_format_picture_aspect_ratio_name(enum di_cta_video_format_picture_aspect_ratio ar)
{
	switch (ar) {
	case DI_CTA_VIDEO_FORMAT_PICTURE_ASPECT_RATIO_4_3:
		return "  4:3  ";
	case DI_CTA_VIDEO_FORMAT_PICTURE_ASPECT_RATIO_16_9:
		return " 16:9  ";
	case DI_CTA_VIDEO_FORMAT_PICTURE_ASPECT_RATIO_64_27:
		return " 64:27 ";
	case DI_CTA_VIDEO_FORMAT_PICTURE_ASPECT_RATIO_256_135:
		return "256:135";
	}
	abort(); /* unreachable */
}

static void
print_vic(uint8_t vic)
{
	const struct di_cta_video_format *fmt;
	int32_t h_blank, v_blank, v_active;
	double refresh, h_freq_hz, pixel_clock_mhz, h_total, v_total;
	char buf[10];

	printf("    VIC %3" PRIu8, vic);

	fmt = di_cta_video_format_from_vic(vic);
	if (fmt == NULL)
		return;

	v_active = fmt->v_active;
	if (fmt->interlaced)
		v_active /= 2;

	h_blank = fmt->h_front + fmt->h_sync + fmt->h_back;
	v_blank = fmt->v_front + fmt->v_sync + fmt->v_back;
	h_total = fmt->h_active + h_blank;

	v_total = v_active + v_blank;
	if (fmt->interlaced)
		v_total += 0.5;

	refresh = (double) fmt->pixel_clock_hz / (h_total * v_total);
	h_freq_hz = (double) fmt->pixel_clock_hz / h_total;
	pixel_clock_mhz = (double) fmt->pixel_clock_hz / (1000 * 1000);

	snprintf(buf, sizeof(buf), "%d%s",
		 fmt->v_active,
		 fmt->interlaced ? "i" : "");

	printf(":");
	printf(" %5dx%-5s", fmt->h_active, buf);
	printf(" %10.6f Hz", refresh);
	printf(" %s", video_format_picture_aspect_ratio_name(fmt->picture_aspect_ratio));
	printf(" %8.3f kHz %13.6f MHz", h_freq_hz / 1000, pixel_clock_mhz);
}

static void
printf_cta_svd(const struct di_cta_svd *svd)
{
	print_vic(svd->vic);
	if (svd->native)
		printf(" (native)");
	printf("\n");
}

static void
printf_cta_svds(const struct di_cta_svd *const *svds)
{
	size_t i;

	for (i = 0; svds[i] != NULL; i++)
		printf_cta_svd(svds[i]);
}

static void
print_cta_hdmi_vic(uint8_t hdmi_vic)
{
	const struct di_cta_hdmi_video_format *fmt;
	int32_t h_blank, v_blank;
	double refresh, h_freq_hz, pixel_clock_mhz, h_total, v_total;
	int horiz_ratio, vert_ratio;

	printf("    HDMI VIC %" PRIu8, hdmi_vic);

	fmt = di_cta_hdmi_video_format_from_hdmi_vic(hdmi_vic);
	if (fmt == NULL)
		return;

	compute_aspect_ratio(fmt->h_active, fmt->v_active, &horiz_ratio, &vert_ratio);

	h_blank = fmt->h_front + fmt->h_sync + fmt->h_back;
	v_blank = fmt->v_front + fmt->v_sync + fmt->v_back;
	h_total = fmt->h_active + h_blank;

	v_total = fmt->v_active + v_blank;

	refresh = (double) fmt->pixel_clock_hz / (h_total * v_total);
	h_freq_hz = (double) fmt->pixel_clock_hz / h_total;
	pixel_clock_mhz = (double) fmt->pixel_clock_hz / (1000 * 1000);

	printf(":");
	printf(" %5dx%-5d", fmt->h_active, fmt->v_active);
	printf(" %10.6f Hz", refresh);
	/* Not part of the spec, but edid-decode prints the aspect ratio. */
	printf(" %3u:%-3u", horiz_ratio, vert_ratio);
	printf(" %8.3f kHz %13.6f MHz", h_freq_hz / 1000, pixel_clock_mhz);
}

static const char *
vesa_display_device_interface_type_name(enum di_cta_vesa_display_device_interface_type type)
{
	switch (type) {
	case DI_CTA_VESA_DISPLAY_DEVICE_INTERFACE_VGA:
		return "Analog (15HD/VGA)";
	case DI_CTA_VESA_DISPLAY_DEVICE_INTERFACE_NAVI_V:
		return "Analog (VESA NAVI-V (15HD))";
	case DI_CTA_VESA_DISPLAY_DEVICE_INTERFACE_NAVI_D:
		return "Analog (VESA NAVI-D)";
	case DI_CTA_VESA_DISPLAY_DEVICE_INTERFACE_LVDS:
		return "LVDS";
	case DI_CTA_VESA_DISPLAY_DEVICE_INTERFACE_RSDS:
		return "RSDS";
	case DI_CTA_VESA_DISPLAY_DEVICE_INTERFACE_DVI_D:
		return "DVI-D";
	case DI_CTA_VESA_DISPLAY_DEVICE_INTERFACE_DVI_I_ANALOG:
		return "DVI-I analog";
	case DI_CTA_VESA_DISPLAY_DEVICE_INTERFACE_DVI_I_DIGITAL:
		return "DVI-I digital";
	case DI_CTA_VESA_DISPLAY_DEVICE_INTERFACE_HDMI_A:
		return "HDMI-A";
	case DI_CTA_VESA_DISPLAY_DEVICE_INTERFACE_HDMI_B:
		return "HDMI-B";
	case DI_CTA_VESA_DISPLAY_DEVICE_INTERFACE_MDDI:
		return "MDDI";
	case DI_CTA_VESA_DISPLAY_DEVICE_INTERFACE_DISPLAYPORT:
		return "DisplayPort";
	case DI_CTA_VESA_DISPLAY_DEVICE_INTERFACE_IEEE_1394:
		return "IEEE-1394";
	case DI_CTA_VESA_DISPLAY_DEVICE_INTERFACE_M1_ANALOG:
		return "M1 analog";
	case DI_CTA_VESA_DISPLAY_DEVICE_INTERFACE_M1_DIGITAL:
		return "M1 digital";
	}
	abort(); /* unreachable */
}

static const char *
vesa_display_device_content_protection_name(enum di_cta_vesa_display_device_content_protection cp)
{
	switch (cp) {
	case DI_CTA_VESA_DISPLAY_DEVICE_CONTENT_PROTECTION_NONE:
		return "None";
	case DI_CTA_VESA_DISPLAY_DEVICE_CONTENT_PROTECTION_HDCP:
		return "HDCP";
	case DI_CTA_VESA_DISPLAY_DEVICE_CONTENT_PROTECTION_DTCP:
		return "DTCP";
	case DI_CTA_VESA_DISPLAY_DEVICE_CONTENT_PROTECTION_DPCP:
		return "DPCP";
	}
	abort(); /* unreachable */
}

static const char *
vesa_display_device_default_orientation_name(enum di_cta_vesa_display_device_default_orientation orientation)
{
	switch (orientation) {
	case DI_CTA_VESA_DISPLAY_DEVICE_DEFAULT_ORIENTATION_LANDSCAPE:
		return "Landscape";
	case DI_CTA_VESA_DISPLAY_DEVICE_DEFAULT_ORIENTATION_PORTAIT:
		return "Portrait";
	case DI_CTA_VESA_DISPLAY_DEVICE_DEFAULT_ORIENTATION_UNFIXED:
		return "Not Fixed";
	case DI_CTA_VESA_DISPLAY_DEVICE_DEFAULT_ORIENTATION_UNDEFINED:
		return "Undefined";
	}
	abort(); /* unreachable */
}

static const char *
vesa_display_device_rotation_cap_name(enum di_cta_vesa_display_device_rotation_cap rot)
{
	switch (rot) {
	case DI_CTA_VESA_DISPLAY_DEVICE_ROTATION_CAP_NONE:
		return "None";
	case DI_CTA_VESA_DISPLAY_DEVICE_ROTATION_CAP_90DEG_CLOCKWISE:
		return "Can rotate 90 degrees clockwise";
	case DI_CTA_VESA_DISPLAY_DEVICE_ROTATION_CAP_90DEG_COUNTERCLOCKWISE:
		return "Can rotate 90 degrees counterclockwise";
	case DI_CTA_VESA_DISPLAY_DEVICE_ROTATION_CAP_90DEG_EITHER:
		return "Can rotate 90 degrees in either direction";
	}
	abort(); /* unreachable */
}

static const char *
vesa_display_device_zero_pixel_location_name(enum di_cta_vesa_display_device_zero_pixel_location loc)
{
	switch (loc) {
	case DI_CTA_VESA_DISPLAY_DEVICE_ZERO_PIXEL_UPPER_LEFT:
		return "Upper Left";
	case DI_CTA_VESA_DISPLAY_DEVICE_ZERO_PIXEL_UPPER_RIGHT:
		return "Upper Right";
	case DI_CTA_VESA_DISPLAY_DEVICE_ZERO_PIXEL_LOWER_LEFT:
		return "Lower Left";
	case DI_CTA_VESA_DISPLAY_DEVICE_ZERO_PIXEL_LOWER_RIGHT:
		return "Lower Right";
	}
	abort(); /* unreachable */
}

static const char *
vesa_display_device_scan_direction_name(enum di_cta_vesa_display_device_scan_direction dir)
{
	switch (dir) {
	case DI_CTA_VESA_DISPLAY_DEVICE_SCAN_DIRECTION_UNDEFINED:
		return "Not defined";
	case DI_CTA_VESA_DISPLAY_DEVICE_SCAN_DIRECTION_FAST_LONG_SLOW_SHORT:
		return "Fast Scan is on the Major (Long) Axis and Slow Scan is on the Minor Axis";
	case DI_CTA_VESA_DISPLAY_DEVICE_SCAN_DIRECTION_FAST_SHORT_SLOW_LONG:
		return "Fast Scan is on the Minor (Short) Axis and Slow Scan is on the Major Axis";
	}
	abort(); /* unreachable */
}

static const char *
vesa_display_device_subpixel_layout_name(enum di_cta_vesa_display_device_subpixel_layout subpixel)
{
	switch (subpixel) {
	case DI_CTA_VESA_DISPLAY_DEVICE_SUBPIXEL_UNDEFINED:
		return "Not defined";
	case DI_CTA_VESA_DISPLAY_DEVICE_SUBPIXEL_RGB_VERT:
		return "RGB vertical stripes";
	case DI_CTA_VESA_DISPLAY_DEVICE_SUBPIXEL_RGB_HORIZ:
		return "RGB horizontal stripes";
	case DI_CTA_VESA_DISPLAY_DEVICE_SUBPIXEL_EDID_CHROM_VERT:
		return "Vertical stripes using primary order";
	case DI_CTA_VESA_DISPLAY_DEVICE_SUBPIXEL_EDID_CHROM_HORIZ:
		return "Horizontal stripes using primary order";
	case DI_CTA_VESA_DISPLAY_DEVICE_SUBPIXEL_QUAD_RGGB:
		return "Quad sub-pixels, red at top left";
	case DI_CTA_VESA_DISPLAY_DEVICE_SUBPIXEL_QUAD_GBRG:
		return "Quad sub-pixels, red at bottom left";
	case DI_CTA_VESA_DISPLAY_DEVICE_SUBPIXEL_DELTA_RGB:
		return "Delta (triad) RGB sub-pixels";
	case DI_CTA_VESA_DISPLAY_DEVICE_SUBPIXEL_MOSAIC:
		return "Mosaic";
	case DI_CTA_VESA_DISPLAY_DEVICE_SUBPIXEL_QUAD_ANY:
		return "Quad sub-pixels, RGB + 1 additional color";
	case DI_CTA_VESA_DISPLAY_DEVICE_SUBPIXEL_FIVE:
		return "Five sub-pixels, RGB + 2 additional colors";
	case DI_CTA_VESA_DISPLAY_DEVICE_SUBPIXEL_SIX:
		return "Six sub-pixels, RGB + 3 additional colors";
	case DI_CTA_VESA_DISPLAY_DEVICE_SUBPIXEL_CLAIRVOYANTE_PENTILE:
		return "Clairvoyante, Inc. PenTile Matrix (tm) layout";
	}
	abort(); /* unreachable */
}

static const char *
vesa_display_device_dithering_type_name(enum di_cta_vesa_display_device_dithering_type dithering)
{
	switch (dithering) {
	case DI_CTA_VESA_DISPLAY_DEVICE_DITHERING_NONE:
		return "None";
	case DI_CTA_VESA_DISPLAY_DEVICE_DITHERING_SPACIAL:
		return "Spacial";
	case DI_CTA_VESA_DISPLAY_DEVICE_DITHERING_TEMPORAL:
		return "Temporal";
	case DI_CTA_VESA_DISPLAY_DEVICE_DITHERING_SPATIAL_AND_TEMPORAL:
		return "Spatial and Temporal";
	}
	abort(); /* unreachable */
}

static const char *
vesa_display_device_frame_rate_conversion_name(enum di_cta_vesa_display_device_frame_rate_conversion conv)
{
	switch (conv) {
	case DI_CTA_VESA_DISPLAY_DEVICE_FRAME_RATE_CONVERSION_NONE:
		return "None";
	case DI_CTA_VESA_DISPLAY_DEVICE_FRAME_RATE_CONVERSION_SINGLE_BUFFERING:
		return "Single Buffering";
	case DI_CTA_VESA_DISPLAY_DEVICE_FRAME_RATE_CONVERSION_DOUBLE_BUFFERING:
		return "Double Buffering";
	case DI_CTA_VESA_DISPLAY_DEVICE_FRAME_RATE_CONVERSION_ADVANCED:
		return "Advanced Frame Rate Conversion";
	}
	abort(); /* unreachable */
}

static float
truncate_chromaticity_coord(float coord)
{
	return floorf(coord * 10000) / 10000;
}

static const char *
vesa_display_device_resp_time_transition_name(enum di_cta_vesa_display_device_resp_time_transition t)
{
	switch (t) {
	case DI_CTA_VESA_DISPLAY_DEVICE_RESP_TIME_BLACK_TO_WHITE:
		return "Black -> White";
	case DI_CTA_VESA_DISPLAY_DEVICE_RESP_TIME_WHITE_TO_BLACK:
		return "White -> Black";
	}
	abort(); /* unreachable */
}

static void
print_cta_vesa_display_device(const struct di_cta_vesa_display_device_block *dddb)
{
	size_t i;

	printf("    Interface Type: %s",
	       vesa_display_device_interface_type_name(dddb->interface_type));
	if (dddb->num_channels != 0) {
		const char *kind;
		switch (dddb->interface_type) {
		case DI_CTA_VESA_DISPLAY_DEVICE_INTERFACE_LVDS:
		case DI_CTA_VESA_DISPLAY_DEVICE_INTERFACE_RSDS:
			kind = "lanes";
			break;
		default:
			kind = "channels";
			break;
		}
		printf(" %d %s", dddb->num_channels, kind);
	}
	printf("\n");

	printf("    Interface Standard Version: %d.%d\n",
	       dddb->interface_version, dddb->interface_release);

	printf("    Content Protection Support: %s\n",
	       vesa_display_device_content_protection_name(dddb->content_protection));

	printf("    Minimum Clock Frequency: %d MHz\n", dddb->min_clock_freq_mhz);
	printf("    Maximum Clock Frequency: %d MHz\n", dddb->max_clock_freq_mhz);
	printf("    Device Native Pixel Format: %dx%d\n",
	       dddb->native_horiz_pixels, dddb->native_vert_pixels);
	printf("    Aspect Ratio: %.2f\n", dddb->aspect_ratio);
	printf("    Default Orientation: %s\n",
	       vesa_display_device_default_orientation_name(dddb->default_orientation));
	printf("    Rotation Capability: %s\n",
	       vesa_display_device_rotation_cap_name(dddb->rotation_cap));
	printf("    Zero Pixel Location: %s\n",
	       vesa_display_device_zero_pixel_location_name(dddb->zero_pixel_location));
	printf("    Scan Direction: %s\n",
	       vesa_display_device_scan_direction_name(dddb->scan_direction));
	printf("    Subpixel Information: %s\n",
	       vesa_display_device_subpixel_layout_name(dddb->subpixel_layout));
	printf("    Horizontal and vertical dot/pixel pitch: %.2f x %.2f mm\n",
	       dddb->horiz_pitch_mm, dddb->vert_pitch_mm);
	printf("    Dithering: %s\n",
	       vesa_display_device_dithering_type_name(dddb->dithering_type));
	printf("    Direct Drive: %s\n", dddb->direct_drive ? "Yes" : "No");
	printf("    Overdrive %srecommended\n",
	       dddb->overdrive_not_recommended ? "not " : "");
	printf("    Deinterlacing: %s\n", dddb->deinterlacing ? "Yes" : "No");

	printf("    Audio Support: %s\n", dddb->audio_support ? "Yes" : "No");
	printf("    Separate Audio Inputs Provided: %s\n",
	       dddb->separate_audio_inputs ? "Yes" : "No");
	printf("    Audio Input Override: %s\n",
	       dddb->audio_input_override ? "Yes" : "No");
	if (dddb->audio_delay_provided)
		printf("    Audio Delay: %d ms\n", dddb->audio_delay_ms);
	else
		printf("    Audio Delay: no information provided\n");

	printf("    Frame Rate/Mode Conversion: %s\n",
	       vesa_display_device_frame_rate_conversion_name(dddb->frame_rate_conversion));
	if (dddb->frame_rate_range_hz != 0)
		printf("    Frame Rate Range: %d fps +/- %d fps\n",
		       dddb->frame_rate_native_hz, dddb->frame_rate_range_hz);
	else
		printf("    Nominal Frame Rate: %d fps\n",
		       dddb->frame_rate_native_hz);
	printf("    Color Bit Depth: %d @ interface, %d @ display\n",
	       dddb->bit_depth_interface, dddb->bit_depth_display);

	if (dddb->additional_primary_chromaticities_len > 0) {
		printf("    Additional Primary Chromaticities:\n");
		for (i = 0; i < dddb->additional_primary_chromaticities_len; i++)
			printf("      Primary %zu:   %.4f, %.4f\n", 4 + i,
			       truncate_chromaticity_coord(dddb->additional_primary_chromaticities[i].x),
			       truncate_chromaticity_coord(dddb->additional_primary_chromaticities[i].y));
	}

	printf("    Response Time %s: %d ms\n",
	       vesa_display_device_resp_time_transition_name(dddb->resp_time_transition),
	       dddb->resp_time_ms);
	printf("    Overscan: %d%% x %d%%\n",
	       dddb->overscan_horiz_pct, dddb->overscan_vert_pct);
}

static uint8_t
encode_max_luminance(float max)
{
	if (max == 0)
		return 0;
	return (uint8_t) (log2f(max / 50) * 32);
}

static uint8_t
encode_min_luminance(float min, float max)
{
	if (min == 0)
		return 0;
	return (uint8_t) (255 * sqrtf(min / max * 100));
}

static void
print_cta_hdr_static_metadata(const struct di_cta_hdr_static_metadata_block *metadata)
{
	printf("    Electro optical transfer functions:\n");
	if (metadata->eotfs->traditional_sdr)
		printf("      Traditional gamma - SDR luminance range\n");
	if (metadata->eotfs->traditional_hdr)
		printf("      Traditional gamma - HDR luminance range\n");
	if (metadata->eotfs->pq)
		printf("      SMPTE ST2084\n");
	if (metadata->eotfs->hlg)
		printf("      Hybrid Log-Gamma\n");

	printf("    Supported static metadata descriptors:\n");
	if (metadata->descriptors->type1)
		printf("      Static metadata type 1\n");

	/* TODO: figure out a way to print raw values? */
	if (metadata->desired_content_max_luminance != 0)
		printf("    Desired content max luminance: %" PRIu8 " (%.3f cd/m^2)\n",
		       encode_max_luminance(metadata->desired_content_max_luminance),
		       metadata->desired_content_max_luminance);
	if (metadata->desired_content_max_frame_avg_luminance != 0)
		printf("    Desired content max frame-average luminance: %" PRIu8 " (%.3f cd/m^2)\n",
		       encode_max_luminance(metadata->desired_content_max_frame_avg_luminance),
		       metadata->desired_content_max_frame_avg_luminance);
	if (metadata->desired_content_min_luminance != 0)
		printf("    Desired content min luminance: %" PRIu8 " (%.3f cd/m^2)\n",
		       encode_min_luminance(metadata->desired_content_min_luminance,
					    metadata->desired_content_max_luminance),
		       metadata->desired_content_min_luminance);
}

static void
print_cta_hdr_dynamic_metadata(const struct di_cta_hdr_dynamic_metadata_block *metadata)
{
	if (metadata->type1) {
		printf("    HDR Dynamic Metadata Type 1\n");
		printf("      Version: %d\n", metadata->type1->type_1_hdr_metadata_version);
	}
	if (metadata->type2) {
		printf("    HDR Dynamic Metadata Type 2\n");
		printf("      Version: %d\n", metadata->type2->ts_103_433_spec_version);
		if (metadata->type2->ts_103_433_1_capable)
			printf("      ETSI TS 103 433-1 capable\n");
		if (metadata->type2->ts_103_433_2_capable)
			printf("      ETSI TS 103 433-2 [i.12] capable\n");
		if (metadata->type2->ts_103_433_3_capable)
			printf("      ETSI TS 103 433-3 [i.13] capable\n");
	}
	if (metadata->type3) {
		printf("    HDR Dynamic Metadata Type 3\n");
	}
	if (metadata->type4) {
		printf("    HDR Dynamic Metadata Type 4\n");
		printf("      Version: %d\n", metadata->type4->type_4_hdr_metadata_version);
	}
	if (metadata->type256) {
		printf("    HDR Dynamic Metadata Type 256\n");
		printf("      Version: %d\n", metadata->type256->graphics_overlay_flag_version);
	}
}

static void
print_cta_vesa_transfer_characteristics(const struct di_cta_vesa_transfer_characteristics_block *tf)
{
	size_t i;

	switch (tf->usage) {
	case DI_CTA_VESA_TRANSFER_CHARACTERISTIC_USAGE_WHITE:
		printf("    White");
		break;
	case DI_CTA_VESA_TRANSFER_CHARACTERISTIC_USAGE_RED:
		printf("    Red");
		break;
	case DI_CTA_VESA_TRANSFER_CHARACTERISTIC_USAGE_GREEN:
		printf("    Green");
		break;
	case DI_CTA_VESA_TRANSFER_CHARACTERISTIC_USAGE_BLUE:
		printf("    Blue");
		break;
	}

	printf(" transfer characteristics:");
	for (i = 0; i < tf->points_len; i++)
		printf(" %u", (uint16_t) roundf(tf->points[i] * 1023.0f));
	printf("\n");

	uncommon_features.cta_transfer_characteristics = true;
}

static const char *
cta_audio_format_name(enum di_cta_audio_format format)
{
	switch (format) {
	case DI_CTA_AUDIO_FORMAT_LPCM:
		return "Linear PCM";
	case DI_CTA_AUDIO_FORMAT_AC3:
		return "AC-3";
	case DI_CTA_AUDIO_FORMAT_MPEG1:
		return "MPEG 1 (Layers 1 & 2)";
	case DI_CTA_AUDIO_FORMAT_MP3:
		return "MPEG 1 Layer 3 (MP3)";
	case DI_CTA_AUDIO_FORMAT_MPEG2:
		return "MPEG2 (multichannel)";
	case DI_CTA_AUDIO_FORMAT_AAC_LC:
		return "AAC LC";
	case DI_CTA_AUDIO_FORMAT_DTS:
		return "DTS";
	case DI_CTA_AUDIO_FORMAT_ATRAC:
		return "ATRAC";
	case DI_CTA_AUDIO_FORMAT_ONE_BIT_AUDIO:
		return "One Bit Audio";
	case DI_CTA_AUDIO_FORMAT_ENHANCED_AC3:
		return "Enhanced AC-3 (DD+)";
	case DI_CTA_AUDIO_FORMAT_DTS_HD:
		return "DTS-HD";
	case DI_CTA_AUDIO_FORMAT_MAT:
		return "MAT (MLP)";
	case DI_CTA_AUDIO_FORMAT_DST:
		return "DST";
	case DI_CTA_AUDIO_FORMAT_WMA_PRO:
		return "WMA Pro";
	case DI_CTA_AUDIO_FORMAT_MPEG4_HE_AAC:
		return "MPEG-4 HE AAC";
	case DI_CTA_AUDIO_FORMAT_MPEG4_HE_AAC_V2:
		return "MPEG-4 HE AAC v2";
	case DI_CTA_AUDIO_FORMAT_MPEG4_AAC_LC:
		return "MPEG-4 AAC LC";
	case DI_CTA_AUDIO_FORMAT_DRA:
		return "DRA";
	case DI_CTA_AUDIO_FORMAT_MPEG4_HE_AAC_MPEG_SURROUND:
		return "MPEG-4 HE AAC + MPEG Surround";
	case DI_CTA_AUDIO_FORMAT_MPEG4_AAC_LC_MPEG_SURROUND:
		return "MPEG-4 AAC LC + MPEG Surround";
	case DI_CTA_AUDIO_FORMAT_MPEGH_3D:
		return "MPEG-H 3D Audio";
	case DI_CTA_AUDIO_FORMAT_AC4:
		return "AC-4";
	case DI_CTA_AUDIO_FORMAT_LPCM_3D:
		return "L-PCM 3D Audio";
	}
	abort();
}

static const char *
cta_sad_mpegh_3d_level_name(enum di_cta_sad_mpegh_3d_level level)
{
	switch (level) {
	case DI_CTA_SAD_MPEGH_3D_LEVEL_UNSPECIFIED:
		return "Unspecified";
	case DI_CTA_SAD_MPEGH_3D_LEVEL_1:
		return "Level 1";
	case DI_CTA_SAD_MPEGH_3D_LEVEL_2:
		return "Level 2";
	case DI_CTA_SAD_MPEGH_3D_LEVEL_3:
		return "Level 3";
	case DI_CTA_SAD_MPEGH_3D_LEVEL_4:
		return "Level 4";
	case DI_CTA_SAD_MPEGH_3D_LEVEL_5:
		return "Level 5";
	}
	abort();
}

static void
print_cta_sads(const struct di_cta_sad *const *sads)
{
	size_t i;
	const struct di_cta_sad *sad;

	for (i = 0; sads[i] != NULL; i++) {
		sad = sads[i];

		printf("    %s:\n", cta_audio_format_name(sad->format));
		if (sad->max_channels != 0)
			printf("      Max channels: %d\n", sad->max_channels);

		if (sad->mpegh_3d)
			printf("      MPEG-H 3D Audio Level: %s\n",
			       cta_sad_mpegh_3d_level_name(sad->mpegh_3d->level));

		printf("      Supported sample rates (kHz):");
		if (sad->supported_sample_rates->has_192_khz)
			printf(" 192");
		if (sad->supported_sample_rates->has_176_4_khz)
			printf(" 176.4");
		if (sad->supported_sample_rates->has_96_khz)
			printf(" 96");
		if (sad->supported_sample_rates->has_88_2_khz)
			printf(" 88.2");
		if (sad->supported_sample_rates->has_48_khz)
			printf(" 48");
		if (sad->supported_sample_rates->has_44_1_khz)
			printf(" 44.1");
		if (sad->supported_sample_rates->has_32_khz)
			printf(" 32");
		printf("\n");

		if (sad->lpcm) {
			printf("      Supported sample sizes (bits):");
			if (sad->lpcm->has_sample_size_24_bits)
				printf(" 24");
			if (sad->lpcm->has_sample_size_20_bits)
				printf(" 20");
			if (sad->lpcm->has_sample_size_16_bits)
				printf(" 16");
			printf("\n");
		}

		if (sad->max_bitrate_kbs != 0)
			printf("      Maximum bit rate: %d kb/s\n", sad->max_bitrate_kbs);

		if (sad->enhanced_ac3 && sad->enhanced_ac3->supports_joint_object_coding)
			printf("      Supports Joint Object Coding\n");
		if (sad->enhanced_ac3 && sad->enhanced_ac3->supports_joint_object_coding_ACMOD28)
			printf("      Supports Joint Object Coding with ACMOD28\n");

		if (sad->mat) {
			if (sad->mat->supports_object_audio_and_channel_based) {
				printf("      Supports Dolby TrueHD, object audio PCM and channel-based PCM\n");
				printf("      Hash calculation %srequired for object audio PCM or channel-based PCM\n",
				       sad->mat->requires_hash_calculation ? "" : "not ");
			} else {
				printf("      Supports only Dolby TrueHD\n");
			}
		}

		if (sad->wma_pro) {
			printf("      Profile: %u\n",sad->wma_pro->profile);
		}

		if (sad->mpegh_3d && sad->mpegh_3d->low_complexity_profile)
			printf("      Supports MPEG-H 3D Audio Low Complexity Profile\n");
		if (sad->mpegh_3d && sad->mpegh_3d->baseline_profile)
			printf("      Supports MPEG-H 3D Audio Baseline Profile\n");

		if (sad->mpeg_aac) {
			printf("      AAC audio frame lengths:%s%s\n",
			       sad->mpeg_aac->has_frame_length_1024 ? " 1024_TL" : "",
			       sad->mpeg_aac->has_frame_length_960 ? " 960_TL" : "");
		}

		if (sad->mpeg_surround) {
			printf("      Supports %s signaled MPEG Surround data\n",
			       sad->mpeg_surround->signaling == DI_CTA_SAD_MPEG_SURROUND_SIGNALING_IMPLICIT ?
			       "only implicitly" : "implicitly and explicitly");
		}

		if (sad->mpeg_aac_le && sad->mpeg_aac_le->supports_multichannel_sound)
			printf("      Supports 22.2ch System H\n");
	}
}

static void
print_ycbcr420_cap_map(const struct di_edid_cta *cta,
		       const struct di_cta_ycbcr420_cap_map_block *map)
{
	const struct di_cta_data_block *const *data_blocks;
	const struct di_cta_data_block *data_block;
	enum di_cta_data_block_tag tag;
	const struct di_cta_svd *const *svds;
	size_t global_svd_index, block_index_offset = 0;
	size_t i, j;

	data_blocks = di_edid_cta_get_data_blocks(cta);

	for (i = 0; data_blocks[i] != NULL; i++) {
		data_block = data_blocks[i];

		tag = di_cta_data_block_get_tag(data_block);
		if (tag != DI_CTA_DATA_BLOCK_VIDEO)
			continue;

		svds = di_cta_data_block_get_video(data_block)->svds;
		for (j = 0; svds[j] != NULL; j++) {
			global_svd_index = svds[j]->original_index + block_index_offset;
			if (di_cta_ycbcr420_cap_map_supported(map, global_svd_index))
				printf_cta_svd(svds[j]);
		}
		if (j > 0)
			block_index_offset += svds[j - 1]->original_index;
	}
}

static void
printf_cta_svrs(const struct di_cta_svr *const *svrs)
{
	size_t i;
	const struct di_cta_svr *svr;

	/* TODO: resolve the references once we parse all timings and print
	 * the resolved timings */

	for (i = 0; svrs[i] != NULL; i++) {
		svr = svrs[i];

		switch (svr->type) {
		case DI_CTA_SVR_TYPE_VIC:
			printf("    VIC %3u\n", svr->vic);
			break;
		case DI_CTA_SVR_TYPE_DTD_INDEX:
			printf("    DTD %3u\n", svr->dtd_index + 1);
			break;
		case DI_CTA_SVR_TYPE_T7T10VTDB:
			printf("    VTDB %3u\n", svr->t7_t10_vtdb_index + 1);
			break;
		case DI_CTA_SVR_TYPE_FIRST_T8VTDB:
			printf("    T8VTDB\n");
			break;
		}
	}
}

static const char *
cta_infoframe_type_name(enum di_cta_infoframe_type type)
{
	switch (type) {
	case DI_CTA_INFOFRAME_TYPE_AUXILIARY_VIDEO_INFORMATION:
		return "Auxiliary Video Information InfoFrame (2)";
	case DI_CTA_INFOFRAME_TYPE_SOURCE_PRODUCT_DESCRIPTION:
		return "Source Product Description InfoFrame (3)";
	case DI_CTA_INFOFRAME_TYPE_AUDIO:
		return "Audio InfoFrame (4)";
	case DI_CTA_INFOFRAME_TYPE_MPEG_SOURCE:
		return "MPEG Source InfoFrame (5)";
	case DI_CTA_INFOFRAME_TYPE_NTSC_VBI:
		return "NTSC VBI InfoFrame (6)";
	case DI_CTA_INFOFRAME_TYPE_DYNAMIC_RANGE_AND_MASTERING:
		return "Dynamic Range and Mastering InfoFrame (7)";
	}
	abort();
}

static void
print_infoframes(const struct di_cta_infoframe_descriptor *const *infoframes)
{
	size_t i;
	const struct di_cta_infoframe_descriptor *infoframe;

	for (i = 0; infoframes[i] != NULL; i++) {
		infoframe = infoframes[i];
		printf("    %s\n",
		       cta_infoframe_type_name(infoframe->type));
	}
}

static void
print_did_type_vii_timing(const struct di_displayid_type_i_ii_vii_timing *t, int vtdb_index)
{
	char buf[32];
	snprintf(buf, 32, "VTDB %d", vtdb_index + 1);
	print_displayid_type_i_ii_vii_timing(t, 4, buf);
}

static void
print_speaker_alloc(const struct di_cta_speaker_allocation *speaker_alloc, const char *prefix)
{
	if (speaker_alloc->fl_fr)
		printf("%sFL/FR - Front Left/Right\n", prefix);
	if (speaker_alloc->lfe1)
		printf("%sLFE1 - Low Frequency Effects 1\n", prefix);
	if (speaker_alloc->fc)
		printf("%sFC - Front Center\n", prefix);
	if (speaker_alloc->bl_br)
		printf("%sBL/BR - Back Left/Right\n", prefix);
	if (speaker_alloc->bc)
		printf("%sBC - Back Center\n", prefix);
	if (speaker_alloc->flc_frc)
		printf("%sFLc/FRc - Front Left/Right of Center\n", prefix);
	if (speaker_alloc->flw_frw)
		printf("%sFLw/FRw - Front Left/Right Wide\n", prefix);
	if (speaker_alloc->tpfl_tpfr)
		printf("%sTpFL/TpFR - Top Front Left/Right\n", prefix);
	if (speaker_alloc->tpc)
		printf("%sTpC - Top Center\n", prefix);
	if (speaker_alloc->tpfc)
		printf("%sTpFC - Top Front Center\n", prefix);
	if (speaker_alloc->ls_rs)
		printf("%sLS/RS - Left/Right Surround\n", prefix);
	if (speaker_alloc->tpbc)
		printf("%sTpBC - Top Back Center\n", prefix);
	if (speaker_alloc->lfe2)
		printf("%sLFE2 - Low Frequency Effects 2\n", prefix);
	if (speaker_alloc->sil_sir)
		printf("%sSiL/SiR - Side Left/Right\n", prefix);
	if (speaker_alloc->tpsil_tpsir)
		printf("%sTpSiL/TpSiR - Top Side Left/Right\n", prefix);
	if (speaker_alloc->tpbl_tpbr)
		printf("%sTpBL/TpBR - Top Back Left/Right\n", prefix);
	if (speaker_alloc->btfc)
		printf("%sBtFC - Bottom Front Center\n", prefix);
	if (speaker_alloc->btfl_btfr)
		printf("%sBtFL/BtFR - Bottom Front Left/Right\n", prefix);
}

static void
print_hdmi_audio(const struct di_cta_hdmi_audio_block *hdmi_audio)
{
	const struct di_cta_hdmi_audio_3d *audio_3d = hdmi_audio->audio_3d;
	const struct di_cta_hdmi_audio_multi_stream *ms = hdmi_audio->multi_stream;

	if (ms) {
		printf("    Max Stream Count: %u\n", ms->max_streams);
		if (ms->supports_non_mixed)
			printf("    Supports MS NonMixed\n");
	}

	if (!audio_3d)
		return;

	print_cta_sads(audio_3d->sads);

	switch (audio_3d->channels) {
	case DI_CTA_HDMI_AUDIO_3D_CHANNELS_UNKNOWN:
		printf("    Unknown Speaker Allocation\n");
		break;
	case DI_CTA_HDMI_AUDIO_3D_CHANNELS_10_2:
		printf("    Speaker Allocation for 10.2 channels:\n");
		break;
	case DI_CTA_HDMI_AUDIO_3D_CHANNELS_22_2:
		printf("    Speaker Allocation for 22.2 channels:\n");
		break;
	case DI_CTA_HDMI_AUDIO_3D_CHANNELS_30_2:
		printf("    Speaker Allocation for 30.2 channels:\n");
		break;
	}

	print_speaker_alloc (&audio_3d->speakers, "      ");
}

static void
print_hdmi_latency(const char *type, bool supported, int latency)
{
	if (!supported) {
		printf("    %s latency: %s not supported\n", type, type);
		return;
	}

	if (latency == 0) {
		printf("    %s latency: invalid or unknown\n", type);
		return;
	}

	printf("    %s latency: %u ms\n", type, latency);
}

static void
print_cta_hdmi(const struct di_cta_vendor_hdmi_block *hdmi)
{
	unsigned int i;

	printf("    Source physical address: %x.%x.%x.%x\n",
	       get_bit_range((uint8_t) (hdmi->source_phys_addr >> 8), 7, 4),
	       get_bit_range((uint8_t) (hdmi->source_phys_addr >> 8), 3, 0),
	       get_bit_range((uint8_t) (hdmi->source_phys_addr & 0xff), 7, 4),
	       get_bit_range((uint8_t) (hdmi->source_phys_addr & 0xff), 3, 0));

	if (hdmi->supports_ai)
		printf("    Supports_AI\n");
	if (hdmi->supports_dc_48bit)
		printf("    DC_48bit\n");
	if (hdmi->supports_dc_36bit)
		printf("    DC_36bit\n");
	if (hdmi->supports_dc_30bit)
		printf("    DC_30bit\n");
	if (hdmi->supports_dc_y444)
		printf("    DC_Y444\n");
	if (hdmi->supports_dvi_dual)
		printf("    DVI_Dual\n");

	if (hdmi->max_tmds_clock > 0)
		printf("    Maximum TMDS clock: %u MHz\n", hdmi->max_tmds_clock);

	if (hdmi->supports_content_graphics || hdmi->supports_content_photo ||
	    hdmi->supports_content_cinema || hdmi->supports_content_game) {
		printf("    Supported Content Types:\n");
		if (hdmi->supports_content_graphics)
			printf("      Graphics\n");
		if (hdmi->supports_content_photo)
			printf("      Photo\n");
		if (hdmi->supports_content_cinema)
			printf("      Cinema\n");
		if (hdmi->supports_content_game)
			printf("      Game\n");
	}

	if (hdmi->has_latency) {
		print_hdmi_latency("Video", hdmi->supports_progressive_video,
				   hdmi->progressive_video_latency);
		print_hdmi_latency("Audio", hdmi->supports_progressive_audio,
				   hdmi->progressive_audio_latency);
	}

	if (hdmi->has_interlaced_latency) {
		print_hdmi_latency("Interlaced video", hdmi->supports_interlaced_video,
				   hdmi->interlaced_video_latency);
		print_hdmi_latency("Interlaced audio", hdmi->supports_interlaced_audio,
				   hdmi->interlaced_audio_latency);
	}

	if (hdmi->vics_len > 0) {
		printf("    Extended HDMI video details:\n");
		printf("      HDMI VICs:\n");
		for (i = 0; i < hdmi->vics_len; i++) {
			printf("    ");
			print_cta_hdmi_vic(hdmi->vics[i]);
			printf("\n");
		}
	}
}

static int
peak_lum_get_index(int peak_lum)
{
	switch (peak_lum) {
	case 0:
		return 0;
	case 200:
		return 1;
	case 300:
		return 2;
	case 400:
		return 3;
	case 500:
		return 4;
	case 600:
		return 5;
	case 800:
		return 6;
	case 1000:
		return 7;
	case 1200:
		return 8;
	case 1500:
		return 9;
	case 2000:
		return 10;
	case 2500:
		return 11;
	case 3000:
		return 12;
	case 4000:
		return 13;
	case 6000:
		return 14;
	case 8000:
		return 15;
	}
	abort(); /* unreachable */
}

static int
ff_peak_lum_get_index(int ff_peak_lum, int peak_lum)
{
	float div;

	if (peak_lum == 0)
		return 0;

	div = (float)ff_peak_lum / (float)peak_lum;

	if (fabs(div - 0.1) <= 1e-5)
		return 0;
	else if (fabs(div - 0.2) <= 1e-5)
		return 1;
	else if (fabs(div - 0.4) <= 1e-5)
		return 2;
	else if (fabs(div - 0.8) <= 1e-5)
		return 3;

	abort(); /* unreachable */
}

static void
print_cta_hdr10plus(const struct di_cta_hdr10plus_block *hdr10plus)
{
	int peak_lum_index, ff_peak_lum_index;

	peak_lum_index = peak_lum_get_index(hdr10plus->peak_lum);
	ff_peak_lum_index = ff_peak_lum_get_index(hdr10plus->ff_peak_lum,
						  hdr10plus->peak_lum);

	printf("    Application Version: %d\n", hdr10plus->version);
	printf("    Full Frame Peak Luminance Index: %d\n", ff_peak_lum_index);
	printf("    Peak Luminance Index: %d\n", peak_lum_index);
}

static double
pq2nits(double pq)
{
	const double m1 = 2610.0 / 16384.0;
	const double m2 = 128.0 * (2523.0 / 4096.0);
	const double c1 = 3424.0 / 4096.0;
	const double c2 = 32.0 * (2413.0 / 4096.0);
	const double c3 = 32.0 * (2392.0 / 4096.0);
	double e = pow(pq, 1.0 / m2);
	double v = e - c1;

	if (v < 0)
		v = 0;
	v /= c2 - c3 * e;
	v = pow(v, 1.0 / m1);
	return v * 10000.0;
}

static void
print_cta_dolby_video(const struct di_cta_dolby_video_block *dv)
{
	switch (dv->version) {
	case DI_CTA_DOLBY_VIDEO_VERSION0:
		printf("    Version: 0 (22 bytes)\n");

		if (dv->v0->yuv422_12bit)
			printf("    Supports YUV422 12 bit\n");
		if (dv->v0->supports_2160p60)
			printf("    Supports 2160p60\n");
		if (dv->v0->global_dimming)
			printf("    Supports global dimming\n");

		printf("    DM Version: %u.%u\n",
		       dv->v0->dynamic_metadata_version_major,
		       dv->v0->dynamic_metadata_version_minor);

		printf("    Target Min PQ: %u (%.8f cd/m^2)\n",
		       dv->v0->target_pq_12b_level_min,
		       pq2nits(dv->v0->target_pq_12b_level_min / 4095.0));
		printf("    Target Max PQ: %u (%u cd/m^2)\n",
		       dv->v0->target_pq_12b_level_min,
		       (unsigned)pq2nits(dv->v0->target_pq_12b_level_min / 4095.0));

		printf("    Rx, Ry: %.8f, %.8f\n", dv->v0->red_x, dv->v0->red_y);
		printf("    Gx, Gy: %.8f, %.8f\n", dv->v0->green_x, dv->v0->green_y);
		printf("    Bx, By: %.8f, %.8f\n", dv->v0->blue_x, dv->v0->blue_y);
		printf("    Wx, Wy: %.8f, %.8f\n", dv->v0->white_x, dv->v0->white_x);
		break;
	case DI_CTA_DOLBY_VIDEO_VERSION1:
		printf("    Version: 1 (%d bytes)\n", dv->v1->unique_primaries ? 12 : 15);

		if (dv->v1->yuv422_12bit)
			printf("    Supports YUV422 12 bit\n");
		if (dv->v1->supports_2160p60)
			printf("    Supports 2160p60\n");
		if (dv->v1->global_dimming)
			printf("    Supports global dimming\n");

		printf("    DM Version: %u.x\n", dv->v1->dynamic_metadata_version);

		switch (dv->v1->colorimetry) {
		case DI_CTA_DOLBY_VIDEO_COLORIMETRY_P3_D65:
			printf("    Colorimetry: P3-D65\n");
			break;
		case DI_CTA_DOLBY_VIDEO_COLORIMETRY_BT_709:
			printf("    Colorimetry: ITU-R BT.709\n");
			break;
		}

		printf("    Low Latency: %s\n",
		       dv->v1->mode_low_latency ? "Standard + Low Latency" : "Only Standard");

		printf("    Target Min Luminance: %.8f cd/m^2\n", dv->v1->target_luminance_min);
		printf("    Target Max Luminance: %u cd/m^2\n", (unsigned)dv->v1->target_luminance_max);

		printf("    %sRx, Ry: %.8f, %.8f\n",
		       dv->v1->unique_primaries ? "Unique " : "", dv->v1->red_x, dv->v1->red_y);
		printf("    %sGx, Gy: %.8f, %.8f\n",
		       dv->v1->unique_primaries ? "Unique " : "", dv->v1->green_x, dv->v1->green_y);
		printf("    %sBx, By: %.8f, %.8f\n",
		       dv->v1->unique_primaries ? "Unique " : "", dv->v1->blue_x, dv->v1->blue_y);
		break;
	case DI_CTA_DOLBY_VIDEO_VERSION2:
		printf("    Version: 2 (12 bytes)\n");

		if (dv->v2->yuv422_12bit)
			printf("    Supports YUV422 12 bit\n");
		if (dv->v2->backlight_control)
			printf("    Supports Backlight Control\n");
		if (dv->v2->global_dimming)
			printf("    Supports global dimming\n");

		printf("    DM Version: %u.x\n", dv->v2->dynamic_metadata_version);

		printf("    Backlt Min Luma: %u cd/m^2\n", (unsigned)dv->v2->backlight_luminance_min);

		printf("    Interface: ");
		if (dv->v2->mode_standard && dv->v2->mode_low_latency_hdmi)
			printf("Standard + Low-Latency + Low-Latency-HDMI\n");
		else if (dv->v2->mode_low_latency_hdmi)
			printf("Low-Latency + Low-Latency-HDMI\n");
		else if (dv->v2->mode_standard)
			printf("Standard + Low-Latency\n");
		else
			printf("Low-Latency\n");

		printf("    Supports 10b 12b 444: ");
		switch (dv->v2->yuv444) {
		case DI_CTA_DOLBY_VIDEO_YUV444_NONE:
			printf("Not supported\n");
			break;
		case DI_CTA_DOLBY_VIDEO_YUV444_10_BITS:
			printf("10 bit\n");
			break;
		case DI_CTA_DOLBY_VIDEO_YUV444_12_BITS:
			printf("12 bit\n");
			break;
		}

		printf("    Target Min PQ v2: %u (%.8f cd/m^2)\n",
		       dv->v2->target_pq_12b_level_min,
		       pq2nits(dv->v2->target_pq_12b_level_min / 4095.0));
		printf("    Target Max PQ v2: %u (%u cd/m^2)\n",
		       dv->v2->target_pq_12b_level_max,
		       (unsigned)pq2nits(dv->v2->target_pq_12b_level_max / 4095.0));

		printf("    Unique Rx, Ry: %.8f, %.8f\n", dv->v2->red_x, dv->v2->red_y);
		printf("    Unique Gx, Gy: %.8f, %.8f\n", dv->v2->green_x, dv->v2->green_y);
		printf("    Unique Bx, By: %.8f, %.8f\n", dv->v2->blue_x, dv->v2->blue_y);
		break;
	}
}

static const char *
max_frl_rate_name(enum di_cta_hdmi_frl frl)
{
	switch (frl) {
	case DI_CTA_HDMI_FRL_3GBPS_3LANES:
		return "3 Gbps per lane on 3 lanes";
	case DI_CTA_HDMI_FRL_6GBPS_3LANES:
		return "3 and 6 Gbps per lane on 3 lanes";
	case DI_CTA_HDMI_FRL_6GBPS_4LANES:
		return "3 and 6 Gbps per lane on 3 lanes, 6 Gbps on 4 lanes";
	case DI_CTA_HDMI_FRL_8GBPS_4LANES:
		return "3 and 6 Gbps per lane on 3 lanes, 6 and 8 Gbps on 4 lanes";
	case DI_CTA_HDMI_FRL_10GBPS_4LANES:
		return "3 and 6 Gbps per lane on 3 lanes, 6, 8 and 10 Gbps on 4 lanes";
	case DI_CTA_HDMI_FRL_12GBPS_4LANES:
		return "3 and 6 Gbps per lane on 3 lanes, 6, 8, 10 and 12 Gbps on 4 lanes";
	default:
		return "Not Supported";
	}
}

static const char *
dsc_max_slices_name(enum di_cta_hdmi_dsc_max_slices max_slice)
{
	switch (max_slice) {
	case DI_CTA_HDMI_DSC_MAX_SLICES_1_340MHZ:
		return "up to 1 slice and up to (340 MHz/Ksliceadjust) pixel clock per slice";
	case DI_CTA_HDMI_DSC_MAX_SLICES_2_340MHZ:
		return "up to 2 slices and up to (340 MHz/Ksliceadjust) pixel clock per slice";
	case DI_CTA_HDMI_DSC_MAX_SLICES_4_340MHZ:
		return "up to 4 slices and up to (340 MHz/Ksliceadjust) pixel clock per slice";
	case DI_CTA_HDMI_DSC_MAX_SLICES_8_340MHZ:
		return "up to 8 slices and up to (340 MHz/Ksliceadjust) pixel clock per slice";
	case DI_CTA_HDMI_DSC_MAX_SLICES_8_400MHZ:
		return "up to 8 slices and up to (400 MHz/Ksliceadjust) pixel clock per slice";
	case DI_CTA_HDMI_DSC_MAX_SLICES_12_400MHZ:
		return "up to 12 slices and up to (400 MHz/Ksliceadjust) pixel clock per slice";
	case DI_CTA_HDMI_DSC_MAX_SLICES_16_400MHZ:
		return "up to 16 slices and up to (400 MHz/Ksliceadjust) pixel clock per slice";
	default:
		return "Not Supported";
	}
}

static void
print_cta_hdmi_scds(const struct di_cta_hdmi_scds *scds)
{
	const struct di_cta_hdmi_dsc *dsc;

	printf("    Version: %u\n", scds->version);
	if (scds->max_tmds_char_rate_mhz) {
		printf("    Maximum TMDS Character Rate: %u MHz\n",
		       scds->max_tmds_char_rate_mhz);
	}
	if (scds->supports_scdc)
		printf("    SCDC Present\n");
	if (scds->supports_scdc_read_request)
		printf("    SCDC Read Request Capable\n");
	if (scds->supports_cable_status)
		printf("    Supports Cable Status\n");
	if (scds->supports_ccbpci)
		printf("    Supports Color Content Bits Per Component Indication\n");
	if (scds->supports_lte_340mcsc_scramble)
		printf("    Supports scrambling for <= 340 Mcsc\n");
	if (scds->supports_3d_independent_view)
		printf("    Supports 3D Independent View signaling\n");
	if (scds->supports_3d_dual_view)
		printf("    Supports 3D Dual View signaling\n");
	if (scds->supports_3d_osd_disparity)
		printf("    Supports 3D OSD Disparity signaling\n");

	if (scds->max_frl_rate != DI_CTA_HDMI_FRL_UNSUPPORTED) {
		printf("    Max Fixed Rate Link: %s\n",
		       max_frl_rate_name (scds->max_frl_rate));
	}

	if (scds->supports_uhd_vic)
		printf("    Supports UHD VIC\n");
	if (scds->supports_dc_48bit_420)
		printf("    Supports 16-bits/component Deep Color 4:2:0 Pixel Encoding\n");
	if (scds->supports_dc_36bit_420)
		printf("    Supports 12-bits/component Deep Color 4:2:0 Pixel Encoding\n");
	if (scds->supports_dc_30bit_420)
		printf("    Supports 10-bits/component Deep Color 4:2:0 Pixel Encoding\n");
	if (scds->supports_fapa_end_extended)
		printf("    Supports FAPA End Extended\n");
	if (scds->supports_qms)
		printf("    Supports QMS\n");
	if (scds->m_delta)
		printf("    Supports Mdelta\n");
	if (scds->supports_cinema_vrr)
		printf("    Supports media rates below VRRmin (CinemaVRR, deprecated)\n");
	if (scds->supports_neg_mvrr)
		printf("    Supports negative Mvrr values\n");
	if (scds->supports_fva)
		printf("    Supports Fast Vactive\n");
	if (scds->supports_allm)
		printf("    Supports Auto Low-Latency Mode\n");
	if (scds->supports_fapa_start_location)
		printf("    Supports a FAPA in blanking after first active video line\n");

	if (scds->vrr_min_hz)
		printf("    VRRmin: %u Hz\n", scds->vrr_min_hz);
	if (scds->vrr_max_hz)
		printf("    VRRmax: %u Hz\n", scds->vrr_max_hz);

	if (scds->qms_tfr_max)
		printf("    Supports QMS TFRmax\n");
	if (scds->qms_tfr_min)
		printf("    Supports QMS TFRmin\n");

	dsc = scds->dsc;
	if (dsc) {
		printf("    Supports VESA DSC 1.2a compression\n");
		if (dsc->supports_native_420)
			printf("    Supports Compressed Video Transport for 4:2:0 Pixel Encoding\n");
		if (dsc->supports_all_bpc)
			printf("    Supports Compressed Video Transport at any valid 1/16th bit bpp\n");
		if (dsc->supports_12bpc)
			printf("    Supports 12 bpc Compressed Video Transport\n");
		if (dsc->supports_10bpc)
			printf("    Supports 10 bpc Compressed Video Transport\n");
		printf("    DSC Max Slices: %s\n",
		       dsc_max_slices_name(dsc->max_slices));
		printf("    DSC Max Fixed Rate Link: %s\n",
		       max_frl_rate_name(dsc->max_frl_rate));
		printf("    Maximum number of bytes in a line of chunks: %u\n",
		       dsc->max_total_chunk_bytes);
	}
}

static const char *
cta_data_block_tag_name(enum di_cta_data_block_tag tag)
{
	switch (tag) {
	case DI_CTA_DATA_BLOCK_AUDIO:
		return "Audio Data Block";
	case DI_CTA_DATA_BLOCK_VIDEO:
		return "Video Data Block";
	case DI_CTA_DATA_BLOCK_SPEAKER_ALLOC:
		return "Speaker Allocation Data Block";
	case DI_CTA_DATA_BLOCK_VESA_DISPLAY_TRANSFER_CHARACTERISTIC:
		return "VESA Display Transfer Characteristics Data Block";
	case DI_CTA_DATA_BLOCK_VIDEO_FORMAT:
		return "Video Format Data Block";
	case DI_CTA_DATA_BLOCK_VIDEO_CAP:
		return "Video Capability Data Block";
	case DI_CTA_DATA_BLOCK_VESA_DISPLAY_DEVICE:
		return "VESA Video Display Device Data Block";
	case DI_CTA_DATA_BLOCK_COLORIMETRY:
		return "Colorimetry Data Block";
	case DI_CTA_DATA_BLOCK_HDR_STATIC_METADATA:
		return "HDR Static Metadata Data Block";
	case DI_CTA_DATA_BLOCK_HDR_DYNAMIC_METADATA:
		return "HDR Dynamic Metadata Data Block";
	case DI_CTA_DATA_BLOCK_NATIVE_VIDEO_RESOLUTION:
		return "Native Video Resolution Data Block";
	case DI_CTA_DATA_BLOCK_VIDEO_FORMAT_PREF:
		return "Video Format Preference Data Block";
	case DI_CTA_DATA_BLOCK_YCBCR420:
		return "YCbCr 4:2:0 Video Data Block";
	case DI_CTA_DATA_BLOCK_YCBCR420_CAP_MAP:
		return "YCbCr 4:2:0 Capability Map Data Block";
	case DI_CTA_DATA_BLOCK_HDMI_AUDIO:
		return "HDMI Audio Data Block";
	case DI_CTA_DATA_BLOCK_ROOM_CONFIG:
		return "Room Configuration Data Block";
	case DI_CTA_DATA_BLOCK_SPEAKER_LOCATION:
		return "Speaker Location Data Block";
	case DI_CTA_DATA_BLOCK_INFOFRAME:
		return "InfoFrame Data Block";
	case DI_CTA_DATA_BLOCK_DISPLAYID_VIDEO_TIMING_VII:
		return "DisplayID Type VII Video Timing Data Block";
	case DI_CTA_DATA_BLOCK_DISPLAYID_VIDEO_TIMING_VIII:
		return "DisplayID Type VIII Video Timing Data Block";
	case DI_CTA_DATA_BLOCK_DISPLAYID_VIDEO_TIMING_X:
		return "DisplayID Type X Video Timing Data Block";
	case DI_CTA_DATA_BLOCK_HDMI_EDID_EXT_OVERRIDE :
		return "HDMI Forum EDID Extension Override Data Block";
	case DI_CTA_DATA_BLOCK_HDMI_SINK_CAP:
		return "HDMI Forum Sink Capability Data Block";
	case DI_CTA_DATA_BLOCK_VENDOR_HDMI:
		return "Vendor-Specific Data Block (HDMI), OUI 00-0C-03";
	case DI_CTA_DATA_BLOCK_DOLBY_VIDEO:
		return "Vendor-Specific Video Data Block (Dolby), OUI 00-D0-46";
	case DI_CTA_DATA_BLOCK_HDR10PLUS:
		return "Vendor-Specific Video Data Block (HDR10+), OUI 90-84-8B";
	case DI_CTA_DATA_BLOCK_VENDOR_HDMI_FORUM:
		return "Vendor-Specific Data Block (HDMI Forum), OUI C4-5D-D8";
	}
	return "Unknown CTA-861 Data Block";
}

static const char *
video_cap_over_underscan_name(enum di_cta_video_cap_over_underscan over_underscan,
			      const char *unknown)
{
	switch (over_underscan) {
	case DI_CTA_VIDEO_CAP_UNKNOWN_OVER_UNDERSCAN:
		return unknown;
	case DI_CTA_VIDEO_CAP_ALWAYS_OVERSCAN:
		return "Always Overscanned";
	case DI_CTA_VIDEO_CAP_ALWAYS_UNDERSCAN:
		return "Always Underscanned";
	case DI_CTA_VIDEO_CAP_BOTH_OVER_UNDERSCAN:
		return "Supports both over- and underscan";
	}
	abort();
}

void
print_cta(const struct di_edid_cta *cta)
{
	const struct di_edid_cta_flags *cta_flags;
	const struct di_cta_data_block *const *data_blocks;
	const struct di_cta_data_block *data_block;
	enum di_cta_data_block_tag data_block_tag;
	const struct di_cta_svd *const *svds;
	const struct di_cta_speaker_alloc_block *speaker_alloc;
	const struct di_cta_video_cap_block *video_cap;
	const struct di_cta_vesa_display_device_block *vesa_display_device;
	const struct di_cta_colorimetry_block *colorimetry;
	const struct di_cta_hdr_static_metadata_block *hdr_static_metadata;
	const struct di_cta_hdr_dynamic_metadata_block *hdr_dynamic_metadata;
	const struct di_cta_vesa_transfer_characteristics_block *transfer_characteristics;
	const struct di_cta_sad *const *sads;
	const struct di_cta_ycbcr420_cap_map_block *ycbcr420_cap_map;
	const struct di_cta_infoframe_block *infoframe;
	const struct di_cta_video_format_pref_block *video_format_pref;
	const struct di_edid_detailed_timing_def *const *detailed_timing_defs;
	const struct di_cta_type_vii_timing_block *type_vii_timing;
	const struct di_cta_hdmi_audio_block *hdmi_audio;
	const struct di_cta_vendor_hdmi_block *vendor_hdmi;
	const struct di_cta_hdr10plus_block *hdr10plus;
	const struct di_cta_dolby_video_block *dolby_video;
	const struct di_cta_hdmi_forum_sink_cap *hdmi_sink_cap;
	const struct di_cta_vendor_hdmi_forum_block *hdmi_forum;
	size_t i;
	int vtdb_index = 0;

	printf("  Revision: %d\n", di_edid_cta_get_revision(cta));

	cta_flags = di_edid_cta_get_flags(cta);
	if (cta_flags->it_underscan) {
		printf("  Underscans IT Video Formats by default\n");
	}
	if (cta_flags->basic_audio) {
		printf("  Basic audio support\n");
	}
	if (cta_flags->ycc444) {
		printf("  Supports YCbCr 4:4:4\n");
	}
	if (cta_flags->ycc422) {
		printf("  Supports YCbCr 4:2:2\n");
	}
	printf("  Native detailed modes: %d\n", cta_flags->native_dtds);

	data_blocks = di_edid_cta_get_data_blocks(cta);
	for (i = 0; data_blocks[i] != NULL; i++) {
		data_block = data_blocks[i];

		data_block_tag = di_cta_data_block_get_tag(data_block);
		printf("  %s:\n", cta_data_block_tag_name(data_block_tag));

		switch (data_block_tag) {
		case DI_CTA_DATA_BLOCK_VIDEO:
			svds = di_cta_data_block_get_video(data_block)->svds;
			printf_cta_svds(svds);
			break;
		case DI_CTA_DATA_BLOCK_YCBCR420:
			svds = di_cta_data_block_get_ycbcr420_video (data_block)->svds;
			printf_cta_svds(svds);
			break;
		case DI_CTA_DATA_BLOCK_SPEAKER_ALLOC:
			speaker_alloc = di_cta_data_block_get_speaker_alloc(data_block);
			print_speaker_alloc(&speaker_alloc->speakers, "    ");
			break;
		case DI_CTA_DATA_BLOCK_VIDEO_CAP:
			video_cap = di_cta_data_block_get_video_cap(data_block);
			printf("    YCbCr quantization: %s\n",
			       video_cap->selectable_ycc_quantization_range ?
			       "Selectable (via AVI YQ)" : "No Data");
			printf("    RGB quantization: %s\n",
			       video_cap->selectable_rgb_quantization_range ?
			       "Selectable (via AVI Q)" : "No Data");
			printf("    PT scan behavior: %s\n",
			       video_cap_over_underscan_name(video_cap->pt_over_underscan,
							     "No Data"));
			printf("    IT scan behavior: %s\n",
			       video_cap_over_underscan_name(video_cap->it_over_underscan,
							     "IT video formats not supported"));
			printf("    CE scan behavior: %s\n",
			       video_cap_over_underscan_name(video_cap->ce_over_underscan,
							     "CE video formats not supported"));
			break;
		case DI_CTA_DATA_BLOCK_VESA_DISPLAY_DEVICE:
			vesa_display_device = di_cta_data_block_get_vesa_display_device(data_block);
			print_cta_vesa_display_device(vesa_display_device);
			break;
		case DI_CTA_DATA_BLOCK_COLORIMETRY:
			colorimetry = di_cta_data_block_get_colorimetry(data_block);
			if (colorimetry->xvycc_601)
				printf("    xvYCC601\n");
			if (colorimetry->xvycc_709)
				printf("    xvYCC709\n");
			if (colorimetry->sycc_601)
				printf("    sYCC601\n");
			if (colorimetry->opycc_601)
				printf("    opYCC601\n");
			if (colorimetry->oprgb)
				printf("    opRGB\n");
			if (colorimetry->bt2020_cycc)
				printf("    BT2020cYCC\n");
			if (colorimetry->bt2020_ycc)
				printf("    BT2020YCC\n");
			if (colorimetry->bt2020_rgb)
				printf("    BT2020RGB\n");
			if (colorimetry->ictcp)
				printf("    ICtCp\n");
			if (colorimetry->st2113_rgb)
				printf("    ST2113RGB\n");
			break;
		case DI_CTA_DATA_BLOCK_HDR_STATIC_METADATA:
			hdr_static_metadata = di_cta_data_block_get_hdr_static_metadata(data_block);
			print_cta_hdr_static_metadata(hdr_static_metadata);
			break;
		case DI_CTA_DATA_BLOCK_HDR_DYNAMIC_METADATA:
			hdr_dynamic_metadata = di_cta_data_block_get_hdr_dynamic_metadata(data_block);
			print_cta_hdr_dynamic_metadata(hdr_dynamic_metadata);
			break;
		case DI_CTA_DATA_BLOCK_VESA_DISPLAY_TRANSFER_CHARACTERISTIC:
			transfer_characteristics = di_cta_data_block_get_vesa_transfer_characteristics(data_block);
			print_cta_vesa_transfer_characteristics(transfer_characteristics);
			break;
		case DI_CTA_DATA_BLOCK_AUDIO:
			sads = di_cta_data_block_get_audio(data_block)->sads;
			print_cta_sads(sads);
			break;
		case DI_CTA_DATA_BLOCK_YCBCR420_CAP_MAP:
			ycbcr420_cap_map = di_cta_data_block_get_ycbcr420_cap_map(data_block);
			print_ycbcr420_cap_map(cta, ycbcr420_cap_map);
			break;
		case DI_CTA_DATA_BLOCK_INFOFRAME:
			infoframe = di_cta_data_block_get_infoframe(data_block);
			printf("    VSIFs: %d\n", infoframe->num_simultaneous_vsifs - 1);
			print_infoframes(infoframe->infoframes);
			break;
		case DI_CTA_DATA_BLOCK_VIDEO_FORMAT_PREF:
			video_format_pref = di_cta_data_block_get_video_format_pref(data_block);
			printf_cta_svrs(video_format_pref->svrs);
			break;
		case DI_CTA_DATA_BLOCK_DISPLAYID_VIDEO_TIMING_VII:
			type_vii_timing = di_cta_data_block_get_did_type_vii_timing(data_block);
			print_did_type_vii_timing(type_vii_timing->timing, vtdb_index);
			vtdb_index++;
			break;
		case DI_CTA_DATA_BLOCK_HDMI_AUDIO:
			hdmi_audio = di_cta_data_block_get_hdmi_audio(data_block);
			print_hdmi_audio(hdmi_audio);
			break;
		case DI_CTA_DATA_BLOCK_VENDOR_HDMI:
			vendor_hdmi = di_cta_data_block_get_vendor_hdmi(data_block);
			print_cta_hdmi(vendor_hdmi);
			break;
		case DI_CTA_DATA_BLOCK_HDR10PLUS:
			hdr10plus = di_cta_data_block_get_hdr10plus(data_block);
			print_cta_hdr10plus(hdr10plus);
			break;
		case DI_CTA_DATA_BLOCK_DOLBY_VIDEO:
			dolby_video = di_cta_data_block_get_dolby_video(data_block);
			print_cta_dolby_video(dolby_video);
			break;
		case DI_CTA_DATA_BLOCK_HDMI_SINK_CAP:
			hdmi_sink_cap = di_cta_data_block_get_hdmi_sink_cap(data_block);
			print_cta_hdmi_scds(&hdmi_sink_cap->scds);
			break;
		case DI_CTA_DATA_BLOCK_VENDOR_HDMI_FORUM:
			hdmi_forum = di_cta_data_block_get_vendor_hdmi_forum(data_block);
			print_cta_hdmi_scds(&hdmi_forum->scds);
			break;
		default:
			break; /* Ignore */
		}
	}

	detailed_timing_defs = di_edid_cta_get_detailed_timing_defs(cta);
	if (detailed_timing_defs[0]) {
		printf("  Detailed Timing Descriptors:\n");
	}
	for (i = 0; detailed_timing_defs[i] != NULL; i++) {
		print_detailed_timing_def(detailed_timing_defs[i]);
	}
}
