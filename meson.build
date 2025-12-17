project('tllist',
        'c',
        version: '1.1.0',
        license: 'MIT',
        meson_version: '>=0.54.0')
tllist = declare_dependency(include_directories: '.')
meson.override_dependency('tllist', tllist)

unittest = executable('unittest', 'test.c', dependencies: [tllist])
test('tllist', unittest)

if not meson.is_subproject()
  install_headers('tllist.h')

  pkg = import('pkgconfig')
  pkg.generate(libraries : [],
               subdirs : ['.'],
               name : 'tllist',
               filebase : 'tllist',
               description : 'A C header file only implementation of a typed linked list')

  install_data(
    'LICENSE', 'README.md',
    install_dir: join_paths(get_option('datadir'), 'doc', 'tllist'))
endif
