#include <stdio.h>
#include <stdlib.h>

#include <libdisplay-info/displayid2.h>

#include "di-edid-decode.h"

static const char *
displayid2_product_primary_use_case_name(enum di_displayid2_product_primary_use_case use_case)
{
	switch (use_case) {
	case DI_DISPLAYID2_PRODUCT_PRIMARY_USE_CASE_EXTENSION:
		return "Same primary use case as the base section";
	case DI_DISPLAYID2_PRODUCT_PRIMARY_USE_CASE_TEST:
		return "Test Structure; test equipment only";
	case DI_DISPLAYID2_PRODUCT_PRIMARY_USE_CASE_GENERIC:
		return "None of the listed primary use cases; generic display";
	case DI_DISPLAYID2_PRODUCT_PRIMARY_USE_CASE_TV:
		return "Television (TV) display";
	case DI_DISPLAYID2_PRODUCT_PRIMARY_USE_CASE_DESKTOP_PRODUCTIVITY:
		return "Desktop productivity display";
	case DI_DISPLAYID2_PRODUCT_PRIMARY_USE_CASE_DESKTOP_GAMING:
		return "Desktop gaming display";
	case DI_DISPLAYID2_PRODUCT_PRIMARY_USE_CASE_PRESENTATION:
		return "Presentation display";
	case DI_DISPLAYID2_PRODUCT_PRIMARY_USE_CASE_HMD_VR:
		return "Head-mounted Virtual Reality (VR) display";
	case DI_DISPLAYID2_PRODUCT_PRIMARY_USE_CASE_HMD_AR:
		return "Head-mounted Augmented Reality (AR) display";
	}
	abort(); /* unreachable */
}

static const char *
displayid2_data_block_tag_name(enum di_displayid2_data_block_tag tag)
{
	switch (tag) {
	case DI_DISPLAYID2_DATA_BLOCK_PRODUCT_ID:
		return "Product Identification Data Block (0x20)";
	case DI_DISPLAYID2_DATA_BLOCK_DISPLAY_PARAMS:
		return "Display Parameters Data Block (0x21)";
	case DI_DISPLAYID2_DATA_BLOCK_TYPE_VII_TIMING:
		return "Video Timing Modes Type 7 - Detailed Timings Data Block";
	case DI_DISPLAYID2_DATA_BLOCK_TYPE_VIII_TIMING:
		return "Video Timing Modes Type 8 - Enumerated Timing Codes Data Block";
	case DI_DISPLAYID2_DATA_BLOCK_TYPE_IX_TIMING:
		return "Video Timing Modes Type 9 - Formula-based Timings Data Block";
	case DI_DISPLAYID2_DATA_BLOCK_DYN_TIMING_RANGE_LIMITS:
		return "Dynamic Video Timing Range Limits Data Block";
	case DI_DISPLAYID2_DATA_BLOCK_DISPLAY_INTERFACE_FEATURES:
		return "Display Interface Features Data Block";
	case DI_DISPLAYID2_DATA_BLOCK_STEREO_DISPLAY_INTERFACE:
		return "Stereo Display Interface Data Block (0x27)";
	case DI_DISPLAYID2_DATA_BLOCK_TILED_DISPLAY_TOPO:
		return "Tiled Display Topology Data Block (0x28)";
	case DI_DISPLAYID2_DATA_BLOCK_CONTAINERID:
		return "ContainerID Data Block";
	case DI_DISPLAYID2_DATA_BLOCK_TYPE_X_TIMING:
		return "Video Timing Modes Type 10 - Formula-based Timings Data Block";
	case DI_DISPLAYID2_DATA_BLOCK_ADAPTIVE_SYNC:
		return "Adaptive Sync Data Block";
	case DI_DISPLAYID2_DATA_BLOCK_ARVR_HMD:
		return "ARVR_HMD Data Block";
	case DI_DISPLAYID2_DATA_BLOCK_ARVR_LAYER:
		return "ARVR_Layer Data Block";
	case DI_DISPLAYID2_DATA_BLOCK_CTA861:
		return "CTA-861 DisplayID Data Block";
	}
	abort(); /* unreachable */
}

void
print_displayid2(const struct di_displayid2 *displayid2)
{
	enum di_displayid2_product_primary_use_case use_case;
	const struct di_displayid2_data_block *const *data_blocks;
	const struct di_displayid2_data_block *data_block;
	enum di_displayid2_data_block_tag tag;
	size_t i;

	printf("  Version: 2.%d\n", di_displayid2_get_revision(displayid2));

	use_case = di_displayid2_get_product_primary_use_case(displayid2);
	printf("  Display Product Primary Use Case: %s\n",
	       displayid2_product_primary_use_case_name(use_case));

	data_blocks = di_displayid2_get_data_blocks(displayid2);
	for (i = 0; data_blocks[i] != NULL; i++) {
		data_block = data_blocks[i];
		tag = di_displayid2_data_block_get_tag(data_block);
		printf("  %s:\n", displayid2_data_block_tag_name(tag));
	}
}
