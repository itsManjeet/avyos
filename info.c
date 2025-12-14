#include <inttypes.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <assert.h>

#include "edid.h"
#include "info.h"
#include "memory-stream.h"

/* Generated file pnp-id-table.c: */
const char *
pnp_id_table(const char *key);

static bool
di_cta_data_block_allowed_multiple(enum di_cta_data_block_tag tag)
{
	/* See CTA-861-H, 7.6 Multiple Instances of Data Blocks. */
	switch (tag) {
	case DI_CTA_DATA_BLOCK_SPEAKER_ALLOC:
	case DI_CTA_DATA_BLOCK_VESA_DISPLAY_TRANSFER_CHARACTERISTIC:
	case DI_CTA_DATA_BLOCK_VIDEO_CAP:
	case DI_CTA_DATA_BLOCK_VESA_DISPLAY_DEVICE:
	case DI_CTA_DATA_BLOCK_COLORIMETRY:
	case DI_CTA_DATA_BLOCK_HDR_STATIC_METADATA:
	case DI_CTA_DATA_BLOCK_VIDEO_FORMAT_PREF:
	case DI_CTA_DATA_BLOCK_YCBCR420_CAP_MAP:
	case DI_CTA_DATA_BLOCK_HDMI_AUDIO:
	case DI_CTA_DATA_BLOCK_ROOM_CONFIG:
	case DI_CTA_DATA_BLOCK_HDMI_EDID_EXT_OVERRIDE:
	case DI_CTA_DATA_BLOCK_HDMI_SINK_CAP:
		return false;
	default:
		return true;
	}
}

static const struct di_cta_data_block *
di_edid_get_cta_data_block(const struct di_edid *edid,
			   enum di_cta_data_block_tag tag)
{
	const struct di_edid_ext *const *ext;

	/*
	 * Here we do not handle blocks that are allowed to occur in
	 * multiple instances.
	 */
	assert(!di_cta_data_block_allowed_multiple(tag));

	for (ext = di_edid_get_extensions(edid); *ext; ext++) {
		const struct di_edid_cta *cta;
		const struct di_cta_data_block *const *block;

		if (di_edid_ext_get_tag(*ext) != DI_EDID_EXT_CEA)
			continue;

		cta = di_edid_ext_get_cta(*ext);
		for (block = di_edid_cta_get_data_blocks(cta); *block; block++) {
			if (di_cta_data_block_get_tag(*block) == tag)
				return *block;
		}
	}

	return NULL;
}

static void
derive_edid_hdr_static_metadata(const struct di_edid *edid,
				struct di_hdr_static_metadata *hsm)
{
	const struct di_cta_data_block *block;
	const struct di_cta_hdr_static_metadata_block *cta_hsm;

	/* By default, everything unset and only traditional gamma supported. */
	hsm->traditional_sdr = true;

	block = di_edid_get_cta_data_block(edid, DI_CTA_DATA_BLOCK_HDR_STATIC_METADATA);
	if (!block)
		return;

	cta_hsm = di_cta_data_block_get_hdr_static_metadata(block);
	assert(cta_hsm);

	hsm->desired_content_max_luminance = cta_hsm->desired_content_max_luminance;
	hsm->desired_content_max_frame_avg_luminance = cta_hsm->desired_content_max_frame_avg_luminance;
	hsm->desired_content_min_luminance = cta_hsm->desired_content_min_luminance;
	hsm->type1 = cta_hsm->descriptors->type1;
	hsm->traditional_sdr = cta_hsm->eotfs->traditional_sdr;
	hsm->traditional_hdr = cta_hsm->eotfs->traditional_hdr;
	hsm->pq = cta_hsm->eotfs->pq;
	hsm->hlg = cta_hsm->eotfs->hlg;
}

static void
derive_edid_color_primaries(const struct di_edid *edid,
			    struct di_color_primaries *cc)
{
	const struct di_edid_chromaticity_coords *cm;
	const struct di_edid_misc_features *misc;

	/* Trust the flag more than the fields. */
	misc = di_edid_get_misc_features(edid);
	if (misc->srgb_is_primary) {
		/*
		 * https://www.w3.org/Graphics/Color/sRGB.html
		 * for lack of access to IEC 61966-2-1
		 */
		cc->primary[0].x = 0.640f; /* red */
		cc->primary[0].y = 0.330f;
		cc->primary[1].x = 0.300f; /* green */
		cc->primary[1].y = 0.600f;
		cc->primary[2].x = 0.150f; /* blue */
		cc->primary[2].y = 0.060f;
		cc->has_primaries = true;
		cc->default_white.x = 0.3127f; /* D65 */
		cc->default_white.y = 0.3290f;
		cc->has_default_white_point = true;

		return;
	}

