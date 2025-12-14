/*
 * Copyright © 2023 Bas Nieuwenhuizen
 * Copyright © 2024 Collabora, Ltd.
 * SPDX-License-Identifier: MIT
 */

#include "util/macros.h"
#include "glsl_types.h"
#include "nak_private.h"
#include "nir_builder.h"

static enum nak_cmat_type
get_nak_cmat_type_for_muladd(struct glsl_cmat_description a_desc,
                             struct glsl_cmat_description b_desc,
                             struct glsl_cmat_description c_desc)
{
   unsigned m = a_desc.rows;
   unsigned k = b_desc.rows;
   unsigned n = c_desc.cols;

   bool a_is_int8 = a_desc.element_type == GLSL_TYPE_INT8 ||
                    a_desc.element_type == GLSL_TYPE_UINT8;
   bool b_is_int8 = b_desc.element_type == GLSL_TYPE_INT8 ||
                    b_desc.element_type == GLSL_TYPE_UINT8;
   bool c_is_int32 = c_desc.element_type == GLSL_TYPE_INT ||
                     c_desc.element_type == GLSL_TYPE_UINT;

   if (m ==  8 && a_is_int8 &&
       n ==  8 && b_is_int8 &&
       k == 16 && c_is_int32)
      return NAK_CMAT_TYPE_M8N8K16_INT;

   if (m == 16 && a_is_int8 &&
       n ==  8 && b_is_int8 &&
       k == 16 && c_is_int32)
      return NAK_CMAT_TYPE_M16N8K16_INT;

   if (m == 16 && a_is_int8 &&
       n ==  8 && b_is_int8 &&
       k == 32 && c_is_int32)
      return NAK_CMAT_TYPE_M16N8K32_INT;

   if (m == 16 && a_is_int8 &&
       n == 16 && b_is_int8 &&
       k == 32 && c_is_int32)
      return NAK_CMAT_TYPE_M16N16K32_INT_SW;

   if (m == 16 && a_desc.element_type == GLSL_TYPE_FLOAT16 &&
       n ==  8 && b_desc.element_type == GLSL_TYPE_FLOAT16 &&
       k ==  8 && glsl_base_type_is_float(c_desc.element_type))
      return NAK_CMAT_TYPE_M16N8K8_FLOAT;

   if (m == 16 && a_desc.element_type == GLSL_TYPE_FLOAT16 &&
       n ==  8 && b_desc.element_type == GLSL_TYPE_FLOAT16 &&
       k == 16 && glsl_base_type_is_float(c_desc.element_type))
       return NAK_CMAT_TYPE_M16N8K16_FLOAT;

   if (m == 16 && a_desc.element_type == GLSL_TYPE_FLOAT16 &&
       n == 16 && b_desc.element_type == GLSL_TYPE_FLOAT16 &&
       k == 16 && glsl_base_type_is_float(c_desc.element_type))
      return NAK_CMAT_TYPE_M16N16K16_FLOAT_SW;

   unreachable("Unable to determine matrix muladd layout!");
}

enum nak_matrix_type_layout {
   NAK_MAT_16x32_INT8,
   NAK_MAT_16X16,
};

static enum nak_matrix_type_layout
determine_matrix_type(struct glsl_cmat_description desc)
{
   bool is_int8 = desc.element_type == GLSL_TYPE_INT8 ||
                  desc.element_type == GLSL_TYPE_UINT8;
   bool is_int8_a = is_int8 && desc.use == GLSL_CMAT_USE_A;
   bool is_int8_b = is_int8 && desc.use == GLSL_CMAT_USE_B;
   ASSERTED bool is_int32 = desc.element_type == GLSL_TYPE_INT ||
                            desc.element_type == GLSL_TYPE_UINT;
   ASSERTED bool is_float16 = desc.element_type == GLSL_TYPE_FLOAT16;
   ASSERTED bool is_float32 = desc.element_type == GLSL_TYPE_FLOAT;
   ASSERTED bool use_accum = desc.use == GLSL_CMAT_USE_ACCUMULATOR;

   /* This format doesn't exist on any hardware we are aware of so far and is
    * part of lowering
    */
   if (desc.rows == 32 && desc.cols == 16 && is_int8_b)
      return NAK_MAT_16x32_INT8;

   /* Even though this condition might be correct, we assert on all the
    * combination we actually verified on hardware.
    */
   if (is_int8_a || is_int8_b) {
      assert(
         (desc.rows ==  8 && desc.cols == 16 && is_int8_a) ||
         (desc.rows == 16 && desc.cols ==  8 && is_int8_b) ||
         (desc.rows == 16 && desc.cols == 16 && is_int8_a) ||
         (desc.rows == 16 && desc.cols == 32 && is_int8_a) ||
         (desc.rows == 32 && desc.cols ==  8 && is_int8_b)
      );
      return NAK_MAT_16x32_INT8;
   } else {
      assert(
         (desc.rows ==  8 && desc.cols ==  8 && is_float16 && !use_accum) ||
         (desc.rows == 16 && desc.cols ==  8 && is_float16              ) ||
         (desc.rows == 16 && desc.cols ==  8 && is_float32              ) ||
         (desc.rows == 16 && desc.cols == 16 && is_float16              ) ||
         (desc.rows == 16 && desc.cols == 16 && is_float32              ) ||
         (desc.rows ==  8 && desc.cols ==  8 && is_int32                ) ||
         (desc.rows == 16 && desc.cols ==  8 && is_int32                ) ||
         (desc.rows == 16 && desc.cols == 16 && is_int32                )
      );
      return NAK_MAT_16X16;
   }
}

