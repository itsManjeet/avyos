/*
 * Copyright 2023 Alyssa Rosenzweig
 * Copyright 2023 Valve Corporation
 * SPDX-License-Identifier: MIT
 */

#include "asahi/lib/agx_abi.h"
#include "compiler/libcl/libcl_vk.h"
#include "util/macros.h"
#include "util/u_math.h"
#include "geometry.h"
#include "libagx_intrinsics.h"
#include "query.h"
#include "tessellator.h"

/* Swap the two non-provoking vertices in odd triangles. This generates a vertex
 * ID list with a consistent winding order.
 *
 * Holding prim and flatshade_first constant, the map : [0, 1, 2] -> [0, 1, 2]
 * is its own inverse. It is hence used both vertex fetch and transform
 * feedback.
 */
uint
libagx_map_vertex_in_tri_strip(uint prim, uint vert, bool flatshade_first)
{
   unsigned pv = flatshade_first ? 0 : 2;

   bool even = (prim & 1) == 0;
   bool provoking = vert == pv;

   return (provoking || even) ? vert : ((3 - pv) - vert);
}

static inline uint
xfb_prim(uint id, uint n, uint copy)
{
   return sub_sat(id, n - 1u) + copy;
}

/*
 * Determine whether an output vertex has an n'th copy in the transform feedback
 * buffer. This is written weirdly to let constant folding remove unnecessary
 * stores when length is known statically.
 */
bool
libagx_xfb_vertex_copy_in_strip(uint n, uint id, uint length, uint copy)
{
   uint prim = xfb_prim(id, n, copy);

   int num_prims = length - (n - 1);
   return copy == 0 || (prim < num_prims && id >= copy && copy < num_prims);
}

uint
libagx_xfb_vertex_offset(uint n, uint invocation_base_prim,
                         uint strip_base_prim, uint id_in_strip, uint copy,
                         bool flatshade_first)
{
   uint prim = xfb_prim(id_in_strip, n, copy);
   uint vert_0 = min(id_in_strip, n - 1);
   uint vert = vert_0 - copy;

   if (n == 3) {
      vert = libagx_map_vertex_in_tri_strip(prim, vert, flatshade_first);
   }

   /* Tally up in the whole buffer */
   uint base_prim = invocation_base_prim + strip_base_prim;
   uint base_vertex = base_prim * n;
   return base_vertex + (prim * n) + vert;
}

uint64_t
libagx_xfb_vertex_address(constant struct agx_geometry_params *p, uint index,
                          uint buffer, uint stride, uint output_offset)
{
   uint xfb_offset = (index * stride) + output_offset;

   return (uintptr_t)(p->xfb_base[buffer]) + xfb_offset;
}

uint
libagx_vertex_id_for_line_loop(uint prim, uint vert, uint num_prims)
{
   /* (0, 1), (1, 2), (2, 0) */
   if (prim == (num_prims - 1) && vert == 1)
      return 0;
   else
      return prim + vert;
}

uint
libagx_vertex_id_for_line_class(enum mesa_prim mode, uint prim, uint vert,
                                uint num_prims)
{
   /* Line list, line strip, or line loop */
   if (mode == MESA_PRIM_LINE_LOOP && prim == (num_prims - 1) && vert == 1)
      return 0;

   if (mode == MESA_PRIM_LINES)
      prim *= 2;

   return prim + vert;
}

uint
libagx_vertex_id_for_tri_fan(uint prim, uint vert, bool flatshade_first)
{
   /* Vulkan spec section 20.1.7 gives (i + 1, i + 2, 0) for a provoking
    * first. OpenGL instead wants (0, i + 1, i + 2) with a provoking last.
    * Piglit clipflat expects us to switch between these orders depending on
    * provoking vertex, to avoid trivializing the fan.
    *
    * Rotate accordingly.
    */
   if (flatshade_first) {
      vert = (vert == 2) ? 0 : (vert + 1);
   }

   /* The simpler form assuming last is provoking. */
   return (vert == 0) ? 0 : prim + vert;
}

