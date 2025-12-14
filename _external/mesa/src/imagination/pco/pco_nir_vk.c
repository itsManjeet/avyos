/*
 * Copyright © 2025 Imagination Technologies Ltd.
 *
 * SPDX-License-Identifier: MIT
 */

/**
 * \file pco_nir_vk.c
 *
 * \brief PCO NIR Vulkan lowering pass.
 */

#include "nir.h"
#include "nir_builder.h"
#include "pco.h"
#include "pco_builder.h"
#include "pco_internal.h"
#include "util/macros.h"

#include <assert.h>
#include <stdbool.h>
#include <stdio.h>

static void
set_resource_used(pco_common_data *common, unsigned desc_set, unsigned binding)
{
   assert(desc_set < ARRAY_SIZE(common->desc_sets));
   pco_descriptor_set_data *desc_set_data = &common->desc_sets[desc_set];
   desc_set_data->used = true;

   assert(desc_set_data->bindings && binding < desc_set_data->binding_count);
   pco_binding_data *binding_data = &desc_set_data->bindings[binding];
   binding_data->used = true;
}

/**
 * \brief Lowers load_vulkan_descriptor.
 *
 * Packs the descriptor set, binding, and array element.
 *
 * \param[in] b NIR builder.
 * \param[in] intr NIR intrinsic instruction.
 * \param[in] common Shader common data.
 * \return The replacement/lowered def.
 */
static nir_def *lower_load_vulkan_descriptor(nir_builder *b,
                                             nir_intrinsic_instr *intr,
                                             pco_common_data *common)
{
   nir_intrinsic_instr *vk_res_idx = nir_src_as_intrinsic(intr->src[0]);
   assert(vk_res_idx->intrinsic == nir_intrinsic_vulkan_resource_index);

   assert(nir_intrinsic_desc_type(intr) == nir_intrinsic_desc_type(vk_res_idx));

   unsigned desc_set = nir_intrinsic_desc_set(vk_res_idx);
   unsigned binding = nir_intrinsic_binding(vk_res_idx);
   unsigned elem = nir_src_as_uint(vk_res_idx->src[0]);

   set_resource_used(common, desc_set, binding);

   uint32_t desc_set_binding = pco_pack_desc(desc_set, binding);
   return nir_imm_ivec3(b, desc_set_binding, elem, 0);
}

/**
 * \brief Lowers a Vulkan-related instruction.
 *
 * \param[in] b NIR builder.
 * \param[in] instr NIR instruction.
 * \param[in] cb_data User callback data.
 * \return The replacement/lowered def.
 */
static nir_def *lower_vk(nir_builder *b, nir_instr *instr, void *cb_data)
{
   pco_common_data *common = cb_data;
   nir_intrinsic_instr *intr = nir_instr_as_intrinsic(instr);

   switch (intr->intrinsic) {
   case nir_intrinsic_load_vulkan_descriptor:
      return lower_load_vulkan_descriptor(b, intr, common);

   default:
      break;
   }

   unreachable("");
}

/**
 * \brief Filters Vulkan-related instructions.
 *
 * \param[in] instr NIR instruction.
 * \param[in] cb_data User callback data.
 * \return True if the instruction matches the filter.
 */
static bool is_vk(const nir_instr *instr, UNUSED const void *cb_data)
{
   if (instr->type != nir_instr_type_intrinsic)
      return false;

   nir_intrinsic_instr *intr = nir_instr_as_intrinsic(instr);
   switch (intr->intrinsic) {
   case nir_intrinsic_load_vulkan_descriptor:
      return true;

   default:
      break;
   }

   return false;
}

/**
 * \brief Vulkan lowering pass.
 *
 * \param[in,out] shader NIR shader.
 * \param[in,out] common Common shader data.
 * \return True if the pass made progress.
 */
bool pco_nir_lower_vk(nir_shader *shader, pco_common_data *common)
{
   bool progress = false;

   progress |= nir_shader_lower_instructions(shader, is_vk, lower_vk, common);

   return progress;
}
