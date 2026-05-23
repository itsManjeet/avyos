#include "textflag.h"

// func fillSolidRGBAAsm(dst *byte, n uintptr, color uint32)
//
// Fills n bytes (multiple of 4) at dst with `color` repeated.
// Uses NEON VDUP to broadcast one 32-bit RGBA value to 4 pixels (16 bytes)
// per VST1 store.
TEXT ·fillSolidRGBAAsm(SB), NOSPLIT, $0-24
	MOVD  dst+0(FP), R0
	MOVD  n+8(FP), R1
	MOVWU color+16(FP), R2

	// Broadcast the 32-bit color to all 4 S-lanes of V0.
	VDUP R2, V0.S4

	CMP $16, R1
	BLT scalar

simd_loop:
	CMP $16, R1
	BLT scalar
	// Store 16 bytes (4 pixels) and post-increment R0 by 16.
	VST1.P [V0.B16], 16(R0)
	SUB    $16, R1
	B      simd_loop

scalar:
	CMP $4, R1
	BLT done
scalar_loop:
	MOVWU R2, (R0)
	ADD   $4, R0
	SUB   $4, R1
	CMP   $4, R1
	BGE   scalar_loop

done:
	RET
