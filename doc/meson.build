sh = find_program('sh', native: true)

scdoc_prog = find_program(scdoc.get_variable('scdoc'), native: true)

foreach man_src : ['fcft_capabilities.3.scd',
                   'fcft_clone.3.scd',
                   'fcft_destroy.3.scd',
                   'fcft_fini.3.scd',
                   'fcft_font_options_create.3.scd',
                   'fcft_font_options_destroy.3.scd',
                   'fcft_from_name.3.scd',
                   'fcft_from_name2.3.scd',
                   'fcft_init.3.scd',
                   'fcft_kerning.3.scd',
                   'fcft_log_init.3.scd',
                   'fcft_precompose.3.scd',
                   'fcft_rasterize_char_utf32.3.scd',
                   'fcft_rasterize_grapheme_utf32.3.scd',
                   'fcft_rasterize_text_run_utf32.3.scd',
                   'fcft_set_emoji_presentation.3.scd',
                   'fcft_set_scaling_filter.3.scd',
                   'fcft_text_run_destroy.3.scd']
  parts = man_src.split('.')
  name = parts[-3]
  section = parts[-2]
  out = '@0@.@1@'.format(name, section)

  custom_target(
    out,
    output: out,
    input: man_src,
    command: [sh, '-c', '@0@ < @INPUT@'.format(scdoc_prog.full_path())],
    capture: true,
    install: true,
    install_dir: join_paths(get_option('mandir'), 'man@0@'.format(section)))
endforeach
