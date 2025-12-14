#include <errno.h>
#include <inttypes.h>
#include <stdlib.h>
#include <sys/types.h>

#include "bits.h"
#include "displayid.h"
#include "displayid2.h"

/**
 * The size of a DisplayID v2 section header.
 */
#define DISPLAYID2_HEADER_SIZE 4
/**
 * The size of the mandatory fields in a DisplayID v2 section (header + checksum).
 */
#define DISPLAYID2_MIN_SIZE (DISPLAYID2_HEADER_SIZE + 1)
/**
 * The maximum size of a DisplayID v2 section.
 */
#define DISPLAYID2_MAX_SIZE 256
/**
 * The size of a DisplayID v2 data block header (tag, revision and size).
 */
#define DISPLAYID2_DATA_BLOCK_HEADER_SIZE 3

static void
add_failure(struct di_displayid2 *displayid2, const char fmt[], ...)
{
	va_list args;

	va_start(args, fmt);
	_di_logger_va_add_failure(displayid2->logger, fmt, args);
	va_end(args);
}

static ssize_t
parse_data_block(struct di_displayid2 *displayid2, const uint8_t *data,
		 size_t size)
{
	uint8_t tag;
	size_t data_block_size;
	struct di_displayid2_data_block *data_block = NULL;

	assert(size >= DISPLAYID2_DATA_BLOCK_HEADER_SIZE);

	tag = data[0x00];
	data_block_size = (size_t) data[0x02] + DISPLAYID2_DATA_BLOCK_HEADER_SIZE;
	if (data_block_size > size) {
		add_failure(displayid2,
			    "The length of this DisplayID data block (%d) exceeds the number of bytes remaining (%zu)",
			    data_block_size, size);
		goto skip;
	}

	data_block = calloc(1, sizeof(*data_block));
	if (!data_block)
		goto error;

	switch (tag) {
	case DI_DISPLAYID2_DATA_BLOCK_PRODUCT_ID:
	case DI_DISPLAYID2_DATA_BLOCK_DISPLAY_PARAMS:
	case DI_DISPLAYID2_DATA_BLOCK_TYPE_VII_TIMING:
	case DI_DISPLAYID2_DATA_BLOCK_TYPE_VIII_TIMING:
	case DI_DISPLAYID2_DATA_BLOCK_TYPE_IX_TIMING:
	case DI_DISPLAYID2_DATA_BLOCK_DYN_TIMING_RANGE_LIMITS:
	case DI_DISPLAYID2_DATA_BLOCK_DISPLAY_INTERFACE_FEATURES:
	case DI_DISPLAYID2_DATA_BLOCK_STEREO_DISPLAY_INTERFACE:
	case DI_DISPLAYID2_DATA_BLOCK_TILED_DISPLAY_TOPO:
	case DI_DISPLAYID2_DATA_BLOCK_CONTAINERID:
	case DI_DISPLAYID2_DATA_BLOCK_TYPE_X_TIMING:
	case DI_DISPLAYID2_DATA_BLOCK_ADAPTIVE_SYNC:
	case DI_DISPLAYID2_DATA_BLOCK_ARVR_HMD:
	case DI_DISPLAYID2_DATA_BLOCK_ARVR_LAYER:
	case DI_DISPLAYID2_DATA_BLOCK_CTA861:
		break; /* Supported */
	case 0x7E:
		goto skip; /* Vendor-specific */
	default:
		add_failure(displayid2,
			    "Unknown DisplayID v2 Data Block (0x%" PRIx8 ", length %" PRIu8 ")",
			    tag, data_block_size - DISPLAYID2_DATA_BLOCK_HEADER_SIZE);
		goto skip;
	}

	data_block->tag = tag;

	assert(displayid2->data_blocks_len < DISPLAYID2_MAX_DATA_BLOCKS);
	displayid2->data_blocks[displayid2->data_blocks_len++] = data_block;
	return (ssize_t) data_block_size;

skip:
	free(data_block);
	return (ssize_t) data_block_size;

error:
	free(data_block);
	return -1;
}

static bool
is_all_zeroes(const uint8_t *data, size_t size)
{
	size_t i;

	for (i = 0; i < size; i++) {
		if (data[i] != 0)
			return false;
	}

	return true;
}

