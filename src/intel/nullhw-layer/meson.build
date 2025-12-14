# Copyright © 2019 Intel Corporation
# SPDX-License-Identifier: MIT

vklayer_intel_nullhw_files = files(
  'intel_nullhw.c',
)

vklayer_intel_nullhw = shared_library(
  'VkLayer_INTEL_nullhw',
  vklayer_intel_nullhw_files,
  c_args : [no_override_init_args],
  dependencies : [idep_vulkan_util, idep_mesautil, vulkan_wsi_deps, dep_dl],
  include_directories : [inc_include, inc_src],
  link_args : cc.get_supported_link_arguments(['-Wl,-Bsymbolic-functions', '-Wl,-z,relro']),
  install : true
)

configure_file(
  configuration : {'library_path' : 'libVkLayer_INTEL_nullhw.so'},
  input : 'VkLayer_INTEL_nullhw.json.in',
  output : '@BASENAME@',
  install : true,
  install_dir : join_paths(get_option('datadir'), 'vulkan', 'explicit_layer.d'),
  install_tag : 'runtime',
)

configure_file(
  configuration : {'library_path' : vklayer_intel_nullhw.full_path()},
  input : 'VkLayer_INTEL_nullhw.json.in',
  output : '@PLAINNAME@_dev.json',
)
devenv.append('VK_ADD_LAYER_PATH', meson.current_build_dir())
