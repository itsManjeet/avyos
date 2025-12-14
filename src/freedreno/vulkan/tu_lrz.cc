/*
 * Copyright © 2022 Igalia S.L.
 * SPDX-License-Identifier: MIT
 */

#include "tu_lrz.h"

#include "tu_clear_blit.h"
#include "tu_cmd_buffer.h"
#include "tu_cs.h"
#include "tu_image.h"

#include "common/freedreno_gpu_event.h"
#include "common/freedreno_lrz.h"

/* See lrz.rst for how HW works. Here are only the implementation notes.
 *
 * LRZ, Depth Test, and Stencil Test
 * =================================
 *
 * LRZ works in both binning and direct rendering modes.
 *
 *               LRZ Test -> LRZ Write (in binning)
 *                  v
 *               Early Stencil Test -> Stencil Write
 *                  v
 *               Early Depth Test -> Depth Write + LRZ feedback
 *                                 + Stencil depthFailOp
 *                  v
 * (!FS_DISABLE) Fragment Shader
 *                  v
 * (!FS_DISABLE) Late Stencil Test -> Stencil Write
 *                  v
 * (!FS_DISABLE) Late Depth Test -> Depth Write + LRZ feedback
 *                                + Stencil depthFailOp
 *
 * Keep the above in mind, when LRZ test fails, no other operations
 * are performed. Especially important with stencil.
 *
 * LRZ is DISABLED until depth attachment is cleared when:
 *   - Depth Write + changing direction of depth test
 *      e.g. from OP_GREATER to OP_LESS;
 *   - Depth Write + OP_ALWAYS or OP_NOT_EQUAL;
 *   - Clearing depth with vkCmdClearAttachments;
 *   - Depth image is a target of blit commands.
 *   - (pre-a650) Not clearing depth attachment with LOAD_OP_CLEAR;
 *   - (pre-a650) Using secondary command buffers;
 * LRZ WRITE is DISABLED until depth attachment is cleared when:
 *   - Depth Write + blending (color blend, logic ops, partial color mask, etc.);
 *   - Fragment may be killed by stencil;
 * LRZ is disabled for CURRENT draw when:
 *   - Fragment shader side-effects (writing to SSBOs, atomic operations, etc);
 *   - Fragment shader writes depth or stencil;
 * LRZ WRITE is DISABLED (via LATE_Z) for CURRENT draw when:
 *   - Fragment may be via killed alpha-to-coverage, discard, sample coverage;
 *
 * There is a documentation on LRZ rules in QCOM's driver:
 *  https://docs.qualcomm.com/bundle/publicresource/topics/80-78185-2/best_practices.html?product=1601111740035277#lrz-do-not-disable
 *
 * A650+ (gen3+)
 * =============
 *
 * While LRZ could be reused between renderpasses LRZ, it is disabled when
 * underlying depth buffer is changed.
 * The following commands could change a depth image:
 * - vkCmdBlitImage*
 * - vkCmdCopyBufferToImage*
 * - vkCmdCopyImage*
 *
 * LRZ Fast-Clear
 * ==============
 *
 * It's always valid to fast-clear. On the other hand we disable
 * fast-clear if depth clear value is not 0.0 or 1.0 because it may be worse
 * for perf if some primitives are expected to fail depth test against the
 * actual depth clear value.
 *
 * LRZ Caches
 * ==========
 *
 * ! The policy here is to flush LRZ cache right after it is changed,
 * so if LRZ data is needed afterwards - there is no need to flush it
 * before using LRZ.
 */

static inline void
tu_lrz_disable_reason(struct tu_cmd_buffer *cmd, const char *reason) {
   cmd->state.rp.lrz_disable_reason = reason;
   cmd->state.rp.lrz_disabled_at_draw = cmd->state.rp.drawcall_count;
   perf_debug(cmd->device, "Disabling LRZ because '%s' at draw %u", reason,
              cmd->state.rp.lrz_disabled_at_draw);
}

static inline void
tu_lrz_write_disable_reason(struct tu_cmd_buffer *cmd, const char *reason) {
   cmd->state.rp.lrz_write_disabled_at_draw = cmd->state.rp.drawcall_count;
   perf_debug(
      cmd->device,
      "Disabling LRZ write for the rest of the RP because '%s' at draw %u",
      reason, cmd->state.rp.lrz_write_disabled_at_draw);
}

template <chip CHIP>
static void
tu6_emit_lrz_buffer(struct tu_cs *cs, struct tu_image *depth_image)
{
   if (!depth_image) {
      tu_cs_emit_regs(cs,
                      A6XX_GRAS_LRZ_BUFFER_BASE(0),
                      A6XX_GRAS_LRZ_BUFFER_PITCH(0),
                      A6XX_GRAS_LRZ_FAST_CLEAR_BUFFER_BASE(0));

      if (CHIP >= A7XX)
         tu_cs_emit_regs(cs, A7XX_GRAS_LRZ_DEPTH_BUFFER_INFO());

      return;
   }

   uint64_t lrz_iova = depth_image->iova + depth_image->lrz_layout.lrz_offset;
   uint64_t lrz_fc_iova =
      depth_image->iova + depth_image->lrz_layout.lrz_fc_offset;
   if (!depth_image->lrz_layout.lrz_fc_offset)
      lrz_fc_iova = 0;

   tu_cs_emit_regs(
      cs, A6XX_GRAS_LRZ_BUFFER_BASE(.qword = lrz_iova),
      A6XX_GRAS_LRZ_BUFFER_PITCH(.pitch = depth_image->lrz_layout.lrz_pitch,
                                 .array_pitch =
                                    depth_image->lrz_layout.lrz_layer_size),
      A6XX_GRAS_LRZ_FAST_CLEAR_BUFFER_BASE(.qword = lrz_fc_iova));

   if (CHIP >= A7XX) {
      tu_cs_emit_regs(cs, A7XX_GRAS_LRZ_DEPTH_BUFFER_INFO(
         .depth_format = tu6_pipe2depth(depth_image->vk.format)
      ));
   }
}

