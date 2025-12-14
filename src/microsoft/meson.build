# Copyright © Microsoft Corporation
# SPDX-License-Identifier: MIT

if with_microsoft_clc or with_spirv_to_dxil or with_microsoft_vk or (with_gallium_d3d12 and not get_option('gallium-d3d12-graphics').disabled())
  subdir('compiler')
  if with_microsoft_clc
    subdir('clc')
  endif
  if with_spirv_to_dxil or with_microsoft_vk
    subdir('spirv_to_dxil')
  endif
  if with_microsoft_vk
    subdir('vulkan')
  endif
endif