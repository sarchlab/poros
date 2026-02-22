package emu

import (
	"testing"

	"golang.org/x/arch/arm64/arm64asm"
)

func TestSTR_LDR_X(t *testing.T) {
	// STR X1, [X2]     => 0xF9000041
	// LDR X0, [X2]     => 0xF9400040
	em := newTestEmulator(0xF9000041, 0xF9400040)
	em.Regs.WriteX(1, 0xDEADBEEF12345678)
	em.Regs.WriteX(2, 0x10000)
	stepN(t, em, 2)
	assertX(t, em, 0, 0xDEADBEEF12345678)
}

func TestSTR_LDR_W(t *testing.T) {
	// STR W1, [X2]     => 0xB9000041
	// LDR W0, [X2]     => 0xB9400040
	em := newTestEmulator(0xB9000041, 0xB9400040)
	em.Regs.WriteW(1, 0xABCD1234)
	em.Regs.WriteX(2, 0x10000)
	stepN(t, em, 2)
	assertW(t, em, 0, 0xABCD1234)
}

func TestSTRB_LDRB(t *testing.T) {
	// STRB W1, [X2]    => 0x39000041
	// LDRB W0, [X2]    => 0x39400040
	em := newTestEmulator(0x39000041, 0x39400040)
	em.Regs.WriteW(1, 0xFF)
	em.Regs.WriteX(2, 0x10000)
	stepN(t, em, 2)
	assertW(t, em, 0, 0xFF)
}

func TestLDRSB(t *testing.T) {
	// First store 0x80 (=-128 signed) at address
	// STRB W1, [X2]    => 0x39000041
	// LDRSB X0, [X2]   => 0x39800040
	enc_strb := uint32(0x39000041)
	enc_ldrsb := uint32(0x39800040)
	verifyDecode(t, enc_ldrsb, arm64asm.LDRSB)

	em := newTestEmulator(enc_strb, enc_ldrsb)
	em.Regs.WriteW(1, 0x80)
	em.Regs.WriteX(2, 0x10000)
	stepN(t, em, 2)
	assertX(t, em, 0, 0xFFFFFFFFFFFFFF80) // sign-extended -128
}

func TestLDR_Offset(t *testing.T) {
	// LDR X0, [X1, #8] => 0xF9400420
	enc := uint32(0xF9400420)
	verifyDecode(t, enc, arm64asm.LDR)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 0x10000)
	em.Mem.WriteU64(0x10008, 0x42)
	stepN(t, em, 1)
	assertX(t, em, 0, 0x42)
}

func TestSTP_LDP(t *testing.T) {
	// STP X1, X2, [X3]       => 0xA9000861
	// LDP X4, X5, [X3]       => 0xA9401464
	em := newTestEmulator(0xA9000861, 0xA9401464)
	em.Regs.WriteX(1, 0x1111)
	em.Regs.WriteX(2, 0x2222)
	em.Regs.WriteX(3, 0x10000)
	stepN(t, em, 2)
	assertX(t, em, 4, 0x1111)
	assertX(t, em, 5, 0x2222)
}

func TestSTUR_LDUR(t *testing.T) {
	// STUR X1, [X2, #-8]   => 0xF81F8041
	// LDUR X0, [X2, #-8]   => 0xF85F8040
	em := newTestEmulator(0xF81F8041, 0xF85F8040)
	em.Regs.WriteX(1, 0xCAFEBABE)
	em.Regs.WriteX(2, 0x10008)
	stepN(t, em, 2)
	assertX(t, em, 0, 0xCAFEBABE)
}

func TestLDR_PreIndex(t *testing.T) {
	// LDR X0, [X1, #16]!  => 0xF8410C20
	enc := uint32(0xF8410C20)
	verifyDecode(t, enc, arm64asm.LDR)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 0x10000)
	em.Mem.WriteU64(0x10010, 0xBEEF)
	stepN(t, em, 1)
	assertX(t, em, 0, 0xBEEF)
	assertX(t, em, 1, 0x10010) // X1 updated to base + offset
}