static void
tu6_write_lrz_reg(struct tu_cmd_buffer *cmd, struct tu_cs *cs,
                  struct tu_reg_value reg)
{
   if (cmd->device->physical_device->info->a6xx.lrz_track_quirk) {
      tu_cs_emit_pkt7(cs, CP_REG_WRITE, 3);
      tu_cs_emit(cs, CP_REG_WRITE_0_TRACKER(TRACK_LRZ));
      tu_cs_emit(cs, reg.reg);
      tu_cs_emit(cs, reg.value);
   } else {
      tu_cs_emit_pkt4(cs, reg.reg, 1);
      tu_cs_emit(cs, reg.value);
   }
}

template <chip CHIP>
static void
tu6_write_lrz_cntl(struct tu_cmd_buffer *cmd, struct tu_cs *cs,
                   struct A6XX_GRAS_LRZ_CNTL cntl)
{
   if (CHIP >= A7XX) {
      /* A7XX split LRZ_CNTL into two seperate registers. */
      struct tu_reg_value cntl2 = A7XX_GRAS_LRZ_CNTL2(
         .disable_on_wrong_dir = cntl.disable_on_wrong_dir,
         .fc_enable = cntl.fc_enable,
      );
      cntl.disable_on_wrong_dir = false;
      cntl.fc_enable = false;

      tu6_write_lrz_reg(cmd, cs, A6XX_GRAS_LRZ_CNTL(cntl));
      tu6_write_lrz_reg(cmd, cs, cntl2);
   } else {
      tu6_write_lrz_reg(cmd, cs, A6XX_GRAS_LRZ_CNTL(cntl));
   }
}

template <chip CHIP>
static void
tu6_disable_lrz_via_depth_view(struct tu_cmd_buffer *cmd, struct tu_cs *cs)
{
   /* Disable direction by writing invalid depth view. */
   tu6_write_lrz_reg(cmd, cs, A6XX_GRAS_LRZ_VIEW_INFO(
      .base_layer = 0b11111111111,
      .layer_count = 0b11111111111,
      .base_mip_level = 0b1111,
   ));

   tu6_write_lrz_cntl<CHIP>(cmd, cs, {
      .enable = true,
      .disable_on_wrong_dir = true,
   });

   tu_emit_event_write<A6XX>(cmd, cs, FD_LRZ_CLEAR);
   tu_emit_event_write<A6XX>(cmd, cs, FD_LRZ_FLUSH);
}

static void
tu_lrz_init_state(struct tu_cmd_buffer *cmd,
                  const struct tu_render_pass_attachment *att,
                  const struct tu_image_view *view)
{
   if (!view->image->lrz_layout.lrz_total_size) {
      assert(!cmd->device->use_lrz || !vk_format_has_depth(att->format));
      return;
   }

   bool clears_depth = att->clear_mask &
      (VK_IMAGE_ASPECT_COLOR_BIT | VK_IMAGE_ASPECT_DEPTH_BIT);
   bool has_gpu_tracking =
      cmd->device->physical_device->info->a6xx.has_lrz_dir_tracking;

   if (!has_gpu_tracking && !clears_depth)
      return;

   /* We need to always have an LRZ view just to disable it if there is a
    * depth attachment, there are any secondaries, and GPU tracking is
    * enabled, in order not to rely on loadOp state which doesn't exist with
    * dynamic rendering in secondaries. Otherwise the secondary will have LRZ
    * enabled and there will be a NULL/garbage LRZ buffer.
    */
   cmd->state.lrz.image_view = view;

   if (!clears_depth && !att->load)
      return;

   cmd->state.lrz.valid = true;
   cmd->state.lrz.valid_at_start = true;
   cmd->state.lrz.disable_write_for_rp = false;
   cmd->state.lrz.prev_direction = TU_LRZ_UNKNOWN;
   /* Be optimistic and unconditionally enable fast-clear in
    * secondary cmdbufs and when reusing previous LRZ state.
    */
   cmd->state.lrz.fast_clear =
      view->image->lrz_layout.lrz_fc_size > 0 && !TU_DEBUG(NOLRZFC);

   cmd->state.lrz.gpu_dir_tracking = has_gpu_tracking;
   cmd->state.lrz.reuse_previous_state = !clears_depth;
}

/* Note: if we enable LRZ here, then tu_lrz_init_state() must at least set
 * lrz.image_view, so that an LRZ buffer is present (even if LRZ is
 * dynamically disabled).
 */

