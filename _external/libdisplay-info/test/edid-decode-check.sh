#!/bin/sh -eu

workdir="$(mktemp -d)"
cleanup() {
	rm -rf "$workdir"
}
trap cleanup EXIT

edid="$1"
diff="${edid%.edid}.diff"
ref="${edid%.edid}.ref"

cp "$ref" "$workdir/ref"
"$DI_EDID_DECODE" <"$edid" >"$workdir/di" || [ $? = 254 ]

if [ -f "$diff" ]; then
	patch "$workdir/ref" "$diff"
fi

diff -u "$workdir/ref" "$workdir/di"
