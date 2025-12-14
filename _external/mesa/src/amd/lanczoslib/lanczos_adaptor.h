/* Copyright 2025 Advanced Micro Devices, Inc.
 * SPDX-License-Identifier: MIT
 *
 * Authors: AMD
 *
 */

#pragma once

#ifdef __cplusplus
extern "C" {
#endif

#include <stdbool.h>
#include <stdint.h>

void generate_lanczos_coeff(float scaling_ratio, uint32_t hw_tap, uint32_t hw_phases, uint16_t *coeff);

#ifdef __cplusplus
}
#endif