uint
libagx_vertex_id_for_tri_class(enum mesa_prim mode, uint prim, uint vert,
                               bool flatshade_first)
{
   if (flatshade_first && mode == MESA_PRIM_TRIANGLE_FAN) {
      vert = vert + 1;
      vert = (vert == 3) ? 0 : vert;
   }

   if (mode == MESA_PRIM_TRIANGLE_FAN && vert == 0)
      return 0;

   if (mode == MESA_PRIM_TRIANGLES)
      prim *= 3;

   /* Triangle list, triangle strip, or triangle fan */
   if (mode == MESA_PRIM_TRIANGLE_STRIP) {
      unsigned pv = flatshade_first ? 0 : 2;

      bool even = (prim & 1) == 0;
      bool provoking = vert == pv;

      vert = ((provoking || even) ? vert : ((3 - pv) - vert));
   }

   return prim + vert;
}

uint
libagx_vertex_id_for_line_adj_class(enum mesa_prim mode, uint prim, uint vert)
{
   /* Line list adj or line strip adj */
   if (mode == MESA_PRIM_LINES_ADJACENCY)
      prim *= 4;

   return prim + vert;
}

uint
libagx_vertex_id_for_tri_strip_adj(uint prim, uint vert, uint num_prims,
                                   bool flatshade_first)
{
   /* See Vulkan spec section 20.1.11 "Triangle Strips With Adjancency".
    *
    * There are different cases for first/middle/last/only primitives and for
    * odd/even primitives.  Determine which case we're in.
    */
   bool last = prim == (num_prims - 1);
   bool first = prim == 0;
   bool even = (prim & 1) == 0;
   bool even_or_first = even || first;

   /* When the last vertex is provoking, we rotate the primitives
    * accordingly. This seems required for OpenGL.
    */
   if (!flatshade_first && !even_or_first) {
      vert = (vert + 4u) % 6u;
   }

   /* Offsets per the spec. The spec lists 6 cases with 6 offsets. Luckily,
    * there are lots of patterns we can exploit, avoiding a full 6x6 LUT.
    *
    * Here we assume the first vertex is provoking, the Vulkan default.
    */
   uint offsets[6] = {
      0,
      first ? 1 : (even ? -2 : 3),
      even_or_first ? 2 : 4,
      last ? 5 : 6,
      even_or_first ? 4 : 2,
      even_or_first ? 3 : -2,
   };

   /* Ensure NIR can see thru the local array */
   uint offset = 0;
   for (uint i = 1; i < 6; ++i) {
      if (i == vert)
         offset = offsets[i];
   }

   /* Finally add to the base of the primitive */
   return (prim * 2) + offset;
}

uint
libagx_vertex_id_for_tri_adj_class(enum mesa_prim mode, uint prim, uint vert,
                                   uint nr, bool flatshade_first)
{
   /* Tri adj list or tri adj strip */
   if (mode == MESA_PRIM_TRIANGLE_STRIP_ADJACENCY) {
      return libagx_vertex_id_for_tri_strip_adj(prim, vert, nr,
                                                flatshade_first);
   } else {
      return (6 * prim) + vert;
   }
}

static uint
vertex_id_for_topology(enum mesa_prim mode, bool flatshade_first, uint prim,
                       uint vert, uint num_prims)
{
   switch (mode) {
   case MESA_PRIM_POINTS:
   case MESA_PRIM_LINES:
   case MESA_PRIM_TRIANGLES:
   case MESA_PRIM_LINES_ADJACENCY:
   case MESA_PRIM_TRIANGLES_ADJACENCY:
      /* Regular primitive: every N vertices defines a primitive */
      return (prim * mesa_vertices_per_prim(mode)) + vert;

   case MESA_PRIM_LINE_LOOP:
      return libagx_vertex_id_for_line_loop(prim, vert, num_prims);

   case MESA_PRIM_LINE_STRIP:
   case MESA_PRIM_LINE_STRIP_ADJACENCY:
      /* (i, i + 1) or (i, ..., i + 3) */
      return prim + vert;

   case MESA_PRIM_TRIANGLE_STRIP: {
      /* Order depends on the provoking vert.
       *
       * First: (0, 1, 2), (1, 3, 2), (2, 3, 4).
       * Last:  (0, 1, 2), (2, 1, 3), (2, 3, 4).
       *
       * Pull the (maybe swapped) vert from the corresponding primitive
       */
      return prim + libagx_map_vertex_in_tri_strip(prim, vert, flatshade_first);
   }

   case MESA_PRIM_TRIANGLE_FAN:
      return libagx_vertex_id_for_tri_fan(prim, vert, flatshade_first);

   case MESA_PRIM_TRIANGLE_STRIP_ADJACENCY:
      return libagx_vertex_id_for_tri_strip_adj(prim, vert, num_prims,
                                                flatshade_first);

   default:
      return 0;
   }
}

