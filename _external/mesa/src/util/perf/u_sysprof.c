/*
 * Copyright 2025 Igalia S.L.
 * SPDX-License-Identifier: MIT
 */

#include "u_sysprof.h"

#include <stdbool.h> /* needed for sysprof-collector.h */
#include <stdio.h>
#include <stdlib.h>
#include <sysprof-collector.h>

struct perf_sysprof_entry {
   SysprofTimeStamp begin;
   /* SysprofCaptureMark itself limits it to 40 characters */
   char name[40];
};

void *
util_sysprof_begin(const char *name)
{
   struct perf_sysprof_entry *trace =
      malloc(sizeof(struct perf_sysprof_entry));

   trace->begin = SYSPROF_CAPTURE_CURRENT_TIME;
   snprintf(trace->name, sizeof(trace->name), "%s", name);

   return trace;
}

void
util_sysprof_end(void **scope)
{
   struct perf_sysprof_entry *trace = (struct perf_sysprof_entry *) *scope;

   sysprof_collector_mark(trace->begin,
                          SYSPROF_CAPTURE_CURRENT_TIME - trace->begin, "Mesa",
                          trace->name, NULL);
   free(trace);
}
