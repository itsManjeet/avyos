/*
 * Copyright 2025 Igalia S.L.
 * SPDX-License-Identifier: MIT
 */

#pragma once

#ifdef __cplusplus
extern "C" {
#endif

void *util_sysprof_begin(const char *name);
void util_sysprof_end(void **scope);

#ifdef __cplusplus
}
#endif
