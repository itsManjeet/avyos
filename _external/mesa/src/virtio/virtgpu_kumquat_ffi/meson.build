# Copyright © 2025 Google
# SPDX-License-Identifier: MIT

inc_virtgpu_kumquat_ffi = include_directories('include')

dep_log = dependency('log',
  version: '>= 0.4.22',
  fallback: ['log', 'dep_log'],
  required: true,
)

virtgpu_kumquat_ffi_args = [
  # we want unsafe blocks inside unsafe functions
  '-Dunsafe_op_in_unsafe_fn',
]

libvirtgpu_kumquat_ffi = static_library(
  'virtgpu_kumquat_ffi',
  'lib.rs',
  gnu_symbol_visibility : 'hidden',
  rust_abi : 'c',
  rust_args : [
    virtgpu_kumquat_ffi_args,
  ],
  link_with: [libmesa_protocols, libmesa_rust_util, libvirtgpu_kumquat],
  dependencies: [dep_mesa3d_util, dep_log]
)

dep_virtgpu_kumquat_ffi = declare_dependency(
  include_directories : [inc_virtgpu_kumquat_ffi],
  link_with : [libvirtgpu_kumquat_ffi]
)
