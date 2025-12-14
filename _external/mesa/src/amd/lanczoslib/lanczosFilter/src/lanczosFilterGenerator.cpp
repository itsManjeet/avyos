/* Copyright 2025 Advanced Micro Devices, Inc.
 * SPDX-License-Identifier: MIT
 *
 * Authors: AMD
 *
 */
#include "lanczosFilterGenerator.h"
#define _USE_MATH_DEFINES
#include <math.h>

const double LanczosFilterGenerator::Epsilon = 0.00000000000000000005;

const float LanczosFilterGenerator::UpdBFuzzy = -6.0206f;
const float LanczosFilterGenerator::UpdBFlat  =  0.0000f;
const float LanczosFilterGenerator::UpdBSharp = +6.0206f;

const float LanczosFilterGenerator::DowndBFuzzy = -12.0412f;
const float LanczosFilterGenerator::DowndBFlat  = -6.02060f;
const float LanczosFilterGenerator::DowndBSharp = -1.00000f;

const float LanczosFilterGenerator::ThresholdRatioLow = 0.8f;
const float LanczosFilterGenerator::ThresholdRatioUp  = 1.0f;

const float LanczosFilterGenerator::PCoef0 = -0.73420f;
const float LanczosFilterGenerator::PCoef1 = 11.5964f;
const float LanczosFilterGenerator::PCoef2 = -20.3973f;
const float LanczosFilterGenerator::PCoef3 = 15.9062f;

const float LanczosFilterGenerator::LancDownScaledBTable[DowndBScales+1][DowndBPoints] =
{
    {6.021f,    4.000f,    2.000f,    0.000f,   -1.000f,   -2.000f,   -4.000f,   -6.021f,   -8.000f,   -10.000f,  -12.041f },
    {1.430900f, 1.430900f, 1.430900f, 1.000000f, 0.010000f, 0.010000f, 0.010000f, 0.010000f, 0.010000f, 0.010000f, 0.010000f},
    {1.430900f, 1.430900f, 1.430900f, 1.000000f, 0.631104f, 0.010000f, 0.010000f, 0.010000f, 0.010000f, 0.010000f, 0.010000f},
    {1.430900f, 1.430900f, 1.430900f, 1.000000f, 0.852667f, 0.683285f, 0.010000f, 0.010000f, 0.010000f, 0.010000f, 0.010000f},
    {1.430900f, 1.430900f, 1.211063f, 1.000000f, 0.911794f, 0.823094f, 0.632013f, 0.371977f, 0.010000f, 0.010000f, 0.010000f},
    {1.430900f, 1.430900f, 1.147498f, 1.000000f, 0.937014f, 0.877198f, 0.760127f, 0.644078f, 0.525000f, 0.388752f, 0.203904f},
    {1.430900f, 1.308486f, 1.117958f, 1.000000f, 0.949518f, 0.901692f, 0.813452f, 0.731170f, 0.656033f, 0.584572f, 0.515552f},
    {1.430900f, 1.257660f, 1.104867f, 1.000000f, 0.955050f, 0.913236f, 0.836873f, 0.767940f, 0.707312f, 0.652090f, 0.601553f},
    {1.430900f, 1.244853f, 1.100741f, 1.000000f, 0.956680f, 0.916528f, 0.843580f, 0.778528f, 0.721578f, 0.670147f, 0.624064f}
};

const float LanczosFilterGenerator::LancUpScaledBTable[UpdBScales+1][UpdBPoints] =
{
    {6.021f,    4.000f,    2.000f,    0.000f,   -2.000f,   -4.000f,   -6.021f   },
    {1.430292f, 1.430292f, 1.170925f, 1.000000f, 0.875461f, 0.769256f, 0.673826f}
};

/**
***************************************************************************************************
*   LanczosFilterGenerator::LanczosFilterGenerator
*
*   @brief
*     LanczosFilterGenerator constructor
*
***************************************************************************************************
*/
LanczosFilterGenerator::LanczosFilterGenerator()
{
}

/**
***************************************************************************************************
*   LanczosFilterGenerator::~LanczosFilterGenerator
*
*   @brief
*     LanczosFilterGenerator destructor
*
***************************************************************************************************
*/
LanczosFilterGenerator::~LanczosFilterGenerator()
{
}

