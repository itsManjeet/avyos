/*
 * Copyright 2020 Advanced Micro Devices, Inc.
 *
 * SPDX-License-Identifier: MIT
 */

#include "si_build_pm4.h"
#include "ac_debug.h"
#include "ac_shadowed_regs.h"
#include "util/u_memory.h"

bool si_init_cp_reg_shadowing(struct si_context *sctx)
{
   if (!si_init_gfx_preamble_state(sctx))
      return false;

   if (sctx->uses_userq_reg_shadowing) {
      sctx->ws->userq_submit_cs_preamble_ib_once(&sctx->gfx_cs, &sctx->cs_preamble_state->base);
      si_pm4_free_state(sctx, sctx->cs_preamble_state, ~0);
      sctx->cs_preamble_state = NULL;
   } else if (sctx->uses_kernelq_reg_shadowing) {
      if (sctx->screen->info.has_fw_based_shadowing) {
         sctx->shadowing.registers =
               si_aligned_buffer_create(sctx->b.screen,
                                        PIPE_RESOURCE_FLAG_UNMAPPABLE | SI_RESOURCE_FLAG_DRIVER_INTERNAL,
                                        PIPE_USAGE_DEFAULT,
                                        sctx->screen->info.fw_based_mcbp.shadow_size,
                                        sctx->screen->info.fw_based_mcbp.shadow_alignment);
         sctx->shadowing.csa =
               si_aligned_buffer_create(sctx->b.screen,
                                        PIPE_RESOURCE_FLAG_UNMAPPABLE | SI_RESOURCE_FLAG_DRIVER_INTERNAL,
                                        PIPE_USAGE_DEFAULT,
                                        sctx->screen->info.fw_based_mcbp.csa_size,
                                        sctx->screen->info.fw_based_mcbp.csa_alignment);
         if (!sctx->shadowing.registers || !sctx->shadowing.csa) {
            mesa_loge("cannot create register shadowing buffer(s)");
            return false;
         } else {
            sctx->ws->cs_set_mcbp_reg_shadowing_va(&sctx->gfx_cs,
                                                   sctx->shadowing.registers->gpu_address,
                                                   sctx->shadowing.csa->gpu_address);
         }
      } else {
         sctx->shadowing.registers =
               si_aligned_buffer_create(sctx->b.screen,
                                        PIPE_RESOURCE_FLAG_UNMAPPABLE | SI_RESOURCE_FLAG_DRIVER_INTERNAL,
                                        PIPE_USAGE_DEFAULT,
                                        SI_SHADOWED_REG_BUFFER_SIZE,
                                        4096);
         if (!sctx->shadowing.registers) {
            mesa_loge("cannot create a shadowed_regs buffer");
            return false;
         }
      }

      /* We need to clear the shadowed reg buffer. */
      si_cp_dma_clear_buffer(sctx, &sctx->gfx_cs, &sctx->shadowing.registers->b.b,
                             0, sctx->shadowing.registers->bo_size, 0);
      si_barrier_after_simple_buffer_op(sctx, 0, &sctx->shadowing.registers->b.b, NULL);

      /* Create the shadowing preamble. */
      struct ac_pm4_state *shadowing_preamble =
         ac_create_shadowing_ib_preamble(&sctx->screen->info,
                                         sctx->shadowing.registers->gpu_address,
                                         sctx->screen->dpbb_allowed);

      if (!shadowing_preamble) {
         mesa_loge("failed to create shadowing_preamble");
         return false;
      }

      /* Initialize shadowed registers as follows. */
      radeon_add_to_buffer_list(sctx, &sctx->gfx_cs, sctx->shadowing.registers,
                                RADEON_USAGE_READWRITE | RADEON_PRIO_DESCRIPTORS);
      if (sctx->shadowing.csa)
         radeon_add_to_buffer_list(sctx, &sctx->gfx_cs, sctx->shadowing.csa,
                                   RADEON_USAGE_READWRITE | RADEON_PRIO_DESCRIPTORS);
      si_pm4_emit_commands(sctx, shadowing_preamble);

      if (sctx->gfx_level < GFX11) {
         struct ac_pm4_state *clear_state = ac_emulate_clear_state(&sctx->screen->info);
         if (!clear_state) {
            ac_pm4_free_state(shadowing_preamble);
            mesa_loge("failed to create clear_state");
            return false;
         }
         si_pm4_emit_commands(sctx, clear_state);
         ac_pm4_free_state(clear_state);
      }

      /* TODO: Gfx11 fails GLCTS if we don't re-emit the preamble at the beginning of every IB. */
      /* TODO: Skipping this may have made register shadowing slower on Gfx11. */
      if (sctx->gfx_level < GFX11) {
         si_pm4_emit_commands(sctx, &sctx->cs_preamble_state->base);

         /* The register values are shadowed, so we won't need to set them again. */
         si_pm4_free_state(sctx, sctx->cs_preamble_state, ~0);
         sctx->cs_preamble_state = NULL;
      }

      if (sctx->gfx_level < GFX12)
         si_set_tracked_regs_to_clear_state(sctx);

      /* Setup preemption. The shadowing preamble will be executed as a preamble IB,
       * which will load register values from memory on a context switch.
       */
      sctx->ws->cs_setup_preemption(&sctx->gfx_cs, shadowing_preamble->pm4,
                                    shadowing_preamble->ndw);
      ac_pm4_free_state(shadowing_preamble);
   }

   return true;
}
