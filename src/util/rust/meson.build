# Copyright © 2024 Google
# SPDX-License-Identifier: MIT

dep_cfg_if = dependency('cfg-if',
  version: '>= 1.0.0',
  fallback: ['cfg-if', 'dep_cfg_if'],
  required: true,
)

dep_thiserror = dependency('thiserror',
  version: '>= 2.0.11',
  fallback: ['thiserror', 'dep_thiserror'],
  required: true,
)

dep_remain = dependency('remain',
  version: '>= 0.2.12',
  fallback: ['remain', 'dep_remain'],
  required: true,
)

dep_zerocopy = dependency('zerocopy',
  version: '>= 0.8.13',
  fallback: ['zerocopy', 'dep_zerocopy'],
  required: true,
)

dep_zerocopy_derive = dependency('zerocopy-derive',
  version: '>= 0.8.13',
  fallback: ['zerocopy-derive', 'dep_zerocopy_derive'],
  required: true,
)

dep_mesa3d_util = [dep_cfg_if, dep_thiserror, dep_remain, dep_zerocopy,
                   dep_zerocopy_derive]

if host_machine.system() == 'linux'
  dep_rustix = dependency('rustix',
    version: '>= 0.38.31',
    fallback: ['rustix', 'dep_rustix'],
    required: true,
  )

  dep_bitflags = dependency('bitflags',
    version: '>= 2.6.0',
    fallback: ['bitflags', 'dep_bitflags'],
    required: true,
  )

  dep_errno = dependency('errno',
    version: '>= 0.3.8',
    fallback: ['errno', 'dep_errno'],
    required: true,
  )
  
  dep_mesa3d_util += [dep_rustix, dep_bitflags, dep_errno]
endif

libmesa_rust_util = static_library(
  'mesa3d_util',
  'lib.rs',
  gnu_symbol_visibility : 'hidden',
  rust_abi : 'rust',
  dependencies: dep_mesa3d_util,
)