/**
***************************************************************************************************
*   LanczosFilterGenerator::GenerateLanczosCoeff
*
*   @brief
*     Generate 4-tap, 128 phase filter coefficients for  lanczos kernel
*
***************************************************************************************************
*/
void LanczosFilterGenerator::GenerateLanczosCoeff(
    float*              pFilter,          ///< [out] Filter coefficients
    float               attenuation,      ///< [in]  Lanczos kernel parameter
    float               kernelInterval,   ///< [in]  Input interval for the fitler kernel
    uint32              taps,             ///< [in]  Number of filter taps
    uint32              phases,           ///< [in]  Number of filter phases
    CoefType            coefMode)         ///< [in]  Kernel type for coefficients
{
    uint32 totalNumberofCoef = phases * taps;

    float attenby2 = attenuation * taps * 0.5f;

    switch(coefMode)
    {
    case CoefType::StandardLanczos:
        attenby2 = 1/attenby2;
        break;

    case CoefType::TruncatedLanczos:
        {
        uint32 targetTaps = taps + AddedTap;
            attenby2 = 1/(attenuation*targetTaps*0.5f);
        }
        break;

    default:
        break;
    }

    uint32 currentPhase;
    uint32 currentTap;

    for(currentPhase = 0; currentPhase < phases; currentPhase++)
    {
        float sumPerPhase = 0.0f;
        for(currentTap = 1; currentTap <= taps; currentTap++)
        {
            uint32 mainFilterIndex = (currentTap * phases) - currentPhase;
            float mainFilterInput = static_cast<float>(M_PI) * (static_cast<float>(2*mainFilterIndex)/totalNumberofCoef - 1.0f);
            mainFilterInput *= kernelInterval;
            float tapValue = 0.0f;

            switch(coefMode)
            {
            case CoefType::ModifiedLanczos:
            case CoefType::StandardLanczos:
            case CoefType::TruncatedLanczos:

                tapValue = Lanczos(mainFilterInput, attenby2);
                break;

            case CoefType::TruncatedSinc:

                tapValue = (kernelInterval < taps/2.0f)?Sinc(mainFilterInput):Lanczos(mainFilterInput, attenuation);
                break;

            default:
                break;
            }
            sumPerPhase += tapValue;
            pFilter[currentPhase*taps + currentTap-1] = tapValue;
        }
        // Normalize each filter phase
        for(currentTap = 0; currentTap < taps; currentTap++)
        {
            pFilter[currentPhase*taps + currentTap] /= sumPerPhase;
        }
    }
}
/**
***************************************************************************************************
*   LanczosFilterGenerator::GenerateSincCoeff
*
*   @brief
*     Generate 4-tap, 32 phase filter coefficients for  UV Sinc kernel
*
***************************************************************************************************
*/
void LanczosFilterGenerator::GenerateSincCoeff(
    float*       pFilter,          ///< [out] Filter coefficients
    float        attenuation,      ///< [in]  Lanczos kernel parameter
    float        kernelInterval,   ///< [in]  Input interval for the fitler kernel
    uint32       taps,             ///< [in]  Number of filter taps
    uint32       phases)           ///< [in]  Number of filter phases
{
    uint32 totalNumberofCoef = phases * taps;
    uint32 currentPhase = 0;
    uint32 currentTap = 0;

    for (currentPhase = 0; currentPhase < phases; currentPhase++)
    {
        float sumPerPhase = 0.0f;
        for (currentTap = 1; currentTap <= taps; currentTap++)
        {
            uint32 mainFilterIndex = currentTap * phases - currentPhase;
            float PiX = static_cast<float>(M_PI) * (static_cast<float>(2 * mainFilterIndex) / totalNumberofCoef - 1.0f);
            PiX = PiX * kernelInterval;
            float tapValue = 0.0f;
            tapValue = Sinc(PiX)*Sinc(PiX*attenuation);
            sumPerPhase += tapValue;
            pFilter[currentPhase*taps + currentTap - 1] = tapValue;
        }
        // Normalize each filter phase
        for (currentTap = 0; currentTap < taps; currentTap++)
        {
            pFilter[currentPhase*taps + currentTap] /= sumPerPhase;
        }
    }
}

/**
***************************************************************************************************
*   LanczosFilterGenerator::Sinc
*
*   @brief
*     Calculates the value of the sinc function at the given input argument
*
*   @return
*     Returns the value of sinc function at the given input
*
***************************************************************************************************
*/
float LanczosFilterGenerator::Sinc(
    float input)  ///< [in] Kernel input
{
    if (fabs(input) > Epsilon)
    {
        float sinus = static_cast<float>(sin(input));
        float result = sinus / input;
        return result;
    }
    return 1.0f;
}

