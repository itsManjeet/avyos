#ifndef DISPLAYID_H
#define DISPLAYID_H

/**
 * Private header for the low-level DisplayID API.
 */

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

#include <libdisplay-info/displayid.h>

#include "log.h"

/**
 * The maximum number of data blocks in a DisplayID section.
 *
 * A DisplayID section has a payload size of 251 bytes, and each data block has
 * a minimum size of 3 bytes.
 */
#define DISPLAYID_MAX_DATA_BLOCKS 83
/**
 * The maximum number of type I timings in a data block.
 *
 * A DisplayID data block has a maximum payload size of 248 bytes, and each type
 * I timing takes up 20 bytes.
 */
#define DISPLAYID_MAX_TYPE_I_TIMINGS 12
/**
 * The maximum number of type II timings in a data block.
 *
 * A DisplayID data block has a maximum payload size of 248 bytes, and each type
 * I timing takes up 11 bytes.
 */
#define DISPLAYID_MAX_TYPE_II_TIMINGS 22
/**
 * The maxiumum number of type III timings in a data block.
 *
 * A DisplayID data block has a maximum payload size of 248 bytes, and each type
 * III timing takes up 3 bytes.
 */
#define DISPLAYID_MAX_TYPE_III_TIMINGS 82

struct di_displayid {
	int version, revision;
	enum di_displayid_product_type product_type;

	struct di_displayid_data_block *data_blocks[DISPLAYID_MAX_DATA_BLOCKS + 1];
	size_t data_blocks_len;

	struct di_logger *logger;
};

struct di_displayid_display_params_priv {
	struct di_displayid_display_params base;
	struct di_displayid_display_params_features features;
};

struct di_displayid_tiled_topo_priv {
	struct di_displayid_tiled_topo base;
	struct di_displayid_tiled_topo_caps caps;
	struct di_displayid_tiled_topo_bezel bezel;
};

struct di_displayid_data_block {
	enum di_displayid_data_block_tag tag;

	/* Used for TYPE_I_TIMING, NULL-terminated */
	struct di_displayid_type_i_ii_vii_timing *type_i_timings[DISPLAYID_MAX_TYPE_I_TIMINGS + 1];
	size_t type_i_timings_len;

	/* Used for TYPE_II_TIMING, NULL-terminated */
	struct di_displayid_type_i_ii_vii_timing *type_ii_timings[DISPLAYID_MAX_TYPE_II_TIMINGS + 1];
	size_t type_ii_timings_len;

	/* Used for TYPE_III_TIMING, NULL-terminated */
	struct di_displayid_type_iii_timing *type_iii_timings[DISPLAYID_MAX_TYPE_III_TIMINGS + 1];
	size_t type_iii_timings_len;

	/* Used for DISPLAY_PARAMS */
	struct di_displayid_display_params_priv display_params;

	/* Used for TILED_DISPLAY_TOPO */
	struct di_displayid_tiled_topo_priv tiled_topo;
};

bool
_di_displayid_parse(struct di_displayid *displayid, const uint8_t *data,
		    size_t size, struct di_logger *logger);

void
_di_displayid_finish(struct di_displayid *displayid);

bool
_di_displayid_parse_type_1_7_timing(struct di_displayid_type_i_ii_vii_timing *timing,
				    struct di_logger *logger,
				    const char *prefix,
				    const uint8_t *data,
				    bool is_type7);

int
_di_displayid_parse_version(const uint8_t *data, size_t size);

#endif
