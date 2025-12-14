/*
 * Copyright 2024 Valve Corporation
 * Copyright 2023 Alyssa Rosenzweig
 * SPDX-License-Identifier: MIT
 */

#pragma once

#ifndef __OPENCL_VERSION__
#error "should only be included from OpenCL"
#endif

/* OpenCL lacks explicitly sized integer types, but we know the sizes of
 * particular integer types. These typedefs allow defining common headers with
 * explicit integer types (and therefore compatible data layouts).
 */
typedef ulong uint64_t;
typedef uint uint32_t;
typedef ushort uint16_t;
typedef uchar uint8_t;

typedef long int64_t;
typedef int int32_t;
typedef short int16_t;
typedef char int8_t;

typedef int64_t intmax_t;
typedef uint64_t uintmax_t;

/* These duplicate the C standard library and are required for the
 * u_intN_min/max implementations.
 */
#define UINT64_MAX 18446744073709551615ul
#define INT64_MAX 9223372036854775807l
#define UINT64_C(c)	c##UL

#define INT8_MIN (-128)
#define INT16_MIN (-32768)
#define INT32_MIN (-2147483648)
#define INT64_MIN (-9223372036854775807l - 1)

#define UINT8_MAX 255
#define INT8_MAX 127
#define UINT16_MAX 65535
#define INT16_MAX 32767
#define UINT32_MAX 4294967295U
#define INT32_MAX 2147483647
#define INT64_MAX 9223372036854775807l
