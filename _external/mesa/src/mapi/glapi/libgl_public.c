/* Copyright (C) 2010 LunarG Inc.
 * SPDX-License-Identifier: MIT
 *
 * Authors:
 *    Chia-I Wu <olv@lunarg.com>
 */

#include "glapi/glapi.h"

/* REALLY_INITIAL_EXEC implies __GLIBC__ */
#if defined(USE_X86_ASM) && defined(REALLY_INITIAL_EXEC)
#include "entry_x86_tls.h"
#define MAPI_TMP_STUB_ASM_GCC_NO_HIDDEN
#elif defined(USE_X86_64_ASM) && defined(REALLY_INITIAL_EXEC)
#include "entry_x86-64_tls.h"
#define MAPI_TMP_STUB_ASM_GCC_NO_HIDDEN
#elif defined(USE_PPC64LE_ASM) && UTIL_ARCH_LITTLE_ENDIAN && defined(REALLY_INITIAL_EXEC)
#include "entry_ppc64le_tls.h"
#define MAPI_TMP_STUB_ASM_GCC_NO_HIDDEN
#else

/* C version of the public entries */
#define MAPI_TMP_DEFINES
#define MAPI_TMP_PUBLIC_ENTRIES_NO_HIDDEN

#if defined(_WIN32) && defined(_WINDOWS_)
#error "Should not include <windows.h> here"
#endif

#endif /* asm */

#include "glapi_mapi_tmp.h"
