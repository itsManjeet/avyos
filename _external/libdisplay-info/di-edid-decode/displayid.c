#include <inttypes.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include <libdisplay-info/cvt.h>
#include <libdisplay-info/displayid.h>

#include "di-edid-decode.h"

static bool is_displayid_base_block = true;

static void
print_displayid_display_params(const struct di_displayid_display_params *params)
{
	printf("    Image size: %.1f mm x %.1f mm\n",
	       params->horiz_image_mm, params->vert_image_mm);
	printf("    Display native pixel format: %dx%d\n",
	       params->horiz_pixels, params->vert_pixels);

	printf("    Feature support flags:\n");
	if (params->features->audio)
		printf("      Audio support on video interface\n");
	if (params->features->separate_audio_inputs)
		printf("      Separate audio inputs provided\n");
	if (params->features->audio_input_override)
		printf("      Audio input override\n");
	if (params->features->power_management)
		printf("      Power management (DPM)\n");
	if (params->features->fixed_timing)
		printf("      Fixed timing\n");
	if (params->features->fixed_pixel_format)
		printf("      Fixed pixel format\n");
	if (params->features->ai)
		printf("      Support ACP, ISRC1, or ISRC2packets\n");
	if (params->features->deinterlacing)
		printf("      De-interlacing\n");

	if (params->gamma != 0)
		printf("    Gamma: %.2f\n", params->gamma);
	printf("    Aspect ratio: %.2f\n", params->aspect_ratio);
	printf("    Dynamic bpc native: %d\n", params->bits_per_color_native);
	printf("    Dynamic bpc overall: %d\n", params->bits_per_color_overall);
}

static void
get_displayid_timing_aspect_ratio(enum di_displayid_timing_aspect_ratio ratio,
				  int *horiz, int *vert)
{
	switch (ratio) {
	case DI_DISPLAYID_TIMING_ASPECT_RATIO_1_1:
		*horiz = *vert = 1;
		return;
	case DI_DISPLAYID_TIMING_ASPECT_RATIO_5_4:
		*horiz = 5;
		*vert = 4;
		return;
	case DI_DISPLAYID_TIMING_ASPECT_RATIO_4_3:
		*horiz = 4;
		*vert = 3;
		return;
	case DI_DISPLAYID_TIMING_ASPECT_RATIO_15_9:
		*horiz = 15;
		*vert = 9;
		return;
	case DI_DISPLAYID_TIMING_ASPECT_RATIO_16_9:
		*horiz = 16;
		*vert = 9;
		return;
	case DI_DISPLAYID_TIMING_ASPECT_RATIO_16_10:
		*horiz = 16;
		*vert = 10;
		return;
	case DI_DISPLAYID_TIMING_ASPECT_RATIO_64_27:
		*horiz = 64;
		*vert = 27;
		return;
	case DI_DISPLAYID_TIMING_ASPECT_RATIO_256_135:
		*horiz = 256;
		*vert = 135;
		return;
	case DI_DISPLAYID_TIMING_ASPECT_RATIO_UNDEFINED:
		*horiz = *vert = 0;
		return;
	}
	abort(); /* Unreachable */
}

static const char *
displayid_type_i_ii_vii_timing_stereo_3d_name(enum di_displayid_type_i_ii_vii_timing_stereo_3d stereo_3d)
{
	switch (stereo_3d) {
	case DI_DISPLAYID_TYPE_I_II_VII_TIMING_STEREO_3D_NEVER:
		return "no 3D stereo";
	case DI_DISPLAYID_TYPE_I_II_VII_TIMING_STEREO_3D_ALWAYS:
		return "3D stereo";
	case DI_DISPLAYID_TYPE_I_II_VII_TIMING_STEREO_3D_USER:
		return "3D stereo depends on user action";
	}
	abort(); /* Unreachable */
}