static unsigned
get_cmat_size(struct glsl_cmat_description matrix_desc)
{
   return matrix_desc.cols * matrix_desc.rows;
}

static unsigned
get_cmat_length(struct glsl_cmat_description matrix_desc)
{
   return get_cmat_size(matrix_desc) / NAK_SUBGROUP_SIZE;
}

static nir_def *
load_cmat_deref(nir_builder *b, nir_deref_instr *src)
{
   struct glsl_cmat_description matrix_desc =
      *glsl_get_cmat_description(src->type);

   return nir_build_load_deref(
      b, get_cmat_length(matrix_desc),
      glsl_base_type_bit_size(matrix_desc.element_type), &src->def, 0);
}

static ALWAYS_INLINE nir_def *
load_cmat_src(nir_builder *b, nir_src src)
{
   return load_cmat_deref(b, nir_src_as_deref(src));
}

static ALWAYS_INLINE struct glsl_cmat_description
cmat_src_desc(nir_src src)
{
   nir_deref_instr *deref = nir_src_as_deref(src);
   return *glsl_get_cmat_description(deref->type);
}

static void
store_cmat_deref(nir_builder *b, nir_deref_instr *dst, nir_def *val)
{
   ASSERTED struct glsl_cmat_description matrix_desc =
      *glsl_get_cmat_description(dst->type);

   assert(val->bit_size == glsl_base_type_bit_size(matrix_desc.element_type));
   assert(val->num_components == get_cmat_length(matrix_desc));

   nir_store_deref(b, dst, val, ~0);
}

static ALWAYS_INLINE void
store_cmat_src(nir_builder *b, nir_src dst_src, nir_def *val)
{
   store_cmat_deref(b, nir_src_as_deref(dst_src), val);
}

static const struct glsl_type *
remap_matrix_type(struct hash_table *mapping, const struct glsl_type *orig)
{
   struct hash_entry *entry = _mesa_hash_table_search(mapping, orig);

   if (entry)
      return entry->data;

   const struct glsl_type *new_type = orig;

   if (glsl_type_is_cmat(orig)) {
      struct glsl_cmat_description matrix_desc =
         *glsl_get_cmat_description(orig);

      new_type = glsl_vector_type(matrix_desc.element_type,
                                  get_cmat_length(matrix_desc));
   } else if (glsl_type_is_array(orig)) {
      const struct glsl_type *elem_type = glsl_get_array_element(orig);
      const struct glsl_type *new_elem_type =
         remap_matrix_type(mapping, elem_type);

      if (elem_type != new_elem_type) {
         new_type = glsl_array_type(new_elem_type, glsl_get_length(orig),
                                    glsl_get_explicit_stride(orig));
      }
   } else if (glsl_type_is_struct(orig)) {
      unsigned i;
      for (i = 0; i < orig->length; i++) {
         const struct glsl_type *field_type = glsl_get_struct_field(orig, i);
         const struct glsl_type *new_field_type =
            remap_matrix_type(mapping, field_type);

         if (field_type != new_field_type) {
            break;
         }
      }

      /* If we found a cmat, remap the structure type */
      if (i < orig->length) {
         struct glsl_struct_field *fields =
            malloc(sizeof(struct glsl_struct_field) * orig->length);

         /* Copy everything that didn't change */
         memcpy(fields, orig->fields.structure,
                sizeof(struct glsl_struct_field) * i);

         /* Remap the rest */
         for (; i < orig->length; i++) {
            fields[i] = *glsl_get_struct_field_data(orig, i);
            fields[i].type = remap_matrix_type(mapping, fields[i].type);
         }

         new_type =
            glsl_struct_type(fields, orig->length, glsl_get_type_name(orig),
                             glsl_struct_type_is_packed(orig));

         free(fields);
      }
   }

   _mesa_hash_table_insert(mapping, orig, (void *)new_type);
   return new_type;
}