/**
***************************************************************************************************
*   LanczosFilterGenerator::Lanczos
*
*   @brief
*     Calculates the value of the Lanczos function at the given input
*     argument and attenuation factor
*
*   @return
*     Returns the value of Lanczos kernel at the given input arguments
*
***************************************************************************************************
*/
float LanczosFilterGenerator::Lanczos(
    float input,        ///< [in] Kernel input value
    float attenuation)  ///< [in] Kernel parameter
{
    return (Sinc(input) * Sinc(attenuation*input));
}

/**
***************************************************************************************************
*   LanczosFilterGenerator::Interpolate
*
*   @brief
*     Interpolate a point on a straight line
*
*   @return
*     Return the interpolated value
*
***************************************************************************************************
*/
float LanczosFilterGenerator::Interpolate(
    float dbValue,    ///< [in] Frequency domain gain
    float sharpMin,   ///< [in] Minimum sharpness
    float sharpMax,   ///< [in] Maximum sharpness
    float dbMin,      ///< [in] Minimum frequency domain gain
    float dbMax)      ///< [in] Maximum frequency domain gain
{
    float slope = (dbMax - dbMin)/(sharpMax - sharpMin);
    return (slope*(dbValue - sharpMin) + dbMin);
}

/**
***************************************************************************************************
*   LanczosFilterGenerator::Ratio2Attenuation
*
*   @brief
*     Interpolate the attenuation factor using the pre-generated table
*
***************************************************************************************************
*/
float LanczosFilterGenerator::Ratio2Attenuation(
    float ratio,         ///< [in] Scaling ratio
    float sharpness)     ///< [in] Sharpness control
{
    float sharpMax  = static_cast<float>(MaxSharpness);
    float sharpMin  = static_cast<float>(MinSharpness);
    float sharpFlat = (sharpMax + sharpMin)/2.0f;
    float dbMax;
    float dbMin;
    float dbValue;
    float attenuation;
    // index to closest table entries for the corresponding dbValue
    int32 tableIndex0;
    int32 tableIndex1;

    if (ratio >= 1.0f)
    {
        if (sharpness < 0)
        {
            dbMax = UpdBFlat;
            dbMin = UpdBFuzzy;
            sharpMax = sharpFlat;
        }
        else
        {
            dbMax = UpdBSharp;
            dbMin = UpdBFlat;
            sharpMin = sharpFlat;
        }
        dbValue = Interpolate(sharpness, sharpMin, sharpMax, dbMin, dbMax);

        tableIndex0 = 0 ;
        while ((tableIndex0 < UpdBPoints - 1) && LancUpScaledBTable[0][tableIndex0] > dbValue && (tableIndex0 < UpdBPoints-1))
        {
            tableIndex0++;
        }
        tableIndex1 = tableIndex0 + 1;
        if (tableIndex0 == UpdBPoints-1)
        {
            tableIndex1 = tableIndex0;
            tableIndex0--;
        }

        sharpMax = LancUpScaledBTable[1][tableIndex0];
        sharpMin = LancUpScaledBTable[1][tableIndex1];
        dbMax = LancUpScaledBTable[0][tableIndex0];
        dbMin = LancUpScaledBTable[0][tableIndex1];
        attenuation = Interpolate(dbValue, dbMax, dbMin, sharpMax, sharpMin);

        return attenuation;
    }
    else if (ratio < ThresholdRatioLow)
    {
        if (sharpness < 0)
        {
            dbMax = DowndBFlat;  //LancDownScaledBTable[0][UpdBPoints/2];
            dbMin = DowndBFuzzy; //LancDownScaledBTable[0][UpdBPoints-1];
            sharpMax = sharpFlat;
        }
        else
        {
            dbMax = DowndBSharp; // LancDownScaledBTable[0][0];
            dbMin = DowndBFlat;  // LancDownScaledBTable[0][UpdBPoints/2];
            sharpMin = sharpFlat;
        }
        dbValue = Interpolate(sharpness, sharpMin, sharpMax, dbMin, dbMax);

    }
    else
    {
        dbMin = Interpolate(ratio, ThresholdRatioLow, ThresholdRatioUp, DowndBFlat, UpdBFlat);
        float dbflat= Interpolate(ratio, ThresholdRatioLow, ThresholdRatioUp, DowndBFlat,  UpdBFlat);
        dbMax = Interpolate(ratio, ThresholdRatioLow, ThresholdRatioUp, DowndBSharp, UpdBSharp);
        sharpMax = float(MaxSharpness);
        sharpMin = float(MinSharpness);

        if (sharpness < 0)
        {
            // interpole between [dbmin, dbflat]
            dbMax = dbflat;
            dbValue = Interpolate(sharpness, sharpMin, 0.0f, dbMin, dbMax);
        }
        else
        {
            // interpole between [dbflat, dbmax]
            dbMin = dbflat;
            dbValue = Interpolate(sharpness, 0.0f, sharpMax, dbMin, dbMax);
        }

        // dbValue must be in the Lancsos db_table, otherwise it must be clipped

        if (dbValue > LancDownScaledBTable[0][0]) //desired attenuation is not reachable
        {
            dbValue = LancDownScaledBTable[0][0];
        }
        else if (dbValue < LancDownScaledBTable[0][DowndBPoints-1]) //desired attenuation is not reachable
        {
            dbValue = LancDownScaledBTable[0][DowndBPoints-1];
        }
    }
    // find the closest index and interpolate to find the dB value
    tableIndex1 = 0;
    while ( (LancDownScaledBTable[0][tableIndex1] > dbValue) && (tableIndex1 < DowndBPoints - 1) )
    {
        tableIndex1++;
    }
    tableIndex0 =  tableIndex1 - 1;
    if (tableIndex1 == 0)
    {
        tableIndex0 = tableIndex1;
        tableIndex1 = 1;
    }

    // compute the attenuation factor required to reach the dB at Nyquist
    // interpolate on the scale axes
    int32_t row0 = static_cast<int>(0.5f + ratio*DowndBScales);
    int32_t row1;
    if (static_cast<float>(row0)/DowndBScales < ratio)
    {
        row1 = row0 + 1;
        if (row1 > DowndBScales)
        {
            row1 = DowndBScales;
            row0 = row1 - 1;
        }
    }
    else
    {
        row1 = row0;
        row0--;
        if (row0 < 1)
        {
            row0 = 1;
            row1 = 2;
        }
    }
    float ratioLow = static_cast<float>(row0)/DowndBScales;
    float ratioUp  = static_cast<float>(row1)/DowndBScales;

    sharpMax = Interpolate(ratio, ratioLow, ratioUp, LancDownScaledBTable[row0][tableIndex0],
            LancDownScaledBTable[row1][tableIndex0]);

    sharpMin = Interpolate(ratio, ratioLow, ratioUp, LancDownScaledBTable[row0][tableIndex1],
            LancDownScaledBTable[row1][tableIndex1]);

    dbMax = LancDownScaledBTable[0][tableIndex0];
    dbMin = LancDownScaledBTable[0][tableIndex1];
    attenuation = Interpolate(dbValue, dbMax, dbMin, sharpMax, sharpMin);
    return attenuation;
}

