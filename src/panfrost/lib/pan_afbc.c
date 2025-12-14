/*
 * Copyright © 2023-2025 Amazon.com, Inc. or its affiliates.
 *
 * SPDX-License-Identifier: MIT
 */

#include "pan_afbc.h"
#include "util/perf/cpu_trace.h"
#include "util/detect_arch.h"
#include "util/u_cpu_detect.h"

#if DETECT_ARCH_AARCH64 || (DETECT_ARCH_ARM && !defined(__SOFTFP__))
/* armhf builds default to VFP, not NEON, and refuses to compile NEON
 * intrinsics unless you tell it "no really". */
#if DETECT_ARCH_ARM
#pragma GCC target ("fpu=neon")
#endif
#include <arm_neon.h>
#endif

#if DETECT_ARCH_AARCH64

/* Arm A64 NEON intrinsics version. */
uint32_t
pan_afbc_payload_layout_packed(unsigned arch,
                               const struct pan_afbc_headerblock *headers,
                               struct pan_afbc_payload_extent *layout,
                               uint32_t nr_blocks, enum pipe_format format,
                               uint64_t modifier)
{
   MESA_TRACE_FUNC();

   uint32_t uncompressed_size =
      pan_afbc_payload_uncompressed_size(format, modifier);
   uint32_t body_size = 0;

   alignas(16) static const uint8_t idx0[16] =
      { 4, 5, 6, ~0, 7, 8, 9, ~0, 10, 11, 12, ~0, 13, 14, 15, ~0 };
   alignas(16) static const uint8_t idx1[16] =
      { 0, 4, 8, 12, 16, 20, 24, 28, 32, 36, 40, 44, 48, 52, 56, 60 };
   alignas(16) static const uint8_t mask[16] =
      { 63, 63, 63, 63, 63, 63, 63, 63, 63, 63, 63, 63, 63, 63, 63, 63 };
   alignas(16) static const uint8_t ones[16] =
      { 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1 };

   uint8x16_t vidx0 = vld1q_u8(idx0);
   uint8x16_t vidx1 = vld1q_u8(idx1);
   uint8x16_t vmask = vld1q_u8(mask);
   uint8x16_t vones = vld1q_u8(ones);

   for (uint32_t i = 0; i < nr_blocks; ++i) {
      uint32_t payload_size = 0;

      /* Skip sum if the 1st subblock is 0 (solid color encoding). */
      if (arch < 7 || headers[i].payload.subblock_sizes[0] & 0x3f) {
         uint8x16_t vhdr = vld1q_u8((uint8_t *)&headers[i]);

         /* Dispatch 6-bit packed 16 subblock sizes into 8-bit vector. */
         uint8x16_t v0 = vqtbl1q_u8(vhdr, vidx0);
         uint8x16_t v1 = vreinterpretq_u8_u32(
            vshrq_n_u32(vreinterpretq_u32_u8(v0), 6));
         uint8x16_t v2 = vreinterpretq_u8_u32(
            vshrq_n_u32(vreinterpretq_u32_u8(v0), 12));
         uint8x16_t v3 = vreinterpretq_u8_u32(
            vshrq_n_u32(vreinterpretq_u32_u8(v0), 18));
         uint8x16x4_t vtbl = {{ v0, v1, v2, v3 }};
         v0 = vqtbl4q_u8(vtbl, vidx1);
         v0 = vandq_u8(v0, vmask);

         /* Sum across vector. */
         payload_size = vaddlvq_u8(v0);

         /* Number of subblocks of size 1. */
         v0 = vceqq_u8(v0, vones);
         v0 = vandq_u8(v0, vones);
         uint32_t nr_ones = vaddvq_u8(v0);

         /* Payload size already stores subblocks of size 1. Fix-up the sum
          * using the number of such subblocks. */
         payload_size += nr_ones * (uncompressed_size - 1);

         payload_size = ALIGN_POT(payload_size, 16);
      }

      layout[i].size = payload_size;
      layout[i].offset = body_size;
      body_size += payload_size;
   }

   return body_size;
}

#else