/* Computes the index in a linear matrix buffer a thread needs to load from in
 * order to execute an MMA on the Matrix.
 *
 * This is a generalized formula based on the Matrix layout descriptions from
 * the CUDA PTX instruction set documentation:
 * https://docs.nvidia.com/cuda/archive/12.8.1/parallel-thread-execution/index.html#matrix-multiply-accumulate-operation-using-mma-instruction
 */
static void
compute_mat(struct nir_builder *b, nir_def *lane_id,
            unsigned idx, nir_def **col, nir_def **row,
            struct glsl_cmat_description desc,
            unsigned scale)
{
   assert(idx < 8 * scale);

   nir_def *quad_id = nir_ushr_imm(b, lane_id, 2);
   nir_def *thread_id_in_quad = nir_iand_imm(b, lane_id, 0x3);

   unsigned row_bound = (desc.use == GLSL_CMAT_USE_B ? 4 : 2) * scale;
   unsigned col_bound = (desc.use == GLSL_CMAT_USE_B ? 2 : 4) * scale;

   scale = 1 << scale;
   *row = quad_id;
   if (idx & row_bound)
      *row = nir_iadd_imm(b, *row, 8);

   *col = nir_iadd_imm(b, nir_imul_imm(b, thread_id_in_quad, scale),
                          idx & (scale - 1));
   if (idx & col_bound)
      *col = nir_iadd_imm(b, *col, scale * 4);
}

static void
compute_mat_16x32_int8(struct nir_builder *b, nir_def *lane_id,
                       unsigned idx, nir_def **col, nir_def **row,
                       struct glsl_cmat_description desc)
{
   compute_mat(b, lane_id, idx, col, row, desc, 2);
}

static void
compute_mat_16x16(struct nir_builder *b, nir_def *lane_id,
                       unsigned idx, nir_def **col, nir_def **row,
                       struct glsl_cmat_description desc)
{
   compute_mat(b, lane_id, idx, col, row, desc, 1);
}

static void
compute_matrix_offsets(struct nir_builder *b, struct glsl_cmat_description desc,
                       enum glsl_matrix_layout layout, nir_def *lane_id,
                       unsigned idx, nir_def **col_offset, nir_def **row_offset)
{
   enum nak_matrix_type_layout cmat_type = determine_matrix_type(desc);
   switch (cmat_type) {
   case NAK_MAT_16x32_INT8:
      compute_mat_16x32_int8(b, lane_id, idx, col_offset, row_offset, desc);
      break;

   case NAK_MAT_16X16:
      compute_mat_16x16(b, lane_id, idx, col_offset, row_offset, desc);
      break;
   }

   /* The layout calculation code relies on col and row being swapped for B
    * row-major and non B col-major matrices.
    */
   if ((desc.use == GLSL_CMAT_USE_B && layout == GLSL_MATRIX_LAYOUT_ROW_MAJOR) ||
       (desc.use != GLSL_CMAT_USE_B && layout != GLSL_MATRIX_LAYOUT_ROW_MAJOR)) {
      nir_def *tmp = *col_offset;
      *col_offset = *row_offset;
      *row_offset = tmp;
   }
}

/* Returns the hw native Matrix muladd operation */
static enum nak_cmat_type
get_hw_nak_cmat_type(enum nak_cmat_type cmat_type, uint8_t sm)
{
   switch (cmat_type) {
   case NAK_CMAT_TYPE_M8N8K16_INT:
      return NAK_CMAT_TYPE_M8N8K16_INT;
   case NAK_CMAT_TYPE_M16N8K16_INT:
      return sm >= 80 ? NAK_CMAT_TYPE_M16N8K16_INT
                      : NAK_CMAT_TYPE_M8N8K16_INT; /* no lowering code yet */
   case NAK_CMAT_TYPE_M16N8K32_INT:
   case NAK_CMAT_TYPE_M16N16K32_INT_SW:
      /* On Turing we only have 8x8x16 */
      return sm >= 80 ? NAK_CMAT_TYPE_M16N8K32_INT
                      : NAK_CMAT_TYPE_M8N8K16_INT;
   case NAK_CMAT_TYPE_M16N8K8_FLOAT:
      return NAK_CMAT_TYPE_M16N8K8_FLOAT;
   case NAK_CMAT_TYPE_M16N8K16_FLOAT:
   case NAK_CMAT_TYPE_M16N16K16_FLOAT_SW:
      return NAK_CMAT_TYPE_M16N8K16_FLOAT;
   default:
      unreachable("Unknown Matrix muladd type.");
   }
}

