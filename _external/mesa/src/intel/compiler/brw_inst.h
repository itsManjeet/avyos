/* -*- c++ -*- */
/*
 * Copyright © 2010-2016 Intel Corporation
 *
 * Permission is hereby granted, free of charge, to any person obtaining a
 * copy of this software and associated documentation files (the "Software"),
 * to deal in the Software without restriction, including without limitation
 * the rights to use, copy, modify, merge, publish, distribute, sublicense,
 * and/or sell copies of the Software, and to permit persons to whom the
 * Software is furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice (including the next
 * paragraph) shall be included in all copies or substantial portions of the
 * Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL
 * THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
 * FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS
 * IN THE SOFTWARE.
 */

#pragma once

#include <assert.h>
#include "brw_reg.h"
#include "compiler/glsl/list.h"

#define MAX_SAMPLER_MESSAGE_SIZE 11

/* The sampler can return a vec5 when sampling with sparse residency. In
 * SIMD32, each component takes up 4 GRFs, so we need to allow up to size-20
 * VGRFs to hold the result.
 */
#define MAX_VGRF_SIZE(devinfo) ((devinfo)->ver >= 20 ? 40 : 20)

struct bblock_t;

struct brw_inst : public exec_node {
private:
   brw_inst &operator=(const brw_inst &);

   void init(enum opcode opcode, uint8_t exec_width, const brw_reg &dst,
             const brw_reg *src, unsigned sources);

public:
   DECLARE_RALLOC_CXX_OPERATORS(brw_inst)

   brw_inst();
   brw_inst(enum opcode opcode, uint8_t exec_size);
   brw_inst(enum opcode opcode, uint8_t exec_size, const brw_reg &dst);
   brw_inst(enum opcode opcode, uint8_t exec_size, const brw_reg &dst,
           const brw_reg &src0);
   brw_inst(enum opcode opcode, uint8_t exec_size, const brw_reg &dst,
           const brw_reg &src0, const brw_reg &src1);
   brw_inst(enum opcode opcode, uint8_t exec_size, const brw_reg &dst,
           const brw_reg &src0, const brw_reg &src1, const brw_reg &src2);
   brw_inst(enum opcode opcode, uint8_t exec_size, const brw_reg &dst,
           const brw_reg src[], unsigned sources);
   brw_inst(const brw_inst &that);
   ~brw_inst();

   void resize_sources(uint8_t num_sources);

   bool is_send_from_grf() const;
   bool is_payload(unsigned arg) const;
   bool is_partial_write(unsigned grf_size = REG_SIZE) const;
   unsigned components_read(unsigned i) const;
   unsigned size_read(const struct intel_device_info *devinfo, int arg) const;
   bool can_do_source_mods(const struct intel_device_info *devinfo) const;
   bool can_do_cmod() const;
   bool can_change_types() const;
   bool has_source_and_destination_hazard() const;

   bool is_3src(const struct brw_compiler *compiler) const;
   bool is_math() const;
   bool is_control_flow_begin() const;
   bool is_control_flow_end() const;
   bool is_control_flow() const;
   bool is_commutative() const;
   bool is_raw_move() const;
   bool can_do_saturate() const;
   bool reads_accumulator_implicitly() const;
   bool writes_accumulator_implicitly(const struct intel_device_info *devinfo) const;

   /**
    * Instructions that use indirect addressing have additional register
    * regioning restrictions.
    */
   bool uses_indirect_addressing() const;

   void remove();
   void insert_before(bblock_t *block, brw_inst *inst);

   /**
    * True if the instruction has side effects other than writing to
    * its destination registers.  You are expected not to reorder or
    * optimize these out unless you know what you are doing.
    */
   bool has_side_effects() const;

   /**
    * True if the instruction might be affected by side effects of other
    * instructions.
    */
   bool is_volatile() const;

   /**
    * Return whether \p arg is a control source of a virtual instruction which
    * shouldn't contribute to the execution type and usual regioning
    * restriction calculations of arithmetic instructions.
    */
   bool is_control_source(unsigned arg) const;

   /**
    * Return the subset of flag registers read by the instruction as a bitset
    * with byte granularity.
    */
   unsigned flags_read(const intel_device_info *devinfo) const;

   /**
    * Return the subset of flag registers updated by the instruction (either
    * partially or fully) as a bitset with byte granularity.
    */
   unsigned flags_written(const intel_device_info *devinfo) const;

   /**
    * Return true if this instruction is a sampler message gathering residency
    * data.
    */
   bool has_sampler_residency() const;

   /**
    * Return true if this instruction is using the address register
    * implicitly.
    */
   bool uses_address_register_implicitly() const;

   uint8_t sources; /**< Number of brw_reg sources. */