uint
libagx_map_to_line_adj(uint id)
{
   /* Sequence (1, 2), (5, 6), (9, 10), ... */
   return ((id & ~1) * 2) + (id & 1) + 1;
}

uint
libagx_map_to_line_strip_adj(uint id)
{
   /* Sequence (1, 2), (2, 3), (4, 5), .. */
   uint prim = id / 2;
   uint vert = id & 1;
   return prim + vert + 1;
}

uint
libagx_map_to_tri_strip_adj(uint id)
{
   /* Sequence (0, 2, 4), (2, 6, 4), (4, 6, 8), (6, 10, 8)
    *
    * Although tri strips with adjacency have 6 cases in general, after
    * disregarding the vertices only available in a geometry shader, there are
    * only even/odd cases. In other words, it's just a triangle strip subject to
    * extra padding.
    *
    * Dividing through by two, the sequence is:
    *
    *   (0, 1, 2), (1, 3, 2), (2, 3, 4), (3, 5, 4)
    */
   uint prim = id / 3;
   uint vtx = id % 3;

   /* Flip the winding order of odd triangles */
   if ((prim % 2) == 1) {
      if (vtx == 1)
         vtx = 2;
      else if (vtx == 2)
         vtx = 1;
   }

   return 2 * (prim + vtx);
}

static void
store_index(uintptr_t index_buffer, uint index_size_B, uint id, uint value)
{
   global uint32_t *out_32 = (global uint32_t *)index_buffer;
   global uint16_t *out_16 = (global uint16_t *)index_buffer;
   global uint8_t *out_8 = (global uint8_t *)index_buffer;

   if (index_size_B == 4)
      out_32[id] = value;
   else if (index_size_B == 2)
      out_16[id] = value;
   else
      out_8[id] = value;
}

static uint
load_index(uintptr_t index_buffer, uint32_t index_buffer_range_el, uint id,
           uint index_size)
{
   bool oob = id >= index_buffer_range_el;

   /* If the load would be out-of-bounds, load the first element which is
    * assumed valid. If the application index buffer is empty with robustness2,
    * index_buffer will point to a zero sink where only the first is valid.
    */
   if (oob) {
      id = 0;
   }

   uint el;
   if (index_size == 1) {
      el = ((constant uint8_t *)index_buffer)[id];
   } else if (index_size == 2) {
      el = ((constant uint16_t *)index_buffer)[id];
   } else {
      el = ((constant uint32_t *)index_buffer)[id];
   }

   /* D3D robustness semantics. TODO: Optimize? */
   if (oob) {
      el = 0;
   }

   return el;
}

uint
libagx_load_index_buffer(constant struct agx_ia_state *p, uint id,
                         uint index_size)
{
   return load_index(p->index_buffer, p->index_buffer_range_el, id, index_size);
}

static void
increment_counters(global uint32_t *a, global uint32_t *b, global uint32_t *c,
                   uint count)
{
   global uint32_t *ptr[] = {a, b, c};

   for (uint i = 0; i < 3; ++i) {
      if (ptr[i]) {
         *(ptr[i]) += count;
      }
   }
}

static unsigned
decomposed_prims_for_vertices_with_tess(enum mesa_prim prim, int vertices,
                                        unsigned verts_per_patch)
{
   if (prim >= MESA_PRIM_PATCHES) {
      return vertices / verts_per_patch;
   } else {
      return u_decomposed_prims_for_vertices(prim, vertices);
   }
}

KERNEL(1)
libagx_increment_ia(global uint32_t *ia_vertices,
                    global uint32_t *ia_primitives,
                    global uint32_t *vs_invocations, global uint32_t *c_prims,
                    global uint32_t *c_invs, constant uint32_t *draw,
                    enum mesa_prim prim, unsigned verts_per_patch)
{
   increment_counters(ia_vertices, vs_invocations, NULL, draw[0] * draw[1]);

   uint prims =
      decomposed_prims_for_vertices_with_tess(prim, draw[0], verts_per_patch) *
      draw[1];

   increment_counters(ia_primitives, c_prims, c_invs, prims);
}

