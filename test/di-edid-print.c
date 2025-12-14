#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>
#include <stdbool.h>
#include <assert.h>

#include <libdisplay-info/info.h>

static const char *
str_or_null(const char *str)
{
	return str ? str : "{null}";
}

static const char *
yes_no(bool cond)
{
	return cond ? "yes" : "no";
}

static void
print_chromaticity(const char *prefix, const struct di_chromaticity_cie1931 *c)
{
	printf("%s: %.3f, %.3f\n", prefix, c->x, c->y);
}

static void
print_info(const struct di_info *info)
{
	const struct di_hdr_static_metadata *hdr_static;
	const struct di_color_primaries *primaries;
	const struct di_supported_signal_colorimetry *ssc;
	char *str;

	str = di_info_get_make(info);
	printf("make: %s\n", str_or_null(str));
	free(str);

	str = di_info_get_model(info);
	printf("model: %s\n", str_or_null(str));
	free(str);

	str = di_info_get_serial(info);
	printf("serial: %s\n", str_or_null(str));
	free(str);

	hdr_static = di_info_get_hdr_static_metadata(info);
	assert(hdr_static);
	printf("HDR static metadata:\n"
	       "luminance %f-%f, maxFALL %f\n"
	       "metadata type1=%s\n"
	       "EOTF tSDR=%s, tHDR=%s, PQ=%s, HLG=%s\n",
	       hdr_static->desired_content_min_luminance,
	       hdr_static->desired_content_max_luminance,
	       hdr_static->desired_content_max_frame_avg_luminance,
	       yes_no(hdr_static->type1),
	       yes_no(hdr_static->traditional_sdr),
	       yes_no(hdr_static->traditional_hdr),
	       yes_no(hdr_static->pq),
	       yes_no(hdr_static->hlg));

	primaries = di_info_get_default_color_primaries(info);
	assert(primaries);
	printf("default color primaries:\n");
	print_chromaticity("    red", &primaries->primary[0]);
	print_chromaticity("  green", &primaries->primary[1]);
	print_chromaticity("   blue", &primaries->primary[2]);
	print_chromaticity("default white", &primaries->default_white);

	printf("default gamma: %.2f\n", di_info_get_default_gamma(info));

	ssc = di_info_get_supported_signal_colorimetry(info);
	assert(ssc);
	printf("signal colorimetry:");
	if (ssc->bt2020_cycc)
		printf(" BT2020_cYCC");
	if (ssc->bt2020_ycc)
		printf(" BT2020_YCC");
	if (ssc->bt2020_rgb)
		printf(" BT2020_RGB");
	if (ssc->st2113_rgb)
		printf(" P3D65+P3DCI");
	if (ssc->ictcp)
		printf(" ICtCp");
	printf("\n");
}

int
main(int argc, char *argv[])
{
	FILE *in;
	static uint8_t raw[32 * 1024];
	size_t size = 0;
	struct di_info *info;

	in = stdin;
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

	print_info(info);
	di_info_destroy(info);

	return 0;
}
