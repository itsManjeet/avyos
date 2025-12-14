/* Copyright 2025 Advanced Micro Devices, Inc.
 * SPDX-License-Identifier: MIT
 *
 * Authors: AMD
 *
 */

#ifndef _LANCZOSFILTERGENERATOR_H_
#define _LANCZOSFILTERGENERATOR_H_
#pragma once

#include <stdint.h>

typedef uint16_t uint16;
typedef uint32_t uint32;
typedef uint64_t uint64;

typedef int16_t int16;
typedef int32_t int32;
typedef int64_t int64;


/**
***************************************************************************************************
* @brief This is the filter for generating Lanczos coefficinets
*
***************************************************************************************************
*/
class LanczosFilterGenerator {

public:

    enum class CoefType : uint32_t
    {
        ModifiedLanczos  = 0,    ///< Modified Lanczos kernel
        StandardLanczos  = 1,    ///< Standard Lanczos kernel
        TruncatedLanczos = 2,    ///< Standard Lanczos for (n+m) taps truncated to n taps
        TruncatedSinc    = 3,    ///< Truncated Sinc Kernel
        Count
    };
    static void GenerateLanczosCoeff(
        float* pFilter,
        float attenuation,
        float kernelInterval,
        uint32 taps,
        uint32 phases,
        CoefType coefMode);

    static void GenerateLanczosCoeff(
        float* pCoef,
        float  scalingRatio,
        uint32 tapCount,
        uint32 phaseCount,
        float kernelInterval = 1.0f,
        float attenuation = 1.0f,
        float sharpness = 0.0f );

    static void ConvertScalingCoeffsToUint(
        uint16* pUintFilter,
        const float* pFloatFilter,
        const uint32 num_taps,
        const uint32 num_phases);

    static void GenerateSincCoeff(
        float* pFilter,
        float attenuation, float kernelInterval,
        uint32 taps, uint32 phases);
        static float Ratio2Attenuation(float ratio, float sharpness);
        static float Ratio2CuttOff(float ratio);
        static void MaxLoc(const float* pFilter,
        uint32 NumTaps,
        uint16& MaxLoc);

protected:

    static const double Epsilon;
    static const uint32 AddedTap   = 2; // Number of taps to add for truncated coefficient generation
    static const uint32 UpdBScales = 1;
    static const int32  UpdBPoints   = 7;
    static const int32  DowndBScales = 8;
    static const int32  DowndBPoints = 11;
    static const int32  MinSharpness = -50;
    static const int32  MaxSharpness =  50;
    static const float UpdBFuzzy;
    static const float UpdBFlat;
    static const float UpdBSharp;
    static const float DowndBFuzzy;
    static const float DowndBFlat;
    static const float DowndBSharp;
    static const float ThresholdRatioLow;
    static const float ThresholdRatioUp;
    static const float LancDownScaledBTable[DowndBScales+1][DowndBPoints];
    static const float LancUpScaledBTable[UpdBScales+1][UpdBPoints];
    static const float PCoef0;
    static const float PCoef1;
    static const float PCoef2;
    static const float PCoef3;
    static float Sinc(float input);
    static float Lanczos(float input, float attenuation);
    static float Interpolate(float dbValue, float sharpmin, float sharpmax, float dbmin, float dbmax);

private:

    LanczosFilterGenerator();
    virtual ~LanczosFilterGenerator();

};

#endif
