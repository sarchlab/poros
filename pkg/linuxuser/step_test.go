package linuxuser

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/sarchlab/poros/pkg/emu"
)

func TestCheckedInstructionFaultIsAtomic(t *testing.T) {
	// GNU assembled post-index X pairs/singles and pre-index Q pairs. A prefix is
	// valid but the full access is not; neither writeback nor first half may commit.
	for _, tc := range []struct {
		name   string
		opcode uint32
		size   int
		pre    bool
	}{
		{"ldp-post", 0xa8c10440, 8, false}, {"stp-post", 0xa8810440, 8, false},
		{"ldp-q-pre", 0xadc10440, 16, true}, {"stp-q-pre", 0xad810440, 16, true},
		{"ldr-post", 0xf8408440, 4, false}, {"str-post", 0xf8008440, 4, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code := make([]byte, 4)
			binary.LittleEndian.PutUint32(code, tc.opcode)
			data := bytes.Repeat([]byte{0x55}, tc.size)
			m := testAddressSpace(t, testRegion{0x1000, 4, elf.PF_R | elf.PF_X, code}, testRegion{0x2000, uint64(tc.size), elf.PF_R | elf.PF_W, data})
			regs := emu.Registers{PC: 0x1000}
			regs.GPR[0] = 0x1234
			regs.GPR[1] = 0x5678
			regs.GPR[2] = 0x2000
			if tc.pre {
				regs.GPR[2] -= 32
			}
			regs.WriteQ(0, 1, 2)
			regs.WriteQ(1, 3, 4)
			before := regs
			original := append([]byte{}, data...)
			_, err := emu.StepChecked(&regs, m)
			var fault *emu.StepError
			if !errors.As(err, &fault) || fault.Kind != "memory_fault" {
				t.Fatalf("expected memory fault: %v", err)
			}
			if err := m.Read(0x2000, data); err != nil {
				t.Fatal(err)
			}
			if regs != before || !bytes.Equal(data, original) {
				t.Fatal("fault changed architectural state")
			}
		})
	}
}
func TestCheckedSuccessfulPairAndNoMagicReturn(t *testing.T) {
	code := make([]byte, 8)
	binary.LittleEndian.PutUint32(code, 0xa8810440)
	binary.LittleEndian.PutUint32(code[4:], 0xd65f03c0) // stp;ret
	data := make([]byte, 16)
	m := testAddressSpace(t, testRegion{0x1000, 8, elf.PF_X, code}, testRegion{0x2000, 16, elf.PF_R | elf.PF_W, data})
	regs := emu.Registers{PC: 0x1000}
	regs.GPR[0] = 11
	regs.GPR[1] = 22
	regs.GPR[2] = 0x2000
	if _, err := emu.StepChecked(&regs, m); err != nil {
		t.Fatal(err)
	}
	if err := m.Read(0x2000, data); err != nil {
		t.Fatal(err)
	}
	if regs.GPR[2] != 0x2010 || binary.LittleEndian.Uint64(data) != 11 || binary.LittleEndian.Uint64(data[8:]) != 22 {
		t.Fatal("store pair/writeback wrong")
	}
	step, err := emu.StepChecked(&regs, m)
	if err != nil || !step.Branched || regs.PC != 0 {
		t.Fatalf("RET was not a normal branch: %v", err)
	}
	if _, err = emu.StepChecked(&regs, m); err == nil {
		t.Fatal("unmapped return address succeeded")
	}
}

