#ifndef CTA_H
#define CTA_H

/**
 * Private header for the low-level CTA API.
 */

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

#include <libdisplay-info/cta.h>
#include <displayid.h>

/**
 * The maximum number of data blocks in an EDID CTA block.
 *
 * Each data block takes at least 1 byte, the CTA block can hold 128 bytes, and
 * the mandatory fields take up 5 bytes (4 header bytes + checksum).
 */
#define EDID_CTA_MAX_DATA_BLOCKS 123
/**
 * The maximum number of detailed timing definitions included in an EDID CTA
 * block.
 *
 * The CTA extension leaves at most 122 bytes for timings, and each timing takes
 * 18 bytes.
 */
#define EDID_CTA_MAX_DETAILED_TIMING_DEFS 6
/**
 * The maximum number of SVD entries in a video data block.
 *
 * Each data block has its size described in a 5-bit field, so its maximum size
 * is 63 bytes, and each SVD uses 1 byte.
 */
#define EDID_CTA_MAX_VIDEO_BLOCK_ENTRIES 63
/**
 * The maximum number of SAD entries in an audio data block.
 *
 * Each data block has its size described in a 5-bit field, so its maximum size
 * is 63 bytes, and each SAD uses 3 bytes.
 */
#define EDID_CTA_MAX_AUDIO_BLOCK_ENTRIES 21
/**
 * The maximum number of Capability Bit Map entries in a YCbCr 4:2:0 video data
 * block.
 *
 * Each data block has its size described in a 5-bit field, so its maximum size
 * is 63 bytes, and each Capability Bit Map uses 1 byte.
 */
#define EDID_CTA_MAX_YCBCR420_CAP_MAP_BLOCK_ENTRIES 63
/**
 * The maximum number of Short InfoFrame Descriptor or Short Vendor-Specific
 * InfoFrame Descriptor entries in a InfoFrame data block.
 *
 * Each data block has its size described in a 5-bit field, so its maximum size
 * is 63 bytes, the header takes up at least 2 bytes and the smallest Short
 * InfoFrame Descriptor is 1 byte.
 */
#define EDID_CTA_INFOFRAME_BLOCK_ENTRIES 61
/**
 * The maximum number of Speaker Location Descriptors in a Speaker Location data
 * block.
 *
 * Each data block has its size described in a 5-bit field, so its maximum size
 * is 63 bytes, and each Speaker Location Descriptors uses at least 2 byte.
 */
#define EDID_CTA_MAX_SPEAKER_LOCATION_BLOCK_ENTRIES 31
/**
 * The maximum number of SVR entries in a video format preference block.
 *
 * Each data block has its size described in a 5-bit field, so its maximum size
 * is 63 bytes, and each SVR uses 1 byte.
 */
#define EDID_CTA_MAX_VIDEO_FORMAT_PREF_BLOCK_ENTRIES 63
/**
 * The maximum number of format entries in a HDMI audio block.
 *
 * Each data block has its size described in a 5-bit field, so its maximum size
 * is 63 bytes, the header takes up 2 bytes and each format entry uses 4 bytes.
 */
#define EDID_CTA_MAX_HDMI_AUDIO_BLOCK_ENTRIES 15

struct di_edid_cta {
	int revision;
	struct di_edid_cta_flags flags;

	/* NULL-terminated */
	struct di_cta_data_block *data_blocks[EDID_CTA_MAX_DATA_BLOCKS + 1];
	size_t data_blocks_len;

	/* NULL-terminated */
	struct di_edid_detailed_timing_def_priv *detailed_timing_defs[EDID_CTA_MAX_DETAILED_TIMING_DEFS + 1];
	size_t detailed_timing_defs_len;

	struct di_logger *logger;
};

struct di_cta_hdr_static_metadata_block_priv {
	struct di_cta_hdr_static_metadata_block base;
	struct di_cta_hdr_static_metadata_eotfs eotfs;
	struct di_cta_hdr_static_metadata_descriptors descriptors;
};

struct di_cta_hdr_dynamic_metadata_type3 {
	uint8_t unused;
};

