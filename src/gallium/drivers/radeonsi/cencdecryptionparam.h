/**************************************************************************
 *
 * Copyright 2025 Advanced Micro Devices, Inc.
 *
 * SPDX-License-Identifier: MIT
 *
 **************************************************************************/
#ifndef SECURE_BUFFER_FORMAT_H
#define SECURE_BUFFER_FORMAT_H

#include <stdint.h>

#define AES_BLOCK_SIZE 16
#define KEY_SIZE_128 16
#define CMAC_SIZE AES_BLOCK_SIZE
#define MAX_SUBSAMPLES 288		/* Maximum subsamples in a sample */

typedef struct PACKED _secure_buffer_header
{
   uint8_t cookie[8];     /* 8-byte cookie with value 'wvcencsb' */
   uint8_t version;       /* Set to 1 */
   uint8_t reserved[55];  /* Reserved for future use */
} secure_buffer_header;

typedef struct PACKED _subsample_description
{
   uint32_t num_bytes_clear;
   uint32_t num_bytes_encrypted;
   uint8_t subsample_flags;       /* Is this the first/last subsample in a sample? */
   uint8_t block_offset;          /* Used only for CTR "cenc" mode */
} subsample_description;

typedef struct PACKED _cenc_encrypt_pattern_desc
{
   uint32_t encrypt;  /* Number of 16 byte blocks to decrypt */
   uint32_t skip;     /* Number of 16 byte blocks to leave in clear */
} cenc_encrypt_pattern_desc;

typedef struct PACKED _sample_description
{
   subsample_description subsamples[MAX_SUBSAMPLES];
   uint8_t iv[AES_BLOCK_SIZE];  /* The IV for the initial subsample */
   cenc_encrypt_pattern_desc pattern;
   uint32_t subsamples_length;  /* The number of subsamples in the sample */
} sample_description;

typedef struct PACKED _native_enforce_policy_info
{
   uint8_t enabled_policy_index[4];
   uint32_t policy_array[32];
} native_enforce_policy_info;

typedef struct PACKED _signed_native_enforce_policy
{
   uint8_t wrapped_key[KEY_SIZE_128];
   native_enforce_policy_info native_policy;
   uint8_t signature[CMAC_SIZE];
} signed_native_enforce_policy;

typedef struct PACKED hw_drm_key_blob_info
{
   uint8_t wrapped_key[KEY_SIZE_128];       /* Content key encrypted with session key */
   uint8_t wrapped_key_iv[AES_BLOCK_SIZE];  /* IV used to encrypt content key */
   union
   {
      struct
      {
         uint32_t drm_session_id : 4;       /* DRM Session ID */
         uint32_t use_hw_drm_aes_ctr : 1;   /* Invoke HW-DRM with AES-CTR for content decryption */
         uint32_t use_hw_drm_aes_cbc : 1;   /* Invoke HW-DRM with AES-CBC for content decryption */
         uint32_t reserved_bits : 26;       /* Reserved fields */
      } s;
      uint32_t value;
   } u;
   signed_native_enforce_policy local_policy;
   uint8_t reserved[128];
} hw_drm_key_blob_info;

typedef struct PACKED amd_secure_buffer_format
{
   secure_buffer_header sb_header;
   sample_description desc;
   hw_drm_key_blob_info key_blob;
} amd_secure_buffer_format;

#endif /* SECURE_BUFFER_FORMAT_H */
