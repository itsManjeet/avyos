/*
 * Copyright 2024 Valve Corporation
 * Copyright 2023 Alyssa Rosenzweig
 * SPDX-License-Identifier: MIT
 */

#pragma once

#ifndef __OPENCL_VERSION__
#error "should only be included from OpenCL"
#endif

/* OpenCL C lacks a standard memcpy, but clang has one that will be plumbed into
 * a NIR memcpy intrinsic. This is not a competent implementation of memcpy for
 * large amounts of data, since it's necessarily single threaded, but memcpy is
 * too useful for shared CPU/GPU code that it's worth making the standard
 * library function work.
 */
#define memcpy __builtin_memcpy
