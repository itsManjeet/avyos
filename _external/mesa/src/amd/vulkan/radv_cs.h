/*
 * Copyright © 2016 Red Hat.
 * Copyright © 2016 Bas Nieuwenhuizen
 *
 * SPDX-License-Identifier: MIT
 */

#ifndef RADV_CS_H
#define RADV_CS_H

#include <assert.h>
#include <stdint.h>
#include <string.h>

#include "radv_cmd_buffer.h"
#include "radv_radeon_winsys.h"
#include "radv_sdma.h"
#include "sid.h"

static inline unsigned
radeon_check_space(struct radeon_winsys *ws, struct radeon_cmdbuf *cs, unsigned needed)
{
   assert(cs->cdw <= cs->reserved_dw);
   if (cs->max_dw - cs->cdw < needed)
      ws->cs_grow(cs, needed);
   cs->reserved_dw = MAX2(cs->reserved_dw, cs->cdw + needed);
   return cs->cdw + needed;
}

#define radeon_begin(cs)                                                                                               \
   struct radeon_cmdbuf *__cs = (cs);                                                                                  \
   uint32_t __cs_num = __cs->cdw;                                                                                      \
   UNUSED uint32_t __cs_reserved_dw = __cs->reserved_dw;                                                               \
   uint32_t *__cs_buf = __cs->buf

#define radeon_end()                                                                                                   \
   do {                                                                                                                \
      __cs->cdw = __cs_num;                                                                                            \
      assert(__cs->cdw <= __cs->max_dw);                                                                               \
      __cs = NULL;                                                                                                     \
   } while (0)

#define radeon_emit(value)                                                                                             \
   do {                                                                                                                \
      assert(__cs_num < __cs_reserved_dw);                                                                             \
      __cs_buf[__cs_num++] = (value);                                                                                  \
   } while (0)

#define radeon_emit_array(values, num)                                                                                 \
   do {                                                                                                                \
      unsigned __n = (num);                                                                                            \
      assert(__cs_num + __n <= __cs_reserved_dw);                                                                      \
      memcpy(__cs_buf + __cs_num, (values), __n * 4);                                                                  \
      __cs_num += __n;                                                                                                 \
   } while (0)

