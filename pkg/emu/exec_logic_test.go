package emu

import (
	"strconv"
	"testing"

	"golang.org/x/arch/arm64/arm64asm"
)

func TestAND_Reg(t *testing.T) {
	// AND X0, X1, X2 => 0x8A020020
	enc := uint32(0x8A020020)
	verifyDecode(t, enc, arm64asm.AND)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 0xFF00FF00)
	em.Regs.WriteX(2, 0x0F0F0F0F)
	stepN(t, em, 1)
	assertX(t, em, 0, 0x0F000F00)
}

func TestORR_Reg(t *testing.T) {
	// ORR X0, X1, X2 => 0xAA020020
	enc := uint32(0xAA020020)
	verifyDecode(t, enc, arm64asm.ORR)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 0xFF00)
	em.Regs.WriteX(2, 0x00FF)
	stepN(t, em, 1)
	assertX(t, em, 0, 0xFFFF)
}

func TestEOR_Reg(t *testing.T) {
	// EOR X0, X1, X2 => 0xCA020020
	enc := uint32(0xCA020020)
	verifyDecode(t, enc, arm64asm.EOR)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 0xFFFF)
	em.Regs.WriteX(2, 0x0F0F)
	stepN(t, em, 1)
	assertX(t, em, 0, 0xF0F0)
}

func TestMOVZ(t *testing.T) {
	// MOVZ X0, #0x1234 decodes as MOV X0, #0x1234
	enc := uint32(0xD2824680)
	verifyDecode(t, enc, arm64asm.MOV)

	em := newTestEmulator(enc)
	stepN(t, em, 1)
	assertX(t, em, 0, 0x1234)
}

func TestMOVZ_Shifted(t *testing.T) {
	// MOVZ X0, #0x5678, LSL #16 decodes as MOV X0, #0x56780000
	enc := uint32(0xD2AACF00)
	verifyDecode(t, enc, arm64asm.MOV)

	em := newTestEmulator(enc)
	stepN(t, em, 1)
	assertX(t, em, 0, 0x56780000)
}

func TestMOVK(t *testing.T) {
	// MOVZ X0, #0x1234          => 0xD2824680
	// MOVK X0, #0x5678, LSL #16 => 0xF2AACF00
	em := newTestEmulator(0xD2824680, 0xF2AACF00)
	stepN(t, em, 2)
	assertX(t, em, 0, 0x56781234)
}

func TestMOVN(t *testing.T) {
	// MOVN X0, #0 decodes as MOV X0, #0xFFFFFFFFFFFFFFFF
	enc := uint32(0x92800000)
	verifyDecode(t, enc, arm64asm.MOV)

	em := newTestEmulator(enc)
	stepN(t, em, 1)
	assertX(t, em, 0, 0xFFFFFFFFFFFFFFFF)
}

func TestCMP_Equal(t *testing.T) {
	// CMP X1, X2 => SUBS XZR, X1, X2 => 0xEB02003F
	enc := uint32(0xEB02003F)
	verifyDecode(t, enc, arm64asm.CMP)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 42)
	em.Regs.WriteX(2, 42)
	stepN(t, em, 1)
	assertFlags(t, em, false, true, true, false)
}

func TestCMP_Less(t *testing.T) {
	enc := uint32(0xEB02003F)
	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 10)
	em.Regs.WriteX(2, 20)
	stepN(t, em, 1)
	if !em.Regs.N {
		t.Error("N should be set when op1 < op2 (signed)")
	}
}

func TestCSEL(t *testing.T) {
	// CMP X1, X2              => 0xEB02003F
	// CSEL X0, X3, X4, EQ    => 0x9A840060
	em := newTestEmulator(0xEB02003F, 0x9A840060)
	em.Regs.WriteX(1, 5)
	em.Regs.WriteX(2, 5)
	em.Regs.WriteX(3, 100)
	em.Regs.WriteX(4, 200)
	stepN(t, em, 2)
	assertX(t, em, 0, 100) // EQ is true, so select X3
}