static nir_def *
lower_cmat_muladd(nir_builder *b, nir_intrinsic_instr *intr, nir_def *cmat_a,
                  nir_def *cmat_b, nir_def *cmat_c,
                  struct glsl_cmat_description a_desc,
                  struct glsl_cmat_description b_desc,
                  struct glsl_cmat_description c_desc,
                  struct glsl_cmat_description d_desc, uint8_t sm)
{
   unsigned dst_length = get_cmat_length(d_desc);

   enum nak_cmat_type cmat_type =
      get_nak_cmat_type_for_muladd(a_desc, b_desc, c_desc);
   enum nak_cmat_type hw_cmat_type = get_hw_nak_cmat_type(cmat_type, sm);

   nir_cmat_signed cmat_signed = nir_intrinsic_cmat_signed_mask(intr);
   bool a_signed = cmat_signed & NIR_CMAT_A_SIGNED;
   bool b_signed = cmat_signed & NIR_CMAT_B_SIGNED;

   const struct nak_nir_cmat_mul_add_flags flags = {
      .cmat_type = hw_cmat_type,
      .a_type = glsl_apply_signedness_to_base_type(a_desc.element_type, a_signed),
      .b_type = glsl_apply_signedness_to_base_type(b_desc.element_type, b_signed),
      .sat = nir_intrinsic_saturate(intr),
   };

   /* Simple case: we can execute the MMA in one instruction */
   if (cmat_type == hw_cmat_type) {
      return nir_cmat_muladd_nv(b, dst_length, cmat_a, cmat_b, cmat_c,
                                .flags = NAK_AS_U32(flags));
   }

   unsigned a_length = get_cmat_length(a_desc);
   unsigned b_length = get_cmat_length(b_desc);
   unsigned c_length = get_cmat_length(c_desc);

   nir_def *a_comps[NIR_MAX_VEC_COMPONENTS];
   nir_def *b_comps[NIR_MAX_VEC_COMPONENTS];
   nir_def *c_comps[NIR_MAX_VEC_COMPONENTS];
   nir_def *d_comps[NIR_MAX_VEC_COMPONENTS];

   for (unsigned i = 0; i < a_length; i++)
      a_comps[i] = nir_channel(b, cmat_a, i);

   for (unsigned i = 0; i < b_length; i++)
      b_comps[i] = nir_channel(b, cmat_b, i);

   for (unsigned i = 0; i < c_length; i++)
      c_comps[i] = nir_channel(b, cmat_c, i);

