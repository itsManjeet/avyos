# Copyright © 2025 Google
# SPDX-License-Identifier: MIT

project(
  'errno',
  'rust',
  version : '0.3.12',
  license : 'MIT OR Apache-2.0',
)

libc = subproject('libc').get_variable('lib')

lib = static_library(
  'libc_errno',
  'src/lib.rs',
  override_options : ['rust_std=2018', 'build.rust_std=2018'],
  link_with: [libc],
  rust_abi : 'rust',
  native : true,
)

dep_errno = declare_dependency(
  link_with : [lib]
)
