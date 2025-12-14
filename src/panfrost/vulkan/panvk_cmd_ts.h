/*
 * Copyright (C) 2025 Arm Ltd.
 * SPDX-License-Identifier: MIT
 */

#ifndef PANVK_CMD_TS_H
#define PANVK_CMD_TS_H

#ifndef PAN_ARCH
#error "PAN_ARCH must be defined"
#endif

#include <stdint.h>

#if PAN_ARCH >= 10
/* The timstamp info subqueue performs extra tasks like writing the info field
 * and handling deferred timestamps. To minimize impact on drawing, choose the
 * compute subqueue. */
#define PANVK_QUERY_TS_INFO_SUBQUEUE (PANVK_SUBQUEUE_COMPUTE)

enum panvk_query_ts_op {
   PANVK_QUERY_TS_OP_MAX = 0,
   PANVK_QUERY_TS_OP_MIN = 1,
};

static uint64_t
panvk_timestamp_info_encode(enum panvk_query_ts_op op, uint64_t sq_mask)
{
   return (((uint64_t)sq_mask) << 32) | (op);
}

static enum panvk_query_ts_op
panvk_timestamp_info_get_op(uint64_t encoded)
{
   return ((uint32_t)encoded);
}

static uint32_t
panvk_timestamp_info_get_sq_mask(uint64_t encoded)
{
   return ((uint32_t)(encoded >> 32));
}
#endif
#endif
