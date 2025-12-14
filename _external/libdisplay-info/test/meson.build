test_harness = find_program('./edid-decode-check.sh', native: true)

di_edid_print = executable(
	'di-edid-print',
	'di-edid-print.c',
	dependencies: di_dep,
	install: false,
)

print_harness = find_program('./edid-print-check.sh', native: true)

test_cases = [
	'acer-p1276',
	'amazon-fire-tv-48',
	'apple-xdr-dp',
	'asus-ux3405',
	'ayaneo-wxga',
	'cta-timings',
	'cta-timings',
	'cta-vfpdb',
	'custom-uncommon-cta-vesa',
	'cvt',
	'dell-2408wfp-dp',
	'goldstar-37lg5000',
	'goldstar-e2441',
	'goldstar-ite6604-hdmi',
	'hisense-55u8k',
	'hitachi-55r6+',
	'hp-5dq99aa-hdmi',
	'jdi-lpm135m467-edp',
	'lenovo-t27p10',
	'msi-mag321curv-dp',
	'panasonic-mei96a2-dp',
	'philips-4k-2023',
	'philips-ftv-2017',
	'philips-ftv-490',
	'qemu',
	'samsung-odyssey-g60sd',
	'samsung-q800t-hdmi2.0',
	'samsung-qn900b-8k',
	'samsung-s27a950d-dp',
	'sun-gh19ps-dvi',
	'tcl-smart-tv-5655',
	'viewsonic-vp2768-dp',
	'vizio-d40fj09',
]

test_env = [
	'DI_EDID_DECODE=' + di_edid_decode.full_path(),
	'DI_EDID_PRINT=' + di_edid_print.full_path(),
]

foreach tc : test_cases
	test(
		'decode-' + tc,
		test_harness,
		args: [files('data/' + tc + '.edid')],
		env: test_env,
		depends: [di_edid_decode],
	)
	test(
		'print-' + tc,
		print_harness,
		args: [files('data/' + tc + '.edid')],
		env: test_env,
		depends: [di_edid_print],
	)
endforeach

test_gen = find_program('./edid-decode-diff.sh', native: true)

gen_targets = []
foreach tc : test_cases
	gen_targets += files('data/' + tc + '.edid')
endforeach

subproject('v4l-utils', required: false)
ref_edid_decode = find_program('edid-decode', native: true, required: false)
if ref_edid_decode.found()
	test_env += 'REF_EDID_DECODE=' + ref_edid_decode.full_path()

	run_target(
		'gen-test-data',
		command: [test_gen] + gen_targets,
		depends: [di_edid_decode, di_edid_print],
		env: test_env,
	)
endif
