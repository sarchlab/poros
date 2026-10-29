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
