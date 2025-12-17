project('fcft', 'c',
        version: '3.3.2',  # Don't forget to update version in man pages
        license: 'MIT',
        meson_version: '>=0.58.0',
        default_options: [
          'c_std=c11',
          'warning_level=1',
          'werror=true',
          'b_ndebug=if-release'])

# See https://autotools.io/libtool/version.html, it explains the
# problem, but note that the way you tell libtool the library version
# is not what the final so file name will end up having.
#
# libtool calls these CURRENT, REVISION and AGE. But since we don't
# define them in the same way, we don't call them that.
#
# Note that MAJOR is the *only* version number that affects the
# SONAME, and thus actual linking.
so_version = [
  '4',      # MAJOR: increment on non-backward compatible ABI changes
  '3',      # MINOR: increment with backward compatible ABI changes
  '2',      # PATCH: increment with non-ABI affecting changes
]

is_debug_build = get_option('buildtype').startswith('debug')

cc = meson.get_compiler('c')

add_project_arguments(
  ['-D_GNU_SOURCE=200809L'] +
  (is_debug_build ? ['-D_DEBUG'] : []) +
  (cc.has_function('memfd_create',
                   args: ['-D_GNU_SOURCE=200809L'],
                   prefix: '#include <sys/mman.h>')
   ? ['-DMEMFD_CREATE']
   : []) +
  (cc.has_argument('-fvisibility=default')
   ? ['-DFCFT_EXPORT=__attribute__((visibility("default")))']
   : ['-DFCFT_EXPORT=']),
  language: 'c')

# Compute the relative path used by compiler invocations.
source_root = meson.current_source_dir().split('/')
build_root = meson.global_build_root().split('/')
relative_dir_parts = []
i = 0
in_prefix = true
foreach p : build_root
  if i >= source_root.length() or not in_prefix or p != source_root[i]
    in_prefix = false
    relative_dir_parts += '..'
  endif
  i += 1
endforeach
i = 0
in_prefix = true
foreach p : source_root
  if i >= build_root.length() or not in_prefix or build_root[i] != p
    in_prefix = false
    relative_dir_parts += p
  endif
  i += 1
endforeach
relative_dir = join_paths(relative_dir_parts) + '/'

if cc.has_argument('-fmacro-prefix-map=/foo=')
  add_project_arguments('-fmacro-prefix-map=@0@='.format(relative_dir), language: 'c')
endif

math = cc.find_library('m')
threads = dependency('threads')
fontconfig = dependency('fontconfig')
freetype = dependency('freetype2')
harfbuzz1 = dependency('harfbuzz', required: get_option('grapheme-shaping'))
harfbuzz2 = dependency('harfbuzz', required: get_option('run-shaping'))
utf8proc = dependency('libutf8proc', required: get_option('run-shaping'))
pixman = dependency('pixman-1')
stdthreads = cc.find_library('stdthreads', required: false)
system_nanosvg = cc.find_library('nanosvg', required: get_option('system-nanosvg'))
system_nanosvgrast = cc.find_library('nanosvgrast', required: get_option('system-nanosvg'))

tllist = dependency('tllist', version: '>=1.0.1', fallback: 'tllist')

if harfbuzz1.found() or harfbuzz2.found()
  add_project_arguments('-DFCFT_HAVE_HARFBUZZ', language: 'c')
endif
if utf8proc.found()
  add_project_arguments('-DFCFT_HAVE_UTF8PROC', language: 'c')
endif

