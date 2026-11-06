package linuxuser

import (
	"bytes"
	"context"
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
			m := &AddressSpace{regions: []mappedRegion{{Mapping{0x1000, 0x1004, elf.PF_R | elf.PF_X}, code}, {Mapping{0x2000, 0x2000 + uint64(tc.size), elf.PF_R | elf.PF_W}, data}}}
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
	m := &AddressSpace{regions: []mappedRegion{{Mapping{0x1000, 0x1008, elf.PF_X}, code}, {Mapping{0x2000, 0x2010, elf.PF_R | elf.PF_W}, data}}}
	regs := emu.Registers{PC: 0x1000}
	regs.GPR[0] = 11
	regs.GPR[1] = 22
	regs.GPR[2] = 0x2000
	if _, err := emu.StepChecked(&regs, m); err != nil {
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

func instructionMemory(opcode uint32) *AddressSpace {
	code := make([]byte, 4)
	binary.LittleEndian.PutUint32(code, opcode)
	return &AddressSpace{regions: []mappedRegion{{Mapping{0x1000, 0x1004, elf.PF_X}, code}}}
}
func TestCheckedRejectsUnsupportedOperandForms(t *testing.T) {
	for _, tc := range []struct {
		name   string
		opcode uint32
	}{
		{"scalar-simd-add", 0x5ee28420}, {"scalar-simd-sub", 0x7ee28420}, {"scalar-simd-neg", 0x7ee0b820},
		{"svc-immediate", 0xd4000021}, {"simd", 0x4ea28420}, {"half-fp", 0x1ee22820}, {"paciasp-hint", 0xd503233f},
		{"fcvtzs-vector-scalar", 0x5ee1b800}, {"fcvtzu-vector-scalar", 0x7ee1b800},
		{"scvtf-fixed", 0x9e42fc00}, {"ucvtf-fixed", 0x1e03fc00}, {"fcvtzs-fixed", 0x9e58fc00}, {"fcvtzu-fixed", 0x1e19fc00},
	} {
		t.Run(tc.name, func(t *testing.T) {
			regs := emu.Registers{PC: 0x1000, N: true}
			regs.GPR[0] = 17
			regs.WriteD(0, 3.5)
			regs.WriteD(1, 7)
			regs.WriteD(2, 11)
			before := regs
			_, err := emu.StepChecked(&regs, instructionMemory(tc.opcode))
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
	if _, err := emu.StepChecked(&regs, instructionMemory(0x9e620000)); err != nil {
		t.Fatal(err)
	}
	if regs.ReadD(0) != 17 {
		t.Fatal("SCVTF result")
	}
	regs.PC = 0x1000
	regs.WriteD(0, 3.5)
	regs.WriteD(1, 7)
	regs.WriteD(2, 11)
	if _, err := emu.StepChecked(&regs, instructionMemory(0x9e780000)); err != nil {
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
			step, err := emu.StepChecked(&regs, instructionMemory(tc.opcode))
			if err != nil || regs.PC != tc.wantPC || step.Branched != tc.branched {
				t.Fatalf("PC=%#x branch=%v err=%v", regs.PC, step.Branched, err)
			}
		})
	}
}

func TestExecutionDecodesCurrentExecutableBytes(t *testing.T) {
	const mov1 = 0xd2800020   // mov x0, #1
	const mov324 = 0xd2802880 // mov x0, #324; aliases mov1 in the decode cache
	m := instructionMemory(mov1)
	p := &Process{Memory: m}
	x, err := p.NewExecution(context.Background(), RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()

	for _, tc := range []struct{ opcode, want uint32 }{{mov1, 1}, {mov324, 324}, {mov1, 1}} {
		p.Registers.PC = 0x1000
		binary.LittleEndian.PutUint32(m.regions[0].data, tc.opcode)
		peek, err := x.Peek(p.Registers.PC)
		if err != nil || peek.Enc != tc.opcode {
			t.Fatalf("peek returned %v, %v", peek, err)
		}
		inst, _, err := x.StepDetailed()
		if err != nil || inst != peek || p.Registers.GPR[0] != uint64(tc.want) {
			t.Fatalf("step returned %v, %v, X0=%d", inst, err, p.Registers.GPR[0])
		}
	}

	p.Registers.PC = 0x1000
	m.regions[0].Flags = elf.PF_R
	if _, err := x.Peek(p.Registers.PC); err == nil {
		t.Fatal("peek succeeded after execute permission was removed")
	}
	if _, _, err := x.StepDetailed(); err == nil || x.Result().Termination != "memory_fault" || x.Result().Instructions != 3 {
		t.Fatalf("expected a fetch fault before another instruction retired: %+v", x.Result())
	}
}
