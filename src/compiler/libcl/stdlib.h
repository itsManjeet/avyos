/*
 * Copyright 2024 Valve Corporation
 * Copyright 2023 Alyssa Rosenzweig
 * SPDX-License-Identifier: MIT
 */

#pragma once

#ifndef __OPENCL_VERSION__
#error "should only be included from OpenCL"
#endif

/* OpenCL C lacks a standard abort, so we plumb through the NIR intrinsic. */
void nir_printf_abort(void);
static inline void abort(void) { nir_printf_abort(); }
