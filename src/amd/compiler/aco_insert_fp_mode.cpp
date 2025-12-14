/*
 * Copyright © 2025 Valve Corporation
 *
 * SPDX-License-Identifier: MIT
 */

#include "aco_builder.h"
#include "aco_ir.h"

#include <vector>

namespace aco {

namespace {

enum mode_field : uint8_t {
   mode_round32 = 0,
   mode_round16_64,
   mode_denorm32,
   mode_denorm16_64,
   mode_fp16_ovfl,

   mode_field_count,
};

using mode_mask = uint8_t;
static_assert(mode_field_count <= sizeof(mode_mask) * 8, "larger mode_mask needed");

struct fp_mode_state {
   uint8_t fields[mode_field_count] = {};
   mode_mask dirty = 0; /* BITFIELD_BIT(enum mode_field) */

   fp_mode_state() = default;

   fp_mode_state(float_mode mode)
   {
      fields[mode_round32] = mode.round32;
      fields[mode_round16_64] = mode.round16_64;
      fields[mode_denorm32] = mode.denorm32;
      fields[mode_denorm16_64] = mode.denorm16_64;
      fields[mode_fp16_ovfl] = 0;
   }

   void join(const fp_mode_state& other)
   {
      dirty |= other.dirty;
      for (unsigned i = 0; i < mode_field_count; i++) {
         if (fields[i] != other.fields[i])
            dirty |= BITFIELD_BIT(i);
      }
   }

   bool require(mode_field field, uint8_t val)
   {
      if (fields[field] == val && !(dirty & BITFIELD_BIT(field)))
         return false;

      fields[field] = val;
      dirty |= BITFIELD_BIT(field);
      return true;
   }

   uint8_t round() const { return fields[mode_round32] | (fields[mode_round16_64] << 2); }