static void
tu_lrz_init_secondary(struct tu_cmd_buffer *cmd,
                      const struct tu_render_pass_attachment *att)
{
   bool has_gpu_tracking =
      cmd->device->physical_device->info->a6xx.has_lrz_dir_tracking;

   if (!has_gpu_tracking)
      return;

   if (!cmd->device->use_lrz || TU_DEBUG(NOLRZ))
      return;

   if (!vk_format_has_depth(att->format))
      return;

   cmd->state.lrz.valid = true;
   cmd->state.lrz.valid_at_start = true;
   cmd->state.lrz.disable_write_for_rp = false;
   cmd->state.lrz.prev_direction = TU_LRZ_UNKNOWN;
   cmd->state.lrz.gpu_dir_tracking = has_gpu_tracking;

   /* We may not have the depth attachment when executing in a secondary
    * inside a render pass. This means we have to be even more optimistic than
    * the normal case and enable fast clear even if the depth image doesn't
    * support it.
    */
   cmd->state.lrz.fast_clear = true;

   /* These are not used inside secondaries */
   cmd->state.lrz.image_view = NULL;
   cmd->state.lrz.reuse_previous_state = false;
}

template <chip CHIP>
bool
tu_lrzfc_depth_supported(float depth) {
   /* A7XX supports fast-clearing to any value, while A6XX only supports 0.0/1.0 */
   return CHIP >= A7XX || depth == 0.0f || depth == 1.0f;
}

/* This is generally the same as tu_lrz_begin_renderpass(), but we skip
 * actually emitting anything. The lrz state needs to be consistent between
 * renderpasses, but only the first should actually emit commands to disable
 * lrz etc.
 */
template <chip CHIP>
void
tu_lrz_begin_resumed_renderpass(struct tu_cmd_buffer *cmd)
{
    /* Track LRZ valid state */
   memset(&cmd->state.lrz, 0, sizeof(cmd->state.lrz));

   if (TU_DEBUG(NOLRZ)) {
      cmd->state.dirty |= TU_CMD_DIRTY_LRZ;
      return;
   }

   uint32_t a;
   for (a = 0; a < cmd->state.pass->attachment_count; a++) {
      if (cmd->state.attachments[a]->image->lrz_layout.lrz_total_size)
         break;
   }

   if (a != cmd->state.pass->attachment_count) {
      const struct tu_render_pass_attachment *att = &cmd->state.pass->attachments[a];
      tu_lrz_init_state(cmd, att, cmd->state.attachments[a]);
      if (att->clear_mask & (VK_IMAGE_ASPECT_COLOR_BIT | VK_IMAGE_ASPECT_DEPTH_BIT)) {
         VkClearValue clear = cmd->state.clear_values[a];
         cmd->state.lrz.depth_clear_value = clear;
         cmd->state.lrz.fast_clear = cmd->state.lrz.fast_clear &&
                                     tu_lrzfc_depth_supported<CHIP>(clear.depthStencil.depth);
      }
      cmd->state.dirty |= TU_CMD_DIRTY_LRZ;
   }
}
TU_GENX(tu_lrz_begin_resumed_renderpass);

template <chip CHIP>
void
tu_lrz_begin_renderpass(struct tu_cmd_buffer *cmd)
{
   const struct tu_render_pass *pass = cmd->state.pass;

   cmd->state.rp.lrz_disable_reason = "";
   cmd->state.rp.lrz_disabled_at_draw = 0;
   cmd->state.rp.lrz_write_disabled_at_draw = 0;

   int lrz_img_count = 0;
   for (unsigned i = 0; i < pass->attachment_count; i++) {
      if (cmd->state.attachments[i]->image->lrz_layout.lrz_total_size)
         lrz_img_count++;
   }

   if (cmd->device->physical_device->info->a6xx.has_lrz_dir_tracking &&
       cmd->state.pass->subpass_count > 1 && lrz_img_count > 1) {
      /* Theoretically we could switch between LRZ buffers during the binning
       * and tiling passes, but it is untested and would add complexity for
       * presumably extremely rare case.
       */
      tu_lrz_disable_reason(cmd, "Several subpasses with different depth attachments");

      for (unsigned i = 0; i < pass->attachment_count; i++) {
         struct tu_image *image = cmd->state.attachments[i]->image;
         tu_disable_lrz<CHIP>(cmd, &cmd->cs, image);
      }

      /* We need a valid LRZ fast-clear base, in case the render pass contents
       * are in secondaries that enable LRZ, so that they can read that LRZ is
       * dynamically disabled. It doesn't matter which we use, so just leave
       * the last one as emitted in tu_disable_lrz().
       */
      memset(&cmd->state.lrz, 0, sizeof(cmd->state.lrz));
      return;
   }

    /* Track LRZ valid state */
   tu_lrz_begin_resumed_renderpass<CHIP>(cmd);

   if (!cmd->state.lrz.valid_at_start) {
      tu6_write_lrz_cntl<CHIP>(cmd, &cmd->cs, {});
      tu6_emit_lrz_buffer<CHIP>(&cmd->cs, NULL);
   }
}
TU_GENX(tu_lrz_begin_renderpass);

void
tu_lrz_begin_secondary_cmdbuf(struct tu_cmd_buffer *cmd)
{
   memset(&cmd->state.lrz, 0, sizeof(cmd->state.lrz));
   uint32_t a = cmd->state.subpass->depth_stencil_attachment.attachment;
   if (a != VK_ATTACHMENT_UNUSED) {
      const struct tu_render_pass_attachment *att = &cmd->state.pass->attachments[a];
      tu_lrz_init_secondary(cmd, att);
   }
}

