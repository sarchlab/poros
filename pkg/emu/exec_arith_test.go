package emu

import (
	"testing"

	"golang.org/x/arch/arm64/arm64asm"
)

// ARM64 instruction encodings (little-endian uint32)

// ADD X0, X1, X2          => 0x8B020020
// ADD W0, W1, W2          => 0x0B020020
// ADD X0, X1, #10         => 0x91002820
// SUB X0, X1, X2          => 0xCB020020
// SUB X0, X1, #10         => 0xD1002820
// ADDS X0, X1, X2         => 0xAB020020
// SUBS X0, X1, X2         => 0xEB020020
// MUL X0, X1, X2          => 0x9B027C20
// MADD X0, X1, X2, X3     => 0x9B020C20
// NEG X0, X1              => 0xCB0103E0

func TestADD_Reg(t *testing.T) {
	// ADD X0, X1, X2
	enc := uint32(0x8B020020)
	verifyDecode(t, enc, arm64asm.ADD)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 100)
	em.Regs.WriteX(2, 200)
	stepN(t, em, 1)
	assertX(t, em, 0, 300)
}

func TestADD_Imm(t *testing.T) {
	// ADD X0, X1, #10
	enc := uint32(0x91002820)
	verifyDecode(t, enc, arm64asm.ADD)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 5)
	stepN(t, em, 1)
	assertX(t, em, 0, 15)
}

func TestADD_W(t *testing.T) {
	// ADD W0, W1, W2
	enc := uint32(0x0B020020)
	verifyDecode(t, enc, arm64asm.ADD)

	em := newTestEmulator(enc)
	em.Regs.WriteW(1, 0xFFFFFFFF)
	em.Regs.WriteW(2, 1)
	stepN(t, em, 1)
	assertW(t, em, 0, 0) // wraps to 0
	// W write zero-extends, so X0 should also be 0
	assertX(t, em, 0, 0)
}

func TestSUB_Reg(t *testing.T) {
	// SUB X0, X1, X2
	enc := uint32(0xCB020020)
	verifyDecode(t, enc, arm64asm.SUB)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 300)
	em.Regs.WriteX(2, 100)
	stepN(t, em, 1)
	assertX(t, em, 0, 200)
}

func TestSUB_Imm(t *testing.T) {
	// SUB X0, X1, #10
	enc := uint32(0xD1002820)
	verifyDecode(t, enc, arm64asm.SUB)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 42)
	stepN(t, em, 1)
	assertX(t, em, 0, 32)
}

func TestADDS(t *testing.T) {
	// ADDS X0, X1, X2
	enc := uint32(0xAB020020)
	verifyDecode(t, enc, arm64asm.ADDS)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 0)
	em.Regs.WriteX(2, 0)
	stepN(t, em, 1)
	assertX(t, em, 0, 0)
	assertFlags(t, em, false, true, false, false) // Z=1
}

func TestSUBS_Flags(t *testing.T) {
	// SUBS X0, X1, X2
	enc := uint32(0xEB020020)
	verifyDecode(t, enc, arm64asm.SUBS)

	// Equal values: result=0, Z=1, C=1 (no borrow)
	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 42)
	em.Regs.WriteX(2, 42)
	stepN(t, em, 1)
	assertX(t, em, 0, 0)
	assertFlags(t, em, false, true, true, false) // Z=1, C=1

	// Negative result
	em = newTestEmulator(enc)
	em.Regs.WriteX(1, 10)
	em.Regs.WriteX(2, 20)
	stepN(t, em, 1)
	if !em.Regs.N {
		t.Error("N should be set for negative result")
	}
}

func TestMUL(t *testing.T) {
	// MUL X0, X1, X2
	enc := uint32(0x9B027C20)
	verifyDecode(t, enc, arm64asm.MUL)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 7)
	em.Regs.WriteX(2, 6)
	stepN(t, em, 1)
	assertX(t, em, 0, 42)
}

func TestMADD(t *testing.T) {
	// MADD X0, X1, X2, X3
	enc := uint32(0x9B020C20)
	verifyDecode(t, enc, arm64asm.MADD)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 3)
	em.Regs.WriteX(2, 4)
	em.Regs.WriteX(3, 10)
	stepN(t, em, 1)
	assertX(t, em, 0, 22) // 10 + 3*4 = 22
}

func TestNEG(t *testing.T) {
	// NEG X0, X1 => SUB X0, XZR, X1 => 0xCB0103E0
	enc := uint32(0xCB0103E0)
	verifyDecode(t, enc, arm64asm.NEG)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 42)
	stepN(t, em, 1)
	assertX(t, em, 0, ^uint64(42)+1)
}

func TestSDIV(t *testing.T) {
	// SDIV X0, X1, X2 => 0x9AC20C20
	enc := uint32(0x9AC20C20)
	verifyDecode(t, enc, arm64asm.SDIV)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, ^uint64(100)+1) // -100
	em.Regs.WriteX(2, 10)
	stepN(t, em, 1)
	assertX(t, em, 0, ^uint64(10)+1) // -10
}

func TestUDIV(t *testing.T) {
	// UDIV X0, X1, X2 => 0x9AC20820
	enc := uint32(0x9AC20820)
	verifyDecode(t, enc, arm64asm.UDIV)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 100)
	em.Regs.WriteX(2, 10)
	stepN(t, em, 1)
	assertX(t, em, 0, 10)
}

func TestUDIV_ByZero(t *testing.T) {
	enc := uint32(0x9AC20820)
	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 100)
	em.Regs.WriteX(2, 0)
	stepN(t, em, 1)
	assertX(t, em, 0, 0) // ARM64: div by zero => 0
}

func TestUMULH(t *testing.T) {
	// UMULH X0, X1, X2 => 0x9BC27C20
	enc := uint32(0x9BC27C20)
	verifyDecode(t, enc, arm64asm.UMULH)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 0xFFFFFFFFFFFFFFFF)
	em.Regs.WriteX(2, 2)
	stepN(t, em, 1)
	assertX(t, em, 0, 1) // (2^64-1)*2 = 2^65-2, hi=1
}

func TestMSUB(t *testing.T) {
	// MSUB X0, X1, X2, X3 => 0x9B028C20
	enc := uint32(0x9B028C20)
	verifyDecode(t, enc, arm64asm.MSUB)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 3)
	em.Regs.WriteX(2, 4)
	em.Regs.WriteX(3, 20)
	stepN(t, em, 1)
	assertX(t, em, 0, 8) // 20 - 3*4 = 8
}