struct di_cta_hdr_dynamic_metadata_block_priv {
	struct di_cta_hdr_dynamic_metadata_block base;
	struct di_cta_hdr_dynamic_metadata_type1 type1;
	struct di_cta_hdr_dynamic_metadata_type2 type2;
	struct di_cta_hdr_dynamic_metadata_type3 type3;
	struct di_cta_hdr_dynamic_metadata_type4 type4;
	struct di_cta_hdr_dynamic_metadata_type256 type256;
};

struct di_cta_video_block_priv {
	struct di_cta_video_block base;
	/* NULL-terminated */
	struct di_cta_svd *svds[EDID_CTA_MAX_VIDEO_BLOCK_ENTRIES + 1];
	size_t svds_len;
};

struct di_cta_ycbcr420_video_block_priv {
	struct di_cta_ycbcr420_video_block base;
	/* NULL-terminated */
	struct di_cta_svd *svds[EDID_CTA_MAX_VIDEO_BLOCK_ENTRIES + 1];
	size_t svds_len;
};

struct di_cta_sad_priv {
	struct di_cta_sad base;
	struct di_cta_sad_sample_rates supported_sample_rates;
	struct di_cta_sad_lpcm lpcm;
	struct di_cta_sad_mpegh_3d mpegh_3d;
	struct di_cta_sad_mpeg_aac mpeg_aac;
	struct di_cta_sad_mpeg_surround mpeg_surround;
	struct di_cta_sad_mpeg_aac_le mpeg_aac_le;
	struct di_cta_sad_enhanced_ac3 enhanced_ac3;
	struct di_cta_sad_mat mat;
	struct di_cta_sad_wma_pro wma_pro;
};

struct di_cta_audio_block_priv {
	struct di_cta_audio_block audio;
	/* NULL-terminated */
	struct di_cta_sad_priv *sads[EDID_CTA_MAX_AUDIO_BLOCK_ENTRIES + 1];
	size_t sads_len;
};

struct di_cta_ycbcr420_cap_map_block {
	bool all;
	uint8_t svd_bitmap[EDID_CTA_MAX_YCBCR420_CAP_MAP_BLOCK_ENTRIES];
};


struct di_cta_hdmi_audio_block_priv {
	struct di_cta_hdmi_audio_block base;
	struct di_cta_hdmi_audio_multi_stream ms;
	struct di_cta_hdmi_audio_3d a3d;
	/* NULL-terminated */
	struct di_cta_sad_priv *sads[EDID_CTA_MAX_HDMI_AUDIO_BLOCK_ENTRIES + 1];
	size_t sads_len;
};

struct di_cta_infoframe_block_priv {
	struct di_cta_infoframe_block base;
	/* NULL-terminated */
	struct di_cta_infoframe_descriptor *infoframes[EDID_CTA_INFOFRAME_BLOCK_ENTRIES + 1];
	size_t infoframes_len;
};

struct di_cta_speaker_location_priv {
	struct di_cta_speaker_location_block base;
	/* NULL-terminated */
	struct di_cta_speaker_location_descriptor *locations[EDID_CTA_MAX_SPEAKER_LOCATION_BLOCK_ENTRIES + 1];
	size_t locations_len;
};

struct di_cta_video_format_pref_priv {
	struct di_cta_video_format_pref_block base;
	/* NULL-terminated */
	struct di_cta_svr *svrs[EDID_CTA_MAX_VIDEO_FORMAT_PREF_BLOCK_ENTRIES + 1];
	size_t svrs_len;
};

struct di_cta_type_vii_timing_priv {
	struct di_cta_type_vii_timing_block base;
	struct di_displayid_type_i_ii_vii_timing timing;
};

struct di_cta_vendor_hdmi_block_priv {
	struct di_cta_vendor_hdmi_block base;
	/* HDMI VIC's. */
	uint8_t *vics;
};

struct di_cta_dolby_video_block_priv {
	struct di_cta_dolby_video_block base;
	struct di_cta_dolby_video_block_v0 v0;
	struct di_cta_dolby_video_block_v1 v1;
	struct di_cta_dolby_video_block_v2 v2;
};

struct di_cta_vendor_hdmi_forum_block_priv {
	struct di_cta_vendor_hdmi_forum_block base;
	struct di_cta_hdmi_dsc dsc;
};