/**
***************************************************************************************************
*   LanczosFilterGenerator::Ratio2CuttOff
*
*   @brief
*     Maps the scaling ratio to the required input interval (Equation holds for 8-tap filter Only)
*
***************************************************************************************************
*/
float LanczosFilterGenerator::Ratio2CuttOff(
    float ratio)     ///< [in] Scaling ratio
{
    float cutoffParam = static_cast<float>
        (PCoef3*(pow(ratio,3.0f)) + PCoef2*(pow(ratio,2.0f)) +
        PCoef1*ratio + PCoef0);

    return cutoffParam;
}

// =====================================================================================================================
void LanczosFilterGenerator::ConvertScalingCoeffsToUint(
    uint16* pUintFilter,
    const float* pFloatFilter,
    const uint32 numTaps,
    const uint32 numPhases)
{
    constexpr uint16  QuantFrac = 10;
    constexpr uint16  CoefOutFrac = 12;
    int32 error = 0;
    int32 halfError = 0;
    int16 quantVal = 0;
    int32 sum = 0;
    uint16 loc = 0;
    float filterVal = 0.0;

    if (pUintFilter != nullptr)
    {
        for (uint32 p = 0; p < (numPhases / 2 + 1); p++)
        {
            sum = 0;
            for (uint32 t = 0; t < numTaps; t++)
            {
                filterVal = pFloatFilter[(p * numTaps) + t];
                quantVal = static_cast<int16>(filterVal * (float)(1 << QuantFrac));
                pUintFilter[(p * numTaps) + t] = static_cast<uint16>(quantVal);
                sum += quantVal;
            }
            error = sum - static_cast<int16>(1 << QuantFrac);

            if (error != 0)
            {
                halfError = error / 2;
                // split adjustment between center taps
                //                _loc = (_num_taps / 2) - 1;
                MaxLoc(&pFloatFilter[p * numTaps], numTaps, loc);
                quantVal = static_cast<int16>(pUintFilter[(p * numTaps) + loc]);
                quantVal -= halfError;
                pUintFilter[(p * numTaps) + loc ] = static_cast<uint16>(quantVal);
                quantVal = static_cast<int16>(pUintFilter[(p * numTaps) + loc - 1]);
                quantVal -= halfError;
                pUintFilter[(p * numTaps) + loc - 1] = static_cast<uint16>(quantVal);
            }
            if (CoefOutFrac > QuantFrac)
            {
                for (uint32 t = 0; t < numTaps; t++)
                {
                    pUintFilter[(p * numTaps) + t] = pUintFilter[(p * numTaps) + t] << (CoefOutFrac - QuantFrac);
                }
            }
        }
    }
}

