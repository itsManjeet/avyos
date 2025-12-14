#ifndef INFO_H
#define INFO_H

/**
 * Private header for the high-level API.
 */

#include <libdisplay-info/info.h>

/**
 * All information here is derived from low-level information contained in
 * struct di_info. These are exposed by the high-level API only.
 */
struct di_derived_info {
	struct di_hdr_static_metadata hdr_static_metadata;
	struct di_color_primaries color_primaries;
	struct di_supported_signal_colorimetry supported_signal_colorimetry;
};

struct di_info {
	struct di_edid *edid;

	char *failure_msg;

	struct di_derived_info derived;
};

#endif