template <chip CHIP>
void
tu_lrz_tiling_begin(struct tu_cmd_buffer *cmd, struct tu_cs *cs)
{
   /* TODO: If lrz was never valid for the entire renderpass, we could exit
    * early here. Sometimes we know this ahead of time and null out
    * image_view, but with LOAD_OP_DONT_CARE this only happens if there were
    * no secondaries.
    */
   if (!cmd->state.lrz.image_view)
      return;

   struct tu_lrz_state *lrz = &cmd->state.lrz;

   tu6_emit_lrz_buffer<CHIP>(cs, lrz->image_view->image);

   if (lrz->reuse_previous_state) {
      /* Reuse previous LRZ state, LRZ cache is assumed to be
       * already invalidated by previous renderpass.
       */
      assert(lrz->gpu_dir_tracking);

      tu6_write_lrz_reg(cmd, cs,
         A6XX_GRAS_LRZ_VIEW_INFO(.dword = lrz->image_view->view.GRAS_LRZ_VIEW_INFO));
      return;
   }

   if (!lrz->valid_at_start) {
      /* If LRZ was never valid so disable it manually here.
       * This is accomplished by making later GRAS_LRZ_CNTL (in binning pass)
       * to fail the comparison of depth views.
       */
      if (lrz->gpu_dir_tracking) {
         tu6_disable_lrz_via_depth_view<CHIP>(cmd, cs);
         tu6_write_lrz_reg(cmd, cs, A6XX_GRAS_LRZ_VIEW_INFO(.dword = 0));
      }
      return;
   }

   if (lrz->fast_clear || lrz->gpu_dir_tracking) {
      if (lrz->gpu_dir_tracking) {
         tu6_write_lrz_reg(cmd, cs,
            A6XX_GRAS_LRZ_VIEW_INFO(.dword = lrz->image_view->view.GRAS_LRZ_VIEW_INFO));
      }

      tu6_write_lrz_cntl<CHIP>(cmd, cs, {
         .enable = true,
         .fc_enable = lrz->fast_clear,
         .disable_on_wrong_dir = lrz->gpu_dir_tracking,
      });

      /* LRZ_CLEAR.fc_enable + LRZ_CLEAR - clears fast-clear buffer;
       * LRZ_CLEAR.disable_on_wrong_dir + LRZ_CLEAR - sets direction to
       *  CUR_DIR_UNSET.
       */
      if (CHIP >= A7XX)
         tu_cs_emit_regs(cs, A7XX_GRAS_LRZ_DEPTH_CLEAR(lrz->depth_clear_value.depthStencil.depth));
      tu_emit_event_write<CHIP>(cmd, cs, FD_LRZ_CLEAR);
   }

   if (!lrz->fast_clear) {
      tu6_clear_lrz<CHIP>(cmd, cs, lrz->image_view->image, &lrz->depth_clear_value);
      /* Even though we disable fast-clear we still have to dirty
       * fast-clear buffer because both secondary cmdbufs and following
       * renderpasses won't know that fast-clear is disabled.
       *
       * TODO: we could avoid this if we don't store depth and don't
       * expect secondary cmdbufs.
       */
      if (lrz->image_view->image->lrz_layout.lrz_fc_size > 0) {
         tu6_dirty_lrz_fc<CHIP>(cmd, cs, lrz->image_view->image);
      }
   }
}
TU_GENX(tu_lrz_tiling_begin);

/* We need to re-emit LRZ state before each tile due to skipsaverestore.
 */
template <chip CHIP>
void
tu_lrz_before_tile(struct tu_cmd_buffer *cmd, struct tu_cs *cs)
{
   struct tu_lrz_state *lrz = &cmd->state.lrz;

   if (!lrz->image_view) {
      tu6_emit_lrz_buffer<CHIP>(cs, NULL);
   } else {
      tu6_emit_lrz_buffer<CHIP>(cs, lrz->image_view->image);

      if (lrz->gpu_dir_tracking) {
         if (!lrz->valid_at_start) {
            /* Make sure we fail the comparison of depth views */
            tu6_write_lrz_reg(cmd, cs, A6XX_GRAS_LRZ_VIEW_INFO(.dword = 0));
         } else {
            tu6_write_lrz_reg(cmd, cs,
               A6XX_GRAS_LRZ_VIEW_INFO(.dword = lrz->image_view->view.GRAS_LRZ_VIEW_INFO));
         }
      }
   }
}
TU_GENX(tu_lrz_before_tile);

template <chip CHIP>
void
tu_lrz_tiling_end(struct tu_cmd_buffer *cmd, struct tu_cs *cs)
{
   if (cmd->state.lrz.fast_clear || cmd->state.lrz.gpu_dir_tracking) {
      tu6_emit_lrz_buffer<CHIP>(cs, cmd->state.lrz.image_view->image);

      if (cmd->state.lrz.gpu_dir_tracking) {
         tu6_write_lrz_reg(cmd, &cmd->cs,
            A6XX_GRAS_LRZ_VIEW_INFO(.dword = cmd->state.lrz.image_view->view.GRAS_LRZ_VIEW_INFO));
      }

      /* Enable flushing of LRZ fast-clear and of direction buffer */
      tu6_write_lrz_cntl<CHIP>(cmd, cs, {
         .enable = true,
         .fc_enable = cmd->state.lrz.fast_clear,
         .disable_on_wrong_dir = cmd->state.lrz.gpu_dir_tracking,
      });
   } else {
      tu6_write_lrz_cntl<CHIP>(cmd, cs, {.enable = false});
   }

   /* If we haven't disabled LRZ during renderpass, we need to disable it here
    * for next renderpass to not use invalid LRZ values.
    */
   bool disable_for_next_rp = cmd->state.lrz.valid_at_start &&
       !cmd->state.lrz.valid;
   /* If the render pass writes depth (with direction) but doesn't set
    * direction on the GPU, the LRZ buffer cannot be used in subsequent
    * render passes because the direction information is lost.
    */
   if (!cmd->state.lrz.gpu_dir_set &&
       cmd->state.lrz.prev_direction != TU_LRZ_UNKNOWN) {
      disable_for_next_rp = true;
   }

   if (cmd->state.lrz.gpu_dir_tracking && disable_for_next_rp) {
      tu6_write_lrz_reg(cmd, cs, A6XX_GRAS_LRZ_VIEW_INFO(
         .base_layer = 0b11111111111,
         .layer_count = 0b11111111111,
         .base_mip_level = 0b1111,
      ));

      tu_emit_event_write<CHIP>(cmd, cs, FD_LRZ_CLEAR);
      tu_emit_event_write<CHIP>(cmd, cs, FD_LRZ_FLUSH);
   } else {
      tu_emit_event_write<CHIP>(cmd, cs, FD_LRZ_FLUSH);
   }
}
TU_GENX(tu_lrz_tiling_end);

