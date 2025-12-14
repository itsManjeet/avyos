/*
 * Copyright 2024 Valve Corporation
 * Copyright 2023 Alyssa Rosenzweig
 * SPDX-License-Identifier: MIT
 */

#pragma once

#ifndef __OPENCL_VERSION__
#error "should only be included from OpenCL"
#endif

#include <stdlib.h>

/* OpenCL C lacks static_assert, a part of C11. This makes static_assert
 * available on both host and device. It is defined as variadic to handle also
 * no-message static_asserts (standardized in C23).
 */
#define _S(x) #x
#define _PASTE_(x, y) x##y
#define _PASTE(x, y) _PASTE_(x, y)
#define static_assert(_COND, ...)                                              \
   typedef char _PASTE(static_assertion, __LINE__)[(_COND) ? 1 : -1]

/* OpenCL C lacks a standard assert. We implement one on top of abort. We are
 * careful to use a single printf so the lines don't get split up if multiple
 * threads assert in parallel.
 */
#ifndef NDEBUG
#define _ASSERT_STRING(x) _ASSERT_STRING_INNER(x)
#define _ASSERT_STRING_INNER(x) #x
#define assert(x) if (!(x)) { \
   printf("Shader assertion fail at " __FILE__ ":" \
          _ASSERT_STRING(__LINE__) "\nExpected " #x "\n\n"); \
   nir_printf_abort(); \
}
#else
#define assert(x)
#endif
