/*
 * Copyright © 2022 Collabora Ltd. and Red Hat Inc.
 * SPDX-License-Identifier: MIT
 */
#include "compiler/libcl/libcl_vk.h"
#include "nvk_query.h"

void
nvk_copy_queries(uint64_t pool_addr, uint query_start, uint query_stride,
                 uint first_query, uint query_count, uint64_t dst_addr,
                 uint64_t dst_stride, uint flags)
{
   uint i = get_sub_group_local_id() + cl_group_id.x * 32;
   if (i >= query_count)
      return;

   uint query = first_query + i;
   global uint *available_arr = (global uint *)pool_addr;
   bool available = available_arr[query] != 0;
   bool write_results = available || (flags & VK_QUERY_RESULT_PARTIAL_BIT);

   uint64_t report_offs = query_start + (uint64_t)query * (uint64_t)query_stride;
   global struct nvk_query_report *report =
      (global void *)(pool_addr + report_offs);

   uint64_t dst_offset = dst_stride * (uint64_t)i;
   uint num_reports = 1;

   if (query_stride == sizeof(struct nvk_query_report)) {
      /* Timestamp queries are the only ones use a single report */
      if (write_results) {
         vk_write_query(dst_addr + dst_offset, 0, flags, report->timestamp);
      }
   } else {
      /* Everything that isn't a timestamp has the invariant that the
       * number of destination entries is equal to the query stride divided
       * by the size of two reports.
       */
      num_reports = query_stride / (2 * sizeof(struct nvk_query_report));

      if (write_results) {
         for (uint r = 0; r < num_reports; ++r) {
            uint delta = report[(r * 2) + 1].value - report[r * 2].value;

            vk_write_query(dst_addr + dst_offset, r, flags, delta);
         }
      }
   }

   if (flags & VK_QUERY_RESULT_WITH_AVAILABILITY_BIT) {
      vk_write_query(dst_addr + dst_offset, num_reports, flags, available);
   }
}
