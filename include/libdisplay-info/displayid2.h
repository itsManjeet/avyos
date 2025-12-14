#ifndef DI_DISPLAYID2_H
#define DI_DISPLAYID2_H

/**
 * libdisplay-info's low-level API for VESA Display Identification Data
 * (DisplayID) version 2.
 *
 * The library implements DisplayID version 2.1, available at:
 * https://vesa.org/vesa-standards/
 */

#include <stdbool.h>
#include <stdint.h>

/**
 * DisplayID v2 data structure.
 */
struct di_displayid2;

/**
 * Get the DisplayID v2 revision.
 */
int
di_displayid2_get_revision(const struct di_displayid2 *displayid2);

/**
 * Get DisplayID v2 data blocks.
 *
 * The returned array is NULL-terminated.
 */
const struct di_displayid2_data_block *const *
di_displayid2_get_data_blocks(const struct di_displayid2 *displayid2);

/**
 * Product primary use case identifier, defined in table 2-3.
 */
enum di_displayid2_product_primary_use_case {
	/* Extension section */
	DI_DISPLAYID2_PRODUCT_PRIMARY_USE_CASE_EXTENSION = 0x00,
	/* Test structure */
	DI_DISPLAYID2_PRODUCT_PRIMARY_USE_CASE_TEST = 0x01,
	/* Generic display */
	DI_DISPLAYID2_PRODUCT_PRIMARY_USE_CASE_GENERIC = 0x02,
	/* Television display */
	DI_DISPLAYID2_PRODUCT_PRIMARY_USE_CASE_TV = 0x03,
	/* Desktop productivity display */
	DI_DISPLAYID2_PRODUCT_PRIMARY_USE_CASE_DESKTOP_PRODUCTIVITY = 0x04,
	/* Desktop gaming display */
	DI_DISPLAYID2_PRODUCT_PRIMARY_USE_CASE_DESKTOP_GAMING = 0x05,
	/* Presentation display */
	DI_DISPLAYID2_PRODUCT_PRIMARY_USE_CASE_PRESENTATION = 0x06,
	/* Head-mounted Virtual Reality (VR) display */
	DI_DISPLAYID2_PRODUCT_PRIMARY_USE_CASE_HMD_VR = 0x07,
	/* Head-mounted Augmented Reality (AR) display */
	DI_DISPLAYID2_PRODUCT_PRIMARY_USE_CASE_HMD_AR = 0x08,
};

/**
 * DisplayID v2 data block tag.
 */
enum di_displayid2_data_block_tag {
	/* Product Identification Data Block */
	DI_DISPLAYID2_DATA_BLOCK_PRODUCT_ID = 0x20,
	/* Display Parameters Data Block */
	DI_DISPLAYID2_DATA_BLOCK_DISPLAY_PARAMS = 0x21,
	/* Type VII Timing – Detailed Timing Data Block */
	DI_DISPLAYID2_DATA_BLOCK_TYPE_VII_TIMING = 0x22,
	/* Type VIII Timing – Enumerated Timing Code Data Block */
	DI_DISPLAYID2_DATA_BLOCK_TYPE_VIII_TIMING = 0x23,
	/* Type IX Timing – Formula-based Timing Data Block */
	DI_DISPLAYID2_DATA_BLOCK_TYPE_IX_TIMING = 0x24,
	/* Dynamic Video Timing Range Limits Data Block */
	DI_DISPLAYID2_DATA_BLOCK_DYN_TIMING_RANGE_LIMITS = 0x25,
	/* Display Interface Features Data Block */
	DI_DISPLAYID2_DATA_BLOCK_DISPLAY_INTERFACE_FEATURES = 0x26,
	/* Stereo Display Interface Data Block */
	DI_DISPLAYID2_DATA_BLOCK_STEREO_DISPLAY_INTERFACE = 0x27,
	/* Tiled Display Topology Data Block */
	DI_DISPLAYID2_DATA_BLOCK_TILED_DISPLAY_TOPO = 0x28,
	/* ContainerID Data Block */
	DI_DISPLAYID2_DATA_BLOCK_CONTAINERID = 0x29,
	/* Type X Timing – Formula-based Timing Data Block */
	DI_DISPLAYID2_DATA_BLOCK_TYPE_X_TIMING = 0x2A,
	/* Adaptive-Sync Data Block */
	DI_DISPLAYID2_DATA_BLOCK_ADAPTIVE_SYNC = 0x2B,
	/* ARVR_HMD Data Block */
	DI_DISPLAYID2_DATA_BLOCK_ARVR_HMD = 0x2C,
	/* ARVR_Layer Data Block */
	DI_DISPLAYID2_DATA_BLOCK_ARVR_LAYER = 0x2D,
	/* CTA-861 Data Block Encapsulation DisplayID Data Block */
	DI_DISPLAYID2_DATA_BLOCK_CTA861 = 0x81,
};

/**
 * A DisplayID v2 data block.
 */
struct di_displayid2_data_block;

/**
 * Get a DisplayID v2 data block tag.
 */
enum di_displayid2_data_block_tag
di_displayid2_data_block_get_tag(const struct di_displayid2_data_block *data_block);

/**
 * Get the DisplayID v2 product primary use case.
 */
enum di_displayid2_product_primary_use_case
di_displayid2_get_product_primary_use_case(const struct di_displayid2 *displayid2);

#endif
