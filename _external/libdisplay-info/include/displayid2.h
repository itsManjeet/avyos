#ifndef DISPLAYID2_H
#define DISPLAYID2_H

/**
 * Private header for the low-level DisplayID v2 API.
 */

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

#include <libdisplay-info/displayid2.h>

#include "log.h"

/**
 * The maximum number of data blocks in a DisplayID v2 section.
 *
 * A DisplayID v2 section has a maximum payload size of 251 bytes (256 bytes
 * maximum size, 5 bytes header), and each data block has a minimum size of
 * 3 bytes.
 */
#define DISPLAYID2_MAX_DATA_BLOCKS 83

struct di_displayid2 {
	int revision;
	enum di_displayid2_product_primary_use_case product_primary_use_case;

	struct di_displayid2_data_block *data_blocks[DISPLAYID2_MAX_DATA_BLOCKS + 1];
	size_t data_blocks_len;

	struct di_logger *logger;
};

struct di_displayid2_data_block {
	enum di_displayid2_data_block_tag tag;
};

bool
_di_displayid2_parse(struct di_displayid2 *displayid2, const uint8_t *data,
		     size_t size, struct di_logger *logger);

void
_di_displayid2_finish(struct di_displayid2 *displayid2);

#endif