func TestLDR_PostIndex(t *testing.T) {
	// LDR X0, [X1], #16   => 0xF8410420
	enc := uint32(0xF8410420)
	verifyDecode(t, enc, arm64asm.LDR)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 0x10000)
	em.Mem.WriteU64(0x10000, 0xDEAD)
	stepN(t, em, 1)
	assertX(t, em, 0, 0xDEAD)
	assertX(t, em, 1, 0x10010) // X1 updated post-load
}

func TestSTRH_LDRH(t *testing.T) {
	// STRH W1, [X2]    => 0x79000041
	// LDRH W0, [X2]    => 0x79400040
	em := newTestEmulator(0x79000041, 0x79400040)
	em.Regs.WriteW(1, 0xABCD)
	em.Regs.WriteX(2, 0x10000)
	stepN(t, em, 2)
	assertW(t, em, 0, 0xABCD)
}

func TestLDRSW(t *testing.T) {
	// LDRSW X0, [X1]   => 0xB9800020
	enc := uint32(0xB9800020)
	verifyDecode(t, enc, arm64asm.LDRSW)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 0x10000)
	em.Mem.WriteU32(0x10000, 0x80000000) // -2147483648 as int32
	stepN(t, em, 1)
	assertX(t, em, 0, 0xFFFFFFFF80000000) // sign-extended
}

func TestSTP_LDP_W(t *testing.T) {
	// STP W1, W2, [X3]       => 0x29000861
	// LDP W4, W5, [X3]       => 0x29401464
	em := newTestEmulator(0x29000861, 0x29401464)
	em.Regs.WriteW(1, 0xAAAA)
	em.Regs.WriteW(2, 0xBBBB)
	em.Regs.WriteX(3, 0x10000)
	stepN(t, em, 2)
	assertW(t, em, 4, 0xAAAA)
	assertW(t, em, 5, 0xBBBB)
}

func TestSTP_LDP_Q(t *testing.T) {
	// STP Q0, Q1, [X2]       => 0xAD000440
	// LDP Q2, Q3, [X2]       => 0xAD400C42  (need to verify)
	// Let me use STP then manual read, plus separate LDP test.

	// Test STP Q0, Q1, [X2]
	em := newTestEmulator(0xAD000440)
	em.Regs.WriteQ(0, 0x1111111122222222, 0x3333333344444444)
	em.Regs.WriteQ(1, 0xAAAAAAAABBBBBBBB, 0xCCCCCCCCDDDDDDDD)
	em.Regs.WriteX(2, 0x10000)
	stepN(t, em, 1)

	// Verify memory contents
	if got := em.Mem.ReadU64(0x10000); got != 0x1111111122222222 {
		t.Errorf("mem[0x10000] = %#x, want 0x1111111122222222", got)
	}
	if got := em.Mem.ReadU64(0x10008); got != 0x3333333344444444 {
		t.Errorf("mem[0x10008] = %#x, want 0x3333333344444444", got)
	}
	if got := em.Mem.ReadU64(0x10010); got != 0xAAAAAAAABBBBBBBB {
		t.Errorf("mem[0x10010] = %#x, want 0xAAAAAAAABBBBBBBB", got)
	}
	if got := em.Mem.ReadU64(0x10018); got != 0xCCCCCCCCDDDDDDDD {
		t.Errorf("mem[0x10018] = %#x, want 0xCCCCCCCCDDDDDDDD", got)
	}
}

func TestLDP_Q(t *testing.T) {
	// LDP Q0, Q1, [X2]       => 0xAD400440
	em := newTestEmulator(0xAD400440)
	em.Regs.WriteX(2, 0x10000)
	em.Mem.WriteU64(0x10000, 0xAAAA)
	em.Mem.WriteU64(0x10008, 0xBBBB)
	em.Mem.WriteU64(0x10010, 0xCCCC)
	em.Mem.WriteU64(0x10018, 0xDDDD)
	stepN(t, em, 1)

	lo0, hi0 := em.Regs.ReadQ(0)
	lo1, hi1 := em.Regs.ReadQ(1)
	if lo0 != 0xAAAA || hi0 != 0xBBBB {
		t.Errorf("Q0 = (%#x, %#x), want (0xAAAA, 0xBBBB)", lo0, hi0)
	}
	if lo1 != 0xCCCC || hi1 != 0xDDDD {
		t.Errorf("Q1 = (%#x, %#x), want (0xCCCC, 0xDDDD)", lo1, hi1)
	}
}
