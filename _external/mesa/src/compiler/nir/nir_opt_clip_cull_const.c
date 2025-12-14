/*
 * Copyright 2024 Valve Corporation
 * SPDX-License-Identifier: MIT
 */

#include "nir.h"
#include "nir_builder.h"

/**
 * If a clip/cull distance is constant >= 0,
 * we know that it will never cause clipping/culling.
 * Remove the sysval_output in that case.
 *
 * Assumes that nir_lower_io_vars_to_temporaries was run,
 * and works best with scalar store_outputs.
 */

/* Return -1 if invalid, 0 if it's a normal clip/cull distance value,
 * and 1 if it's a no-op value.
 */
static int
analyze_clip_cull_value(nir_intrinsic_instr *intr)
{
   if (intr->intrinsic != nir_intrinsic_store_output)
      return -1;

   const nir_io_semantics io_sem = nir_intrinsic_io_semantics(intr);
   const unsigned location = io_sem.location;

   if (io_sem.no_sysval_output)
      return -1;

   if (location != VARYING_SLOT_CLIP_DIST0 && location != VARYING_SLOT_CLIP_DIST1)
      return -1;

   nir_def *val = intr->src[0].ssa;
   for (unsigned i = 0; i < val->num_components; i++) {
      nir_scalar s = nir_scalar_resolved(val, i);
      if (!nir_scalar_is_const(s))
         return 0;
      float distance = nir_scalar_as_float(s);

      /* NaN gets clipped, and INF after interpolation is NaN. */
      if (isnan(distance) || distance < 0.0 || distance == INFINITY)
         return 0;
   }

   return 1;
}

static bool
opt_clip_cull_vs_tes(nir_builder *b, nir_intrinsic_instr *intr, void *unused)
{
   if (analyze_clip_cull_value(intr) == 1) {
      nir_remove_sysval_output(intr, MESA_SHADER_FRAGMENT);
      return true;
   }

   return false;
}

/* The types of stores are first gathered for all stores. If a certain slot
 * component is only written by no-op stores, they are removed.
 */
typedef struct {
   bool has_normal_store[8];
   bool has_noop_store[8];
} gs_info;

static unsigned
get_clip_io_index(nir_intrinsic_instr *intr)
{
   nir_io_semantics sem = nir_intrinsic_io_semantics(intr);
   assert(intr->intrinsic == nir_intrinsic_store_output);
   assert(sem.location == VARYING_SLOT_CLIP_DIST0 ||
          sem.location == VARYING_SLOT_CLIP_DIST1);
   assert(sem.num_slots == 1);
   assert(nir_src_as_uint(*nir_get_io_offset_src(intr)) == 0); /* no indirect */
   assert(intr->src[0].ssa->num_components == 1); /* scalar */

   return (sem.location - VARYING_SLOT_CLIP_DIST0) * 4 +
          nir_intrinsic_component(intr);
}

static bool
gather_clip_cull_gs(nir_builder *b, nir_intrinsic_instr *intr, void *unused)
{
   gs_info *info = (gs_info *)unused;
   int r = analyze_clip_cull_value(intr);

   if (r == 1)
      info->has_noop_store[get_clip_io_index(intr)] = true;
   else if (r == 0)
      info->has_normal_store[get_clip_io_index(intr)] = true;

   return false;
}

static bool
opt_clip_cull_gs(nir_builder *b, nir_intrinsic_instr *intr, void *unused)
{
   gs_info *info = (gs_info *)unused;

   if (intr->intrinsic == nir_intrinsic_store_output &&
       (nir_intrinsic_io_semantics(intr).location == VARYING_SLOT_CLIP_DIST0 ||
        nir_intrinsic_io_semantics(intr).location == VARYING_SLOT_CLIP_DIST1)) {
      unsigned index = get_clip_io_index(intr);

      if (info->has_noop_store[index] && !info->has_normal_store[index]) {
         nir_remove_sysval_output(intr, MESA_SHADER_FRAGMENT);
         return true;
      }
   }

   return false;
}

bool
nir_opt_clip_cull_const(nir_shader *shader)
{
   if (shader->info.stage == MESA_SHADER_GEOMETRY) {
      gs_info info = {0};
      nir_shader_intrinsics_pass(shader, gather_clip_cull_gs, nir_metadata_all, &info);
      return nir_shader_intrinsics_pass(shader, opt_clip_cull_gs, nir_metadata_all, &info);
   } else {
      return nir_shader_intrinsics_pass(shader, opt_clip_cull_vs_tes, nir_metadata_all, NULL);
   }
}