   if (hw_cmat_type == NAK_CMAT_TYPE_M8N8K16_INT &&
         (cmat_type == NAK_CMAT_TYPE_M16N8K32_INT ||
          cmat_type == NAK_CMAT_TYPE_M16N16K32_INT_SW)) {
      const unsigned a_hw_length = 4;
      const unsigned b_hw_length = 4;
      const unsigned c_hw_length = 2;
      const unsigned d_hw_length = 2;

      for (unsigned i = 0; i < dst_length / d_hw_length; i++) {
         unsigned cmat_a_lo_offset = (i % 2) * a_hw_length;
         unsigned cmat_a_hi_offset = cmat_a_lo_offset + 8;

         unsigned cmat_b_lo_offset = (i / 2) * b_hw_length;
         if (cmat_type == NAK_CMAT_TYPE_M16N16K32_INT_SW)
            cmat_b_lo_offset *= 2;
         unsigned cmat_b_hi_offset = cmat_b_lo_offset + 4;

         unsigned cmat_c_offset = i * c_hw_length;

         nir_def *cmat_a_lo = nir_vec(b, &a_comps[cmat_a_lo_offset], a_hw_length);
         nir_def *cmat_a_hi = nir_vec(b, &a_comps[cmat_a_hi_offset], a_hw_length);
         nir_def *cmat_b_lo = nir_vec(b, &b_comps[cmat_b_lo_offset], b_hw_length);
         nir_def *cmat_b_hi = nir_vec(b, &b_comps[cmat_b_hi_offset], b_hw_length);
         nir_def *c_part = nir_vec(b, &c_comps[cmat_c_offset], c_hw_length);

         nir_def *new_c = nir_cmat_muladd_nv(b, d_hw_length, cmat_a_lo,
                                             cmat_b_lo, c_part,
                                             .flags = NAK_AS_U32(flags));
         nir_def *tmp_d = nir_cmat_muladd_nv(b, d_hw_length, cmat_a_hi,
                                             cmat_b_hi, new_c,
                                             .flags = NAK_AS_U32(flags));

         for (unsigned c = 0; c < d_hw_length; c++)
            d_comps[i * d_hw_length + c] = nir_channel(b, tmp_d, c);
      }
   } else if ((cmat_type == NAK_CMAT_TYPE_M16N16K32_INT_SW &&
               hw_cmat_type == NAK_CMAT_TYPE_M16N8K32_INT) ||
              (cmat_type == NAK_CMAT_TYPE_M16N16K16_FLOAT_SW &&
               hw_cmat_type == NAK_CMAT_TYPE_M16N8K16_FLOAT))  {
      nir_def *cmat_b_lo = nir_vec(b,  b_comps,               b_length / 2);
      nir_def *cmat_b_hi = nir_vec(b, &b_comps[b_length / 2], b_length / 2);

      nir_def *cmat_c_lo = nir_vec(b,  c_comps,               c_length / 2);
      nir_def *cmat_c_hi = nir_vec(b, &c_comps[c_length / 2], c_length / 2);

      nir_def *cmat_d_lo = nir_cmat_muladd_nv(b, dst_length / 2, cmat_a,
                                              cmat_b_lo, cmat_c_lo,
                                              .flags = NAK_AS_U32(flags));
      nir_def *cmat_d_hi = nir_cmat_muladd_nv(b, dst_length / 2, cmat_a,
                                              cmat_b_hi, cmat_c_hi,
                                              .flags = NAK_AS_U32(flags));

      for (unsigned i = 0; i < dst_length / 2; i++) {
         d_comps[i]                  = nir_channel(b, cmat_d_lo, i);
         d_comps[i + dst_length / 2] = nir_channel(b, cmat_d_hi, i);
      }
   } else {
      assert(0 && "lowering not implemented");
   }

   return nir_vec(b, d_comps, dst_length);
}

static nir_def *
lower_cmat_convert(nir_builder *b, nir_intrinsic_instr *intr, nir_def *cmat,
                   struct glsl_cmat_description a_desc,
                   struct glsl_cmat_description d_desc)
{
   nir_cmat_signed cmat_signed_mask = nir_intrinsic_cmat_signed_mask(intr);

   enum glsl_base_type src_type = glsl_apply_signedness_to_base_type(
      a_desc.element_type, cmat_signed_mask & NIR_CMAT_A_SIGNED);
   enum glsl_base_type dst_type = glsl_apply_signedness_to_base_type(
      d_desc.element_type, cmat_signed_mask & NIR_CMAT_RESULT_SIGNED);

   /* We want to shuffle the smaller values for better packing. */
   bool conv_narrows =
      glsl_base_type_bit_size(src_type) > glsl_base_type_bit_size(dst_type);
   nir_op op =
      nir_type_conversion_op(nir_get_nir_type_for_glsl_base_type(src_type),
                             nir_get_nir_type_for_glsl_base_type(dst_type),
                             nir_rounding_mode_undef);

   /* If the result type is smaller, we convert before shuffling. */
   if (conv_narrows)
      cmat = nir_build_alu1(b, op, cmat);

   enum nak_matrix_type_layout a_layout = determine_matrix_type(a_desc);
   enum nak_matrix_type_layout d_layout = determine_matrix_type(d_desc);