if freetype.version().version_compare('>=24.2.18')  # 2.12.0, TODO: double-check...
  if get_option('svg-backend') == 'librsvg'
    cairo = dependency('cairo')
    librsvg = dependency('librsvg-2.0')
    rsvg = declare_dependency(
      sources: ['3rd-party/freetype-rsvg/rsvg-port.c',
                '3rd-party/freetype-rsvg/rsvg-port.h'],
      compile_args: '-Wno-deprecated-declarations',
      dependencies: [cairo, librsvg])
    nanosvg = declare_dependency()
    svg_backend = 'librsvg'
    add_project_arguments(
      ['-DHAVE_LIBRSVG', '-DFCFT_ENABLE_SVG_LIBRSVG=1'], language: 'c')

  elif get_option('svg-backend') == 'nanosvg'
    if system_nanosvg.found() and system_nanosvgrast.found()
      nanosvg = declare_dependency(
        sources: ['svg-backend-nanosvg.c', 'svg-backend-nanosvg.h'],
        dependencies: [system_nanosvg, system_nanosvgrast])
      rsvg = declare_dependency()
      svg_backend = 'nanosvg (system)'
    else
      nanosvg = declare_dependency(
        sources: ['nanosvg.c', '3rd-party/nanosvg/src/nanosvg.h',
                  'nanosvgrast.c', '3rd-party/nanosvg/src/nanosvgrast.h',
                  'svg-backend-nanosvg.c'],
        include_directories: '.',
        dependencies: math)
      rsvg = declare_dependency()
      svg_backend = 'nanosvg (bundled)'
    endif
    add_project_arguments('-DFCFT_ENABLE_SVG_NANOSVG=1', language: 'c')

  else
    rsvg = declare_dependency()
    nanosvg = declare_dependency()
    svg_backend = 'disabled'
  endif

else
  rsvg = declare_dependency()
  nanosvg = declare_dependency()
  svg_backend = 'disabled'
endif

env = find_program('env', native: true)
generate_unicode_precompose_sh = files('generate-unicode-precompose.sh')
unicode_data = custom_target(
  'unicode-data',
  input: 'unicode/UnicodeData.txt',
  output: 'unicode-compose-table.h',
  command: [env, 'LC_ALL=C', generate_unicode_precompose_sh, '@INPUT@', '@OUTPUT@'])

python = find_program('python3')
generate_emoji_data_py = files('generate-emoji-data.py')
emoji_data = custom_target(
  'emoji-data',
  input: 'unicode/emoji-data.txt',
  output: 'emoji-data.h',
  command: [python, generate_emoji_data_py, '@INPUT@', '@OUTPUT@'])

generate_version_sh = files('generate-version.sh')
version = custom_target(
  'generate_version',
  build_always_stale: true,
  output: 'version.h',
  command: [env, 'LC_ALL=C', generate_version_sh, meson.project_version(), '@CURRENT_SOURCE_DIR@', '@OUTPUT@'])

fcft_lib = build_target(
  'fcft',
  'fcft.c',
  'fcft/fcft.h', 'fcft/stride.h',
  'log.c', 'log.h',
  unicode_data, emoji_data, version,
  target_type: meson.is_subproject() ? 'static_library' : 'library',
  version: '.'.join(so_version),
  dependencies: [math, threads, fontconfig, freetype, harfbuzz1, harfbuzz2, utf8proc, pixman, tllist, rsvg, nanosvg, stdthreads],
  gnu_symbol_visibility: 'hidden',
  install: not meson.is_subproject())

fcft = declare_dependency(
  include_directories: '.', link_with: fcft_lib, dependencies: [pixman])
meson.override_dependency('fcft', fcft)

if get_option('examples')
  subdir('example')
endif

check = dependency('check', required: false)
if check.found()
  fcft_test = executable('test-fcft', 'test.c', dependencies: [check, fcft])
  test('fcft', fcft_test, args: get_option('test-text-shaping') ? ['--text-shaping'] : [])
endif

if not meson.is_subproject()
  install_headers('fcft/fcft.h', 'fcft/stride.h', subdir: 'fcft')

  pkg = import('pkgconfig')
  pkg.generate(
    fcft_lib,
    description : 'Simple font loading and glyph rasterization library')

  scdoc = dependency('scdoc', native: true, required: get_option('docs'))
  if scdoc.found()
    install_data(
      'LICENSE', 'README.md', 'CHANGELOG.md',
      install_dir: join_paths(get_option('datadir'), 'doc', 'fcft'))

    subdir('doc')
  endif
endif

summary(
  {
    'OT-SVG backend': svg_backend,
    'Grapheme shaping': harfbuzz1.found() or harfbuzz2.found(),
    'Run shaping': harfbuzz2.found() and utf8proc.found(),
    'Test text shaping': get_option('test-text-shaping'),
    'Documentation': not meson.is_subproject() and scdoc.found(),
  },
  bool_yn: true
)
