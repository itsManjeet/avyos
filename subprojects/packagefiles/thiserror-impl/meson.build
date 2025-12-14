# Copyright © 2024 Google
# SPDX-License-Identifier: MIT

project(
  'thiserror-impl',
  'rust',
  version : '2.0.11',
  license : 'MIT OR Apache-2.0',
)

syn = subproject('syn').get_variable('lib')
quote = subproject('quote').get_variable('lib')
proc_macro2 = subproject('proc-macro2').get_variable('lib')

rust = import('rust')
lib = rust.proc_macro(
  'thiserror_impl',
  'src/lib.rs',
  override_options : ['rust_std=2021', 'build.rust_std=2021'],
  link_with : [syn, quote, proc_macro2]
)

dep_thiserror_impl = declare_dependency(
  link_with : [lib, syn, quote, proc_macro2]
)
