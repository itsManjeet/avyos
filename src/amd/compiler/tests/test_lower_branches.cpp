/*
 * Copyright © 2025 Valve Corporation
 *
 * SPDX-License-Identifier: MIT
 */
#include "helpers.h"

using namespace aco;

BEGIN_TEST(lower_branches.remove_block.single_linear_succ_multiple_logical_succs)
   if (!setup_cs(NULL, GFX12))
      return;

   while (program->blocks.size() < 7)
      program->create_and_insert_block();

   Block* if_block = &program->blocks[0];
   Block* then_logical = &program->blocks[1];
   Block* then_linear = &program->blocks[2];
   Block* invert = &program->blocks[3];
   Block* else_logical = &program->blocks[4];
   Block* else_linear = &program->blocks[5];
   Block* endif_block = &program->blocks[6];

   if_block->kind |= block_kind_branch;
   then_logical->kind |= block_kind_uniform;
   then_linear->kind |= block_kind_uniform;
   invert->kind |= block_kind_invert;
   else_logical->kind |= block_kind_uniform;
   else_linear->kind |= block_kind_uniform;
   endif_block->kind |= block_kind_uniform | block_kind_merge | block_kind_top_level;

   /* Set up logical CF */
   then_logical->logical_preds.push_back(if_block->index);
   else_logical->logical_preds.push_back(if_block->index);
   endif_block->logical_preds.push_back(then_logical->index);
   endif_block->logical_preds.push_back(else_logical->index);

   /* Set up linear CF */
   then_logical->linear_preds.push_back(if_block->index);
   then_linear->linear_preds.push_back(if_block->index);
   invert->linear_preds.push_back(then_logical->index);
   invert->linear_preds.push_back(then_linear->index);
   else_logical->linear_preds.push_back(invert->index);
   else_linear->linear_preds.push_back(invert->index);
   endif_block->linear_preds.push_back(else_logical->index);
   endif_block->linear_preds.push_back(else_linear->index);

   /* BB0 has a single linear successor but multiple logical successors. try_remove_simple_block()
    * should skip this.
    */
   //>> ACO shader stage: SW (CS), HW (COMPUTE_SHADER)
   //! BB1
   //! /* logical preds: BB0, / linear preds: BB0, / kind: uniform, */
   //!    s1: %0:s[0] = s_mov_b32 0
   //! BB6
   //! /* logical preds: BB1, BB0, / linear preds: BB1, / kind: uniform, top-level, merge, */
   //!    s_endpgm
   bld.reset(if_block);
   bld.sop1(aco_opcode::s_mov_b64, Definition(exec, s2), Operand::c64(1));
   bld.branch(aco_opcode::p_cbranch_z, Operand(exec, s2), then_linear->index, then_logical->index)
      .instr->branch()
      .never_taken = true;

   bld.reset(then_logical);
   bld.sop1(aco_opcode::s_mov_b32, Definition(PhysReg(0), s1), Operand::c32(0));
   bld.branch(aco_opcode::p_branch, invert->index);

   bld.reset(then_linear);
   bld.branch(aco_opcode::p_branch, invert->index);

   bld.reset(invert);
   bld.sop2(aco_opcode::s_andn2_b64, Definition(exec, s2), Definition(scc, s1), Operand::c64(-1),
            Operand(exec, s2));
   bld.branch(aco_opcode::p_cbranch_z, Operand(exec, s2), else_linear->index, else_logical->index);

   bld.reset(else_logical);
   bld.branch(aco_opcode::p_branch, endif_block->index);

   bld.reset(else_linear);
   bld.branch(aco_opcode::p_branch, endif_block->index);

   bld.reset(endif_block);
   bld.sop1(aco_opcode::s_mov_b64, Definition(exec, s2), Operand::c64(-1));

   finish_lower_branches_test();
END_TEST

BEGIN_TEST(lower_branches.remove_block.update_preds_on_partial_fail)
   if (!setup_cs(NULL, GFX12))
      return;

   while (program->blocks.size() < 7)
      program->create_and_insert_block();

   //>> BB0
   //! /* logical preds: / linear preds: / kind: top-level, */
   //!    s_cbranch_scc0 block:BB5
   bld.reset(&program->blocks[0]);
   bld.branch(aco_opcode::p_cbranch_nz, Operand(scc, s1), 2, 1);
   program->blocks[1].linear_preds.push_back(0);
   program->blocks[2].linear_preds.push_back(0);

   bld.reset(&program->blocks[1]);
   bld.branch(aco_opcode::p_branch, 3);
   program->blocks[3].linear_preds.push_back(1);

   //! BB2
   //! /* logical preds: / linear preds: BB0, / kind: */
   //!    s_cbranch_scc1 block:BB6
   bld.reset(&program->blocks[2]);
   bld.branch(aco_opcode::p_cbranch_nz, Operand(scc, s1), 6, 3);
   program->blocks[3].linear_preds.push_back(2);
   program->blocks[6].linear_preds.push_back(2);

   /* BB3 has BB1 and BB2 as predecessors. We can replace BB1's jump with one to BB5, but not BB2's
    * because we can't fallthrough from BB2 to BB5. If we skip removing a predecessor from BB3, we
    * should still update BB3's linear predecessor vector. */
   //! BB3
   //! /* logical preds: / linear preds: BB2, / kind: */
   //!    s_branch block:BB5
   bld.reset(&program->blocks[3]);
   bld.branch(aco_opcode::p_branch, 5);
   program->blocks[5].linear_preds.push_back(3);

   //! BB4
   //! /* logical preds: / linear preds: / kind: uniform, */
   //!    s_endpgm
   //! BB5
   //! /* logical preds: / linear preds: BB3, BB0, / kind: uniform, */
   //!    s_endpgm
   //! BB6
   //! /* logical preds: / linear preds: BB2, / kind: uniform, */
   //!    s_endpgm

   finish_lower_branches_test();
END_TEST