KERNEL(1024)
libagx_increment_ia_restart(global uint32_t *ia_vertices,
                            global uint32_t *ia_primitives,
                            global uint32_t *vs_invocations,
                            global uint32_t *c_prims, global uint32_t *c_invs,
                            constant uint32_t *draw, uint64_t index_buffer,
                            uint32_t index_buffer_range_el,
                            uint32_t restart_index, uint32_t index_size_B,
                            enum mesa_prim prim, unsigned verts_per_patch)
{
   uint tid = cl_global_id.x;
   unsigned count = draw[0];
   local uint scratch;

   uint start = draw[2];
   uint partial = 0;

   /* Count non-restart indices */
   for (uint i = tid; i < count; i += 1024) {
      uint index = load_index(index_buffer, index_buffer_range_el, start + i,
                              index_size_B);

      if (index != restart_index)
         partial++;
   }

   /* Accumulate the partials across the workgroup */
   scratch = 0;
   barrier(CLK_LOCAL_MEM_FENCE);
   atomic_add(&scratch, partial);
   barrier(CLK_LOCAL_MEM_FENCE);

   /* Elect a single thread from the workgroup to increment the counters */
   if (tid == 0) {
      increment_counters(ia_vertices, vs_invocations, NULL, scratch * draw[1]);
   }

   /* TODO: We should vectorize this */
   if ((ia_primitives || c_prims || c_invs) && tid == 0) {
      uint accum = 0;
      int last_restart = -1;
      for (uint i = 0; i < count; ++i) {
         uint index = load_index(index_buffer, index_buffer_range_el, start + i,
                                 index_size_B);

         if (index == restart_index) {
            accum += decomposed_prims_for_vertices_with_tess(
               prim, i - last_restart - 1, verts_per_patch);
            last_restart = i;
         }
      }

      {
         accum += decomposed_prims_for_vertices_with_tess(
            prim, count - last_restart - 1, verts_per_patch);
      }

      increment_counters(ia_primitives, c_prims, c_invs, accum * draw[1]);
   }
}

/*
 * Return the ID of the first thread in the workgroup where cond is true, or
 * 1024 if cond is false across the workgroup.
 */
static uint
first_true_thread_in_workgroup(bool cond, local uint *scratch)
{
   barrier(CLK_LOCAL_MEM_FENCE);
   scratch[get_sub_group_id()] = sub_group_ballot(cond)[0];
   barrier(CLK_LOCAL_MEM_FENCE);

   uint first_group =
      ctz(sub_group_ballot(scratch[get_sub_group_local_id()])[0]);
   uint off = ctz(first_group < 32 ? scratch[first_group] : 0);
   return (first_group * 32) + off;
}

/*
 * When unrolling the index buffer for a draw, we translate the old indirect
 * draws to new indirect draws. This routine allocates the new index buffer and
 * sets up most of the new draw descriptor.
 */
static global void *
setup_unroll_for_draw(global struct agx_heap *heap, constant uint *in_draw,
                      global uint *out, enum mesa_prim mode, uint index_size_B)
{
   /* Determine an upper bound on the memory required for the index buffer.
    * Restarts only decrease the unrolled index buffer size, so the maximum size
    * is the unrolled size when the input has no restarts.
    */
   uint max_prims = u_decomposed_prims_for_vertices(mode, in_draw[0]);
   uint max_verts = max_prims * mesa_vertices_per_prim(mode);
   uint alloc_size = max_verts * index_size_B;

   /* Allocate unrolled index buffer.
    *
    * TODO: For multidraw, should be atomic. But multidraw+unroll isn't
    * currently wired up in any driver.
    */
   uint old_heap_bottom_B = agx_heap_alloc_nonatomic_offs(heap, alloc_size);

   /* Setup most of the descriptor. Count will be determined after unroll. */
   out[1] = in_draw[1];                       /* instance count */
   out[2] = old_heap_bottom_B / index_size_B; /* index offset */
   out[3] = in_draw[3];                       /* index bias */
   out[4] = in_draw[4];                       /* base instance */

   /* Return the index buffer we allocated */
   return (global uchar *)heap->base + old_heap_bottom_B;
}

