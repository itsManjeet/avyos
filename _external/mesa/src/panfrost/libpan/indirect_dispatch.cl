/*
 * Copyright 2024 Collabora Ltd.
 * SPDX-License-Identifier: MIT
 */

#include "compiler/libcl/libcl.h"
#include "compiler/libcl/libcl_vk.h"
#include "genxml/gen_macros.h"
#include "lib/pan_encoder.h"

#if (PAN_ARCH >= 6 && PAN_ARCH <= 9)
KERNEL(1)
panlib_indirect_dispatch(constant VkDispatchIndirectCommand *cmd,
                         uint32_t size_x, uint32_t size_y, uint32_t size_z,
                         global uint8_t *indirect_job,
                         global uint32_t *num_wg_sysval_x,
                         global uint32_t *num_wg_sysval_y,
                         global uint32_t *num_wg_sysval_z)
{
   const uint32_t num_x = cmd->x;
   const uint32_t num_y = cmd->y;
   const uint32_t num_z = cmd->z;

   /* In case there is nothing to dispatch, set a null job otherwise we ensure
    * that the type is compute */
   const bool is_no_op = num_x * num_y * num_z == 0;
   struct mali_job_header_packed job_header;

   pan_pack(&job_header, JOB_HEADER, cfg) {
      cfg.type = is_no_op ? MALI_JOB_TYPE_NULL : MALI_JOB_TYPE_COMPUTE;
   }
   pan_merge((global struct mali_job_header_packed *)indirect_job, &job_header, JOB_HEADER);

   /* If there is nothing else to do, let's just return now */
   if (is_no_op)
      return;

#if PAN_ARCH == 9
   global struct mali_compute_payload_packed *payload =
      (global struct mali_compute_payload_packed *)(indirect_job +
                                                    pan_section_offset(
                                                       COMPUTE_JOB, PAYLOAD));

   pan_unpack(payload, COMPUTE_PAYLOAD, unpacked_job_payloads);
   pan_pack(payload, COMPUTE_PAYLOAD, cfg) {
      memcpy(&cfg, &unpacked_job_payloads, sizeof(cfg));
      cfg.workgroup_count_x = num_x;
      cfg.workgroup_count_y = num_y;
      cfg.workgroup_count_z = num_z;
   }
#else
   global struct mali_invocation_packed *invocation =
      (global struct mali_invocation_packed *)(indirect_job +
                                               pan_section_offset(COMPUTE_JOB,
                                                                  INVOCATION));

   pan_pack_work_groups_compute(invocation, num_x, num_y, num_z, size_x,
                                size_y, size_z, false, false);
#endif

   *num_wg_sysval_x = num_x;
   *num_wg_sysval_y = num_y;
   *num_wg_sysval_z = num_z;
}
#endif
