package emu

import (
	"testing"

	"golang.org/x/arch/arm64/arm64asm"
)

func TestB_Unconditional(t *testing.T) {
	// B #0x10 (forward 16 bytes = 4 instructions) => 0x14000004
	enc := uint32(0x14000004)
	verifyDecode(t, enc, arm64asm.B)

	em := newTestEmulator(enc)
	stepN(t, em, 1)
	assertPC(t, em, 0x10) // 0 + 16 = 16
}

func TestB_Backward(t *testing.T) {
	// B #-0x8 (backward 8 bytes) => 0x17FFFFFE
	enc := uint32(0x17FFFFFE)
	verifyDecode(t, enc, arm64asm.B)

	em := newTestEmulator(0, 0, enc) // at PC=8
	em.Regs.PC = 8
	stepN(t, em, 1)
	assertPC(t, em, 0) // 8 - 8 = 0
}

func TestBL(t *testing.T) {
	// BL #0x10 => 0x94000004
	enc := uint32(0x94000004)
	verifyDecode(t, enc, arm64asm.BL)

	em := newTestEmulator(enc)
	stepN(t, em, 1)
	assertPC(t, em, 0x10)
	assertX(t, em, 30, 4) // LR = PC + 4 = 4
}

func TestRET(t *testing.T) {
	// RET => 0xD65F03C0
	enc := uint32(0xD65F03C0)
	verifyDecode(t, enc, arm64asm.RET)

	em := newTestEmulator(enc)
	em.Regs.WriteX(30, 0x1000) // LR
	stepN(t, em, 1)
	assertPC(t, em, 0x1000)
}

func TestCBZ_Taken(t *testing.T) {
	// CBZ X0, #0x10 => 0xB4000080
	enc := uint32(0xB4000080)
	verifyDecode(t, enc, arm64asm.CBZ)

	em := newTestEmulator(enc)
	em.Regs.WriteX(0, 0) // zero -> branch taken
	stepN(t, em, 1)
	assertPC(t, em, 0x10)
}

func TestCBZ_NotTaken(t *testing.T) {
	enc := uint32(0xB4000080)
	em := newTestEmulator(enc)
	em.Regs.WriteX(0, 42) // non-zero -> fall through
	stepN(t, em, 1)
	assertPC(t, em, 4) // PC + 4
}

func TestCBNZ_Taken(t *testing.T) {
	// CBNZ X0, #0x10 => 0xB5000080
	enc := uint32(0xB5000080)
	verifyDecode(t, enc, arm64asm.CBNZ)

	em := newTestEmulator(enc)
	em.Regs.WriteX(0, 1)
	stepN(t, em, 1)
	assertPC(t, em, 0x10)
}

func TestCBNZ_NotTaken(t *testing.T) {
	enc := uint32(0xB5000080)
	em := newTestEmulator(enc)
	em.Regs.WriteX(0, 0)
	stepN(t, em, 1)
	assertPC(t, em, 4)
}

func TestTBZ_Taken(t *testing.T) {
	// TBZ W0, #0, #0x10 => 0x36000080 (b5=0 means W register)
	enc := uint32(0x36000080)
	verifyDecode(t, enc, arm64asm.TBZ)

	em := newTestEmulator(enc)
	em.Regs.WriteW(0, 0) // bit 0 is 0 -> branch
	stepN(t, em, 1)
	assertPC(t, em, 0x10)
}

func TestTBZ_NotTaken(t *testing.T) {
	enc := uint32(0x36000080)
	em := newTestEmulator(enc)
	em.Regs.WriteW(0, 1) // bit 0 is 1 -> fall through
	stepN(t, em, 1)
	assertPC(t, em, 4)
}

func TestTBNZ_Taken(t *testing.T) {
	// TBNZ W0, #0, #0x10 => 0x37000080
	enc := uint32(0x37000080)
	verifyDecode(t, enc, arm64asm.TBNZ)

	em := newTestEmulator(enc)
	em.Regs.WriteW(0, 1) // bit 0 is 1 -> branch
	stepN(t, em, 1)
	assertPC(t, em, 0x10)
}

func TestB_Cond_EQ_Taken(t *testing.T) {
	// B.EQ #0x10 => 0x54000080
	enc := uint32(0x54000080)
	verifyDecode(t, enc, arm64asm.B)

	em := newTestEmulator(enc)
	em.Regs.Z = true // EQ condition
	stepN(t, em, 1)
	assertPC(t, em, 0x10)
}

func TestB_Cond_EQ_NotTaken(t *testing.T) {
	enc := uint32(0x54000080)
	em := newTestEmulator(enc)
	em.Regs.Z = false
	stepN(t, em, 1)
	assertPC(t, em, 4) // fall through
}

func TestB_Cond_NE_Taken(t *testing.T) {
	// B.NE #0x10 => 0x54000081
	enc := uint32(0x54000081)
	verifyDecode(t, enc, arm64asm.B)

	em := newTestEmulator(enc)
	em.Regs.Z = false
	stepN(t, em, 1)
	assertPC(t, em, 0x10)
}

func TestBR(t *testing.T) {
	// BR X1 => 0xD61F0020
	enc := uint32(0xD61F0020)
	verifyDecode(t, enc, arm64asm.BR)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 0x2000)
	stepN(t, em, 1)
	assertPC(t, em, 0x2000)
}

func TestBLR(t *testing.T) {
	// BLR X1 => 0xD63F0020
	enc := uint32(0xD63F0020)
	verifyDecode(t, enc, arm64asm.BLR)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 0x3000)
	stepN(t, em, 1)
	assertPC(t, em, 0x3000)
	assertX(t, em, 30, 4) // LR = original PC + 4
}