   /* Matrix layout conversion code. For some conversions we also need
    * to fix the layout, so we shuffle values around to achieve that.
    */
   if (a_layout != d_layout) {
      nir_def *lane_id = nir_load_subgroup_invocation(b);
      unsigned mask    = a_layout == NAK_MAT_16X16 ? 0x1 : 0x2;
      unsigned compare = a_layout == NAK_MAT_16X16 ? 0x2 : 0x1;

      nir_def *adj;
      if (a_layout == NAK_MAT_16X16) {
         adj = nir_ishl_imm(b, nir_iand_imm(b, lane_id, mask), 1);
      } else {
         adj = nir_ushr_imm(b, nir_iand_imm(b, lane_id, mask), 1);
      }

      /* lane_id & 0x1c + (lane_id & mask << 1) */
      /* lane_id & 0x1c + (lane_id & mask >> 1) */
      nir_def *lane0 = nir_iadd(b, nir_iand_imm(b, lane_id, 0x1c), adj);
      /* lane_id & 0x1c + (lane_id & mask << 1) + mask */
      /* lane_id & 0x1c + (lane_id & mask >> 1) + mask */
      nir_def *lane1 = nir_iadd_imm(b, lane0, mask);
      nir_def *cond = nir_ieq_imm(b, nir_iand_imm(b, lane_id, compare), 0);

      if (cmat->num_components == 4) {
         nir_def *xy = nir_channels(b, cmat, 0x3);
         nir_def *zw = nir_channels(b, cmat, 0xc);

         nir_def *xy0 = nir_shuffle(b, xy, lane0);
         nir_def *zw0 = nir_shuffle(b, xy, lane1);
         nir_def *xy1 = nir_shuffle(b, zw, lane0);
         nir_def *zw1 = nir_shuffle(b, zw, lane1);

         xy = nir_bcsel(b, cond, xy0, xy1);
         zw = nir_bcsel(b, cond, zw0, zw1);

         cmat = nir_vec4(b,
            nir_channel(b, xy, 0),
            nir_channel(b, xy, 1),
            nir_channel(b, zw, 0),
            nir_channel(b, zw, 1)
         );
      } else if (cmat->num_components == 8 && a_layout == NAK_MAT_16X16) {
         nir_def *abcd = nir_channels(b, cmat, 0x0f);
         nir_def *efgh = nir_channels(b, cmat, 0xf0);

         nir_def *abef0 = nir_shuffle(b, abcd, lane0);
         nir_def *cdgh0 = nir_shuffle(b, abcd, lane1);
         nir_def *abef1 = nir_shuffle(b, efgh, lane0);
         nir_def *cdgh1 = nir_shuffle(b, efgh, lane1);

         nir_def *abef = nir_bcsel(b, cond, abef0, abef1);
         nir_def *cdgh = nir_bcsel(b, cond, cdgh0, cdgh1);

         cmat = nir_vec8(b,
            nir_channel(b, abef, 0),
            nir_channel(b, abef, 1),
            nir_channel(b, cdgh, 0),
            nir_channel(b, cdgh, 1),
            nir_channel(b, abef, 2),
            nir_channel(b, abef, 3),
            nir_channel(b, cdgh, 2),
            nir_channel(b, cdgh, 3)
         );
      } else if (cmat->num_components == 8 && a_layout == NAK_MAT_16x32_INT8) {
         nir_def *abef = nir_channels(b, cmat, 0x33);
         nir_def *cdgh = nir_channels(b, cmat, 0xcc);

         nir_def *abcd0 = nir_shuffle(b, abef, lane0);
         nir_def *efgh0 = nir_shuffle(b, abef, lane1);
         nir_def *abcd1 = nir_shuffle(b, cdgh, lane0);
         nir_def *efgh1 = nir_shuffle(b, cdgh, lane1);

         nir_def *abcd = nir_bcsel(b, cond, abcd0, abcd1);
         nir_def *efgh = nir_bcsel(b, cond, efgh0, efgh1);

         cmat = nir_vec8(b,
            nir_channel(b, abcd, 0),
            nir_channel(b, abcd, 1),
            nir_channel(b, abcd, 2),
            nir_channel(b, abcd, 3),
            nir_channel(b, efgh, 0),
            nir_channel(b, efgh, 1),
            nir_channel(b, efgh, 2),
            nir_channel(b, efgh, 3)
         );
      } else {
         unreachable("unsupported component counts for Matrix layout conversion");
      }
   }

   /* If the result type is not smaller, we convert after shuffling */
   if (!conv_narrows)
      cmat = nir_build_alu1(b, op, cmat);

   return cmat;
}

