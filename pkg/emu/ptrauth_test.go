package emu

import (
	"encoding/binary"
	"strings"
	"testing"

	"golang.org/x/arch/arm64/arm64asm"
)

type authMemory struct{ word uint32 }

func (m authMemory) Fetch(_ uint64, b []byte) error {
	binary.LittleEndian.PutUint32(b, m.word)
	return nil
}
func (m authMemory) Read(_ uint64, _ []byte) error  { panic("unexpected read") }
func (m authMemory) Write(_ uint64, _ []byte) error { panic("unexpected write") }
func TestDisabledAddressAuthentication(t *testing.T) {
	// Apple Clang -march=armv8.3-a assembled encodings. Generic-arm64 native
	// probes retain values even with wrong modifier/key, while arm64e traps.
	for _, w := range []uint32{0xd503237f, 0xd503233f, 0xd50323bf, 0xd50323ff, 0xd503211f, 0xd503215f, 0xd503219f, 0xd50321df, 0xd503231f, 0xd503235f, 0xd503239f, 0xd50323df, 0xdac10020, 0xdac10462, 0xdac108a4, 0xdac10ce6, 0xdac11128, 0xdac1156a, 0xdac119ac, 0xdac11dee, 0xdac123e0, 0xdac127e1, 0xdac12be2, 0xdac12fe3, 0xdac133e4, 0xdac137e5, 0xdac13be6, 0xdac13fe7} {
		r := Registers{PC: 0x1000, SP: 0x8000, PointerAuth: PointerAuthDisabled, N: true}
		for i := range r.GPR {
			r.GPR[i] = 0xa500000000000001 + uint64(i)
		}
		before := r
		result, e := StepChecked(&r, authMemory{w})
		if e != nil {
			t.Fatalf("%#x: %v", w, e)
		}
		before.PC += 4
		if r != before || result.Instruction.Enc != w || result.Instruction.Op != arm64asm.NOP {
			t.Fatalf("disabled authentication changed state %#x", w)
		}
		for _, mode := range []PointerAuthMode{PointerAuthUnspecified, PointerAuthEnabled} {
			r = before
			r.PointerAuth = mode
			beforeFault := r
			if _, e = StepChecked(&r, authMemory{w}); e == nil || !strings.Contains(e.Error(), "pointer authentication") {
				t.Fatalf("mode %d accepted %#x", mode, w)
			}
			if r != beforeFault {
				t.Fatal("fault changed state")
			}
		}
	}
}
func TestDisabledAuthenticatedBranches(t *testing.T) {
	for _, tc := range []struct {
		word   uint32
		target int
		link   bool
		op     arm64asm.Op
	}{
		{0xd71f0801, 0, false, arm64asm.BR}, {0xd71f0c43, 2, false, arm64asm.BR}, {0xd61f089f, 4, false, arm64asm.BR}, {0xd61f0cbf, 5, false, arm64asm.BR},
		{0xd73f08c7, 6, true, arm64asm.BLR}, {0xd73f0d09, 8, true, arm64asm.BLR}, {0xd63f095f, 10, true, arm64asm.BLR}, {0xd63f0d7f, 11, true, arm64asm.BLR},
		{0xd65f0bff, 30, false, arm64asm.RET}, {0xd65f0fff, 30, false, arm64asm.RET},
	} {
		r := Registers{PC: 0x1000, PointerAuth: PointerAuthDisabled}
		r.GPR[30] = 0xdead
		r.GPR[tc.target] = 0x4000
		step, e := StepChecked(&r, authMemory{tc.word})
		if e != nil || r.PC != 0x4000 || !step.Branched || step.Instruction.Op != tc.op || step.Instruction.Enc != tc.word {
			t.Fatalf("%#x %+v %v", tc.word, r, e)
		}
		if tc.link && r.GPR[30] != 0x1004 {
			t.Fatal("link lost")
		}
		if !tc.link && tc.target != 30 && r.GPR[30] != 0xdead {
			t.Fatal("unexpected link")
		}
	}
}
func TestAuthDecoderRejectsNeighboringUnsupportedOperations(t *testing.T) {
	for _, w := range []uint32{0xdac143e0, 0xdac147e1, 0xd50320ff, 0xdac123c0, 0x9ac13020, 0xd503201f /* ordinary NOP is handled by base decoder */} {
		_, _, recognized := disabledAuthOperation(w)
		if recognized {
			t.Fatalf("unimplemented/reserved form lowered %#x", w)
		}
	}
}

func TestSyscallPersonalityIsExplicit(t *testing.T) {
	for _, abi := range []SyscallABI{SyscallLinux, SyscallDarwin} {
		for _, word := range []uint32{0xd4000001, 0xd4001001} {
			r := Registers{PC: 0x1000, SyscallABI: abi}
			before := r
			step, e := StepChecked(&r, authMemory{word})
			valid := (abi == SyscallLinux && word == 0xd4000001) || (abi == SyscallDarwin && word == 0xd4001001)
			if (e == nil) != valid || step.Syscall != valid || r != before {
				t.Fatalf("abi=%d word=%#x step=%+v err=%v", abi, word, step, e)
			}
		}
	}
}
