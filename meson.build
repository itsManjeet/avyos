project('wbg', 'c',
        version: '1.3.0',
        license: 'MIT',
        meson_version: '>=0.58.0',
        default_options: [
            'c_std=c18',
            'warning_level=1',
            'werror=true',
            'b_ndebug=if-release'])

is_debug_build = get_option('buildtype').startswith('debug')

add_project_arguments(
  ['-D_POSIX_C_SOURCE=200809L', '-D_GNU_SOURCE'] +
  (is_debug_build ? ['-D_DEBUG'] : []),
  language: 'c',
)

cc = meson.get_compiler('c')

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
pixman = dependency('pixman-1')
system_nanosvg = cc.find_library('nanosvg', required: get_option('system-nanosvg'))
system_nanosvgrast = cc.find_library('nanosvgrast', required: get_option('system-nanosvg'))
png = dependency('libpng', required: get_option('png'))
jpg = dependency('libjpeg', required: get_option('jpeg'))
webp = dependency('libwebp', required: get_option('webp'))
jxl = dependency('libjxl', required: get_option('jxl'))
jxl_threads = dependency('libjxl_threads', required: false)

if system_nanosvg.found() and system_nanosvgrast.found()
  svg = declare_dependency(
    dependencies: [system_nanosvg, system_nanosvgrast, math]
  )
  svg_lib = 'nanosvg (system)'
else
  svg = declare_dependency(
    sources: ['nanosvg.c', '3rd-party/nanosvg/src/nanosvg.h',
              'nanosvgrast.c', '3rd-party/nanosvg/src/nanosvgrast.h'],
    include_directories: '.',
    dependencies: math,
  )
  svg_lib = 'nanosvg (bundled)'
endif
have_svg = get_option('svg')

if not png.found() and not jpg.found() and not jxl.found() and not webp.found() and not have_svg
  error('you must enable at least one image format')
endif

if png.found()
  add_project_arguments('-DWBG_HAVE_PNG=1', language: 'c')
endif
if jpg.found()
  add_project_arguments('-DWBG_HAVE_JPG=1', language:'c')
endif
if webp.found()
  add_project_arguments('-DWBG_HAVE_WEBP=1', language:'c')
endif
if have_svg
  add_project_arguments('-DWBG_HAVE_SVG=1', language:'c')
endif
if jxl.found()
  add_project_arguments('-DWBG_HAVE_JXL=1', language:'c')
  if jxl_threads.found()
    add_project_arguments('-DWBG_HAVE_JXL_THREADS=1', language:'c')
  endif
else
  jxl_threads = dependency('', required: false)
endif

wayland_protocols = dependency('wayland-protocols')
wayland_client = dependency('wayland-client')
tllist = dependency('tllist', version: '>=1.0.1', fallback: 'tllist')

wayland_protocols_datadir = wayland_protocols.get_variable('pkgdatadir')

wscanner = dependency('wayland-scanner', native: true)
wscanner_prog = find_program(
  wscanner.get_variable('wayland_scanner'), native: true)

wl_proto_headers = []
wl_proto_src = []
foreach prot : [
    'external/wlr-layer-shell-unstable-v1.xml',
    wayland_protocols_datadir + '/stable/xdg-shell/xdg-shell.xml']


  wl_proto_headers += custom_target(
    prot.underscorify() + '-client-header',
    output: '@BASENAME@.h',
    input: prot,
    command: [wscanner_prog, 'client-header', '@INPUT@', '@OUTPUT@'])

  wl_proto_src += custom_target(
    prot.underscorify() + '-private-code',
    output: '@BASENAME@.c',
    input: prot,
    command: [wscanner_prog, 'private-code', '@INPUT@', '@OUTPUT@'])
endforeach

env = find_program('env', native: true)
generate_version_sh = files('generate-version.sh')
version = custom_target(
  'generate_version',
  build_always_stale: true,
  output: 'version.h',
  command: [env, 'LC_ALL=C', generate_version_sh, meson.project_version(), '@CURRENT_SOURCE_DIR@', '@OUTPUT@'])

image_format_sources = []
if png.found()
  image_format_sources += ['png.c', 'png-wbg.h']
endif
if jpg.found()
  image_format_sources += ['jpg.c', 'jpg.h']
endif
if jxl.found()
  image_format_sources += ['jxl.c', 'jxl.h']
endif
if webp.found()
  image_format_sources += ['webp.c', 'webp.h']
endif
if have_svg
  image_format_sources += ['svg.c', 'svg.h']
endif

executable(
    'wbg',
    'main.c',
    'log.c', 'log.h',
    'shm.c', 'shm.h',
    'stride.h',
    'wbg-features.h',
    image_format_sources,
    wl_proto_src + wl_proto_headers, version,
    dependencies: [pixman, png, jpg, jxl, jxl_threads, webp, svg, wayland_client, tllist],
    install: true)

summary(
  {
    'PNG support': png.found(),
    'JPEG support': jpg.found(),
    'JPEG XL support': jxl.found(),
    'WebP support': webp.found(),
    'SVG support': have_svg ? svg_lib : false,
  },
  bool_yn: true
)
