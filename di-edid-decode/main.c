#include <assert.h>
#include <inttypes.h>
#include <math.h>
#include <stdio.h>
#include <stdlib.h>
#include <getopt.h>

#include <libdisplay-info/displayid.h>
#include <libdisplay-info/edid.h>
#include <libdisplay-info/info.h>

#include "di-edid-decode.h"

struct uncommon_features uncommon_features = {0};

static const struct option long_options[] = {
	{ "help", no_argument, 0, 'h' },
	{ 0, 0, 0, 0 }
};

static void usage(void)
{
	fprintf(stderr, "Usage:		di-edid-decode  <options> [in]\n"
			" [in]:		EDID file to parse. Read from standard input (stdin),\n"
			"		if none given.\n"
			"Example :	di-edid-decode /sys/class/drm/card0-DP-2/edid \n"
			"\nOptions:\n"
			"-h, --help	display the help message\n");
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
	abort();
}

static void
print_ext(const struct di_edid_ext *ext, size_t ext_index)
{
	const char *tag_name;
	const struct di_displayid *displayid;
	const struct di_displayid2 *displayid2;

	tag_name = ext_tag_name(di_edid_ext_get_tag(ext));
	printf("\n----------------\n\n");
	printf("Block %zu, %s:\n", ext_index + 1, tag_name);

	switch (di_edid_ext_get_tag(ext)) {
	case DI_EDID_EXT_CEA:
		print_cta(di_edid_ext_get_cta(ext));
		break;
	case DI_EDID_EXT_DISPLAYID:
		displayid = di_edid_ext_get_displayid(ext);
		displayid2 = di_edid_ext_get_displayid2(ext);
		if (displayid2)
			print_displayid2(displayid2);
		else if (displayid)
			print_displayid(displayid);
		break;
	default:
		break; /* Ignore */
	}
}

static size_t
edid_checksum_index(size_t block_index)
{
	return 128 * (block_index + 1) - 1;
}

int
main(int argc, char *argv[])
{
	FILE *in;
	static uint8_t raw[32 * 1024];
	size_t size = 0;
	const struct di_edid *edid;
	struct di_info *info;
	const struct di_edid_ext *const *exts;
	const char *failure_msg;
	size_t i;
	int opt;

	in = stdin;
	while (1) {
		int option_index = 0;
		opt = getopt_long(argc, argv, "h", long_options, &option_index);
		if (opt == -1)
			break;

		switch (opt) {
		case 'h':
			usage();
			return -1;
		default:
			usage();
			return -1;
		}
	}

	if (argc > 1) {
		in = fopen(argv[1], "r");
		if (!in) {
			perror("failed to open input file");
			return 1;
		}
	}

	while (!feof(in)) {
		size += fread(&raw[size], 1, sizeof(raw) - size, in);
		if (ferror(in)) {
			perror("fread failed");
			return 1;
		} else if (size >= sizeof(raw)) {
			fprintf(stderr, "input too large\n");
			return 1;
		}
	}

	fclose(in);

	info = di_info_parse_edid(raw, size);
	if (!info) {
		perror("di_edid_parse failed");
		return 1;
	}

	edid = di_info_get_edid(info);
	print_edid(edid);

	exts = di_edid_get_extensions(edid);

	for (i = 0; exts[i] != NULL; i++);
	if (i > 0) {
		printf("  Extension blocks: %zu\n", i);
	}

	printf("Checksum: 0x%02hhx\n", raw[edid_checksum_index(0)]);

	for (i = 0; exts[i] != NULL; i++) {
		print_ext(exts[i], i);
		printf("Checksum: 0x%02hhx\n", raw[edid_checksum_index(i + 1)]);
	}

	printf("\n----------------\n\n");

	failure_msg = di_info_get_failure_msg(info);
	if (failure_msg) {
		printf("Failures:\n\n%s", failure_msg);
		printf("EDID conformity: FAIL\n");
	} else {
		printf("EDID conformity: PASS\n");
	}

	if (uncommon_features.color_point_descriptor) {
		fprintf(stderr, "The EDID blob contains an uncommon Color "
				"Point Descriptor. Please share the EDID blob "
				"with upstream!\n");
	}
	if (uncommon_features.color_management_data) {
		fprintf(stderr, "The EDID blob contains an uncommon Color "
				"Management Data Descriptor. Please share the "
				"EDID blob with upstream!\n");
	}
	if (uncommon_features.cta_transfer_characteristics) {
		fprintf(stderr, "The EDID blob contains an uncommon CTA VESA "
				"Display Transfer Characteristic data block. "
				"Please share the EDID blob with upstream!\n");
	}

	di_info_destroy(info);
	return failure_msg ? 254 : 0;
}
