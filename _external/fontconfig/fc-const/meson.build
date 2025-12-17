fcconst_h = custom_target('fcconst.h',
  output: 'fcconst.h',
  input: ['fcconst.list', fcobjs_h],
  command: [find_program('fc-const.py'), '@INPUT0@', '@INPUT1@', '--output', '@OUTPUT@'])
test_const_name_c = custom_target('test_const_name.c',
  output: 'test_const_name.c',
  input: ['fcconst.list', fcobjs_h],
  command: [find_program('fc-const.py'), '-t', '@INPUT0@', '@INPUT1@', '--output', '@OUTPUT@'])
