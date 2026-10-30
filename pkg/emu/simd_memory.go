package emu

import (
	"encoding/binary"
	"fmt"
	"golang.org/x/arch/arm64/arm64asm"
)

// multipleSIMDShape admits contiguous LD1/ST1 register lists, not lane/replicate
// or interleaved LD2/ST2 and larger structure forms.
func multipleSIMDShape(word uint32) (count, width int, ok bool) {
	post := word&(1<<23) != 0
	if post {
		if word&0xbfa00000 != 0x0c800000 {
			return
		}
	} else if word&0xbfbf0000 != 0x0c000000 {
		return
	}
	switch (word >> 12) & 15 {
	case 7:
		count = 1
	case 10:
		count = 2
	case 6:
		count = 3
	case 2:
		count = 4
	default:
		return
	}
	width = 8
	if word&(1<<30) != 0 {
		width = 16
	}
	return count, width, true
}

func (e *executor) execSIMDMemory(inst arm64asm.Inst) error {
	count, width, ok := multipleSIMDShape(inst.Enc)
	if !ok {
		return &UnsupportedInstructionError{inst}
	}
	rn := int((inst.Enc >> 5) & 31)
	addr := e.regs.SP
	if rn != 31 {
		addr = e.regs.GPR[rn]
	}
	var data [64]byte
	span := count * width
	first := int(inst.Enc & 31)
	if inst.Op == arm64asm.LD1 {
		if e.checked != nil {
			if err := e.checked.Read(addr, data[:span]); err != nil {
				// Native M2 may partially commit vector registers. CheckedStep
				// cannot resume that fault state; do not claim an atomic fault.
				return fmt.Errorf("partial SIMD load fault state is not modeled (%v): %w", err, &UnsupportedInstructionError{inst})
			}
		} else {
			for i := 0; i < span; i += 8 {
				binary.LittleEndian.PutUint64(data[i:], e.mem.ReadU64(addr+uint64(i)))
			}
		}
		for i := 0; i < count; i++ {
			low := binary.LittleEndian.Uint64(data[i*width:])
			var high uint64
			if width == 16 {
				high = binary.LittleEndian.Uint64(data[i*width+8:])
			}
			e.regs.WriteQ((first+i)&31, low, high)
		}
	} else {
		for i := 0; i < count; i++ {
			low, high := e.regs.ReadQ((first + i) & 31)
			binary.LittleEndian.PutUint64(data[i*width:], low)
			if width == 16 {
				binary.LittleEndian.PutUint64(data[i*width+8:], high)
			}
		}
		if err := e.writeMemory(addr, 8, data[:span]); err != nil {
			// Atomic CheckedMemory failure preserves memory/monitor state;
			// partial native SIMD-store exceptions are outside this profile.
			return fmt.Errorf("partial SIMD store fault state is not modeled (%v): %w", err, &UnsupportedInstructionError{inst})
		}
	}

	if inst.Enc&(1<<23) != 0 {
		rm := int((inst.Enc >> 16) & 31)
		increment := uint64(span)
		if rm != 31 {
			increment = e.regs.GPR[rm]
		}
		if rn == 31 {
			e.regs.SP = addr + increment
		} else {
			e.regs.GPR[rn] = addr + increment
		}
	}
	return nil
}
