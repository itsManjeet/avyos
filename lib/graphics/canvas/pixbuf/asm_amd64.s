#include "textflag.h"

// func fillSolidRGBAAsm(dst *byte, n uintptr, color uint32)
//
// Fills n bytes (multiple of 4) at dst with `color` repeated.
// Uses SSE2 PSHUFD to broadcast one 32-bit RGBA value to 4 pixels (16 bytes)
// per store.
TEXT ·fillSolidRGBAAsm(SB), NOSPLIT, $0-24
	MOVQ dst+0(FP), DI
	MOVQ n+8(FP), CX
	MOVL color+16(FP), AX

	// Broadcast AX (32-bit color) to all 4 dword lanes of XMM0.
	MOVD   AX, X0
	PSHUFD $0x00, X0, X0

	CMPQ CX, $16
	JL   scalar

simd_loop:
	CMPQ  CX, $16
	JL    scalar
	MOVOU X0, (DI)
	ADDQ  $16, DI
	SUBQ  $16, CX
	JMP   simd_loop

scalar:
	CMPQ CX, $4
	JL   done
scalar_loop:
	MOVL AX, (DI)
	ADDQ $4, DI
	SUBQ $4, CX
	CMPQ CX, $4
	JGE  scalar_loop

done:
	RET