static bool
lower_cmat_instr(nir_builder *b,
                 nir_instr *instr,
                 struct hash_table *type_mapping,
                 const struct nak_compiler *nak)
{
   /* Remap deref types */
   if (instr->type == nir_instr_type_deref) {
      nir_deref_instr *deref = nir_instr_as_deref(instr);
      const struct glsl_type *new_type =
         remap_matrix_type(type_mapping, deref->type);

      if (new_type != deref->type) {
         deref->type = new_type;
         return true;
      } else {
         return false;
      }
   }

   if (instr->type != nir_instr_type_intrinsic)
      return false;

   nir_intrinsic_instr *intr = nir_instr_as_intrinsic(instr);
   b->cursor = nir_before_instr(instr);

   switch (intr->intrinsic) {
   case nir_intrinsic_cmat_construct: {
      const unsigned length = get_cmat_length(cmat_src_desc(intr->src[0]));
      nir_def *r = nir_replicate(b, intr->src[1].ssa, length);

      store_cmat_src(b, intr->src[0], r);
      nir_instr_remove(instr);
      return true;
   }

   case nir_intrinsic_cmat_load: {
      const struct glsl_cmat_description desc = cmat_src_desc(intr->src[0]);
      const unsigned length = get_cmat_length(desc);
      const enum glsl_matrix_layout layout = nir_intrinsic_matrix_layout(intr);

      nir_deref_instr *deref =
         nir_instr_as_deref(intr->src[1].ssa->parent_instr);
      nir_def *stride = intr->src[2].ssa;

      nir_def *vars[NIR_MAX_VEC_COMPONENTS];
      for (unsigned i = 0; i < length; ++i)
         vars[i] = nir_undef(b, 1, glsl_base_type_bit_size(desc.element_type));

      nir_def *lane_id = nir_load_subgroup_invocation(b);

      for (unsigned idx = 0; idx < length; idx++) {
         nir_def *col_offset;
         nir_def *row_offset;

         compute_matrix_offsets(b, desc, layout, lane_id, idx,
                                &col_offset, &row_offset);

         row_offset = nir_imul(b, row_offset, stride);

         col_offset = nir_u2uN(b, col_offset, deref->def.bit_size);
         row_offset = nir_u2uN(b, row_offset, deref->def.bit_size);

         nir_deref_instr *iter_deref =
            nir_build_deref_ptr_as_array(b, deref, row_offset);
         iter_deref = nir_build_deref_cast(
            b, &iter_deref->def, deref->modes,
            glsl_scalar_type(desc.element_type),
            glsl_base_type_bit_size(desc.element_type) / 8);
         iter_deref =
            nir_build_deref_ptr_as_array(b, iter_deref, col_offset);

         vars[idx] = nir_load_deref(b, iter_deref);
      }

      nir_def *mat = nir_vec(b, vars, length);
      store_cmat_src(b, intr->src[0], mat);
      nir_instr_remove(instr);
      return true;
   }

   case nir_intrinsic_cmat_store: {
      enum glsl_matrix_layout layout = nir_intrinsic_matrix_layout(intr);

      nir_deref_instr *deref = nir_src_as_deref(intr->src[0]);
      nir_def *stride = intr->src[2].ssa;

      const struct glsl_cmat_description desc = cmat_src_desc(intr->src[1]);
      const unsigned length = get_cmat_length(desc);
      nir_def *src = load_cmat_src(b, intr->src[1]);

      nir_def *vars[NIR_MAX_VEC_COMPONENTS];
      for (unsigned i = 0; i < length; i++)
         vars[i] = nir_channel(b, src, i);

      nir_def *lane_id = nir_load_subgroup_invocation(b);

      for (unsigned idx = 0; idx < length; idx++) {
         nir_def *col_offset;
         nir_def *row_offset;

         compute_matrix_offsets(b, desc, layout, lane_id, idx,
                                &col_offset, &row_offset);

         row_offset = nir_imul(b, row_offset, stride);

         col_offset = nir_u2uN(b, col_offset, deref->def.bit_size);
         row_offset = nir_u2uN(b, row_offset, deref->def.bit_size);

         nir_deref_instr *iter_deref =
            nir_build_deref_ptr_as_array(b, deref, row_offset);
         iter_deref = nir_build_deref_cast(
            b, &iter_deref->def, deref->modes,
            glsl_scalar_type(desc.element_type),
            glsl_base_type_bit_size(desc.element_type) / 8);
         iter_deref =
            nir_build_deref_ptr_as_array(b, iter_deref, col_offset);

         nir_store_deref(b, iter_deref, vars[idx], 1);
      }

      nir_instr_remove(instr);
      return true;
   }

   case nir_intrinsic_cmat_length: {
      const unsigned length = get_cmat_length(nir_intrinsic_cmat_desc(intr));
      nir_def_replace(&intr->def, nir_imm_int(b, length));
      return true;
   }

   case nir_intrinsic_cmat_muladd: {
      const struct glsl_cmat_description d_desc = cmat_src_desc(intr->src[0]);

      const struct glsl_cmat_description a_desc = cmat_src_desc(intr->src[1]);
      const struct glsl_cmat_description b_desc = cmat_src_desc(intr->src[2]);
      const struct glsl_cmat_description c_desc = cmat_src_desc(intr->src[3]);

      nir_def *cmat_a = load_cmat_src(b, intr->src[1]);
      nir_def *cmat_b = load_cmat_src(b, intr->src[2]);
      nir_def *cmat_c = load_cmat_src(b, intr->src[3]);

      nir_def *ret = lower_cmat_muladd(b, intr, cmat_a, cmat_b, cmat_c, a_desc,
                                       b_desc, c_desc, d_desc, nak->sm);
      store_cmat_src(b, intr->src[0], ret);
      nir_instr_remove(&intr->instr);
      return true;
   }

   case nir_intrinsic_cmat_unary_op: {
      nir_def *src = load_cmat_src(b, intr->src[1]);
      nir_op op = nir_intrinsic_alu_op(intr);

      nir_def *ret = nir_build_alu1(b, op, src);
      store_cmat_src(b, intr->src[0], ret);

      nir_instr_remove(instr);
      return true;
   }

   case nir_intrinsic_cmat_binary_op: {
      nir_def *src_a = load_cmat_src(b, intr->src[1]);
      nir_def *src_b = load_cmat_src(b, intr->src[2]);
      nir_op op = nir_intrinsic_alu_op(intr);

      nir_def *ret = nir_build_alu2(b, op, src_a, src_b);
      store_cmat_src(b, intr->src[0], ret);

      nir_instr_remove(instr);
      return true;
   }

   case nir_intrinsic_cmat_scalar_op: {
      nir_def *src_a = load_cmat_src(b, intr->src[1]);
      nir_op op = nir_intrinsic_alu_op(intr);

      nir_def *ret = nir_build_alu2(b, op, src_a, intr->src[2].ssa);
      store_cmat_src(b, intr->src[0], ret);

      nir_instr_remove(instr);
      return true;
   }

   case nir_intrinsic_cmat_bitcast: {
      nir_def *mat = load_cmat_src(b, intr->src[1]);
      store_cmat_src(b, intr->src[0], mat);
      nir_instr_remove(instr);
      return true;
   }

   case nir_intrinsic_cmat_extract: {
      nir_def *mat = load_cmat_src(b, intr->src[0]);
      nir_def *index = intr->src[1].ssa;
      nir_def *elem = nir_vector_extract(b, mat, index);
      nir_def_replace(&intr->def, elem);
      return true;
   }

   case nir_intrinsic_cmat_insert: {
      nir_def *elem = intr->src[1].ssa;
      nir_def *mat = load_cmat_src(b, intr->src[2]);
      nir_def *index = intr->src[3].ssa;

      nir_def *r = nir_vector_insert(b, mat, elem, index);
      store_cmat_src(b, intr->src[0], r);

      nir_instr_remove(instr);
      return true;
   }

   case nir_intrinsic_cmat_copy: {
      nir_build_copy_deref(b, intr->src[0].ssa, intr->src[1].ssa);
      nir_instr_remove(instr);
      return true;
   }

   case nir_intrinsic_cmat_convert: {
      struct glsl_cmat_description dst_desc = cmat_src_desc(intr->src[0]);
      struct glsl_cmat_description src_desc = cmat_src_desc(intr->src[1]);

      nir_def *cmat = load_cmat_src(b, intr->src[1]);
      nir_def *ret = lower_cmat_convert(b, intr, cmat, src_desc, dst_desc);
      store_cmat_src(b, intr->src[0], ret);

      nir_instr_remove(instr);
      return true;
   }

   default:
      return false;
   }
}

