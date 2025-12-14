/*
 * Copyright 2024 Valve Corporation
 * Copyright 2023 Alyssa Rosenzweig
 * SPDX-License-Identifier: MIT
 */

#pragma once

#ifndef __OPENCL_VERSION__
#error "should only be included from OpenCL"
#endif

#define CL_FLT_EPSILON 1.1920928955078125e-7f

/* OpenCL C lacks roundf and llroundf, we can emulate it */
static inline float roundf(float x)
{
   return trunc(x + copysign(0.5f - 0.25f * CL_FLT_EPSILON, x));
}

static inline long long llroundf(float x)
{
   return roundf(x);
}

static inline long lrintf(float x)
{
   return (long)roundf(x);
}

static inline float fabsf(float x)
{
   return fabs(x);
}