func TestCSEL_NotTaken(t *testing.T) {
	em := newTestEmulator(0xEB02003F, 0x9A840060)
	em.Regs.WriteX(1, 5)
	em.Regs.WriteX(2, 10)
	em.Regs.WriteX(3, 100)
	em.Regs.WriteX(4, 200)
	stepN(t, em, 2)
	assertX(t, em, 0, 200) // EQ is false, so select X4
}

func TestCSNEG_CondTrue(t *testing.T) {
	// CMP X3, X4                => 0xEB04007F
	// CSNEG X0, X1, X2, EQ     => 0xDA820420
	em := newTestEmulator(0xEB04007F, 0xDA820420)
	em.Regs.WriteX(3, 42)
	em.Regs.WriteX(4, 42) // equal, so EQ is true
	em.Regs.WriteX(1, 100)
	em.Regs.WriteX(2, 200)
	stepN(t, em, 2)
	assertX(t, em, 0, 100) // cond true: Rn
}

func TestCSNEG_CondFalse(t *testing.T) {
	// CMP X3, X4                => 0xEB04007F
	// CSNEG X0, X1, X2, EQ     => 0xDA820420
	em := newTestEmulator(0xEB04007F, 0xDA820420)
	em.Regs.WriteX(3, 42)
	em.Regs.WriteX(4, 99) // not equal, so EQ is false
	em.Regs.WriteX(1, 100)
	em.Regs.WriteX(2, 5)
	stepN(t, em, 2)
	// cond false: -Rm = -5
	want := ^uint64(5) + 1 // two's complement of 5
	assertX(t, em, 0, want)
}

func TestCNEG_CondTrue(t *testing.T) {
	// CMP X3, X4                => 0xEB04007F
	// CNEG X0, X1, EQ           => 0xDA811420
	enc := uint32(0xDA811420)
	verifyDecode(t, enc, arm64asm.CNEG)

	em := newTestEmulator(0xEB04007F, enc)
	em.Regs.WriteX(3, 42)
	em.Regs.WriteX(4, 42) // equal, so EQ is true
	em.Regs.WriteX(1, 7)
	stepN(t, em, 2)
	assertX(t, em, 0, ^uint64(7)+1) // CNEG alias: negate when EQ is true.
}

func TestCNEG_CondFalse(t *testing.T) {
	// CMP X3, X4                => 0xEB04007F
	// CNEG X0, X1, EQ           => 0xDA811420
	enc := uint32(0xDA811420)
	verifyDecode(t, enc, arm64asm.CNEG)

	em := newTestEmulator(0xEB04007F, enc)
	em.Regs.WriteX(3, 42)
	em.Regs.WriteX(4, 99) // not equal, so EQ is false
	em.Regs.WriteX(1, 5)
	stepN(t, em, 2)
	// CNEG alias: preserve when EQ is false.
	want := uint64(5)
	assertX(t, em, 0, want)
}

func TestLSL_Imm(t *testing.T) {
	// LSL X0, X1, #4 => UBFM X0, X1, #60, #59 => 0xD37CEC20
	enc := uint32(0xD37CEC20)
	verifyDecode(t, enc, arm64asm.LSL)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 1)
	stepN(t, em, 1)
	assertX(t, em, 0, 16) // 1 << 4 = 16
}

func TestLSR_Imm(t *testing.T) {
	// LSR X0, X1, #4 => UBFM X0, X1, #4, #63 => 0xD344FC20
	enc := uint32(0xD344FC20)
	verifyDecode(t, enc, arm64asm.LSR)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 0x100)
	stepN(t, em, 1)
	assertX(t, em, 0, 0x10)
}

func TestASR_Imm(t *testing.T) {
	// ASR X0, X1, #4 => SBFM X0, X1, #4, #63 => 0x9344FC20
	enc := uint32(0x9344FC20)
	verifyDecode(t, enc, arm64asm.ASR)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, ^uint64(16)+1) // -16
	stepN(t, em, 1)
	assertX(t, em, 0, ^uint64(0)) // -1 (all bits set)
}