static const char *
displayid_type_i_ii_vii_timing_sync_polarity_name(enum di_displayid_type_i_ii_vii_timing_sync_polarity pol)
{
	switch (pol) {
	case DI_DISPLAYID_TYPE_I_II_VII_TIMING_SYNC_NEGATIVE:
		return "N";
	case DI_DISPLAYID_TYPE_I_II_VII_TIMING_SYNC_POSITIVE:
		return "P";
	}
	abort(); /* Unreachable */
}

void
print_displayid_type_i_ii_vii_timing(const struct di_displayid_type_i_ii_vii_timing *t,
				     int indent, const char *prefix)
{
	int horiz_total, vert_total;
	int horiz_back_porch, vert_back_porch;
	int horiz_ratio, vert_ratio;
	double pixel_clock_hz, refresh, horiz_freq_hz;

	get_displayid_timing_aspect_ratio(t->aspect_ratio,
					  &horiz_ratio, &vert_ratio);

	horiz_total = t->horiz_active + t->horiz_blank;
	vert_total = t->vert_active + t->vert_blank;
	pixel_clock_hz = t->pixel_clock_mhz * 1000 * 1000;
	refresh = pixel_clock_hz / (horiz_total * vert_total);
	horiz_freq_hz = pixel_clock_hz / horiz_total;

	printf("%*s%s:", indent, "", prefix);
	printf(" %5dx%-5d", t->horiz_active, t->vert_active);
	if (t->interlaced) {
		printf("i");
	}
	printf(" %10.6f Hz", refresh);
	printf(" %3d:%-3d", horiz_ratio, vert_ratio);
	printf(" %8.3f kHz %13.6f MHz", horiz_freq_hz / 1000,
	       t->pixel_clock_mhz);
	printf(" (aspect ");
	if (t->aspect_ratio == DI_DISPLAYID_TIMING_ASPECT_RATIO_UNDEFINED)
		printf("undefined");
	else
		printf("%d:%d", horiz_ratio, vert_ratio);
	printf(", %s", displayid_type_i_ii_vii_timing_stereo_3d_name(t->stereo_3d));
	if (t->preferred)
		printf(", preferred");
	printf(")\n");

	horiz_back_porch = t->horiz_blank - t->horiz_sync_width - t->horiz_offset;
	printf("%*sHfront %4d Hsync %3d Hback %4d Hpol %s", indent + 8 + (int)strlen(prefix), "",
	       t->horiz_offset, t->horiz_sync_width, horiz_back_porch,
	       displayid_type_i_ii_vii_timing_sync_polarity_name(t->horiz_sync_polarity));
	printf("\n");

	vert_back_porch = t->vert_blank - t->vert_sync_width - t->vert_offset;
	printf("%*sVfront %4d Vsync %3d Vback %4d Vpol %s", indent + 8 + (int)strlen(prefix), "",
	       t->vert_offset, t->vert_sync_width, vert_back_porch,
	       displayid_type_i_ii_vii_timing_sync_polarity_name(t->vert_sync_polarity));
	printf("\n");
}

static void
print_displayid_type_i_timing_block(const struct di_displayid_data_block *data_block)
{
	size_t i;
	const struct di_displayid_type_i_ii_vii_timing *const *timings;

	timings = di_displayid_data_block_get_type_i_timings(data_block);
	for (i = 0; timings[i] != NULL; i++)
		print_displayid_type_i_ii_vii_timing(timings[i], 4, "DTD");
}

static void
print_displayid_type_ii_timing_block(const struct di_displayid_data_block *data_block)
{
	size_t i;
	const struct di_displayid_type_i_ii_vii_timing *const *timings;

	timings = di_displayid_data_block_get_type_ii_timings(data_block);
	for (i = 0; timings[i] != NULL; i++)
		print_displayid_type_i_ii_vii_timing(timings[i], 4, "DTD");
}