	cm = di_edid_get_chromaticity_coords(edid);

	/*
	 * Broken EDID might have only partial values.
	 * Require all values to report anything.
	 */
	if (cm->red_x > 0.0f &&
	    cm->red_y > 0.0f &&
	    cm->green_x > 0.0f &&
	    cm->green_y > 0.0f &&
	    cm->blue_x > 0.0f &&
	    cm->blue_y > 0.0f) {
		cc->primary[0].x = cm->red_x;
		cc->primary[0].y = cm->red_y;
		cc->primary[1].x = cm->green_x;
		cc->primary[1].y = cm->green_y;
		cc->primary[2].x = cm->blue_x;
		cc->primary[2].y = cm->blue_y;
		cc->has_primaries = true;
	}
	if (cm->white_x > 0.0f && cm->white_y > 0.0f) {
		cc->default_white.x = cm->white_x;
		cc->default_white.y = cm->white_y;
		cc->has_default_white_point = true;
	}
}

static void
derive_edid_supported_signal_colorimetry(const struct di_edid *edid,
					 struct di_supported_signal_colorimetry *ssc)
{
	const struct di_cta_data_block *block;
	const struct di_cta_colorimetry_block *cm;

	/* Defaults to all unsupported. */

	block = di_edid_get_cta_data_block(edid, DI_CTA_DATA_BLOCK_COLORIMETRY);
	if (!block)
		return;

	cm = di_cta_data_block_get_colorimetry(block);
	assert(cm);

	ssc->bt2020_cycc = cm->bt2020_cycc;
	ssc->bt2020_ycc = cm->bt2020_ycc;
	ssc->bt2020_rgb = cm->bt2020_rgb;
	ssc->st2113_rgb = cm->st2113_rgb;
	ssc->ictcp = cm->ictcp;
}

struct di_info *
di_info_parse_edid(const void *data, size_t size)
{
	struct memory_stream failure_msg;
	struct di_edid *edid;
	struct di_info *info;
	char *failure_msg_str = NULL;

	if (!memory_stream_open(&failure_msg))
		return NULL;

	edid = _di_edid_parse(data, size, failure_msg.fp);
	if (!edid)
		goto err_failure_msg_file;

	info = calloc(1, sizeof(*info));
	if (!info)
		goto err_edid;

	info->edid = edid;

	failure_msg_str = memory_stream_close(&failure_msg);
	if (failure_msg_str && failure_msg_str[0] != '\0')
		info->failure_msg = failure_msg_str;
	else
		free(failure_msg_str);

	derive_edid_hdr_static_metadata(info->edid, &info->derived.hdr_static_metadata);
	derive_edid_color_primaries(info->edid, &info->derived.color_primaries);
	derive_edid_supported_signal_colorimetry(info->edid, &info->derived.supported_signal_colorimetry);

	return info;

err_edid:
	_di_edid_destroy(edid);
err_failure_msg_file:
	memory_stream_cleanup(&failure_msg);
	return NULL;
}

void
di_info_destroy(struct di_info *info)
{
	_di_edid_destroy(info->edid);
	free(info->failure_msg);
	free(info);
}

const struct di_edid *
di_info_get_edid(const struct di_info *info)
{
	return info->edid;
}

const char *
di_info_get_failure_msg(const struct di_info *info)
{
	return info->failure_msg;
}

static void
encode_ascii_byte(FILE *out, char ch)
{
	uint8_t c = (uint8_t)ch;

	/*
	 * Replace ASCII control codes and non-7-bit codes
	 * with an escape string. The result is guaranteed to be valid
	 * UTF-8.
	 */
	if (c < 0x20 || c >= 0x7f)
		fprintf(out, "\\x%02x", c);
	else
		fputc(c, out);
}

static void
encode_ascii_string(FILE *out, const char *str)
{
	size_t len = strlen(str);
	size_t i;

	for (i = 0; i < len; i++)
		encode_ascii_byte(out, str[i]);
}

char *
di_info_get_make(const struct di_info *info)
{
	const struct di_edid_vendor_product *evp;
	char pnp_id[(sizeof(evp->manufacturer)) + 1] = { 0, };
	const char *manuf;
	struct memory_stream m;

	if (!info->edid)
		return NULL;

	if (!memory_stream_open(&m))
		return NULL;

	evp = di_edid_get_vendor_product(info->edid);
	memcpy(pnp_id, evp->manufacturer, sizeof(evp->manufacturer));

	manuf = pnp_id_table(pnp_id);
	if (manuf) {
		encode_ascii_string(m.fp, manuf);
		return memory_stream_close(&m);
	}

	fputs("PNP(", m.fp);
	encode_ascii_string(m.fp, pnp_id);
	fputs(")", m.fp);

	return memory_stream_close(&m);
}