func TestSXTW(t *testing.T) {
	// SXTW X0, W1 => SBFM X0, X1, #0, #31 => 0x93407C20
	enc := uint32(0x93407C20)
	verifyDecode(t, enc, arm64asm.SXTW)

	em := newTestEmulator(enc)
	em.Regs.WriteW(1, 0x80000000) // -2147483648 as int32
	stepN(t, em, 1)
	assertX(t, em, 0, 0xFFFFFFFF80000000) // sign-extended
}

func TestCLZ(t *testing.T) {
	// CLZ X0, X1 => 0xDAC01020
	enc := uint32(0xDAC01020)
	verifyDecode(t, enc, arm64asm.CLZ)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 0x0000000100000000)
	stepN(t, em, 1)
	assertX(t, em, 0, 31)
}

func TestADRP(t *testing.T) {
	// ADRP X0, #0x1000 => 0xB0000000
	// ADRP computes (PC & ~0xFFF) + offset
	// Decoded: ADRP X0, .+0x1000
	enc := uint32(0xB0000000)
	verifyDecode(t, enc, arm64asm.ADRP)

	// Load instruction at address 0x1000 (page-aligned)
	em := NewEmulator()
	em.Mem.WriteU32(0x1000, enc)
	em.Regs.PC = 0x1000
	stepN(t, em, 1)
	// base = 0x1000 & ~0xFFF = 0x1000
	// result = 0x1000 + 0x1000 = 0x2000
	assertX(t, em, 0, 0x2000)
}

func TestANDS(t *testing.T) {
	// ANDS X0, X1, X2 => 0xEA020020
	enc := uint32(0xEA020020)
	verifyDecode(t, enc, arm64asm.ANDS)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 0)
	em.Regs.WriteX(2, 0xFF)
	stepN(t, em, 1)
	assertX(t, em, 0, 0)
	if !em.Regs.Z {
		t.Error("Z should be set when result is 0")
	}
}

func TestTST(t *testing.T) {
	// TST X1, X2 => ANDS XZR, X1, X2 => 0xEA02003F
	enc := uint32(0xEA02003F)
	verifyDecode(t, enc, arm64asm.TST)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 0x10)
	em.Regs.WriteX(2, 0x10)
	stepN(t, em, 1)
	if em.Regs.Z {
		t.Error("Z should not be set for non-zero result")
	}
}

func TestMVN(t *testing.T) {
	// MVN X0, X1 => ORN X0, XZR, X1 => 0xAA2103E0
	enc := uint32(0xAA2103E0)
	verifyDecode(t, enc, arm64asm.MVN)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 0)
	stepN(t, em, 1)
	assertX(t, em, 0, 0xFFFFFFFFFFFFFFFF)
}

func TestBitfieldExtractAliasWidth(t *testing.T) {
	// Encode UBFM/SBFM with imms=lsb+width-1. The decoder exposes the
	// UBFX/SBFX aliases as lsb,width; the handlers must not reinterpret width.
	for _, size := range []uint{32, 64} {
		for _, signed := range []bool{false, true} {
			for lsb := uint(0); lsb < size-1; lsb++ {
				for width := uint(1); width < size-lsb; width++ {
					base := uint32(0x53000000)
					if size == 64 {
						base = 0xd3400000
					}
					if signed {
						base &^= 0x40000000
					}
					opcode := base | uint32(lsb)<<16 | uint32(lsb+width-1)<<10 | 1<<5
					for _, source := range []uint64{0, 1, 0x8000000080000000, 0x200000000a8d42, ^uint64(0)} {
						regs := Registers{PC: 0x1000}
						regs.GPR[1] = source
						if _, err := StepChecked(&regs, authMemory{opcode}); err != nil {
							t.Fatalf("%#x: %v", opcode, err)
						}
						if size == 32 {
							source = uint64(uint32(source))
						}
						mask := (uint64(1) << width) - 1
						want := (source >> lsb) & mask
						if signed && want&(uint64(1)<<(width-1)) != 0 {
							want |= ^mask
						}
						if size == 32 {
							want = uint64(uint32(want))
						}
						if regs.GPR[0] != want {
							t.Fatalf("size=%d signed=%v lsb=%d width=%d source=%#x got=%#x want=%#x", size, signed, lsb, width, source, regs.GPR[0], want)
						}
					}
				}
			}
		}
	}
}