func instructionMemory(t *testing.T, opcode uint32) *AddressSpace {
	code := make([]byte, 4)
	binary.LittleEndian.PutUint32(code, opcode)
	return testAddressSpace(t, testRegion{0x1000, 4, elf.PF_X, code})
}
func TestCheckedRejectsUnsupportedOperandForms(t *testing.T) {
	for _, tc := range []struct {
		name   string
		opcode uint32
	}{
		{"scalar-simd-add", 0x5ee28420}, {"scalar-simd-sub", 0x7ee28420}, {"scalar-simd-neg", 0x7ee0b820},
		{"svc-immediate", 0xd4000021}, {"simd", 0x4ea28420}, {"half-fp", 0x1ee22820}, {"paciasp-hint", 0xd503233f},
		{"scvtf-vector-scalar", 0x5e61d800}, {"ucvtf-vector-scalar", 0x7e61d800}, {"fcvtzs-vector-scalar", 0x5ee1b800}, {"fcvtzu-vector-scalar", 0x7ee1b800},
		{"scvtf-fixed", 0x9e42fc00}, {"ucvtf-fixed", 0x1e03fc00}, {"fcvtzs-fixed", 0x9e58fc00}, {"fcvtzu-fixed", 0x1e19fc00},
	} {
		t.Run(tc.name, func(t *testing.T) {
			regs := emu.Registers{PC: 0x1000, N: true}
			regs.GPR[0] = 17
			regs.WriteD(0, 3.5)
			regs.WriteD(1, 7)
			regs.WriteD(2, 11)
			before := regs
			_, err := emu.StepChecked(&regs, instructionMemory(t, tc.opcode))
			var fault *emu.StepError
			if !errors.As(err, &fault) || fault.Kind != "unsupported_instruction" {
				t.Fatalf("wrong rejection: %v", err)
			}
			if regs != before {
				t.Fatal("unsupported instruction modified registers")
			}
		})
	}
}
func TestCheckedSupportedScalarConversions(t *testing.T) {
	regs := emu.Registers{PC: 0x1000}
	regs.GPR[0] = 17
	if _, err := emu.StepChecked(&regs, instructionMemory(t, 0x9e620000)); err != nil {
		t.Fatal(err)
	}
	if regs.ReadD(0) != 17 {
		t.Fatal("SCVTF result")
	}
	regs.PC = 0x1000
	regs.WriteD(0, 3.5)
	regs.WriteD(1, 7)
	regs.WriteD(2, 11)
	if _, err := emu.StepChecked(&regs, instructionMemory(t, 0x9e780000)); err != nil {
		t.Fatal(err)
	}
	if regs.GPR[0] != 3 {
		t.Fatal("FCVTZS result")
	}
}
func TestCheckedBranchEvent(t *testing.T) {
	for _, tc := range []struct {
		name           string
		opcode         uint32
		z              bool
		x0, lr, wantPC uint64
		branched       bool
	}{
		{"b-eq-untaken", 0x54000040, false, 0, 0, 0x1004, false},
		{"b-eq-taken", 0x54000040, true, 0, 0, 0x1008, true},
		{"b-eq-fallthrough-target", 0x54000020, true, 0, 0, 0x1004, false},
		{"cbz-untaken", 0xb4000040, false, 1, 0, 0x1004, false},
		{"cbz-taken", 0xb4000040, false, 0, 0, 0x1008, true},
		{"ret", 0xd65f03c0, false, 0, 0x1020, 0x1020, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			regs := emu.Registers{PC: 0x1000, Z: tc.z}
			regs.GPR[0] = tc.x0
			regs.GPR[30] = tc.lr
			step, err := emu.StepChecked(&regs, instructionMemory(t, tc.opcode))
			if err != nil || regs.PC != tc.wantPC || step.Branched != tc.branched {
				t.Fatalf("PC=%#x branch=%v err=%v", regs.PC, step.Branched, err)
			}
		})
	}
}

type testRegion struct {
	start, size uint64
	flags       elf.ProgFlag
	data        []byte
}

func testAddressSpace(t *testing.T, regions ...testRegion) *AddressSpace {
	t.Helper()
	m := &AddressSpace{}
	for _, r := range regions {
		if err := m.mapRegion(r.start, r.size, r.flags, r.data); err != nil {
			t.Fatal(err)
		}
	}
	return m
}
