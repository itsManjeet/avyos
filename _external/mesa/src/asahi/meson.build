# Copyright 2018 Rob Clark
# Copyright 2019 Collabora, Ltd.
# SPDX-License-Identifier: MIT

# This is used only for the r/e tooling on macOS
dep_iokit = dependency('IOKit', required : false)

inc_asahi = include_directories([
   '.', 'layout', 'lib', 'genxml', 'compiler'
])

subdir('isa')

if with_gallium_asahi or with_asahi_vk or with_tools.contains('asahi')
   subdir('genxml')
   subdir('libagx')
   subdir('compiler')
   subdir('clc')
endif

if with_gallium_asahi or with_asahi_vk
   subdir('layout')
   subdir('lib')
elif dep_iokit.found()
   # Just build enough for libwrap.dylib
   subdir('genxml')
   subdir('lib')
endif

if with_asahi_vk
   subdir('vulkan')
endif

if with_tools.contains('drm-shim')
  subdir('drm-shim')
endif