// =====================================================================================================================
void LanczosFilterGenerator::MaxLoc(
    const float* pFilter,
    uint32 numTaps,
    uint16& maxLoc)
{
    float maxVal = 0;
    maxLoc = (numTaps / 2) - 1;
    for (uint32 i = 0; i < numTaps; i++)
    {
        if (pFilter[i] > maxVal)
        {
            maxVal = pFilter[i];
            maxLoc = i;
        }
    }
    if (maxLoc == 0)
    {
        maxLoc = 1;//safeguard condition in order to avoid getting values out of the
                   // array boundaries.
    }
}

/** Minimum of two values: */
#define MIN2( A, B )   ( (A)<(B) ? (A) : (B) )
/** Maximum of two values: */
#define MAX2( A, B )   ( (A)>(B) ? (A) : (B) )

// =====================================================================================================================
void LanczosFilterGenerator::GenerateLanczosCoeff(
    float* pCoef,                ///< [in] coef buffer
    float  scalingRatio,         ///< [in] scaling ratio
    uint32  tapCount,             ///< [in] number of taps
    uint32  phaseCount,
    float kernelInterval,
    float attenuation,
    float sharpness)
{
    if (pCoef != nullptr)
    {
        LanczosFilterGenerator::CoefType coefType = LanczosFilterGenerator::CoefType::ModifiedLanczos;


        // 4-tap and 8-tap filters use two different kernel functions for their coefficients
        // the parameters for each filter mode is assigned separately
        if (tapCount == 4) //TapCount4
        {
            coefType =
                (scalingRatio < 1) ? LanczosFilterGenerator::CoefType::TruncatedLanczos :
                LanczosFilterGenerator::CoefType::ModifiedLanczos;

            if (scalingRatio < 1)
            {
                kernelInterval = LanczosFilterGenerator::Ratio2CuttOff(1 / scalingRatio);
                attenuation = (scalingRatio <= 1) ? 1.0f : 1.0f / (MIN2(kernelInterval, (tapCount + 2) / 2.0f));
                kernelInterval = MIN2(kernelInterval, tapCount / 2.0f);
            }
            else
            {
                attenuation = LanczosFilterGenerator::Ratio2Attenuation(1 / scalingRatio, sharpness);
            }
        }
        else
        {
            coefType = (scalingRatio <= 1) ?
                LanczosFilterGenerator::CoefType::TruncatedLanczos : LanczosFilterGenerator::CoefType::TruncatedSinc;
            kernelInterval = LanczosFilterGenerator::Ratio2CuttOff(1 / scalingRatio);
            attenuation = (scalingRatio <= 1) ? 1.0f : 1.0f / (MIN2(kernelInterval, (tapCount + 2) / 2.0f));
            kernelInterval = MIN2(kernelInterval, tapCount / 2.0f);
        }

        LanczosFilterGenerator::GenerateLanczosCoeff(pCoef, attenuation, kernelInterval, tapCount, phaseCount, coefType);
    }
}