KERNEL(1024)
libagx_unroll_restart(global struct agx_heap *heap, uint64_t index_buffer,
                      constant uint *in_draw, global uint32_t *out_draw,
                      uint32_t max_draws, uint32_t restart_index,
                      uint32_t index_buffer_size_el, uint32_t index_size_log2,
                      uint32_t flatshade_first, uint mode__11)
{
   uint32_t index_size_B = 1 << index_size_log2;
   enum mesa_prim mode = libagx_uncompact_prim(mode__11);
   uint tid = cl_local_id.x;
   uint count = in_draw[0];

   local uintptr_t out_ptr;
   if (tid == 0) {
      out_ptr = (uintptr_t)setup_unroll_for_draw(heap, in_draw, out_draw, mode,
                                                 index_size_B);
   }

   barrier(CLK_LOCAL_MEM_FENCE);

   uintptr_t in_ptr = (uintptr_t)(libagx_index_buffer(
      index_buffer, index_buffer_size_el, in_draw[2], index_size_B));

   local uint scratch[32];

   uint out_prims = 0;
   uint needle = 0;
   uint per_prim = mesa_vertices_per_prim(mode);
   while (needle < count) {
      /* Search for next restart or the end. Lanes load in parallel. */
      uint next_restart = needle;
      for (;;) {
         uint idx = next_restart + tid;
         bool restart =
            idx >= count || load_index(in_ptr, index_buffer_size_el, idx,
                                       index_size_B) == restart_index;

         uint next_offs = first_true_thread_in_workgroup(restart, scratch);

         next_restart += next_offs;
         if (next_offs < 1024)
            break;
      }

      /* Emit up to the next restart. Lanes output in parallel */
      uint subcount = next_restart - needle;
      uint subprims = u_decomposed_prims_for_vertices(mode, subcount);
      uint out_prims_base = out_prims;
      for (uint i = tid; i < subprims; i += 1024) {
         for (uint vtx = 0; vtx < per_prim; ++vtx) {
            uint id =
               vertex_id_for_topology(mode, flatshade_first, i, vtx, subprims);
            uint offset = needle + id;

            uint x = ((out_prims_base + i) * per_prim) + vtx;
            uint y =
               load_index(in_ptr, index_buffer_size_el, offset, index_size_B);

            store_index(out_ptr, index_size_B, x, y);
         }
      }

      out_prims += subprims;
      needle = next_restart + 1;
   }

   if (tid == 0)
      out_draw[0] = out_prims * per_prim;
}

uint
libagx_setup_xfb_buffer(global struct agx_geometry_params *p, uint i,
                        uint stride, uint max_output_end,
                        uint vertices_per_prim)
{
   uint xfb_offset = *(p->xfb_offs_ptrs[i]);
   p->xfb_base[i] = p->xfb_base_original[i] + xfb_offset;

   /* Let output_end = output_offset + output_size.
    *
    * Primitive P will write up to (but not including) offset:
    *
    *    xfb_offset + ((P - 1) * (verts_per_prim * stride))
    *               + ((verts_per_prim - 1) * stride)
    *               + output_end
    *
    * To fit all outputs for P, that value must be less than the XFB
    * buffer size for the output with maximal output_end, as everything
    * else is constant here across outputs within a buffer/primitive:
    *
    *    floor(P) <= (stride + size - xfb_offset - output_end)
    *                 // (stride * verts_per_prim)
    */
   int numer_s = p->xfb_size[i] + (stride - max_output_end) - xfb_offset;
   uint numer = max(numer_s, 0);
   return numer / (stride * vertices_per_prim);
}

void
libagx_write_strip(GLOBAL uint32_t *index_buffer, uint32_t inv_index_offset,
                   uint32_t prim_index_offset, uint32_t vertex_offset,
                   uint32_t verts_in_prim, uint3 info)
{
   _libagx_write_strip(index_buffer, inv_index_offset + prim_index_offset,
                       vertex_offset, verts_in_prim, info.x, info.y, info.z);
}

void
libagx_pad_index_gs(global int *index_buffer, uint inv_index_offset,
                    uint nr_indices, uint alloc)
{
   for (uint i = nr_indices; i < alloc; ++i) {
      index_buffer[inv_index_offset + i] = -1;
   }
}

