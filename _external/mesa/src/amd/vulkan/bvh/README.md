
# GFX12

GFX12 introduces a new BVH encoding for the image_bvh_dual_intersect_ray and image_bvh8_intersect_ray instructions.

## BVH8 box node

| bitsize/range | name | description |
| ------------ | ---- | ----------- |
| 32 | `internal_child_offset` | Offset of child BVH8 box nodes in units of 8 bytes. |
| 32 | `primitive_child_offset` | Offset of child primitive nodes in units of 8 bytes. |
| 32 | `unused` | Used by amdvlk for storing the parent node ID. |
| 32 | `origin_x` | x-offset applied to all child AABBs. |
| 32 | `origin_y` | y-offset applied to all child AABBs. |
| 32 | `origin_z` | z-offset applied to all child AABBs. |
| 8 | `exponent_x` | |
| 8 | `exponent_y` | |
| 8 | `exponent_z` | |
| 4 | `unused` | |
| 4 | `child_count_minus_one` | |
| 32 | `obb_matrix_index` | Selects a matrix for transforming the ray before performing intersection tests. `0x7F` to disable OBB. |
| 96x8 | `children[8]` | |

`children[8]` element layout:

| bitsize/range | name | description |
| ------------ | ---- | ----------- |
| 12 | `min_x` | Fixed point child AABB coordinate. |
| 12 | `min_y` | |
| 4 | `cull_flags` | |
| 4 | `unused` | |
| 12 | `min_z` | |
| 12 | `max_x` | |
| 8 | `cull_mask` | |
| 12 | `max_y` | |
| 4 | `node_type` | |
| 4 | `node_size` | Increment for the child offset in units of 128 bytes. |

The coordinates of child AABBs are encoded as follows:
- min: `floor((x - origin_x) / extent)`
- max: `ceil((x - origin_x) / extent) - 1`

image_bvh8_intersect_ray will return the node IDs of the child nodes.

## Primitive node

Highlevel layout:

| bitsize/range | name | description |
| ------------ | ---- | ----------- |
| 52 | `header` | Misc information about this node. |
| | `vertex_prefixes[3]` | |
| | `data` | Compressed vertex positions followed by primitive/geometry index data. |
| 29x`triangle_pair_count` | `pair_desc[triangle_pair_count]` | Misc information about a triangle pair. |

`header` layout:

| bitsize/range | name | description |
| ------------ | ---- | ----------- |
| 5 | `x_vertex_bits_minus_one` | |
| 5 | `y_vertex_bits_minus_one` | |
| 5 | `z_vertex_bits_minus_one` | |
| 5 | `trailing_zero_bits` | |
| 4 | `geometry_index_base_bits_div_2` | |
| 4 | `geometry_index_bits_div_2` | |
| 3 | `triangle_pair_count_minus_one` | |
| 1 | `vertex_type` | |
| 5 | `primitive_index_base_bits` | |
| 5 | `primitive_index_bits` | |
| 10 | `indices_midpoint` | Bit offset where the geometry and primitive indices start (geometry indices in negative direction, primitive indices in positive direction) |

The `data` field is split in three sections:
1. Vertex data, this is a list of floats which share the same
   prefix and the same number of trailing zero bits. The decompressed
   value (for example the x component of a vertex) is
   `(prefix << 32 - prefix_bits_x) | read(x_vertex_bits) << trailing_zero_bits` where `prefix_bits_x` is derived from
   `x_vertex_bits` and `trailing_zero_bits`
   (`32 - x_vertex_bits - trailing_zero_bits`).
2. Geometry indices.
3. Primitive indices.

Geometry indices are encoded the same way with the only difference being that geometry indices are read/written in negative direction starting from `indices_midpoint`. The indices section starts with a `*_index_base_bits`-bit value `*_index_base` which is the index of the first triangle. Subsequent triangles use indices calculated based on a `*_index_bits`-bit value:
- `*_index = read(*_index_bits)` if `*_index_bits >= *_index_base_bits`
- `*_index = read(*_index_bits) | (*_index_base & ~BITFIELD_MASK(*_index_bits))` otherwise.

`pair_desc(s)` layout:

| bitsize/range | name | description |
| ------------ | ---- | ----------- |
| 1 | `prim_range_stop` | |
| 1 | `tri1_double_sided` | |
| 1 | `tri1_opaque` | |
| 4 | `tri1_v0_index` | Indices into `data`, `0xF` for procedural nodes. |
| 4 | `tri1_v1_index` | `0xF` for procedural nodes. |
| 4 | `tri1_v2_index` | |
| `tri0` has identical fields: |
| 1 | `tri0_double_sided` | |
| 1 | `tri0_opaque` | |
| 4 | `tri0_v0_index` | |
| 4 | `tri0_v1_index` | |
| 4 | `tri0_v2_index` | |

image_bvh8_intersect_ray will return the following data for triangle nodes:

| VGPR index | value |
| ---------- | ----- |
| 0 | t0 |
| 1 | `(procedural0 << 31) \| u0` |
| 2 | `(opaque0 << 31) \| v0` |
| 3 | `(primitive_index0 << 1) \| backface0` |
| 4 | t1 |
| 5 | `(procedural1 << 31) \| u1` |
| 6 | `(opaque1 << 31) \| v1` |
| 7 | `(primitive_index1 << 1) \| backface1` |
| 8 | `(geometry_index0 << 2) \| navigation_bits` |
| 9 | `(geometry_index1 << 2) \| navigation_bits` |

image_bvh8_intersect_ray will return the following data for procedural nodes:

| VGPR index | value |
| ---------- | ----- |
| 3 | `primitive_index0 << 1` |
| 8 | `(geometry_index0 << 2) \| navigation_bits` |
| 9 | `(geometry_index1 << 2) \| navigation_bits` |

`navigation_bits` is 0 if there are more triangle pairs to process, 1 if this was the last triangle pair and 3 if `prim_range_stop` is set.

## Instance node

| bitsize/range | name | description |
| ------------ | ---- | ----------- |
| 32x3x4 | `world_to_object` | |
| 62 | `bvh_addr` | Units of 4 bytes. |
| 1 | `aabbs` | Does the BLAS (only) contain AABBs? Used for pointer flag based culling. |
| 1 | `unused` | |
| 32 | `unused` | |
| 24 | `user_data` | Returned by the intersect instruction for instance nodes. |
| 8 | `cull_mask` | |
| The instance node can have up to 4 quantized child nodes: |
| 32 | `origin_x` | x-offset applied to all child AABBs. |
| 32 | `origin_y` | y-offset applied to all child AABBs. |
| 32 | `origin_z` | z-offset applied to all child AABBs. |
| 8 | `exponent_x` | |
| 8 | `exponent_y` | |
| 8 | `exponent_z` | |
| 4 | `unused` | |
| 4 | `child_count_minus_one` | |
| 96x4 | `children[4]` | |

image_bvh8_intersect_ray will return:

| VGPR index | value |
| ---------- | ----- |
| 2 | BLAS addr lo |
| 3 | BLAS addr hi |
| 6 | `user_data` |
| 7 | `(child_ids[0] & 0xFF) \| ((child_ids[1] & 0xFF) << 8) \| ((child_ids[2] & 0xFF) << 16) \| ((child_ids[3] & 0xFF) << 24)` |
