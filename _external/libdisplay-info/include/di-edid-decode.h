#ifndef DI_EDID_DECODE_H
#define DI_EDID_DECODE_H

#include <stdbool.h>

#include <libdisplay-info/edid.h>
#include <libdisplay-info/cta.h>
#include <libdisplay-info/displayid.h>

struct uncommon_features {
	bool color_point_descriptor;
	bool color_management_data;
	bool cta_transfer_characteristics;
};

extern struct uncommon_features uncommon_features;

struct di_edid;
struct di_edid_detailed_timing_def;
struct di_edid_cta;
struct di_displayid;

void
print_edid(const struct di_edid *edid);

void
print_detailed_timing_def(const struct di_edid_detailed_timing_def *def);

void
print_cta(const struct di_edid_cta *cta);

void
print_displayid(const struct di_displayid *displayid);

void
print_displayid2(const struct di_displayid2 *displayid2);

void
print_displayid_type_i_ii_vii_timing(const struct di_displayid_type_i_ii_vii_timing *t,
				  int indent, const char *prefix);

void
compute_aspect_ratio(int width, int height, int *horiz_ratio, int *vert_ratio);

#endif