   /**
    * Execution size of the instruction.  This is used by the generator to
    * generate the correct binary for the given instruction.  Current valid
    * values are 1, 4, 8, 16, 32.
    */
   uint8_t exec_size;

   /**
    * Channel group from the hardware execution and predication mask that
    * should be applied to the instruction.  The subset of channel enable
    * signals (calculated from the EU control flow and predication state)
    * given by [group, group + exec_size) will be used to mask GRF writes and
    * any other side effects of the instruction.
    */
   uint8_t group;

   uint8_t mlen; /**< SEND message length */
   uint8_t ex_mlen; /**< SENDS extended message length */
   uint8_t sfid; /**< SFID for SEND instructions */
   /** The number of hardware registers used for a message header. */
   uint8_t header_size;
   uint32_t desc; /**< SEND[S] message descriptor immediate */
   uint32_t ex_desc; /**< SEND[S] extended message descriptor immediate */

   uint32_t offset; /**< spill/unspill offset or texture offset bitfield */
   unsigned size_written; /**< Data written to the destination register in bytes. */

   enum opcode opcode; /* BRW_OPCODE_* or FS_OPCODE_* */
   enum brw_conditional_mod conditional_mod; /**< BRW_CONDITIONAL_* */
   enum brw_predicate predicate;

   tgl_swsb sched; /**< Scheduling info. */

   union {
      struct {
         /* Chooses which flag subregister (f0.0 to f3.1) is used for
          * conditional mod and predication.
          */
         unsigned flag_subreg:3;

         /**
          * Systolic depth used by DPAS instruction.
          */
         unsigned sdepth:4;

         /**
          * Repeat count used by DPAS instruction.
          */
         unsigned rcount:4;

         unsigned pad:4;

         bool predicate_inverse:1;
         bool writes_accumulator:1; /**< instruction implicitly writes accumulator */
         bool force_writemask_all:1;
         bool no_dd_clear:1;
         bool no_dd_check:1;
         bool saturate:1;
         bool check_tdr:1; /**< Only valid for SEND; turns it into a SENDC */
         bool send_has_side_effects:1; /**< Only valid for SHADER_OPCODE_SEND */
         bool send_is_volatile:1; /**< Only valid for SHADER_OPCODE_SEND */
         bool send_ex_bso:1; /**< Only for SHADER_OPCODE_SEND, use extended
                              *   bindless surface offset (26bits instead of
                              *   20bits)
                              */
         /**
          * Only for SHADER_OPCODE_SEND, @offset field contains an immediate
          * part of the extended descriptor that must be encoded in the
          * instruction.
          */
         bool send_ex_desc_imm:1;
         /**
          * The predication mask applied to this instruction is guaranteed to
          * be uniform and a superset of the execution mask of the present block.
          * No currently enabled channel will be disabled by the predicate.
          */
         bool predicate_trivial:1;
         bool eot:1;
         bool keep_payload_trailing_zeros:1;
         /**
          * Whether the parameters of the SEND instructions are build with
          * NoMask (for A32 messages this covers only the surface handle, for
          * A64 messages this covers the load address).
          */
         bool has_no_mask_send_params:1;
      };
      uint32_t bits;
   };

   brw_reg dst;
   brw_reg *src;
   brw_reg builtin_src[4];

#ifndef NDEBUG
   /** @{
    * Annotation for the generated IR.
    */
   const char *annotation;
   /** @} */
#endif

   bblock_t *block;
};

/**
 * Make the execution of \p inst dependent on the evaluation of a possibly
 * inverted predicate.
 */
static inline brw_inst *
set_predicate_inv(enum brw_predicate pred, bool inverse,
                  brw_inst *inst)
{
   inst->predicate = pred;
   inst->predicate_inverse = inverse;
   return inst;
}

/**
 * Make the execution of \p inst dependent on the evaluation of a predicate.
 */
static inline brw_inst *
set_predicate(enum brw_predicate pred, brw_inst *inst)
{
   return set_predicate_inv(pred, false, inst);
}

/**
 * Write the result of evaluating the condition given by \p mod to a flag
 * register.
 */
static inline brw_inst *
set_condmod(enum brw_conditional_mod mod, brw_inst *inst)
{
   inst->conditional_mod = mod;
   return inst;
}

/**
 * Clamp the result of \p inst to the saturation range of its destination
 * datatype.
 */
static inline brw_inst *
set_saturate(bool saturate, brw_inst *inst)
{
   inst->saturate = saturate;
   return inst;
}

/**
 * Return the number of dataflow registers written by the instruction (either
 * fully or partially) counted from 'floor(reg_offset(inst->dst) /
 * register_size)'.  The somewhat arbitrary register size unit is 4B for the
 * UNIFORM and IMM files and 32B for all other files.
 */
