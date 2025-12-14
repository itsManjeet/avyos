project(
	'libdisplay-info',
	'c',
	version: '0.4.0-dev',
	license: 'MIT',
	meson_version: '>= 0.57.0',
	default_options: [
		'c_std=c11',
		'warning_level=3',
		'wrap_mode=nodownload',
	],
)

version = meson.project_version().split('-')[0]
version_major = version.split('.')[0]
version_minor = version.split('.')[1]
assert(version_major == '0')

dep_hwdata = dependency('hwdata', required: false, native: true)
if dep_hwdata.found()
	hwdata_dir = dep_hwdata.get_variable(pkgconfig: 'pkgdatadir')
	pnp_ids = files(hwdata_dir / 'pnp.ids')
else
	pnp_ids = files('/usr/share/hwdata/pnp.ids')
endif

gen_search_table = find_program('tool/gen-search-table.py')
pnp_id_table = custom_target(
	'pnp-id-table.c',
	command: [ gen_search_table, pnp_ids, '@OUTPUT@', 'pnp_id_table' ],
	output: 'pnp-id-table.c',
)

cc = meson.get_compiler('c')

math = cc.find_library('m', required: false)

add_project_arguments(['-D_POSIX_C_SOURCE=200809L'], language: 'c')

add_project_arguments(cc.get_supported_arguments([
	'-Wundef',
	'-Wmissing-prototypes',
	'-Walloca',
	'-Wdeclaration-after-statement',
	'-Wconversion',

	'-Wno-unused-parameter',
	'-Wno-missing-field-initializers',

	'-Werror=implicit',
]), language: 'c')

symbols_file = 'libdisplay-info.map'
symbols_flag = '-Wl,--version-script,@0@'.format(meson.current_source_dir() / symbols_file)

di_lib = library(
	'display-info',
	[
		'cta.c',
		'cta-vic-table.c',
		'cvt.c',
		'displayid.c',
		'displayid2.c',
		'dmt-table.c',
		'edid.c',
		'gtf.c',
		'hdmi-vic-table.c',
		'info.c',
		'log.c',
		'memory-stream.c',
		pnp_id_table,
	],
	include_directories: include_directories('include'),
	dependencies: [math],
	link_args: symbols_flag,
	link_depends: symbols_file,
	install: true,
	version: version,
	soversion: version_minor,
)

install_subdir(
	'include/libdisplay-info',
	install_dir: get_option('includedir'),
)

pkgconfig = import('pkgconfig')
pkgconfig.generate(
	di_lib,
	filebase: 'libdisplay-info',
	name: 'libdisplay-info',
	url: 'https://gitlab.freedesktop.org/emersion/libdisplay-info',
	description: 'EDID and DisplayID library',
)

di_dep = declare_dependency(
	link_with: di_lib,
	include_directories: include_directories('include'),
)

meson.override_dependency('libdisplay-info', di_dep)

subdir('di-edid-decode')
subdir('test')