func TestMOVKNativePreservesOldDestination(t *testing.T) {
	var native struct {
		Cases []struct {
			Bits, Shift, Immediate int
			Destination, Output    string
		}
	}
	nativeAtomicObservation(t, "movk-probe.c", &native)
	if len(native.Cases) != 96 {
		t.Fatal("native MOVK cases missing")
	}
	for _, tc := range native.Cases {
		word := uint32(0x72800001) | uint32(tc.Shift/16)<<21 | uint32(tc.Immediate)<<5
		if tc.Bits == 64 {
			word |= 1 << 31
		}
		destination, e := strconv.ParseUint(tc.Destination, 16, 64)
		if e != nil {
			t.Fatal(e)
		}
		want, e := strconv.ParseUint(tc.Output, 16, 64)
		if e != nil {
			t.Fatal(e)
		}
		r := Registers{PC: 0x1000, C: true}
		r.GPR[1] = destination
		before := r
		if _, err := StepChecked(&r, authMemory{word}); err != nil || r.GPR[1] != want {
			t.Fatal(tc, err, r.GPR[1])
		}
		before.GPR[1] = want
		before.PC += 4
		if before != r {
			t.Fatal("MOVK changed unrelated state")
		}
	}
}

func TestBFXILNativeVectors(t *testing.T) {
	var native struct {
		Cases []struct {
			Bits, LSB, Width            int
			Alias                       bool
			Destination, Source, Output string
		}
	}
	nativeAtomicObservation(t, "bfxil-probe.c", &native)
	if len(native.Cases) != 640 {
		t.Fatal("native BFXIL cases missing")
	}
	for _, tc := range native.Cases {
		parse := func(s string) uint64 {
			v, e := strconv.ParseUint(s, 16, 64)
			if e != nil {
				t.Fatal(e)
			}
			return v
		}
		word := uint32(0x33000001) | uint32(tc.LSB)<<16 | uint32(tc.LSB+tc.Width-1)<<10
		if tc.Bits == 64 {
			word |= 0x80400000
		}
		rn := 2
		if tc.Alias {
			rn = 1
		}
		word |= uint32(rn) << 5
		r := Registers{PC: 0x1000, C: true}
		r.GPR[1] = parse(tc.Destination)
		r.GPR[rn] = parse(tc.Source)
		before := r
		want := parse(tc.Output)
		if _, e := StepChecked(&r, authMemory{word}); e != nil || r.GPR[1] != want {
			t.Fatal(tc, e, r.GPR[1])
		}
		before.GPR[1] = want
		before.PC += 4
		if r != before {
			t.Fatal("BFXIL changed unrelated state")
		}
	}
}

func TestScalarReversalOneHotAndZeroRegisters(t *testing.T) {
	for _, size := range []int{32, 64} {
		for _, reverseBytes := range []bool{false, true} {
			base := uint32(0x5ac00000)
			if size == 64 {
				base |= 0x80000000
			}
			if reverseBytes {
				base |= 0x800
				if size == 64 {
					base |= 0x400
				}
			}
			for bit := 0; bit < size; bit++ {
				r := Registers{PC: 0x1000}
				r.GPR[2] = uint64(1) << bit
				target := size - 1 - bit
				if reverseBytes {
					target = (size/8-1-bit/8)*8 + bit%8
				}
				if _, err := StepChecked(&r, authMemory{base | 2<<5 | 1}); err != nil || r.GPR[1] != (uint64(1)<<target) {
					t.Fatal(size, reverseBytes, bit, err, r.GPR[1])
				}
			}
			r := Registers{PC: 0x1000}
			r.GPR[1] = ^uint64(0)
			if _, err := StepChecked(&r, authMemory{base | 31<<5 | 1}); err != nil || r.GPR[1] != 0 {
				t.Fatal("zero source", err, r.GPR[1])
			}
			before := r
			if _, err := StepChecked(&r, authMemory{base | 1<<5 | 31}); err != nil {
				t.Fatal(err)
			}
			before.PC += 4
			if r != before {
				t.Fatal("zero destination changed state")
			}
		}
	}
}

