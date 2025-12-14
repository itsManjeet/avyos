/*
 * Copyright © 2020 Valve Corporation
 *
 * SPDX-License-Identifier: MIT
 */

#include "aco_builder.h"
#include "aco_ir.h"

#include <vector>

namespace aco {
namespace {

/* there can also be LDS and VALU clauses, but I don't see how those are interesting */
enum clause_type {
   clause_smem,
   clause_other,
   /* GFX10: */
   clause_vmem,
   clause_flat,
   /* GFX11: */
   clause_mimg_load,
   clause_mimg_store,
   clause_mimg_atomic,
   clause_mimg_sample,
   clause_vmem_load,
   clause_vmem_store,
   clause_vmem_atomic,
   clause_flat_load,
   clause_flat_store,
   clause_flat_atomic,
   clause_bvh,
};

void
emit_clause(Builder& bld, unsigned num_instrs, aco_ptr<Instruction>* instrs)
{
   if (num_instrs > 1)
      bld.sopp(aco_opcode::s_clause, num_instrs - 1);

   for (unsigned i = 0; i < num_instrs; i++)
      bld.insert(std::move(instrs[i]));
}

clause_type
get_type(Program* program, aco_ptr<Instruction>& instr)
{
   if (instr->isSMEM() && !instr->operands.empty())
      return clause_smem;

   if (program->gfx_level >= GFX11) {
      if (instr->isMIMG()) {
         uint8_t vmem_type = get_vmem_type(program->gfx_level, program->family, instr.get());
         switch (vmem_type) {
         case vmem_bvh: return clause_bvh;
         case vmem_sampler: return clause_mimg_sample;
         case vmem_nosampler:
            if (instr_info.is_atomic[(unsigned)instr->opcode])
               return clause_mimg_atomic;
            else if (instr->definitions.empty())
               return clause_mimg_store;
            else
               return clause_mimg_load;
         default: return clause_other;
         }
      } else if (instr->isMTBUF() || instr->isScratch() || instr->isMUBUF() || instr->isGlobal()) {
         if (instr_info.is_atomic[(unsigned)instr->opcode])
            return clause_vmem_atomic;
         else if (instr->definitions.empty())
            return clause_vmem_store;
         else
            return clause_vmem_load;
      } else if (instr->isFlat()) {
         if (instr_info.is_atomic[(unsigned)instr->opcode])
            return clause_flat_atomic;
         else if (instr->definitions.empty())
            return clause_flat_store;
         else
            return clause_flat_load;
      }
   } else {
      /* Exclude stores from clauses before GFX11. */
      if (instr->definitions.empty())
         return clause_other;

      if (instr->isVMEM() && !instr->operands.empty()) {
         if (program->gfx_level == GFX10 && instr->isMIMG() && get_mimg_nsa_dwords(instr.get()) > 0)
            return clause_other;
         else
            return clause_vmem;
      } else if (instr->isScratch() || instr->isGlobal()) {
         return clause_vmem;
      } else if (instr->isFlat()) {
         return clause_flat;
      }
   }
   return clause_other;
}

} /* end namespace */

void
form_hard_clauses(Program* program)
{
   /* The ISA documentation says 63 is the maximum for GFX11/12, but according to
    * LLVM there are HW bugs with more than 32 instructions.
    */
   const unsigned max_clause_length = program->gfx_level >= GFX11 ? 32 : 63;
   for (Block& block : program->blocks) {
      unsigned num_instrs = 0;
      aco_ptr<Instruction> current_instrs[63];
      clause_type current_type = clause_other;

      std::vector<aco_ptr<Instruction>> new_instructions;
      new_instructions.reserve(block.instructions.size());
      Builder bld(program, &new_instructions);

      for (unsigned i = 0; i < block.instructions.size(); i++) {
         aco_ptr<Instruction>& instr = block.instructions[i];

         clause_type type = get_type(program, instr);
         if (type != current_type || num_instrs == max_clause_length ||
             (num_instrs && !should_form_clause(current_instrs[0].get(), instr.get()))) {
            emit_clause(bld, num_instrs, current_instrs);
            num_instrs = 0;
            current_type = type;
         }

         if (type == clause_other) {
            bld.insert(std::move(instr));
            continue;
         }

         current_instrs[num_instrs++] = std::move(instr);
      }

      emit_clause(bld, num_instrs, current_instrs);

      block.instructions = std::move(new_instructions);
   }
}
} // namespace aco
