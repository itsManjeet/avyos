/* Copyright 2025 Advanced Micro Devices, Inc.
 * SPDX-License-Identifier: MIT
 *
 * Authors: AMD
 *
 */

#include "lanczos_adaptor.h"
#include "lanczosFilterGenerator.h"

#define MaxHwNumTabs     8
#define HwNumPhases     64
#define HwNumTabsChroma  2

void generate_lanczos_coeff(float scaling_ratio, uint32_t hw_tap, uint32_t hw_phases, uint16_t *coeff)
{
   float filterCoeffs[MaxHwNumTabs * HwNumPhases] = {0};

   LanczosFilterGenerator::GenerateLanczosCoeff(filterCoeffs, scaling_ratio, hw_tap, hw_phases);
   LanczosFilterGenerator::ConvertScalingCoeffsToUint(coeff, filterCoeffs, hw_tap, hw_phases);
}