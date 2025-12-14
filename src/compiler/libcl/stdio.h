/*
 * Copyright 2024 Valve Corporation
 * Copyright 2023 Alyssa Rosenzweig
 * SPDX-License-Identifier: MIT
 */

#pragma once

#ifndef __OPENCL_VERSION__
#error "should only be included from OpenCL"
#endif

/* FILE * pointers can be useful in function signatures shared across
 * host/device, but are meaningless in OpenCL. Turn them into void* to allow
 * consistent prototype across host/device even though there won't be an actual
 * file pointer on the device side.
 */
#define FILE void