/* Arm A32 NEON intrinsics and generic version. */
uint32_t
pan_afbc_payload_layout_packed(unsigned arch,
                               const struct pan_afbc_headerblock *headers,
                               struct pan_afbc_payload_extent *layout,
                               uint32_t nr_blocks, enum pipe_format format,
                               uint64_t modifier)
{
   MESA_TRACE_FUNC();

   uint32_t uncompressed_size =
      pan_afbc_payload_uncompressed_size(format, modifier);
   uint32_t body_size = 0;

#if DETECT_ARCH_ARM && !defined(__SOFTFP__)

   if (unlikely(!util_get_cpu_caps()->has_neon))
      goto no_neon;

   /* Arm A32 NEON intrinsics version. */

   alignas(16) static const uint8_t idx0[2][8] =
      { { 4, 5, 6, ~0, 7, 8, 9, ~0 }, { 2, 3, 4, ~0, 5, 6, 7, ~0 } };
   alignas(8) static const uint8_t idx1[8] =
      { 0, 4, 8, 12, 16, 20, 24, 28 };
   alignas(16) static const uint8_t mask[16] =
      { 63, 63, 63, 63, 63, 63, 63, 63, 63, 63, 63, 63, 63, 63, 63, 63 };
   alignas(16) static const uint8_t ones[16] =
      { 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1 };

   uint8x8_t vidx00 = vld1_u8(idx0[0]);
   uint8x8_t vidx01 = vld1_u8(idx0[1]);
   uint8x8_t vidx1 = vld1_u8(idx1);
   uint8x16_t vmask = vld1q_u8(mask);
   uint8x16_t vones = vld1q_u8(ones);

   for (uint32_t i = 0; i < nr_blocks; ++i) {
      uint32_t payload_size = 0;

      /* Skip sum if the 1st subblock is 0 (solid color encoding). */
      if (arch < 7 || headers[i].payload.subblock_sizes[0] & 0x3f) {
         /* vld1_u8_x2() isn't widely available yet. */
         uint8x8_t vhdr0 = vld1_u8(&headers[i].u8[0]);
         uint8x8_t vhdr1 = vld1_u8(&headers[i].u8[8]);
         uint8x8x2_t vhdr = {{ vhdr0, vhdr1 }};

         /* Dispatch 6-bit packed 16 payload sizes into 8-bit vector. Note
          * that the NEON TBL instr in A32 only supports doubleword operands
          * while VSHR also supports quadword. Not sure how to mix doubleword
          * and quadword intrinsics and get compilers to correctly alias D and
          * Q registers though (128-bit register Q0 is an alias for the two
          * consecutive 64-bit registers D0 and D1), so stick with doubleword
          * intrinsics here. */
         uint8x8_t v00 = vtbl2_u8(vhdr, vidx00);
         uint8x8_t v01 = vtbl1_u8(vhdr1, vidx01);
         uint8x8_t v10 = vreinterpret_u8_u32(
            vshr_n_u32(vreinterpret_u32_u8(v00), 6));
         uint8x8_t v11 = vreinterpret_u8_u32(
            vshr_n_u32(vreinterpret_u32_u8(v01), 6));
         uint8x8_t v20 = vreinterpret_u8_u32(
            vshr_n_u32(vreinterpret_u32_u8(v00), 12));
         uint8x8_t v21 = vreinterpret_u8_u32(
            vshr_n_u32(vreinterpret_u32_u8(v01), 12));
         uint8x8_t v30 = vreinterpret_u8_u32(
            vshr_n_u32(vreinterpret_u32_u8(v00), 18));
         uint8x8_t v31 = vreinterpret_u8_u32(
            vshr_n_u32(vreinterpret_u32_u8(v01), 18));
         uint8x8x4_t vtbl0 = {{ v00, v01, v10, v11 }};
         uint8x8x4_t vtbl1 = {{ v20, v21, v30, v31 }};
         v00 = vtbl4_u8(vtbl0, vidx1);
         v01 = vtbl4_u8(vtbl1, vidx1);
         uint8x16_t v0 = vandq_u8(vcombine_u8(v00, v01), vmask);

         /* Sum across vector. */
         uint64x2_t v1 = vpaddlq_u32(vpaddlq_u16(vpaddlq_u8(v0)));
         payload_size = vget_lane_u64(vadd_u64(vget_low_u64(v1),
                                               vget_high_u64(v1)), 0);

         /* Number of subblocks of size 1. */
         v0 = vceqq_u8(v0, vones);
         v0 = vandq_u8(v0, vones);
         v1 = vpaddlq_u32(vpaddlq_u16(vpaddlq_u8(v0)));
         uint32_t nr_ones = vget_lane_u64(vadd_u64(vget_low_u64(v1),
                                                   vget_high_u64(v1)), 0);

         /* Payload size already stores subblocks of size 1. Fix-up the sum
          * using the number of such subblocks. */
         payload_size += nr_ones * (uncompressed_size - 1);

         payload_size = ALIGN_POT(payload_size, 16);
      }

      layout[i].size = payload_size;
      layout[i].offset = body_size;
      body_size += payload_size;
   }

   return body_size;

 no_neon:
#endif

   /* Generic version. */

   /* XXX: It might be faster to copy the header from non-cacheable memory
    * into a cacheline sized chunk in cacheable memory in order to avoid too
    * many uncached transactions. Not sure though, so it needs testing. */

   for (uint32_t i = 0; i < nr_blocks; i++) {
      uint32_t payload_size =
         pan_afbc_payload_size(arch, headers[i], uncompressed_size);
      layout[i].size = payload_size;
      layout[i].offset = body_size;
      body_size += payload_size;
   }

   return body_size;
}

#endif
