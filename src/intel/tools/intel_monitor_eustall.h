/*
 * Copyright 2024 Intel Corporation
 * SPDX-License-Identifier: MIT
 */

#pragma once

#include "perf/intel_perf.h"

struct eustall_config {
   struct intel_perf_query_eustall_result result;
   struct intel_device_info *devinfo;
   int drm_fd;
   uint8_t* buf;
   size_t buf_len;
   int record_size;

   int fd;
   uint64_t poll_period_ns;
   int sample_rate;
   uint32_t min_event_count;
};

struct eustall_config* eustall_setup(int drm_fd,
                                     struct intel_device_info* devinfo);
bool eustall_sample(void *cfg);
void eustall_dump_results(void *cfg, FILE* file);
void eustall_close(void *cfg);