KERNEL(1)
libagx_gs_setup_indirect(
   uint64_t index_buffer, constant uint *draw,
   global uintptr_t *vertex_buffer /* output */,
   global struct agx_ia_state *ia /* output */,
   global struct agx_geometry_params *p /* output */,
   global struct agx_heap *heap,
   uint64_t vs_outputs /* Vertex (TES) output mask */,
   uint32_t index_size_B /* 0 if no index bffer */,
   uint32_t index_buffer_range_el,
   uint32_t prim /* Input primitive type, enum mesa_prim */,
   int is_prefix_summing, uint max_indices, enum agx_gs_shape shape)
{
   /* Determine the (primitives, instances) grid size. */
   uint vertex_count = draw[0];
   uint instance_count = draw[1];

   ia->verts_per_instance = vertex_count;

   /* Calculate number of primitives input into the GS */
   uint prim_per_instance = u_decomposed_prims_for_vertices(prim, vertex_count);
   p->input_primitives = prim_per_instance * instance_count;

   /* Invoke VS as (vertices, instances); GS as (primitives, instances) */
   p->vs_grid[0] = vertex_count;
   p->vs_grid[1] = instance_count;

   p->gs_grid[0] = prim_per_instance;
   p->gs_grid[1] = instance_count;

   p->primitives_log2 = util_logbase2_ceil(prim_per_instance);

   /* If indexing is enabled, the third word is the offset into the index buffer
    * in elements. Apply that offset now that we have it. For a hardware
    * indirect draw, the hardware would do this for us, but for software input
    * assembly we need to do it ourselves.
    */
   if (index_size_B) {
      ia->index_buffer = libagx_index_buffer(
         index_buffer, index_buffer_range_el, draw[2], index_size_B);

      ia->index_buffer_range_el =
         libagx_index_buffer_range_el(index_buffer_range_el, draw[2]);
   }

   /* We need to allocate VS and GS count buffers, do so now */
   uint vertex_buffer_size =
      libagx_tcs_in_size(vertex_count * instance_count, vs_outputs);

   if (is_prefix_summing) {
      p->count_buffer = agx_heap_alloc_nonatomic(
         heap, p->input_primitives * p->count_buffer_stride);
   }

   p->input_buffer =
      (uintptr_t)agx_heap_alloc_nonatomic(heap, vertex_buffer_size);
   *vertex_buffer = p->input_buffer;

   p->input_mask = vs_outputs;

   /* Allocate the index buffer and write the draw consuming it */
   global VkDrawIndexedIndirectCommand *cmd = (global void *)p->indirect_desc;

   *cmd = (VkDrawIndexedIndirectCommand){
      .indexCount = agx_gs_rast_vertices(shape, max_indices, prim_per_instance,
                                         instance_count),
      .instanceCount = agx_gs_rast_instances(shape, max_indices,
                                             prim_per_instance, instance_count),
   };

   if (shape == AGX_GS_SHAPE_DYNAMIC_INDEXED) {
      cmd->firstIndex =
         agx_heap_alloc_nonatomic_offs(heap, cmd->indexCount * 4) / 4;

      p->output_index_buffer =
         (global uint *)(heap->base + (cmd->firstIndex * 4));
   }
}

/*
 * Returns (work_group_scan_inclusive_add(x), work_group_sum(x)). Implemented
 * manually with subgroup ops and local memory since Mesa doesn't do those
 * lowerings yet.
 */
static uint2
libagx_work_group_scan_inclusive_add(uint x, local uint *scratch)
{
   uint sg_id = get_sub_group_id();

   /* Partial prefix sum of the subgroup */
   uint sg = sub_group_scan_inclusive_add(x);

   /* Reduction (sum) for the subgroup */
   uint sg_sum = sub_group_broadcast(sg, 31);

   /* Write out all the subgroups sums */
   barrier(CLK_LOCAL_MEM_FENCE);
   scratch[sg_id] = sg_sum;
   barrier(CLK_LOCAL_MEM_FENCE);

   /* Read all the subgroup sums. Thread T in subgroup G reads the sum of all
    * threads in subgroup T.
    */
   uint other_sum = scratch[get_sub_group_local_id()];

   /* Exclusive sum the subgroup sums to get the total before the current group,
    * which can be added to the total for the current group.
    */
   uint other_sums = sub_group_scan_exclusive_add(other_sum);
   uint base = sub_group_broadcast(other_sums, sg_id);
   uint prefix = base + sg;

   /* Reduce the workgroup using the prefix sum we already did */
   uint reduction = sub_group_broadcast(other_sums + other_sum, 31);

   return (uint2)(prefix, reduction);
}

