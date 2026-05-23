//go:build linux

package drmkms

import "testing"

func TestScaleAbsolute(t *testing.T) {
	info := inputAbsInfo{Minimum: 0, Maximum: 32767}

	if got := scaleAbsolute(0, info, 1280); got != 0 {
		t.Fatalf("scaleAbsolute(min) = %v, want 0", got)
	}
	if got := scaleAbsolute(32767, info, 1280); got != 1279 {
		t.Fatalf("scaleAbsolute(max) = %v, want 1279", got)
	}

	mid := scaleAbsolute(16384, info, 1280)
	if mid < 639 || mid > 640 {
		t.Fatalf("scaleAbsolute(mid) = %v, want approximately 639.5", mid)
	}
}

func TestScaleAbsoluteInvalidRangeFallsBackToClampedValue(t *testing.T) {
	info := inputAbsInfo{Minimum: 0, Maximum: 0}

	if got := scaleAbsolute(-10, info, 800); got != 0 {
		t.Fatalf("scaleAbsolute(negative) = %v, want 0", got)
	}
	if got := scaleAbsolute(9999, info, 800); got != 799 {
		t.Fatalf("scaleAbsolute(overflow) = %v, want 799", got)
	}
}