/* Packet building helpers. Don't use directly. */
#define __radeon_set_reg_seq(reg, num, idx, prefix_name, packet, reset_filter_cam)                                     \
   do {                                                                                                                \
      assert((reg) >= prefix_name##_REG_OFFSET && (reg) < prefix_name##_REG_END);                                      \
      radeon_emit(PKT3(packet, num, 0) | PKT3_RESET_FILTER_CAM_S(reset_filter_cam));                                   \
      radeon_emit((((reg) - prefix_name##_REG_OFFSET) >> 2) | ((idx) << 28));                                          \
   } while (0)

#define __radeon_set_reg(reg, idx, value, prefix_name, packet)                                                         \
   do {                                                                                                                \
      __radeon_set_reg_seq(reg, 1, idx, prefix_name, packet, 0);                                                       \
      radeon_emit(value);                                                                                              \
   } while (0)

/* Packet building helpers for CONFIG registers. */
#define radeon_set_config_reg_seq(reg, num) __radeon_set_reg_seq(reg, num, 0, SI_CONFIG, PKT3_SET_CONFIG_REG, 0)

#define radeon_set_config_reg(reg, value) __radeon_set_reg(reg, 0, value, SI_CONFIG, PKT3_SET_CONFIG_REG)

/* Packet building helpers for CONTEXT registers. */
#define radeon_set_context_reg_seq(reg, num) __radeon_set_reg_seq(reg, num, 0, SI_CONTEXT, PKT3_SET_CONTEXT_REG, 0)

#define radeon_set_context_reg(reg, value) __radeon_set_reg(reg, 0, value, SI_CONTEXT, PKT3_SET_CONTEXT_REG)

#define radeon_set_context_reg_idx(reg, idx, value) __radeon_set_reg(reg, idx, value, SI_CONTEXT, PKT3_SET_CONTEXT_REG)

#define radeon_opt_set_context_reg(cmdbuf, reg, reg_enum, value)                                                       \
   do {                                                                                                                \
      struct radv_cmd_buffer *__cmdbuf = (cmdbuf);                                                                     \
      struct radv_tracked_regs *__tracked_regs = &__cmdbuf->tracked_regs;                                              \
      const uint32_t __value = (value);                                                                                \
      if (!BITSET_TEST(__tracked_regs->reg_saved_mask, (reg_enum)) ||                                                  \
          __tracked_regs->reg_value[(reg_enum)] != __value) {                                                          \
         radeon_set_context_reg(reg, __value);                                                                         \
         BITSET_SET(__tracked_regs->reg_saved_mask, (reg_enum));                                                       \
         __tracked_regs->reg_value[(reg_enum)] = __value;                                                              \
         __cmdbuf->state.context_roll_without_scissor_emitted = true;                                                  \
      }                                                                                                                \
   } while (0)

#define radeon_opt_set_context_reg2(cmdbuf, reg, reg_enum, v1, v2)                                                     \
   do {                                                                                                                \
      struct radv_cmd_buffer *__cmdbuf = (cmdbuf);                                                                     \
      struct radv_tracked_regs *__tracked_regs = &__cmdbuf->tracked_regs;                                              \
      const uint32_t __v1 = (v1), __v2 = (v2);                                                                         \
      if (!BITSET_TEST_RANGE_INSIDE_WORD(__tracked_regs->reg_saved_mask, (reg_enum), (reg_enum) + 1, 0x3) ||           \
          __tracked_regs->reg_value[(reg_enum)] != __v1 || __tracked_regs->reg_value[(reg_enum) + 1] != __v2) {        \
         radeon_set_context_reg_seq(reg, 2);                                                                           \
         radeon_emit(__v1);                                                                                            \
         radeon_emit(__v2);                                                                                            \
         BITSET_SET_RANGE_INSIDE_WORD(__tracked_regs->reg_saved_mask, (reg_enum), (reg_enum) + 1);                     \
         __tracked_regs->reg_value[(reg_enum)] = __v1;                                                                 \
         __tracked_regs->reg_value[(reg_enum) + 1] = __v2;                                                             \
         cmdbuf->state.context_roll_without_scissor_emitted = true;                                                    \
      }                                                                                                                \
   } while (0)

#define radeon_opt_set_context_reg3(cmdbuf, reg, reg_enum, v1, v2, v3)                                                 \
   do {                                                                                                                \
      struct radv_cmd_buffer *__cmdbuf = (cmdbuf);                                                                     \
      struct radv_tracked_regs *__tracked_regs = &__cmdbuf->tracked_regs;                                              \
      const uint32_t __v1 = (v1), __v2 = (v2), __v3 = (v3);                                                            \
      if (!BITSET_TEST_RANGE_INSIDE_WORD(__tracked_regs->reg_saved_mask, (reg_enum), (reg_enum) + 2, 0x7) ||           \
          __tracked_regs->reg_value[(reg_enum)] != __v1 || __tracked_regs->reg_value[(reg_enum) + 1] != __v2 ||        \
          __tracked_regs->reg_value[(reg_enum) + 2] != __v3) {                                                         \
         radeon_set_context_reg_seq(reg, 3);                                                                           \
         radeon_emit(__v1);                                                                                            \
         radeon_emit(__v2);                                                                                            \
         radeon_emit(__v3);                                                                                            \
         BITSET_SET_RANGE_INSIDE_WORD(__tracked_regs->reg_saved_mask, (reg_enum), (reg_enum) + 2);                     \
         __tracked_regs->reg_value[(reg_enum)] = __v1;                                                                 \
         __tracked_regs->reg_value[(reg_enum) + 1] = __v2;                                                             \
         __tracked_regs->reg_value[(reg_enum) + 2] = __v3;                                                             \
         cmdbuf->state.context_roll_without_scissor_emitted = true;                                                    \
      }                                                                                                                \
   } while (0)

#define radeon_opt_set_context_reg4(cmdbuf, reg, reg_enum, v1, v2, v3, v4)                                             \
   do {                                                                                                                \
      struct radv_cmd_buffer *__cmdbuf = (cmdbuf);                                                                     \
      struct radv_tracked_regs *__tracked_regs = &__cmdbuf->tracked_regs;                                              \
      const uint32_t __v1 = (v1), __v2 = (v2), __v3 = (v3), __v4 = (v4);                                               \
      if (!BITSET_TEST_RANGE_INSIDE_WORD(__tracked_regs->reg_saved_mask, (reg_enum), (reg_enum) + 3, 0xf) ||           \
          __tracked_regs->reg_value[(reg_enum)] != __v1 || __tracked_regs->reg_value[(reg_enum) + 1] != __v2 ||        \
          __tracked_regs->reg_value[(reg_enum) + 2] != __v3 || __tracked_regs->reg_value[(reg_enum) + 3] != __v4) {    \
         radeon_set_context_reg_seq(reg, 4);                                                                           \
         radeon_emit(__v1);                                                                                            \
         radeon_emit(__v2);                                                                                            \
         radeon_emit(__v3);                                                                                            \
         radeon_emit(__v4);                                                                                            \
         BITSET_SET_RANGE_INSIDE_WORD(__tracked_regs->reg_saved_mask, (reg_enum), (reg_enum) + 3);                     \
         __tracked_regs->reg_value[(reg_enum)] = __v1;                                                                 \
         __tracked_regs->reg_value[(reg_enum) + 1] = __v2;                                                             \
         __tracked_regs->reg_value[(reg_enum) + 2] = __v3;                                                             \
         __tracked_regs->reg_value[(reg_enum) + 3] = __v4;                                                             \
         cmdbuf->state.context_roll_without_scissor_emitted = true;                                                    \
      }                                                                                                                \
   } while (0)

#define radeon_opt_set_context_regn(cmdbuf, reg, values, saved_values, num)                                            \
   do {                                                                                                                \
      struct radv_cmd_buffer *__cmdbuf = (cmdbuf);                                                                     \
      if (memcmp(values, saved_values, sizeof(uint32_t) * (num))) {                                                    \
         radeon_set_context_reg_seq(reg, num);                                                                         \
         radeon_emit_array(values, num);                                                                               \
         memcpy(saved_values, values, sizeof(uint32_t) * (num));                                                       \
         __cmdbuf->state.context_roll_without_scissor_emitted = true;                                                  \
      }                                                                                                                \
   } while (0)

/* Packet building helpers for SH registers. */
#define radeon_set_sh_reg_seq(reg, num) __radeon_set_reg_seq(reg, num, 0, SI_SH, PKT3_SET_SH_REG, 0)

#define radeon_set_sh_reg(reg, value) __radeon_set_reg(reg, 0, value, SI_SH, PKT3_SET_SH_REG)

#define radeon_set_sh_reg_idx(info, reg, idx, value)                                                                   \
   do {                                                                                                                \
      assert((idx));                                                                                                   \
      unsigned __opcode = PKT3_SET_SH_REG_INDEX;                                                                       \
      if ((info)->gfx_level < GFX10)                                                                                   \
         __opcode = PKT3_SET_SH_REG;                                                                                   \
      __radeon_set_reg(reg, idx, value, SI_SH, __opcode);                                                              \
   } while (0)

/* Packet building helpers for UCONFIG registers. */
#define radeon_set_uconfig_reg_seq(reg, num) __radeon_set_reg_seq(reg, num, 0, CIK_UCONFIG, PKT3_SET_UCONFIG_REG, 0)

#define radeon_set_uconfig_reg(reg, value) __radeon_set_reg(reg, 0, value, CIK_UCONFIG, PKT3_SET_UCONFIG_REG)

#define radeon_set_uconfig_reg_idx(info, reg, idx, value)                                                              \
   do {                                                                                                                \
      assert((idx));                                                                                                   \
      unsigned __opcode = PKT3_SET_UCONFIG_REG_INDEX;                                                                  \
      if ((info)->gfx_level < GFX9 || ((info)->gfx_level == GFX9 && (info)->me_fw_version < 26))                       \
         __opcode = PKT3_SET_UCONFIG_REG;                                                                              \
      __radeon_set_reg(reg, idx, value, CIK_UCONFIG, __opcode);                                                        \
   } while (0)

/*
 * On GFX10, there is a bug with the ME implementation of its content addressable memory (CAM),
 * that means that it can skip register writes due to not taking correctly into account the
 * fields from the GRBM_GFX_INDEX. With this bit we can force the write.
 */
#define radeon_set_uconfig_perfctr_reg_seq(gfx_level, ring, reg, num)                                                  \
   do {                                                                                                                \
      const bool __filter_cam_workaround = (gfx_level) >= GFX10 && (ring) == AMD_IP_GFX;                               \
      __radeon_set_reg_seq(reg, num, 0, CIK_UCONFIG, PKT3_SET_UCONFIG_REG, __filter_cam_workaround);                   \
   } while (0)

#define radeon_set_uconfig_perfctr_reg(gfx_level, ring, reg, value)                                                    \
   do {                                                                                                                \
      radeon_set_uconfig_perfctr_reg_seq(gfx_level, ring, reg, 1);                                                     \
      radeon_emit(value);                                                                                              \
   } while (0)

#define radeon_set_privileged_config_reg(reg, value)                                                                   \
   do {                                                                                                                \
      assert((reg) < CIK_UCONFIG_REG_OFFSET);                                                                          \
      radeon_emit(PKT3(PKT3_COPY_DATA, 4, 0));                                                                         \
      radeon_emit(COPY_DATA_SRC_SEL(COPY_DATA_IMM) | COPY_DATA_DST_SEL(COPY_DATA_PERF));                               \
      radeon_emit(value);                                                                                              \
      radeon_emit(0); /* unused */                                                                                     \
      radeon_emit((reg) >> 2);                                                                                         \
      radeon_emit(0); /* unused */                                                                                     \
   } while (0)

#define radeon_event_write_predicate(event_type, predicate)                                                            \
   do {                                                                                                                \
      unsigned __event_type = (event_type);                                                                            \
      radeon_emit(PKT3(PKT3_EVENT_WRITE, 0, predicate));                                                               \
      radeon_emit(EVENT_TYPE(__event_type) | EVENT_INDEX(__event_type == V_028A90_VS_PARTIAL_FLUSH ||                  \
                                                               __event_type == V_028A90_PS_PARTIAL_FLUSH ||            \
                                                               __event_type == V_028A90_CS_PARTIAL_FLUSH               \
                                                            ? 4                                                        \
                                                            : 0));                                                     \
   } while (0)

#define radeon_event_write(event_type) radeon_event_write_predicate(event_type, false)

#define radeon_emit_32bit_pointer(sh_offset, va, info)                                                                 \
   do {                                                                                                                \
      assert((va) == 0 || ((va) >> 32) == (info)->address32_hi);                                                       \
      radeon_set_sh_reg(sh_offset, va);                                                                                \
   } while (0)

#define radeon_emit_64bit_pointer(sh_offset, va)                                                                       \
   do {                                                                                                                \
      radeon_set_sh_reg_seq(sh_offset, 2);                                                                             \
      radeon_emit(va);                                                                                                 \
      radeon_emit(va >> 32);                                                                                           \
   } while (0)

/* GFX12 generic packet building helpers for PAIRS packets. Don't use these directly. */

/* Reserved 1 DWORD to emit the packet header when the sequence ends. */
#define __gfx12_begin_regs(header) uint32_t header = __cs_num++

/* Set a register unconditionally. */
#define __gfx12_set_reg(reg, value, base_offset)                                                                       \
   do {                                                                                                                \
      radeon_emit(((reg) - (base_offset)) >> 2);                                                                       \
      radeon_emit(value);                                                                                              \
   } while (0)

/* Set 1 context register optimally. */
#define __gfx12_opt_set_reg(cmdbuf, reg, reg_enum, value, base_offset)                                                 \
   do {                                                                                                                \
      struct radv_tracked_regs *__tracked_regs = &(cmdbuf)->tracked_regs;                                              \
      const uint32_t __value = (value);                                                                                \
      if (!BITSET_TEST(__tracked_regs->reg_saved_mask, (reg_enum)) ||                                                  \
          __tracked_regs->reg_value[(reg_enum)] != __value) {                                                          \
         __gfx12_set_reg((reg), __value, base_offset);                                                                 \
         BITSET_SET(__tracked_regs->reg_saved_mask, (reg_enum));                                                       \
         __tracked_regs->reg_value[(reg_enum)] = __value;                                                              \
      }                                                                                                                \
   } while (0)

/* Set 2 context registers optimally. */
#define __gfx12_opt_set_reg2(cmdbuf, reg, reg_enum, v1, v2, base_offset)                                               \
   do {                                                                                                                \
      struct radv_tracked_regs *__tracked_regs = &(cmdbuf)->tracked_regs;                                              \
      const uint32_t __v1 = (v1), __v2 = (v2);                                                                         \
      if (!BITSET_TEST_RANGE_INSIDE_WORD(__tracked_regs->reg_saved_mask, (reg_enum), (reg_enum) + 1, 0x3) ||           \
          __tracked_regs->reg_value[(reg_enum)] != __v1 || __tracked_regs->reg_value[(reg_enum) + 1] != __v2) {        \
         __gfx12_set_reg((reg), __v1, base_offset);                                                                    \
         __gfx12_set_reg((reg) + 4, __v2, base_offset);                                                                \
         BITSET_SET_RANGE_INSIDE_WORD(__tracked_regs->reg_saved_mask, (reg_enum), (reg_enum) + 1);                     \
         __tracked_regs->reg_value[(reg_enum)] = __v1;                                                                 \
         __tracked_regs->reg_value[(reg_enum) + 1] = __v2;                                                             \
      }                                                                                                                \
   } while (0)

/* End the sequence and emit the packet header. */
#define __gfx12_end_regs(header, packet)                                                                               \
   do {                                                                                                                \
      if ((header) + 1 == __cs_num) {                                                                                  \
         __cs_num--; /* no registers have been set, back off */                                                        \
      } else {                                                                                                         \
         unsigned __dw_count = __cs_num - (header) - 2;                                                                \
         __cs_buf[(header)] = PKT3((packet), __dw_count, 0) | PKT3_RESET_FILTER_CAM_S(1);                              \
      }                                                                                                                \
   } while (0)

/* GFX12 generic packet building helpers for buffered registers. Don't use these directly. */
#define __gfx12_push_reg(cmdbuf, reg, value, base_offset)                                                              \
   do {                                                                                                                \
      struct radv_cmd_buffer *__cmdbuf = (cmdbuf);                                                                     \
      unsigned __i = __cmdbuf->num_buffered_sh_regs++;                                                                 \
      assert(__i < ARRAY_SIZE(__cmdbuf->gfx12.buffered_sh_regs));                                                      \
      __cmdbuf->gfx12.buffered_sh_regs[__i].reg_offset = ((reg) - (base_offset)) >> 2;                                 \
      __cmdbuf->gfx12.buffered_sh_regs[__i].reg_value = value;                                                         \
   } while (0)

/* GFX12 packet building helpers for PAIRS packets. */
#define gfx12_begin_context_regs() __gfx12_begin_regs(__cs_context_reg_header)

#define gfx12_set_context_reg(reg, value) __gfx12_set_reg(reg, value, SI_CONTEXT_REG_OFFSET)

#define gfx12_opt_set_context_reg(cmdbuf, reg, reg_enum, value)                                                        \
   __gfx12_opt_set_reg(cmdbuf, reg, reg_enum, value, SI_CONTEXT_REG_OFFSET)

#define gfx12_opt_set_context_reg2(cmdbuf, reg, reg_enum, v1, v2)                                                      \
   __gfx12_opt_set_reg2(cmdbuf, reg, reg_enum, v1, v2, SI_CONTEXT_REG_OFFSET)

#define gfx12_end_context_regs() __gfx12_end_regs(__cs_context_reg_header, PKT3_SET_CONTEXT_REG_PAIRS)

/* GFX12 packet building helpers for buffered registers. */
#define gfx12_push_sh_reg(cmdbuf, reg, value) __gfx12_push_reg(cmdbuf, reg, value, SI_SH_REG_OFFSET)

#define gfx12_push_32bit_pointer(cmdbuf, sh_offset, va, info)                                                          \
   do {                                                                                                                \
      assert((va) == 0 || ((va) >> 32) == (info)->address32_hi);                                                       \
      gfx12_push_sh_reg(cmdbuf, sh_offset, va);                                                                        \
   } while (0)

#define gfx12_emit_buffered_sh_regs(num_regs, regs)                                                                    \
   do {                                                                                                                \
      unsigned __reg_count = *(num_regs);                                                                              \
      if (__reg_count) {                                                                                               \
         radeon_emit(PKT3(PKT3_SET_SH_REG_PAIRS, __reg_count * 2 - 1, 0) | PKT3_RESET_FILTER_CAM_S(1));                \
         radeon_emit_array(regs, __reg_count * 2);                                                                     \
         *(num_regs) = 0;                                                                                              \
      }                                                                                                                \
   } while (0)

ALWAYS_INLINE static void
radv_cp_wait_mem(struct radeon_cmdbuf *cs, const enum radv_queue_family qf, const uint32_t op, const uint64_t va,
                 const uint32_t ref, const uint32_t mask)
{
   assert(op == WAIT_REG_MEM_EQUAL || op == WAIT_REG_MEM_NOT_EQUAL || op == WAIT_REG_MEM_GREATER_OR_EQUAL);

   if (qf == RADV_QUEUE_GENERAL || qf == RADV_QUEUE_COMPUTE) {
      radeon_begin(cs);
      radeon_emit(PKT3(PKT3_WAIT_REG_MEM, 5, false));
      radeon_emit(op | WAIT_REG_MEM_MEM_SPACE(1));
      radeon_emit(va);
      radeon_emit(va >> 32);
      radeon_emit(ref);  /* reference value */
      radeon_emit(mask); /* mask */
      radeon_emit(4);    /* poll interval */
      radeon_end();
   } else if (qf == RADV_QUEUE_TRANSFER) {
      radv_sdma_emit_wait_mem(cs, op, va, ref, mask);
   } else {
      unreachable("unsupported queue family");
   }
}

ALWAYS_INLINE static unsigned
radv_cs_write_data_head(const struct radv_device *device, struct radeon_cmdbuf *cs, const enum radv_queue_family qf,
                        const unsigned engine_sel, const uint64_t va, const unsigned count, const bool predicating)
{
   /* Return the correct cdw at the end of the packet so the caller can assert it. */
   const unsigned cdw_end = radeon_check_space(device->ws, cs, 4 + count);

   if (qf == RADV_QUEUE_GENERAL || qf == RADV_QUEUE_COMPUTE) {
      radeon_begin(cs);
      radeon_emit(PKT3(PKT3_WRITE_DATA, 2 + count, predicating));
      radeon_emit(S_370_DST_SEL(V_370_MEM) | S_370_WR_CONFIRM(1) | S_370_ENGINE_SEL(engine_sel));
      radeon_emit(va);
      radeon_emit(va >> 32);
      radeon_end();
   } else if (qf == RADV_QUEUE_TRANSFER) {
      radv_sdma_emit_write_data_head(cs, va, count);
   } else {
      unreachable("unsupported queue family");
   }

   return cdw_end;
}

ALWAYS_INLINE static void
radv_cs_write_data(const struct radv_device *device, struct radeon_cmdbuf *cs, const enum radv_queue_family qf,
                   const unsigned engine_sel, const uint64_t va, const unsigned count, const uint32_t *dwords,
                   const bool predicating)
{
   ASSERTED const unsigned cdw_end = radv_cs_write_data_head(device, cs, qf, engine_sel, va, count, predicating);

   radeon_begin(cs);
   radeon_emit_array(dwords, count);
   radeon_end();
   assert(cs->cdw == cdw_end);
}

void radv_cs_emit_write_event_eop(struct radeon_cmdbuf *cs, enum amd_gfx_level gfx_level, enum radv_queue_family qf,
                                  unsigned event, unsigned event_flags, unsigned dst_sel, unsigned data_sel,
                                  uint64_t va, uint32_t new_fence, uint64_t gfx9_eop_bug_va);

void radv_cs_emit_cache_flush(struct radeon_winsys *ws, struct radeon_cmdbuf *cs, enum amd_gfx_level gfx_level,
                              uint32_t *flush_cnt, uint64_t flush_va, enum radv_queue_family qf,
                              enum radv_cmd_flush_bits flush_bits, enum rgp_flush_bits *sqtt_flush_bits,
                              uint64_t gfx9_eop_bug_va);

void radv_emit_cond_exec(const struct radv_device *device, struct radeon_cmdbuf *cs, uint64_t va, uint32_t count);

void radv_cs_write_data_imm(struct radeon_cmdbuf *cs, unsigned engine_sel, uint64_t va, uint32_t imm);

static inline void
radv_emit_pm4_commands(struct radeon_cmdbuf *cs, const struct ac_pm4_state *pm4)
{
   radeon_begin(cs);
   radeon_emit_array(pm4->pm4, pm4->ndw);
   radeon_end();
}

#endif /* RADV_CS_H */