KERNEL(1024)
_libagx_prefix_sum(global uint *buffer, uint len, uint words, uint word)
{
   local uint scratch[32];
   uint tid = cl_local_id.x;

   /* Main loop: complete workgroups processing 1024 values at once */
   uint i, count = 0;
   uint len_remainder = len % 1024;
   uint len_rounded_down = len - len_remainder;

   for (i = tid; i < len_rounded_down; i += 1024) {
      global uint *ptr = &buffer[(i * words) + word];
      uint value = *ptr;
      uint2 sums = libagx_work_group_scan_inclusive_add(value, scratch);

      *ptr = count + sums[0];
      count += sums[1];
   }

   /* The last iteration is special since we won't have a full subgroup unless
    * the length is divisible by the subgroup size, and we don't advance count.
    */
   global uint *ptr = &buffer[(i * words) + word];
   uint value = (tid < len_remainder) ? *ptr : 0;
   uint scan = libagx_work_group_scan_inclusive_add(value, scratch)[0];

   if (tid < len_remainder) {
      *ptr = count + scan;
   }
}

KERNEL(1024)
libagx_prefix_sum_geom(constant struct agx_geometry_params *p)
{
   _libagx_prefix_sum(p->count_buffer, p->input_primitives,
                      p->count_buffer_stride / 4, cl_group_id.x);
}

KERNEL(1024)
libagx_prefix_sum_tess(global struct libagx_tess_args *p, global uint *c_prims,
                       global uint *c_invs, uint increment_stats__2)
{
   _libagx_prefix_sum(p->counts, p->nr_patches, 1 /* words */, 0 /* word */);

   /* After prefix summing, we know the total # of indices, so allocate the
    * index buffer now. Elect a thread for the allocation.
    */
   barrier(CLK_LOCAL_MEM_FENCE);
   if (cl_local_id.x != 0)
      return;

   /* The last element of an inclusive prefix sum is the total sum */
   uint total = p->nr_patches > 0 ? p->counts[p->nr_patches - 1] : 0;

   /* Allocate 4-byte indices */
   uint32_t elsize_B = sizeof(uint32_t);
   uint32_t size_B = total * elsize_B;
   uint alloc_B = agx_heap_alloc_nonatomic_offs(p->heap, size_B);
   p->index_buffer = (global uint32_t *)(((uintptr_t)p->heap->base) + alloc_B);

   /* ...and now we can generate the API indexed draw */
   global uint32_t *desc = p->out_draws;

   desc[0] = total;              /* count */
   desc[1] = 1;                  /* instance_count */
   desc[2] = alloc_B / elsize_B; /* start */
   desc[3] = 0;                  /* index_bias */
   desc[4] = 0;                  /* start_instance */

   /* If necessary, increment clipper statistics too. This is only used when
    * there's no geometry shader following us. See agx_nir_lower_gs.c for more
    * info on the emulation. We just need to calculate the # of primitives
    * tessellated.
    */
   if (increment_stats__2) {
      uint prims = p->points_mode ? total
                   : p->isolines  ? (total / 2)
                                  : (total / 3);

      increment_counters(c_prims, c_invs, NULL, prims);
   }
}

uintptr_t
libagx_vertex_output_address(uintptr_t buffer, uint64_t mask, uint vtx,
                             gl_varying_slot location)
{
   /* Written like this to let address arithmetic work */
   return buffer + ((uintptr_t)libagx_tcs_in_offs_el(vtx, location, mask)) * 16;
}

uintptr_t
libagx_geometry_input_address(constant struct agx_geometry_params *p, uint vtx,
                              gl_varying_slot location)
{
   return libagx_vertex_output_address(p->input_buffer, p->input_mask, vtx,
                                       location);
}

unsigned
libagx_input_vertices(constant struct agx_ia_state *ia)
{
   return ia->verts_per_instance;
}

global uint *
libagx_load_xfb_count_address(constant struct agx_geometry_params *p, int index,
                              int count_words, uint unrolled_id)
{
   return &p->count_buffer[(unrolled_id * count_words) + index];
}