static bool
lower_cmat_impl(nir_function_impl *impl,
                struct hash_table *type_mapping,
                const struct nak_compiler *nak)
{
   bool progress = false;

   /* Remap all cmat temp var to array of scalars */
   nir_foreach_function_temp_variable(var, impl) {
      const struct glsl_type *new_type =
         remap_matrix_type(type_mapping, var->type);
      if (new_type != var->type) {
         var->type = new_type;
         progress = true;
      }
   }

   nir_builder b = nir_builder_create(impl);
   nir_foreach_block_reverse_safe(block, impl) {
      nir_foreach_instr_reverse_safe(instr, block) {
         if (lower_cmat_instr(&b, instr, type_mapping, nak))
            progress = true;
      }
   }

   return nir_progress(progress, impl, nir_metadata_control_flow);
}

bool
nak_nir_lower_cmat(nir_shader *nir, const struct nak_compiler *nak)
{
   bool progress = false;

   if (nir->info.stage != MESA_SHADER_COMPUTE ||
       !nir->info.cs.has_cooperative_matrix)
      return false;

   struct hash_table *type_mapping = _mesa_pointer_hash_table_create(NULL);

   /* Remap all cmat shader temp var to array of scalars */
   nir_foreach_variable_with_modes(var, nir, nir_var_shader_temp) {
      const struct glsl_type *new_type =
         remap_matrix_type(type_mapping, var->type);

      if (new_type != var->type) {
         var->type = new_type;
         progress = true;
      }
   }

   nir_foreach_function_impl(impl, nir) {
      if (lower_cmat_impl(impl, type_mapping, nak))
         progress = true;
   }

   return progress;
}