func TestScalarReversalNativeVectors(t *testing.T) {
	var native struct {
		Cases []struct {
			Operation     string
			Bits          int
			Alias         bool
			Input, Output string
		}
	}
	nativeAtomicObservation(t, "rbit-probe.c", &native)
	if len(native.Cases) != 544 {
		t.Fatal("native reversal cases missing")
	}
	for _, tc := range native.Cases {
		word := uint32(0x5ac00001)
		if tc.Bits == 64 {
			word |= 0x80000000
		}
		if tc.Operation == "rev" {
			word |= 0x800
			if tc.Bits == 64 {
				word |= 0x400
			}
		}
		rn := 2
		if tc.Alias {
			rn = 1
		}
		word |= uint32(rn) << 5
		input, err := strconv.ParseUint(tc.Input, 16, 64)
		if err != nil {
			t.Fatal(err)
		}
		want, err := strconv.ParseUint(tc.Output, 16, 64)
		if err != nil {
			t.Fatal(err)
		}
		r := Registers{PC: 0x1000, C: true}
		r.GPR[1] = ^uint64(0)
		r.GPR[rn] = input
		before := r
		if _, err := StepChecked(&r, authMemory{word}); err != nil || r.GPR[1] != want {
			t.Fatal(tc, err, r.GPR[1])
		}
		before.GPR[1] = want
		before.PC += 4
		if r != before {
			t.Fatal("reversal changed unrelated state")
		}
	}
}

func TestCSINVNativeConditionsAndAliases(t *testing.T) {
	var native struct {
		Cases []struct {
			Bits, Condition, Alias, NZCV int
			A, B, Output                 string
		}
	}
	nativeAtomicObservation(t, "csinv-probe.c", &native)
	if len(native.Cases) != 1536 {
		t.Fatal("missing native CSINV vectors")
	}
	for _, tc := range native.Cases {
		parse := func(s string) uint64 {
			v, e := strconv.ParseUint(s, 16, 64)
			if e != nil {
				t.Fatal(e)
			}
			return v
		}
		rd := 3
		if tc.Alias != 0 {
			rd = tc.Alias
		}
		word := uint32(0x5a800000) | 2<<16 | uint32(tc.Condition)<<12 | 1<<5 | uint32(rd)
		if tc.Bits == 64 {
			word |= 1 << 31
		}
		r := Registers{PC: 0x1000, N: tc.NZCV&8 != 0, Z: tc.NZCV&4 != 0, C: tc.NZCV&2 != 0, V: tc.NZCV&1 != 0}
		r.GPR[1] = parse(tc.A)
		r.GPR[2] = parse(tc.B)
		before := r
		want := parse(tc.Output)
		if _, err := StepChecked(&r, authMemory{word}); err != nil || r.GPR[rd] != want {
			t.Fatal(tc, err, r.GPR[rd])
		}
		before.GPR[rd] = want
		before.PC += 4
		if before != r {
			t.Fatal("CSINV changed unrelated state")
		}
	}
	// Actual dyld form: CSINV W8,W8,WZR,CS.
	for _, carry := range []bool{false, true} {
		r := Registers{PC: 0x1000, C: carry}
		r.GPR[8] = 0x123456789abcdef0
		want := uint64(0xffffffff)
		if carry {
			want = 0x9abcdef0
		}
		if _, err := StepChecked(&r, authMemory{0x5a9f2108}); err != nil || r.GPR[8] != want {
			t.Fatal(carry, err, r.GPR[8])
		}
	}
}
