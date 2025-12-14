# Copyright 2017 Intel Corporation
# Copyright 2025 Lima Project
# SPDX-License-Identifier: MIT

lima_pack = custom_target(
  'lima_pack.h',
  input : ['gen_pack.py', 'common.xml', 'lima_pack_header.h'],
  output : 'lima_pack.h',
  command : [prog_python, '@INPUT@'],
  capture : true,
)

idep_lima_pack = declare_dependency(
  sources : [lima_pack],
  dependencies : dep_valgrind,
  include_directories : include_directories('.'),
)