struct di_cta_hdmi_forum_sink_cap_priv {
	struct di_cta_hdmi_forum_sink_cap base;
	struct di_cta_hdmi_dsc dsc;
};

struct di_cta_data_block {
	enum di_cta_data_block_tag tag;

	/* Used for DI_CTA_DATA_BLOCK_VIDEO */
	struct di_cta_video_block_priv video;
	/* Used for DI_CTA_DATA_BLOCK_YCBCR420 */
	struct di_cta_ycbcr420_video_block_priv ycbcr420;
	/* used for DI_CTA_DATA_BLOCK_AUDIO */
	struct di_cta_audio_block_priv audio;
	/* Used for DI_CTA_DATA_BLOCK_SPEAKER_ALLOC */
	struct di_cta_speaker_alloc_block speaker_alloc;
	/* Used for DI_CTA_DATA_BLOCK_VIDEO_CAP */
	struct di_cta_video_cap_block video_cap;
	/* Used for DI_CTA_DATA_BLOCK_VESA_DISPLAY_DEVICE */
	struct di_cta_vesa_display_device_block vesa_display_device;
	/* Used for DI_CTA_DATA_BLOCK_COLORIMETRY */
	struct di_cta_colorimetry_block colorimetry;
	/* Used for DI_CTA_DATA_BLOCK_HDR_STATIC_METADATA */
	struct di_cta_hdr_static_metadata_block_priv hdr_static_metadata;
	/* Used for DI_CTA_DATA_BLOCK_HDR_DYNAMIC_METADATA */
	struct di_cta_hdr_dynamic_metadata_block_priv hdr_dynamic_metadata;
	/* Used for DI_CTA_DATA_BLOCK_VESA_DISPLAY_TRANSFER_CHARACTERISTIC */
	struct di_cta_vesa_transfer_characteristics_block vesa_transfer_characteristics;
	/* Used for DI_CTA_DATA_BLOCK_YCBCR420_CAP_MAP */
	struct di_cta_ycbcr420_cap_map_block ycbcr420_cap_map;
	/* Used for DI_CTA_DATA_BLOCK_HDMI_AUDIO */
	struct di_cta_hdmi_audio_block_priv hdmi_audio;
	/* Used for DI_CTA_DATA_BLOCK_INFOFRAME */
	struct di_cta_infoframe_block_priv infoframe;
	/* Used for DI_CTA_DATA_BLOCK_ROOM_CONFIG */
	struct di_cta_room_configuration_block room_config;
	/* Used for DI_CTA_DATA_BLOCK_SPEAKER_LOCATION */
	struct di_cta_speaker_location_priv speaker_location;
	/* Used for DI_CTA_DATA_BLOCK_VIDEO_FORMAT_PREF */
	struct di_cta_video_format_pref_priv video_format_pref;
	/* Used for DI_CTA_DATA_BLOCK_DISPLAYID_VIDEO_TIMING_VII */
	struct di_cta_type_vii_timing_priv did_vii_timing;
	/* Used for DI_CTA_DATA_BLOCK_VENDOR_HDMI */
	struct di_cta_vendor_hdmi_block_priv vendor_hdmi;
	/* Used for DI_CTA_DATA_BLOCK_HDR10PLUS */
	struct di_cta_hdr10plus_block hdr10plus;
	/* Used for DI_CTA_DATA_BLOCK_DOLBY_VIDEO */
	struct di_cta_dolby_video_block_priv dolby_video;
	/* Used for DI_CTA_DATA_BLOCK_HDMI_SINK_CAP */
	struct di_cta_hdmi_forum_sink_cap_priv hdmi_sink_cap;
	/* Used for DI_CTA_DATA_BLOCK_VENDOR_HDMI_FORUM */
	struct di_cta_vendor_hdmi_forum_block_priv vendor_hdmi_forum;
};

extern const struct di_cta_video_format _di_cta_video_formats[];
extern const size_t _di_cta_video_formats_len;

extern const struct di_cta_hdmi_video_format _di_cta_hdmi_video_formats[];
extern const size_t _di_cta_hdmi_video_formats_len;

bool
_di_edid_cta_parse(struct di_edid_cta *cta, const uint8_t *data, size_t size,
		   struct di_logger *logger);

void
_di_edid_cta_finish(struct di_edid_cta *cta);

#endif