   uint8_t denorm() const { return fields[mode_denorm32] | (fields[mode_denorm16_64] << 2); }
};

struct fp_mode_ctx {
   std::vector<fp_mode_state> block_states;
   Program* program;
};

void
emit_set_mode(Builder& bld, const fp_mode_state& state)
{
   bool set_round = state.dirty & (BITFIELD_BIT(mode_round32) | BITFIELD_BIT(mode_round16_64));
   bool set_denorm = state.dirty & (BITFIELD_BIT(mode_denorm32) | BITFIELD_BIT(mode_denorm16_64));
   bool set_fp16_ovfl = state.dirty & BITFIELD_BIT(mode_fp16_ovfl);

   if (bld.program->gfx_level >= GFX10) {
      if (set_round)
         bld.sopp(aco_opcode::s_round_mode, state.round());
      if (set_denorm)
         bld.sopp(aco_opcode::s_denorm_mode, state.denorm());
   } else if (set_round || set_denorm) {
      /* "((size - 1) << 11) | register" (MODE is encoded as register 1) */
      uint8_t val = state.round() | (state.denorm() << 4);
      bld.sopk(aco_opcode::s_setreg_imm32_b32, Operand::literal32(val), (7 << 11) | 1);
   }

   if (set_fp16_ovfl) {
      /* "((size - 1) << 11 | (offset << 6) | register" (MODE is encoded as register 1, we
       * want to set a single bit at offset 23)
       */
      bld.sopk(aco_opcode::s_setreg_imm32_b32, Operand::literal32(state.fields[mode_fp16_ovfl]),
               (0 << 11) | (23 << 6) | 1);
   }
}

mode_mask
vmem_default_needs(Instruction* instr)
{
   switch (instr->opcode) {
   case aco_opcode::buffer_atomic_fcmpswap:
   case aco_opcode::buffer_atomic_fmin:
   case aco_opcode::buffer_atomic_fmax:
   case aco_opcode::buffer_atomic_add_f32:
   case aco_opcode::flat_atomic_fcmpswap:
   case aco_opcode::flat_atomic_fmin:
   case aco_opcode::flat_atomic_fmax:
   case aco_opcode::flat_atomic_add_f32:
   case aco_opcode::global_atomic_fcmpswap:
   case aco_opcode::global_atomic_fmin:
   case aco_opcode::global_atomic_fmax:
   case aco_opcode::global_atomic_add_f32:
   case aco_opcode::image_atomic_fcmpswap:
   case aco_opcode::image_atomic_fmin:
   case aco_opcode::image_atomic_fmax:
   case aco_opcode::image_atomic_add_flt: return BITFIELD_BIT(mode_denorm32);
   case aco_opcode::buffer_atomic_fcmpswap_x2:
   case aco_opcode::buffer_atomic_fmin_x2:
   case aco_opcode::buffer_atomic_fmax_x2:
   case aco_opcode::buffer_atomic_pk_add_f16:
   case aco_opcode::buffer_atomic_pk_add_bf16:
   case aco_opcode::flat_atomic_fcmpswap_x2:
   case aco_opcode::flat_atomic_fmin_x2:
   case aco_opcode::flat_atomic_fmax_x2:
   case aco_opcode::flat_atomic_pk_add_f16:
   case aco_opcode::flat_atomic_pk_add_bf16:
   case aco_opcode::global_atomic_fcmpswap_x2:
   case aco_opcode::global_atomic_fmin_x2:
   case aco_opcode::global_atomic_fmax_x2:
   case aco_opcode::global_atomic_pk_add_f16:
   case aco_opcode::global_atomic_pk_add_bf16:
   case aco_opcode::image_atomic_pk_add_f16:
   case aco_opcode::image_atomic_pk_add_bf16: return BITFIELD_BIT(mode_denorm16_64);
   default: return 0;
   }
}

mode_mask
instr_default_needs(fp_mode_ctx* ctx, Block* block, Instruction* instr)
{
   if ((instr->isVMEM() || instr->isFlatLike()) && ctx->program->gfx_level < GFX12)
      return vmem_default_needs(instr);

   switch (instr->opcode) {
   case aco_opcode::s_branch:
   case aco_opcode::s_cbranch_scc0:
   case aco_opcode::s_cbranch_scc1:
   case aco_opcode::s_cbranch_vccz:
   case aco_opcode::s_cbranch_vccnz:
   case aco_opcode::s_cbranch_execz:
   case aco_opcode::s_cbranch_execnz:
      if (instr->salu().imm > block->index)
         return 0;
      FALLTHROUGH;
   case aco_opcode::s_swappc_b64:
   case aco_opcode::s_setpc_b64:
   case aco_opcode::s_call_b64:
      /* Restore defaults on loop back edges and calls. */
      return BITFIELD_MASK(mode_field_count);
   case aco_opcode::ds_cmpst_f32:
   case aco_opcode::ds_min_f32:
   case aco_opcode::ds_max_f32:
   case aco_opcode::ds_add_f32:
   case aco_opcode::ds_min_src2_f32:
   case aco_opcode::ds_max_src2_f32:
   case aco_opcode::ds_add_src2_f32:
   case aco_opcode::ds_cmpst_rtn_f32:
   case aco_opcode::ds_min_rtn_f32:
   case aco_opcode::ds_max_rtn_f32:
   case aco_opcode::ds_add_rtn_f32: return BITFIELD_BIT(mode_denorm32);
   case aco_opcode::ds_cmpst_f64:
   case aco_opcode::ds_min_f64:
   case aco_opcode::ds_max_f64:
   case aco_opcode::ds_min_src2_f64:
   case aco_opcode::ds_max_src2_f64:
   case aco_opcode::ds_cmpst_rtn_f64:
   case aco_opcode::ds_min_rtn_f64:
   case aco_opcode::ds_max_rtn_f64:
   case aco_opcode::ds_pk_add_f16:
   case aco_opcode::ds_pk_add_rtn_f16:
   case aco_opcode::ds_pk_add_bf16:
   case aco_opcode::ds_pk_add_rtn_bf16: return BITFIELD_BIT(mode_denorm16_64);
   case aco_opcode::v_cvt_pk_u8_f32: return BITFIELD_BIT(mode_round32);
   default: break;
   }

   if (!instr->isVALU() && !instr->isSALU() && !instr->isVINTRP())
      return 0;
   if (instr->definitions.empty())
      return 0;

   const aco_alu_opcode_info& info = instr_info.alu_opcode_infos[(int)instr->opcode];

   mode_mask res = 0;

   for (unsigned i = 0; i < info.num_operands; i++) {
      aco_type type = info.op_types[i];
      if (type.base_type != aco_base_type_float && type.base_type != aco_base_type_bfloat)
         continue;

      if (type.bit_size == 32)
         res |= BITFIELD_BIT(mode_denorm32);
      else if (type.bit_size >= 16)
         res |= BITFIELD_BIT(mode_denorm16_64);
   }

   aco_type type = info.def_types[0];
   if (type.base_type == aco_base_type_float || type.base_type == aco_base_type_bfloat) {
      if (type.bit_size == 32)
         res |= BITFIELD_BIT(mode_denorm32) | BITFIELD_BIT(mode_round32);
      else if (type.bit_size >= 16)
         res |= BITFIELD_BIT(mode_denorm16_64) | BITFIELD_BIT(mode_round16_64);

      if (type.bit_size <= 16)
         res |= BITFIELD_BIT(mode_fp16_ovfl);
   }

   if (instr->opcode == aco_opcode::v_fma_mixlo_f16 || instr->opcode == aco_opcode::v_fma_mixlo_f16)
      res |= BITFIELD_BIT(mode_round32);
   else if (instr->opcode == aco_opcode::v_fma_mix_f32 && instr->valu().opsel_hi)
      res |= BITFIELD_BIT(mode_denorm16_64);

   return res;
}

void
emit_set_mode_block(fp_mode_ctx* ctx, Block* block)
{
   Builder bld(ctx->program, block);
   fp_mode_state fp_state;
   const fp_mode_state default_state(block->fp_mode);

   if (block->index == 0) {
      bool inital_unknown = (ctx->program->info.merged_shader_compiled_separately &&
                             ctx->program->stage.sw == SWStage::GS) ||
                            (ctx->program->info.merged_shader_compiled_separately &&
                             ctx->program->stage.sw == SWStage::TCS);

      if (inital_unknown) {
         fp_state.dirty = BITFIELD_MASK(mode_field_count) & ~BITFIELD_BIT(mode_fp16_ovfl);
      } else {
         float_mode program_mode;
         program_mode.val = ctx->program->config->float_mode;
         fp_state = fp_mode_state(program_mode);
      }
   } else if (block->linear_preds.empty()) {
      fp_state = default_state;
   } else {
      assert(block->linear_preds[0] < block->index);
      fp_state = ctx->block_states[block->linear_preds[0]];
      for (unsigned i = 1; i < block->linear_preds.size(); i++) {
         unsigned pred = block->linear_preds[i];
         fp_mode_state other = pred < block->index
                                  ? ctx->block_states[pred]
                                  : fp_mode_state(ctx->program->blocks[pred].fp_mode);
         fp_state.join(other);
      }
   }

   /* If we don't know the value, set it to the default one next time. */
   u_foreach_bit (field, fp_state.dirty)
      fp_state.fields[field] = default_state.fields[field];

   for (std::vector<aco_ptr<Instruction>>::iterator it = block->instructions.begin();
        it < block->instructions.end(); ++it) {
      bool set_mode = false;

      Instruction* instr = it->get();

      if (instr->opcode == aco_opcode::p_v_cvt_f16_f32_rtne ||
          instr->opcode == aco_opcode::p_s_cvt_f16_f32_rtne) {
         set_mode |= fp_state.require(mode_round16_64, fp_round_ne);
         set_mode |= fp_state.require(mode_fp16_ovfl, default_state.fields[mode_fp16_ovfl]);
         set_mode |= fp_state.require(mode_denorm16_64, default_state.fields[mode_denorm16_64]);
         if (instr->opcode == aco_opcode::p_v_cvt_f16_f32_rtne)
            instr->opcode = aco_opcode::v_cvt_f16_f32;
         else
            instr->opcode = aco_opcode::s_cvt_f16_f32;
      } else if (instr->opcode == aco_opcode::p_v_cvt_pk_fp8_f32_ovfl) {
         set_mode |= fp_state.require(mode_fp16_ovfl, 1);
         instr->opcode = aco_opcode::v_cvt_pk_fp8_f32;
      } else {
         mode_mask default_needs = instr_default_needs(ctx, block, instr);
         u_foreach_bit (i, default_needs)
            set_mode |= fp_state.require((mode_field)i, default_state.fields[i]);
      }

      if (set_mode) {
         bld.reset(&block->instructions, it);
         emit_set_mode(bld, fp_state);
         fp_state.dirty = 0;
         /* Update the iterator if it was invalidated */
         it = bld.it;
      }
   }

   if (block->kind & block_kind_end_with_regs) {
      /* Restore default. */
      for (unsigned i = 0; i < mode_field_count; i++)
         fp_state.require((mode_field)i, default_state.fields[i]);
      if (fp_state.dirty) {
         bld.reset(block);
         emit_set_mode(bld, fp_state);
         fp_state.dirty = 0;
      }
   }

   ctx->block_states[block->index] = fp_state;
}

} // namespace

bool
instr_is_vmem_fp_atomic(Instruction* instr)
{
   return vmem_default_needs(instr) != 0;
}

void
insert_fp_mode(Program* program)
{
   fp_mode_ctx ctx;
   ctx.program = program;
   ctx.block_states.resize(program->blocks.size());

   for (Block& block : program->blocks)
      emit_set_mode_block(&ctx, &block);
}

} // namespace aco
