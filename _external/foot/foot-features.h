#pragma once

#include <stdio.h>

extern const char version_and_features[];

static inline void
print_version_and_features(const char *prefix)
{
    fputs(prefix, stdout);
    fputs(version_and_features, stdout);
    fputc('\n', stdout);
}