inline unsigned
regs_written(const brw_inst *inst)
{
   assert(inst->dst.file != UNIFORM && inst->dst.file != IMM);
   return DIV_ROUND_UP(reg_offset(inst->dst) % REG_SIZE +
                       inst->size_written -
                       MIN2(inst->size_written, reg_padding(inst->dst)),
                       REG_SIZE);
}

/**
 * Return the number of dataflow registers read by the instruction (either
 * fully or partially) counted from 'floor(reg_offset(inst->src[i]) /
 * register_size)'.  The somewhat arbitrary register size unit is 4B for the
 * UNIFORM files and 32B for all other files.
 */
inline unsigned
regs_read(const struct intel_device_info *devinfo, const brw_inst *inst, unsigned i)
{
   if (inst->src[i].file == IMM)
      return 1;

   const unsigned reg_size = inst->src[i].file == UNIFORM ? 4 : REG_SIZE;
   return DIV_ROUND_UP(reg_offset(inst->src[i]) % reg_size +
                       inst->size_read(devinfo, i) -
                       MIN2(inst->size_read(devinfo, i), reg_padding(inst->src[i])),
                       reg_size);
}

enum brw_reg_type get_exec_type(const brw_inst *inst);

static inline unsigned
get_exec_type_size(const brw_inst *inst)
{
   return brw_type_size_bytes(get_exec_type(inst));
}

static inline bool
is_send(const brw_inst *inst)
{
   return inst->mlen || inst->is_send_from_grf();
}

/**
 * Return whether the instruction isn't an ALU instruction and cannot be
 * assumed to complete in-order.
 */
static inline bool
is_unordered(const intel_device_info *devinfo, const brw_inst *inst)
{
   return is_send(inst) || (devinfo->ver < 20 && inst->is_math()) ||
          inst->opcode == BRW_OPCODE_DPAS ||
          (devinfo->has_64bit_float_via_math_pipe &&
           (get_exec_type(inst) == BRW_TYPE_DF ||
            inst->dst.type == BRW_TYPE_DF));
}

static inline bool
has_bfloat_operands(const brw_inst *inst)
{
   if (brw_type_is_bfloat(inst->dst.type))
      return true;

   for (int i = 0; i < inst->sources; i++) {
      if (brw_type_is_bfloat(inst->src[i].type))
         return true;
   }

   return false;
}

bool has_dst_aligned_region_restriction(const intel_device_info *devinfo,
                                        const brw_inst *inst,
                                        brw_reg_type dst_type);

static inline bool
has_dst_aligned_region_restriction(const intel_device_info *devinfo,
                                   const brw_inst *inst)
{
   return has_dst_aligned_region_restriction(devinfo, inst, inst->dst.type);
}

bool has_subdword_integer_region_restriction(const intel_device_info *devinfo,
                                             const brw_inst *inst,
                                             const brw_reg *srcs, unsigned num_srcs);

static inline bool
has_subdword_integer_region_restriction(const intel_device_info *devinfo,
                                        const brw_inst *inst)
{
   return has_subdword_integer_region_restriction(devinfo, inst,
                                                  inst->src, inst->sources);
}

bool is_identity_payload(const struct intel_device_info *devinfo,
                         brw_reg_file file, const brw_inst *inst);

bool is_multi_copy_payload(const struct intel_device_info *devinfo,
                           const brw_inst *inst);

bool is_coalescing_payload(const struct brw_shader &s, const brw_inst *inst);

bool has_bank_conflict(const struct brw_isa_info *isa, const brw_inst *inst);

/* Return the subset of flag registers that an instruction could
 * potentially read or write based on the execution controls and flag
 * subregister number of the instruction.
 */
static inline unsigned
brw_flag_mask(const brw_inst *inst, unsigned width)
{
   assert(util_is_power_of_two_nonzero(width));
   const unsigned start = (inst->flag_subreg * 16 + inst->group) &
                          ~(width - 1);
   const unsigned end = start + ALIGN(inst->exec_size, width);
   return ((1 << DIV_ROUND_UP(end, 8)) - 1) & ~((1 << (start / 8)) - 1);
}

static inline unsigned
brw_bit_mask(unsigned n)
{
   return (n >= CHAR_BIT * sizeof(brw_bit_mask(n)) ? ~0u : (1u << n) - 1);
}

static inline unsigned
brw_flag_mask(const brw_reg &r, unsigned sz)
{
   if (r.file == ARF) {
      const unsigned start = (r.nr - BRW_ARF_FLAG) * 4 + r.subnr;
      const unsigned end = start + sz;
      return brw_bit_mask(end) & ~brw_bit_mask(start);
   } else {
      return 0;
   }
}