static const char *
displayid_tiled_topo_missing_recv_behavior_name(enum di_displayid_tiled_topo_missing_recv_behavior behavior)
{
	switch (behavior) {
	case DI_DISPLAYID_TILED_TOPO_MISSING_RECV_UNDEF:
		return "Undefined";
	case DI_DISPLAYID_TILED_TOPO_MISSING_RECV_TILE_ONLY:
		return "Image is displayed at the Tile Location";
	}
	abort(); /* unreachable */
}

static const char *
displayid_tiled_topo_single_recv_behavior_name(enum di_displayid_tiled_topo_single_recv_behavior behavior)
{
	switch (behavior) {
	case DI_DISPLAYID_TILED_TOPO_SINGLE_RECV_UNDEF:
		return "Undefined";
	case DI_DISPLAYID_TILED_TOPO_SINGLE_RECV_TILE_ONLY:
		return "Image is displayed at the Tile Location";
	case DI_DISPLAYID_TILED_TOPO_SINGLE_RECV_SCALED:
		return "Image is scaled to fit the entire tiled display";
	case DI_DISPLAYID_TILED_TOPO_SINGLE_RECV_CLONED:
		return "Image is cloned to all other tiles";
	}
	abort(); /* unreachable */
}

static void
print_displayid_tiled_topo(const struct di_displayid_tiled_topo *tiled_topo)
{
	printf("    Capabilities:\n");
	printf("      Behavior if it is the only tile: %s\n",
	       displayid_tiled_topo_single_recv_behavior_name(tiled_topo->caps->single_recv_behavior));
	printf("      Behavior if more than one tile and fewer than total number of tiles: %s\n",
	       displayid_tiled_topo_missing_recv_behavior_name(tiled_topo->caps->missing_recv_behavior));

	if (tiled_topo->caps->single_enclosure)
		printf("    Tiled display consists of a single physical display enclosure\n");
	else
		printf("    Tiled display consists of multiple physical display enclosures\n");


	printf("    Num horizontal tiles: %d Num vertical tiles: %d\n",
	       tiled_topo->total_horiz_tiles, tiled_topo->total_vert_tiles);
	printf("    Tile location: %d, %d\n",
	       tiled_topo->horiz_tile_location - 1,
	       tiled_topo->vert_tile_location - 1);
	printf("    Tile resolution: %dx%d\n",
	       tiled_topo->horiz_tile_pixels, tiled_topo->vert_tile_lines);

	if (tiled_topo->bezel != NULL) {
		printf("    Top bevel size: %.1f pixels\n",
		       tiled_topo->bezel->top_px);
		printf("    Bottom bevel size: %.1f pixels\n",
		       tiled_topo->bezel->bottom_px);
		printf("    Right bevel size: %.1f pixels\n",
		       tiled_topo->bezel->right_px);
		printf("    Left bevel size: %.1f pixels\n",
		       tiled_topo->bezel->left_px);
	}

	printf("    Tiled Display Manufacturer/Vendor ID: %.3s\n",
	       tiled_topo->vendor_id);
	printf("    Tiled Display Product ID Code: %" PRIu16 "\n",
	       tiled_topo->product_code);
	printf("    Tiled Display Serial Number: %" PRIu32 "\n",
	       tiled_topo->serial_number);
}