char *
di_info_get_model(const struct di_info *info)
{
	const struct di_edid_vendor_product *evp;
	const struct di_edid_display_descriptor *const *desc;
	struct memory_stream m;
	size_t i;
	enum di_edid_display_descriptor_tag tag;
	const char *str;

	if (!info->edid)
		return NULL;

	if (!memory_stream_open(&m))
		return NULL;

	desc = di_edid_get_display_descriptors(info->edid);
	for (i = 0; desc[i]; i++) {
		tag = di_edid_display_descriptor_get_tag(desc[i]);
		if (tag != DI_EDID_DISPLAY_DESCRIPTOR_PRODUCT_NAME)
			continue;
		str = di_edid_display_descriptor_get_string(desc[i]);
		if (str[0] == '\0')
			continue;
		encode_ascii_string(m.fp, str);
		return memory_stream_close(&m);
	}

	evp = di_edid_get_vendor_product(info->edid);
	fprintf(m.fp, "0x%04" PRIX16, evp->product);

	return memory_stream_close(&m);
}

char *
di_info_get_serial(const struct di_info *info)
{
	const struct di_edid_display_descriptor *const *desc;
	const struct di_edid_vendor_product *evp;
	struct memory_stream m;
	size_t i;
	enum di_edid_display_descriptor_tag tag;
	const char *str;

	if (!info->edid)
		return NULL;

	if (!memory_stream_open(&m))
		return NULL;

	desc = di_edid_get_display_descriptors(info->edid);
	for (i = 0; desc[i]; i++) {
		tag = di_edid_display_descriptor_get_tag(desc[i]);
		if (tag != DI_EDID_DISPLAY_DESCRIPTOR_PRODUCT_SERIAL)
			continue;
		str = di_edid_display_descriptor_get_string(desc[i]);
		if (str[0] == '\0')
			continue;
		encode_ascii_string(m.fp, str);
		return memory_stream_close(&m);
	}

	evp = di_edid_get_vendor_product(info->edid);
	if (evp->serial != 0) {
		fprintf(m.fp, "0x%08" PRIX32, evp->serial);
		return memory_stream_close(&m);
	}

	memory_stream_cleanup(&m);
	return NULL;
}

const struct di_hdr_static_metadata *
di_info_get_hdr_static_metadata(const struct di_info *info)
{
	return &info->derived.hdr_static_metadata;
}

const struct di_color_primaries *
di_info_get_default_color_primaries(const struct di_info *info)
{
	return &info->derived.color_primaries;
}

const struct di_supported_signal_colorimetry *
di_info_get_supported_signal_colorimetry(const struct di_info *info)
{
	return &info->derived.supported_signal_colorimetry;
}

static const struct di_displayid *
edid_get_displayid(const struct di_edid *edid)
{
	const struct di_edid_ext *const *ext;

	for (ext = di_edid_get_extensions(edid); *ext; ext++) {
		enum di_edid_ext_tag tag = di_edid_ext_get_tag(*ext);

		if (tag == DI_EDID_EXT_DISPLAYID)
			return di_edid_ext_get_displayid(*ext);
	}

	return NULL;
}

static const struct di_displayid_display_params *
displayid_get_display_params(const struct di_displayid *did)
{
	const struct di_displayid_data_block *const *block =
		di_displayid_get_data_blocks(did);

	for (; *block; block++) {
		enum di_displayid_data_block_tag tag = di_displayid_data_block_get_tag(*block);

		if (tag == DI_DISPLAYID_DATA_BLOCK_DISPLAY_PARAMS)
			return di_displayid_data_block_get_display_params(*block);
	}

	return NULL;
}

float
di_info_get_default_gamma(const struct di_info *info)
{
	const struct di_edid *edid;
	const struct di_displayid *did;
	const struct di_edid_misc_features *misc;

	edid = di_info_get_edid(info);
	if (!edid)
		return 0.0f;

	did = edid_get_displayid(edid);
	if (did) {
		const struct di_displayid_display_params *did_params;

		did_params = displayid_get_display_params(did);
		if (did_params)
			return did_params->gamma;
	}

	/* Trust the flag more than the gamma field value. */
	misc = di_edid_get_misc_features(edid);
	if (misc->srgb_is_primary)
		return 2.2f;

	return di_edid_get_basic_gamma(edid);
}