static bool
is_data_block_end(const uint8_t *data, size_t size)
{
	if (size < DISPLAYID2_DATA_BLOCK_HEADER_SIZE)
		return true;
	return is_all_zeroes(data, DISPLAYID2_DATA_BLOCK_HEADER_SIZE);
}

static bool
validate_checksum(const uint8_t *data, size_t size)
{
	uint8_t sum = 0;
	size_t i;

	for (i = 0; i < size; i++) {
		sum += data[i];
	}

	return sum == 0;
}

bool
_di_displayid2_parse(struct di_displayid2 *displayid2, const uint8_t *data,
		     size_t size, struct di_logger *logger)
{
	size_t section_size, i, max_data_block_size;
	ssize_t data_block_size;
	int version;
	uint8_t product_primary_use_case;

	if (size < DISPLAYID2_MIN_SIZE) {
		errno = EINVAL;
		return false;
	}

	displayid2->logger = logger;

	version = _di_displayid_parse_version(data, size);
	displayid2->revision = get_bit_range(data[0x00], 3, 0);
	if (version != 2) {
		errno = ENOTSUP;
		return false;
	}

	section_size = (size_t) data[0x01] + DISPLAYID2_MIN_SIZE;
	if (section_size > DISPLAYID2_MAX_SIZE || section_size > size) {
		errno = EINVAL;
		return false;
	}

	if (!validate_checksum(data, section_size)) {
		errno = EINVAL;
		return false;
	}

	product_primary_use_case = data[0x02];
	switch (product_primary_use_case) {
	case DI_DISPLAYID2_PRODUCT_PRIMARY_USE_CASE_EXTENSION:
	case DI_DISPLAYID2_PRODUCT_PRIMARY_USE_CASE_TEST:
	case DI_DISPLAYID2_PRODUCT_PRIMARY_USE_CASE_GENERIC:
	case DI_DISPLAYID2_PRODUCT_PRIMARY_USE_CASE_TV:
	case DI_DISPLAYID2_PRODUCT_PRIMARY_USE_CASE_DESKTOP_PRODUCTIVITY:
	case DI_DISPLAYID2_PRODUCT_PRIMARY_USE_CASE_DESKTOP_GAMING:
	case DI_DISPLAYID2_PRODUCT_PRIMARY_USE_CASE_PRESENTATION:
	case DI_DISPLAYID2_PRODUCT_PRIMARY_USE_CASE_HMD_VR:
	case DI_DISPLAYID2_PRODUCT_PRIMARY_USE_CASE_HMD_AR:
		displayid2->product_primary_use_case = product_primary_use_case;
		break;
	default:
		errno = EINVAL;
		return false;
	}

	i = DISPLAYID2_HEADER_SIZE;
	max_data_block_size = 0;
	while (i < section_size - 1) {
		max_data_block_size = section_size - 1 - i;
		if (is_data_block_end(&data[i], max_data_block_size))
			break;
		data_block_size = parse_data_block(displayid2, &data[i],
						   max_data_block_size);
		if (data_block_size < 0)
			return false;
		assert(data_block_size > 0);
		i += (size_t) data_block_size;
	}
	if (!is_all_zeroes(&data[i], max_data_block_size)) {
		if (max_data_block_size < DISPLAYID2_DATA_BLOCK_HEADER_SIZE) {
			add_failure(displayid2,
				    "Not enough bytes remain (%zu) for a DisplayID data block and the DisplayID filler is non-0.",
				    max_data_block_size);
		} else {
			add_failure(displayid2, "Padding: Contains non-zero bytes.");
		}
	}

	displayid2->logger = NULL;
	return true;
}

void
_di_displayid2_finish(struct di_displayid2 *displayid2)
{
	size_t i;

	for (i = 0; i < displayid2->data_blocks_len; i++)
		free(displayid2->data_blocks[i]);
}

int
di_displayid2_get_revision(const struct di_displayid2 *displayid2)
{
	return displayid2->revision;
}

enum di_displayid2_product_primary_use_case
di_displayid2_get_product_primary_use_case(const struct di_displayid2 *displayid2)
{
	return displayid2->product_primary_use_case;
}

const struct di_displayid2_data_block *const *
di_displayid2_get_data_blocks(const struct di_displayid2 *displayid2)
{
	return (const struct di_displayid2_data_block *const *) displayid2->data_blocks;
}

enum di_displayid2_data_block_tag
di_displayid2_data_block_get_tag(const struct di_displayid2_data_block *data_block)
{
	return data_block->tag;
}