static void
print_displayid_type_iii_timing(const struct di_displayid_type_iii_timing *t)
{
	struct di_cvt_options cvt_options = {0};
	struct di_cvt_timing cvt_timing = {0};
	int hratio, vratio;
	double hbl, htotal;

	switch (t->algo) {
	case DI_DISPLAYID_TYPE_III_TIMING_CVT_STANDARD_BLANKING:
		cvt_options.red_blank_ver = DI_CVT_REDUCED_BLANKING_NONE;
		break;
	case DI_DISPLAYID_TYPE_III_TIMING_CVT_REDUCED_BLANKING:
		cvt_options.red_blank_ver = DI_CVT_REDUCED_BLANKING_V1;
		break;
	}

	cvt_options.h_pixels = t->horiz_active;

	get_displayid_timing_aspect_ratio(t->aspect_ratio, &hratio, &vratio);
	if (t->aspect_ratio == DI_DISPLAYID_TIMING_ASPECT_RATIO_UNDEFINED)
		return;

	cvt_options.v_lines = (cvt_options.h_pixels * vratio) / hratio;
	cvt_options.ip_freq_rqd = t->refresh_rate_hz;
	cvt_options.int_rqd = t->interlaced;

	di_cvt_compute(&cvt_timing, &cvt_options);

	hbl = cvt_timing.h_front_porch + cvt_timing.h_sync + cvt_timing.h_back_porch;
	htotal = cvt_timing.total_active_pixels + hbl;

	printf("    CVT: %5dx%-5d", (int)cvt_options.h_pixels, (int)cvt_options.v_lines);
	printf(" %10.6f Hz", cvt_timing.act_frame_rate);
	printf(" %3u:%-3u", hratio, vratio);
	printf(" %8.3f kHz %13.6f MHz", cvt_timing.act_pixel_freq * 1000 / htotal,
	       (double) cvt_timing.act_pixel_freq);
	printf(" (aspect %d:%d%s)", hratio, vratio,
	       t->preferred ? ", preferred" : "");
	printf("\n");
}

static void
print_displayid_type_iii_timing_block(const struct di_displayid_data_block *data_block)
{
	size_t i;
	const struct di_displayid_type_iii_timing *const *timings;

	timings = di_displayid_data_block_get_type_iii_timings(data_block);
	for (i = 0; timings[i] != NULL; i++)
		print_displayid_type_iii_timing(timings[i]);
}

static const char *
displayid_product_type_name(enum di_displayid_product_type type)
{
	switch (type) {
	case DI_DISPLAYID_PRODUCT_TYPE_EXTENSION:
		return "Extension Section";
	case DI_DISPLAYID_PRODUCT_TYPE_TEST:
		return "Test Structure; test equipment only";
	case DI_DISPLAYID_PRODUCT_TYPE_DISPLAY_PANEL:
		return "Display panel or other transducer, LCD or PDP module, etc.";
	case DI_DISPLAYID_PRODUCT_TYPE_STANDALONE_DISPLAY:
		return "Standalone display device";
	case DI_DISPLAYID_PRODUCT_TYPE_TV_RECEIVER:
		return "Television receiver";
	case DI_DISPLAYID_PRODUCT_TYPE_REPEATER:
		return "Repeater/translator";
	case DI_DISPLAYID_PRODUCT_TYPE_DIRECT_DRIVE:
		return "DIRECT DRIVE monitor";
	}
	abort();
}

