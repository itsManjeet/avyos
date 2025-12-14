# Copyright © 2017 Intel Corporation
# SPDX-License-Identifier: MIT

inc_loader = include_directories('.')

if with_platform_wayland
  wp_dir = dep_wl_protocols.get_variable(pkgconfig : 'pkgdatadir', internal : 'pkgdatadir')
  wp_protos = {
    'fifo-v1': 'staging/fifo/fifo-v1.xml',
    'commit-timing-v1': 'staging/commit-timing/commit-timing-v1.xml',
    'linux-dmabuf-unstable-v1': 'unstable/linux-dmabuf/linux-dmabuf-unstable-v1.xml',
    'presentation-time': 'stable/presentation-time/presentation-time.xml',
    'tearing-control-v1': 'staging/tearing-control/tearing-control-v1.xml',
    'linux-drm-syncobj-v1': 'staging/linux-drm-syncobj/linux-drm-syncobj-v1.xml',
    'color-management-v1': 'staging/color-management/color-management-v1.xml',
  }
  wp_files = {}
  foreach name, xml : wp_protos
    code = custom_target(
      name + '-protocol.c',
      input : files(join_paths(wp_dir, xml)),
      output : name + '-protocol.c',
      command : [prog_wl_scanner, wl_scanner_arg, '@INPUT@', '@OUTPUT@'],
    )
    header = custom_target(
      name + '-client-protocol.h',
      input : files(join_paths(wp_dir, xml)),
      output : name + '-client-protocol.h',
      command : [prog_wl_scanner, 'client-header', '@INPUT@', '@OUTPUT@'],
    )
    wp_files += { name: [code, header] }
  endforeach

  libloader_wayland_helper = static_library(
    'loader_wayland_helper',
    'loader_wayland_helper.c',
    wp_files['presentation-time'],
    gnu_symbol_visibility : 'hidden',
    include_directories : [inc_include, inc_src, inc_gallium],
    dependencies : [
      idep_mesautil,
      dep_wayland_client,
    ],
    build_by_default : false,
  )
else
  libloader_wayland_helper = []
endif

loader_c_args = [
  '-DUSE_DRICONF',
]
libloader = static_library(
  'loader',
  ['loader_dri_helper.c', 'loader.c', sha1_h],
  c_args : loader_c_args,
  gnu_symbol_visibility : 'hidden',
  include_directories : [inc_include, inc_src, inc_util, inc_gallium],
  dependencies : [idep_mesautil, dep_libdrm, dep_thread, dep_xcb, dep_xcb_xrandr],
  build_by_default : false,
)
