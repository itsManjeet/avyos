#!/bin/sh -eu

workdir="$(mktemp -d)"
cleanup() {
	rm -rf "$workdir"
}
trap cleanup EXIT

edid="$1"
ref="${edid%.edid}.print"
"$DI_EDID_PRINT" <"$edid" >"$workdir/printout"

diff -u "$ref" "$workdir/printout"
