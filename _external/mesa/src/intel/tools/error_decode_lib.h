/*
 * Copyright 2024-2025 Intel Corporation
 * SPDX-License-Identifier: MIT
 */

#pragma once

#include <stdint.h>
#include <stdbool.h>

const char *ascii85_decode_char(const char *in, uint32_t *v);

int zlib_inflate(uint32_t **ptr, int len);

int ascii85_decode(const char *in, uint32_t **out, bool inflate);
