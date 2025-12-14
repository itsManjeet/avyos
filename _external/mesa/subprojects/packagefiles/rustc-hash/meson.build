project(
  'rustc_hash',
  'rust',
  version : '2.1.1',
  license : 'MIT OR Apache-2.0',
)

rust_args = [
  '--cfg', 'feature="std"'
]

lib = static_library(
  'rustc_hash',
  'src/lib.rs',
  rust_args : rust_args,
  override_options : ['rust_std=2021', 'build.rust_std=2021'],
  rust_abi : 'rust',
)

dep_rustc_hash = declare_dependency(
  link_with : [lib],
)