template <chip CHIP>
void
tu_lrz_sysmem_begin(struct tu_cmd_buffer *cmd, struct tu_cs *cs)
{
   if (cmd->device->physical_device->info->a6xx.has_lrz_feedback) {
      tu_lrz_tiling_begin<CHIP>(cmd, cs);
      return;
   }

   if (!cmd->state.lrz.image_view)
      return;

   /* Actually, LRZ buffer could be filled in sysmem, in theory to
    * be used in another renderpass, but the benefit is rather dubious.
    */

   struct tu_lrz_state *lrz = &cmd->state.lrz;

   if (cmd->device->physical_device->info->a6xx.has_lrz_dir_tracking) {
      tu_disable_lrz<CHIP>(cmd, cs, lrz->image_view->image);
      /* Make sure depth view comparison will fail. */
      tu6_write_lrz_reg(cmd, cs,
         A6XX_GRAS_LRZ_VIEW_INFO(.dword = 0));
   } else {
      tu6_emit_lrz_buffer<CHIP>(cs, lrz->image_view->image);
      /* Even though we disable LRZ writes in sysmem mode - there is still
       * LRZ test, so LRZ should be cleared.
       */
      if (lrz->fast_clear) {
         tu6_write_lrz_cntl<CHIP>(cmd, &cmd->cs, {
            .enable = true,
            .fc_enable = true,
         });

         if (CHIP >= A7XX)
            tu_cs_emit_regs(cs, A7XX_GRAS_LRZ_DEPTH_CLEAR(lrz->depth_clear_value.depthStencil.depth));
         tu_emit_event_write<CHIP>(cmd, &cmd->cs, FD_LRZ_CLEAR);
         tu_emit_event_write<CHIP>(cmd, &cmd->cs, FD_LRZ_FLUSH);
      } else {
         tu6_clear_lrz<CHIP>(cmd, cs, lrz->image_view->image, &lrz->depth_clear_value);
      }
   }
}
TU_GENX(tu_lrz_sysmem_begin);

template <chip CHIP>
void
tu_lrz_sysmem_end(struct tu_cmd_buffer *cmd, struct tu_cs *cs)
{
   if (cmd->device->physical_device->info->a6xx.has_lrz_feedback) {
      tu_lrz_tiling_end<CHIP>(cmd, cs);
      return;
   }

   tu_emit_event_write<CHIP>(cmd, &cmd->cs, FD_LRZ_FLUSH);
}
TU_GENX(tu_lrz_sysmem_end);

/* Disable LRZ outside of renderpass. */
template <chip CHIP>
void
tu_disable_lrz(struct tu_cmd_buffer *cmd, struct tu_cs *cs,
               struct tu_image *image)
{
   if (!cmd->device->physical_device->info->a6xx.has_lrz_dir_tracking)
      return;

   if (!image->lrz_layout.lrz_total_size)
      return;

   tu6_emit_lrz_buffer<CHIP>(cs, image);
   tu6_disable_lrz_via_depth_view<CHIP>(cmd, cs);
}
TU_GENX(tu_disable_lrz);

/* Disable LRZ from the CPU, for host image copy */
template <chip CHIP>
void
tu_disable_lrz_cpu(struct tu_device *device, struct tu_image *image)
{
   if (!device->physical_device->info->a6xx.has_lrz_dir_tracking)
      return;

   if (!image->lrz_layout.lrz_total_size)
      return;

   const unsigned lrz_dir_offset = offsetof(fd_lrzfc_layout<CHIP>, dir_track);
   uint8_t *lrz_dir_tracking =
      (uint8_t *)image->map + image->lrz_layout.lrz_fc_offset + lrz_dir_offset;

   *lrz_dir_tracking = FD_LRZ_GPU_DIR_DISABLED;

   if (image->bo->cached_non_coherent) {
      tu_bo_sync_cache(
         device, image->bo,
         image->bo_offset + image->lrz_layout.lrz_offset + lrz_dir_offset, 1,
         TU_MEM_SYNC_CACHE_TO_GPU);
   }
}
TU_GENX(tu_disable_lrz_cpu);