static const char *
displayid_data_block_tag_name(enum di_displayid_data_block_tag tag)
{
	switch (tag) {
	case DI_DISPLAYID_DATA_BLOCK_PRODUCT_ID:
		return "Product Identification Data Block (0x00)";
	case DI_DISPLAYID_DATA_BLOCK_DISPLAY_PARAMS:
		return "Display Parameters Data Block (0x01)";
	case DI_DISPLAYID_DATA_BLOCK_COLOR_CHARACT:
		return "Color Characteristics Data Block";
	case DI_DISPLAYID_DATA_BLOCK_TYPE_I_TIMING:
		return "Video Timing Modes Type 1 - Detailed Timings Data Block";
	case DI_DISPLAYID_DATA_BLOCK_TYPE_II_TIMING:
		return "Video Timing Modes Type 2 - Detailed Timings Data Block";
	case DI_DISPLAYID_DATA_BLOCK_TYPE_III_TIMING:
		return "Video Timing Modes Type 3 - Short Timings Data Block";
	case DI_DISPLAYID_DATA_BLOCK_TYPE_IV_TIMING:
		return "Video Timing Modes Type 4 - DMT Timings Data Block";
	case DI_DISPLAYID_DATA_BLOCK_VESA_TIMING:
		return "Supported Timing Modes Type 1 - VESA DMT Timings Data Block";
	case DI_DISPLAYID_DATA_BLOCK_CEA_TIMING:
		return "Supported Timing Modes Type 2 - CTA-861 Timings Data Block";
	case DI_DISPLAYID_DATA_BLOCK_TIMING_RANGE_LIMITS:
		return "Video Timing Range Data Block";
	case DI_DISPLAYID_DATA_BLOCK_PRODUCT_SERIAL:
		return "Product Serial Number Data Block";
	case DI_DISPLAYID_DATA_BLOCK_ASCII_STRING:
		return "GP ASCII String Data Block";
	case DI_DISPLAYID_DATA_BLOCK_DISPLAY_DEVICE_DATA:
		return "Display Device Data Data Block";
	case DI_DISPLAYID_DATA_BLOCK_INTERFACE_POWER_SEQ:
		return "Interface Power Sequencing Data Block";
	case DI_DISPLAYID_DATA_BLOCK_TRANSFER_CHARACT:
		return "Transfer Characteristics Data Block";
	case DI_DISPLAYID_DATA_BLOCK_DISPLAY_INTERFACE:
		return "Display Interface Data Block";
	case DI_DISPLAYID_DATA_BLOCK_STEREO_DISPLAY_INTERFACE:
		return "Stereo Display Interface Data Block (0x10)";
	case DI_DISPLAYID_DATA_BLOCK_TYPE_V_TIMING:
		return "Video Timing Modes Type 5 - Short Timings Data Block";
	case DI_DISPLAYID_DATA_BLOCK_TILED_DISPLAY_TOPO:
		return "Tiled Display Topology Data Block (0x12)";
	case DI_DISPLAYID_DATA_BLOCK_TYPE_VI_TIMING:
		return "Video Timing Modes Type 6 - Detailed Timings Data Block";
	}
	abort();
}

void
print_displayid(const struct di_displayid *displayid)
{
	const struct di_displayid_data_block *const *data_blocks;
	const struct di_displayid_data_block *data_block;
	enum di_displayid_data_block_tag tag;
	size_t i;
	const struct di_displayid_display_params *display_params;
	const struct di_displayid_tiled_topo *tiled_topo;

	printf("  Version: %d.%d\n", di_displayid_get_version(displayid),
	       di_displayid_get_revision(displayid));

	if (is_displayid_base_block)
		printf("  Display Product Type: %s\n",
		       displayid_product_type_name(di_displayid_get_product_type(displayid)));
	is_displayid_base_block = false;

	data_blocks = di_displayid_get_data_blocks(displayid);
	for (i = 0; data_blocks[i] != NULL; i++) {
		data_block = data_blocks[i];
		tag = di_displayid_data_block_get_tag(data_block);
		printf("  %s:\n", displayid_data_block_tag_name(tag));
		switch (tag) {
		case DI_DISPLAYID_DATA_BLOCK_DISPLAY_PARAMS:
			display_params = di_displayid_data_block_get_display_params(data_block);
			print_displayid_display_params(display_params);
			break;
		case DI_DISPLAYID_DATA_BLOCK_TYPE_I_TIMING:
			print_displayid_type_i_timing_block(data_block);
			break;
		case DI_DISPLAYID_DATA_BLOCK_TILED_DISPLAY_TOPO:
			tiled_topo = di_displayid_data_block_get_tiled_topo(data_block);
			print_displayid_tiled_topo(tiled_topo);
			break;
		case DI_DISPLAYID_DATA_BLOCK_TYPE_II_TIMING:
			print_displayid_type_ii_timing_block(data_block);
			break;
		case DI_DISPLAYID_DATA_BLOCK_TYPE_III_TIMING:
			print_displayid_type_iii_timing_block(data_block);
			break;
		default:
			break; /* Ignore */
		}
	}
}