uint
libagx_previous_xfb_primitives(global struct agx_geometry_params *p,
                               int static_count, int count_index,
                               int count_words, bool prefix_sum,
                               uint unrolled_id)
{
   if (static_count >= 0) {
      /* If the number of outputted vertices per invocation is known statically,
       * we can calculate the base.
       */
      return unrolled_id * static_count;
   } else {
      /* Otherwise, load from the count buffer buffer. Note that the sums are
       * inclusive, so index 0 is nonzero. This requires a little fixup here. We
       * use a saturating unsigned subtraction so we don't read out-of-bounds.
       *
       * If we didn't prefix sum, there's only one element.
       */
      uint prim_minus_1 = prefix_sum ? sub_sat(unrolled_id, 1u) : 0;
      uint count = p->count_buffer[(prim_minus_1 * count_words) + count_index];

      return unrolled_id == 0 ? 0 : count;
   }
}

/* Like u_foreach_bit, specialized for XFB to enable loop unrolling */
#define libagx_foreach_xfb(word, index)                                        \
   for (uint i = 0; i < 4; ++i)                                                \
      if (word & BITFIELD_BIT(i))

void
libagx_pre_gs(global struct agx_geometry_params *p, uint streams,
              uint buffers_written, uint4 buffer_to_stream, int4 count_index,
              uint4 stride, uint4 output_end, int4 static_count,
              uint invocations, uint vertices_per_prim,
              global uint *gs_invocations, global uint *gs_primitives,
              global uint *c_primitives, global uint *c_invocations)
{
   unsigned count_words = !!(count_index[0] >= 0) + !!(count_index[1] >= 0) +
                          !!(count_index[2] >= 0) + !!(count_index[3] >= 0);
   bool prefix_sum = count_words && buffers_written;
   uint unrolled_in_prims = p->input_primitives;

   /* Determine the number of primitives generated in each stream */
   uint4 in_prims = 0;
   libagx_foreach_xfb(streams, i) {
      in_prims[i] = libagx_previous_xfb_primitives(
         p, static_count[i], count_index[i], count_words, prefix_sum,
         unrolled_in_prims);

      *(p->prims_generated_counter[i]) += in_prims[i];
   }

   uint4 prims = in_prims;
   uint emitted_prims = prims[0] + prims[1] + prims[2] + prims[3];

   if (buffers_written) {
      libagx_foreach_xfb(buffers_written, i) {
         uint max_prims = libagx_setup_xfb_buffer(
            p, i, stride[i], output_end[i], vertices_per_prim);

         unsigned stream = buffer_to_stream[i];
         prims[stream] = min(prims[stream], max_prims);
      }

      int4 overflow = prims < in_prims;

      libagx_foreach_xfb(streams, i) {
         p->xfb_verts[i] = prims[i] * vertices_per_prim;

         *(p->xfb_overflow[i]) += (bool)overflow[i];
         *(p->xfb_prims_generated_counter[i]) += prims[i];
      }

      *(p->xfb_any_overflow) += any(overflow);

      /* Update XFB counters */
      libagx_foreach_xfb(buffers_written, i) {
         uint32_t prim_stride_B = stride[i] * vertices_per_prim;
         unsigned stream = buffer_to_stream[i];

         global uint *ptr = p->xfb_offs_ptrs[i];
         if ((uintptr_t)ptr == AGX_ZERO_PAGE_ADDRESS) {
            ptr = (global uint *)AGX_SCRATCH_PAGE_ADDRESS;
         }

         *ptr += prims[stream] * prim_stride_B;
      }
   }

   /* The geometry shader is invoked once per primitive (after unrolling
    * primitive restart). From the spec:
    *
    *    In case of instanced geometry shaders (see section 11.3.4.2) the
    *    geometry shader invocations count is incremented for each separate
    *    instanced invocation.
    */
   *gs_invocations += unrolled_in_prims * invocations;
   *gs_primitives += emitted_prims;

   /* Clipper queries are not well-defined, so we can emulate them in lots of
    * silly ways. We need the hardware counters to implement them properly. For
    * now, just consider all primitives emitted as passing through the clipper.
    * This satisfies spec text:
    *
    *    The number of primitives that reach the primitive clipping stage.
    *
    * and
    *
    *    If at least one vertex of the primitive lies inside the clipping
    *    volume, the counter is incremented by one or more. Otherwise, the
    *    counter is incremented by zero or more.
    */
   *c_primitives += emitted_prims;
   *c_invocations += emitted_prims;
}