/* Clear LRZ, used for out of renderpass depth clears. */
template <chip CHIP>
void
tu_lrz_clear_depth_image(struct tu_cmd_buffer *cmd,
                         struct tu_image *image,
                         const VkClearDepthStencilValue *pDepthStencil,
                         uint32_t rangeCount,
                         const VkImageSubresourceRange *pRanges)
{
   if (!rangeCount || !image->lrz_layout.lrz_total_size ||
       !cmd->device->physical_device->info->a6xx.has_lrz_dir_tracking)
      return;

   /* We cannot predict which depth subresource would be used later on,
    * so we just pick the first one with depth cleared and clear the LRZ.
    */
   const VkImageSubresourceRange *range = NULL;
   for (unsigned i = 0; i < rangeCount; i++) {
      if (pRanges[i].aspectMask &
            (VK_IMAGE_ASPECT_COLOR_BIT | VK_IMAGE_ASPECT_DEPTH_BIT)) {
         range = &pRanges[i];
         break;
      }
   }

   if (!range)
      return;

   bool fast_clear = image->lrz_layout.lrz_fc_size &&
                     tu_lrzfc_depth_supported<CHIP>(pDepthStencil->depth) &&
                     !TU_DEBUG(NOLRZFC);

   tu6_emit_lrz_buffer<CHIP>(&cmd->cs, image);

   tu6_write_lrz_reg(cmd, &cmd->cs, A6XX_GRAS_LRZ_VIEW_INFO(
         .base_layer = range->baseArrayLayer,
         .layer_count = vk_image_subresource_layer_count(&image->vk, range),
         .base_mip_level = range->baseMipLevel,
   ));

   tu6_write_lrz_cntl<CHIP>(cmd, &cmd->cs, {
      .enable = true,
      .fc_enable = fast_clear,
      .disable_on_wrong_dir = true,
   });

   if (CHIP >= A7XX)
      tu_cs_emit_regs(&cmd->cs, A7XX_GRAS_LRZ_DEPTH_CLEAR(pDepthStencil->depth));
   tu_emit_event_write<CHIP>(cmd, &cmd->cs, FD_LRZ_CLEAR);
   tu_emit_event_write<CHIP>(cmd, &cmd->cs, FD_LRZ_FLUSH);

   if (!fast_clear) {
      tu6_clear_lrz<CHIP>(cmd, &cmd->cs, image, (const VkClearValue*) pDepthStencil);
   }
}
TU_GENX(tu_lrz_clear_depth_image);

template <chip CHIP>
void
tu_lrz_disable_during_renderpass(struct tu_cmd_buffer *cmd,
                                 const char *reason)
{
   assert(cmd->state.pass);

   tu_lrz_disable_reason(cmd, reason);

   cmd->state.lrz.valid = false;
   cmd->state.dirty |= TU_CMD_DIRTY_LRZ;
}
TU_GENX(tu_lrz_disable_during_renderpass);

template <chip CHIP>
void
tu_lrz_flush_valid_during_renderpass(struct tu_cmd_buffer *cmd,
                                     struct tu_cs *cs)
{
   /* Even if state is valid, we cannot be sure that secondary
    * command buffer has the same sticky disable_write_for_rp.
    */
   if (cmd->state.lrz.valid && !cmd->state.lrz.disable_write_for_rp)
      return;

   tu6_write_lrz_reg(cmd, cs, A6XX_GRAS_LRZ_VIEW_INFO(
      .base_layer = 0b11111111111,
      .layer_count = 0b11111111111,
      .base_mip_level = 0b1111,
   ));
}
TU_GENX(tu_lrz_flush_valid_during_renderpass);

template <chip CHIP>
static struct A6XX_GRAS_LRZ_CNTL
tu6_calculate_lrz_state(struct tu_cmd_buffer *cmd,
                        const uint32_t a)
{
   const struct tu_shader *fs = cmd->state.shaders[MESA_SHADER_FRAGMENT];
   bool z_test_enable = cmd->vk.dynamic_graphics_state.ds.depth.test_enable;
   bool z_write_enable = cmd->vk.dynamic_graphics_state.ds.depth.write_enable;
   bool z_bounds_enable = cmd->vk.dynamic_graphics_state.ds.depth.bounds_test.enable;
   VkCompareOp depth_compare_op =
      cmd->vk.dynamic_graphics_state.ds.depth.compare_op;

   struct A6XX_GRAS_LRZ_CNTL gras_lrz_cntl = { 0 };

   cmd->state.lrz.force_late_z =
      fs->variant->writes_pos && !fs->variant->fs.early_fragment_tests;

   if (!cmd->state.lrz.valid) {
      return gras_lrz_cntl;
   }

   /* If depth test is disabled we shouldn't touch LRZ.
    * Same if there is no depth attachment.
    */
   if (a == VK_ATTACHMENT_UNUSED || !z_test_enable || !cmd->device->use_lrz)
      return gras_lrz_cntl;

   if (!cmd->state.lrz.gpu_dir_tracking && !cmd->state.attachments) {
      /* Without on-gpu LRZ direction tracking - there is nothing we
       * can do to enable LRZ in secondary command buffers.
       */
      return gras_lrz_cntl;
   }

   /* See comment in tu_pipeline about disabling LRZ write for blending. */
   bool reads_dest = cmd->state.blend_reads_dest;

   gras_lrz_cntl.enable = true;
   gras_lrz_cntl.lrz_write =
      z_write_enable &&
      !(fs->fs.lrz.status & TU_LRZ_FORCE_DISABLE_WRITE);
   gras_lrz_cntl.z_write_enable = z_write_enable;
   gras_lrz_cntl.z_bounds_enable = z_bounds_enable;
   gras_lrz_cntl.fc_enable = cmd->state.lrz.fast_clear;
   gras_lrz_cntl.dir_write = cmd->state.lrz.gpu_dir_tracking;
   gras_lrz_cntl.disable_on_wrong_dir = cmd->state.lrz.gpu_dir_tracking;

   if (CHIP >= A7XX)
      gras_lrz_cntl.z_func = tu6_compare_func(depth_compare_op);

   /* LRZ is disabled until it is cleared, which means that one "wrong"
    * depth test or shader could disable LRZ until depth buffer is cleared.
    */
   bool disable_lrz = false;
   bool temporary_disable_lrz = false;

   /* What happens in FS could affect LRZ, e.g.: writes to gl_FragDepth or early
    * fragment tests.  We have to skip LRZ testing and updating, but as long as
    * the depth direction stayed the same we can continue with LRZ testing later.
    */
   bool disable_lrz_due_to_fs = fs->fs.lrz.status & TU_LRZ_FORCE_DISABLE_LRZ;

   /* Specifying depth write direction in shader may help us. E.g.
    * If depth test is GREATER and FS specifies FRAG_DEPTH_LAYOUT_LESS
    * it means that LRZ won't kill any fragment that shouldn't be killed,
    * in other words, FS can only reduce the depth value which could
    * make fragment to NOT pass with GREATER depth test. We just have to
    * enable late Z test.
    */
   if (!disable_lrz_due_to_fs && fs->variant->writes_pos &&
       !fs->variant->fs.early_fragment_tests &&
       !cmd->device->instance->ignore_frag_depth_direction) {
      if (fs->variant->fs.depth_layout == FRAG_DEPTH_LAYOUT_NONE ||
          fs->variant->fs.depth_layout == FRAG_DEPTH_LAYOUT_ANY) {
         disable_lrz_due_to_fs = true;
      } else {
         if (fs->variant->fs.depth_layout == FRAG_DEPTH_LAYOUT_GREATER) {
            disable_lrz_due_to_fs =
               depth_compare_op != VK_COMPARE_OP_LESS &&
               depth_compare_op != VK_COMPARE_OP_LESS_OR_EQUAL;
         } else if (fs->variant->fs.depth_layout == FRAG_DEPTH_LAYOUT_LESS) {
            disable_lrz_due_to_fs =
               depth_compare_op != VK_COMPARE_OP_GREATER &&
               depth_compare_op != VK_COMPARE_OP_GREATER_OR_EQUAL;
         }
         /* FRAG_DEPTH_LAYOUT_UNCHANGED is always OK.*/
      }

      cmd->state.lrz.force_late_z = disable_lrz_due_to_fs;
   } else if (fs->variant->writes_pos && !fs->variant->fs.early_fragment_tests) {
      disable_lrz_due_to_fs = true;
      cmd->state.lrz.force_late_z = true;
   } else {
      cmd->state.lrz.force_late_z = false;
   }

   if (disable_lrz_due_to_fs) {
      if (cmd->state.lrz.prev_direction != TU_LRZ_UNKNOWN || !cmd->state.lrz.gpu_dir_tracking) {
         perf_debug(cmd->device, "Skipping LRZ due to FS");
         temporary_disable_lrz = true;
      } else {
         tu_lrz_disable_reason(cmd, "FS writes depth or has side-effects (TODO: fix for gpu-direction-tracking case)");
         disable_lrz = true;
      }
   }

   /* If Z is not written - it doesn't affect LRZ buffer state.
    * Which means two things:
    * - Don't lock direction until Z is written for the first time;
    * - If Z isn't written and direction IS locked it's possible to just
    *   temporary disable LRZ instead of fully bailing out, when direction
    *   is changed.
    */

   enum tu_lrz_direction lrz_direction = TU_LRZ_UNKNOWN;
   switch (depth_compare_op) {
   case VK_COMPARE_OP_ALWAYS:
   case VK_COMPARE_OP_NOT_EQUAL:
      /* OP_ALWAYS and OP_NOT_EQUAL could have depth value of any direction,
       * so if there is a depth write - LRZ must be disabled.
       */
      if (z_write_enable) {
         tu_lrz_disable_reason(cmd, "Depth write + ALWAYS/NOT_EQUAL");
         disable_lrz = true;
         gras_lrz_cntl.dir = LRZ_DIR_INVALID;
      } else {
         perf_debug(cmd->device, "Skipping LRZ due to ALWAYS/NOT_EQUAL");
         temporary_disable_lrz = true;
      }
      break;
   case VK_COMPARE_OP_EQUAL:
   case VK_COMPARE_OP_NEVER:
      /* Blob disables LRZ for OP_EQUAL, and from our empirical
       * evidence it is a right thing to do.
       *
       * Both OP_EQUAL and OP_NEVER don't change LRZ buffer so
       * we could just temporary disable LRZ.
       */
      temporary_disable_lrz = true;
      break;
   case VK_COMPARE_OP_GREATER:
   case VK_COMPARE_OP_GREATER_OR_EQUAL:
      lrz_direction = TU_LRZ_GREATER;
      gras_lrz_cntl.greater = true;
      gras_lrz_cntl.dir = LRZ_DIR_GE;
      break;
   case VK_COMPARE_OP_LESS:
   case VK_COMPARE_OP_LESS_OR_EQUAL:
      lrz_direction = TU_LRZ_LESS;
      gras_lrz_cntl.greater = false;
      gras_lrz_cntl.dir = LRZ_DIR_LE;
      break;
   default:
      unreachable("bad VK_COMPARE_OP value or uninitialized");
      break;
   };

   /* If depthfunc direction is changed, bail out on using LRZ. The
    * LRZ buffer encodes a min/max depth value per block, but if
    * we switch from GT/GE <-> LT/LE, those values cannot be
    * interpreted properly.
    */
   if (cmd->state.lrz.prev_direction != TU_LRZ_UNKNOWN &&
       lrz_direction != TU_LRZ_UNKNOWN &&
       cmd->state.lrz.prev_direction != lrz_direction) {
      if (z_write_enable) {
         tu_lrz_disable_reason(cmd, "Depth write + compare-op direction change");
         disable_lrz = true;
      } else {
         perf_debug(cmd->device, "Skipping LRZ due to direction change");
         temporary_disable_lrz = true;
      }
   }

   /* Consider the following sequence of depthfunc changes:
    *
    * - COMPARE_OP_GREATER -> COMPARE_OP_EQUAL -> COMPARE_OP_GREATER
    * LRZ is disabled during COMPARE_OP_EQUAL but could be enabled
    * during second VK_COMPARE_OP_GREATER.
    *
    * - COMPARE_OP_GREATER -> COMPARE_OP_EQUAL -> COMPARE_OP_LESS
    * Here, LRZ is disabled during COMPARE_OP_EQUAL and should become
    * invalid during COMPARE_OP_LESS.
    *
    * This shows that we should keep last KNOWN direction.
    */
   if (z_write_enable && lrz_direction != TU_LRZ_UNKNOWN)
      cmd->state.lrz.prev_direction = lrz_direction;

   if (cmd->vk.dynamic_graphics_state.ds.stencil.test_enable) {
      bool frag_may_be_killed_by_stencil =
         !(cmd->vk.dynamic_graphics_state.ds.stencil.front.op.compare ==
              VK_COMPARE_OP_ALWAYS &&
           cmd->vk.dynamic_graphics_state.ds.stencil.back.op.compare ==
              VK_COMPARE_OP_ALWAYS);

      /* Stencil test happens after LRZ is written, so if stencil could kill
       * the fragment - we cannot write LRZ.
       */
      if (!cmd->state.lrz.disable_write_for_rp &&
          frag_may_be_killed_by_stencil) {
         tu_lrz_write_disable_reason(cmd, "Stencil may kill fragments");
         cmd->state.lrz.disable_write_for_rp = true;
      }

      /* Because the LRZ test runs first, failing the LRZ test may result in
       * skipping the stencil test and subsequent stencil write. This is ok if
       * stencil is only written when the depth test passes, because then the
       * LRZ test will also pass, but if it may be written when the depth or
       * stencil test fails then we need to disable the LRZ test for the draw.
       */
      if (cmd->state.stencil_written_on_depth_fail)
         temporary_disable_lrz = true;
   }

   /* Writing depth with blend enabled means we need to invalidate LRZ,
    * because the written depth value could mean that a later draw with
    * depth enabled (where we would otherwise write LRZ) could have
    * fragments which don't pass the depth test due to this draw.  For
    * example, consider this sequence of draws, with depth mode GREATER:
    *
    *   draw A:
    *     z=0.1, fragments pass
    *   draw B:
    *     z=0.4, fragments pass
    *     blend enabled (LRZ write disabled)
    *     depth write enabled
    *   draw C:
    *     z=0.2, fragments don't pass
    *     blend disabled
    *     depth write enabled
    *
    * Normally looking at the state in draw C, we'd assume we could
    * enable LRZ write.  But this would cause early-z/lrz to discard
    * fragments from draw A which should be visible due to draw B.
    */
   if (reads_dest && z_write_enable && cmd->device->instance->conservative_lrz) {
      if (!cmd->state.lrz.disable_write_for_rp) {
         tu_lrz_write_disable_reason(cmd, "Depth write + blending");
         cmd->state.lrz.disable_write_for_rp = true;
      }
   }

   if (disable_lrz)
      cmd->state.lrz.valid = false;

   if (cmd->state.lrz.disable_write_for_rp)
      gras_lrz_cntl.lrz_write = false;

   if (temporary_disable_lrz)
      gras_lrz_cntl.enable = false;

   cmd->state.lrz.enabled = cmd->state.lrz.valid && gras_lrz_cntl.enable;
   if (!cmd->state.lrz.enabled)
      memset(&gras_lrz_cntl, 0, sizeof(gras_lrz_cntl));

   if (cmd->state.lrz.enabled && gras_lrz_cntl.lrz_write &&
       cmd->state.lrz.prev_direction != TU_LRZ_UNKNOWN) {
      cmd->state.lrz.gpu_dir_set = true;
   }

   return gras_lrz_cntl;
}

template <chip CHIP>
void
tu6_emit_lrz(struct tu_cmd_buffer *cmd, struct tu_cs *cs)
{
   const uint32_t a = cmd->state.subpass->depth_stencil_attachment.attachment;
   struct A6XX_GRAS_LRZ_CNTL gras_lrz_cntl = tu6_calculate_lrz_state<CHIP>(cmd, a);

   tu6_write_lrz_cntl<CHIP>(cmd, cs, gras_lrz_cntl);
   tu_cs_emit_regs(cs, A6XX_RB_LRZ_CNTL(.enable = gras_lrz_cntl.enable));
}
TU_GENX(tu6_emit_lrz);
