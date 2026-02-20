package loader

import (
	"encoding/binary"
	"fmt"

	"golang.org/x/arch/arm64/arm64asm"
)

// Instruction represents a single decoded ARM64 instruction.
type Instruction struct {
	Addr    uint64
	Raw     uint32
	Inst    arm64asm.Inst
	IsValid bool
}

// DisassembleN decodes up to n ARM64 instructions from the given bytes
// starting at the specified base address.
func DisassembleN(data []byte, baseAddr uint64, n int) []Instruction {
	var result []Instruction

	for off := 0; off < len(data) && len(result) < n; off += 4 {
		if off+4 > len(data) {
			break
		}

		raw := binary.LittleEndian.Uint32(data[off : off+4])
		inst, err := arm64asm.Decode(data[off : off+4])

		entry := Instruction{
			Addr: baseAddr + uint64(off),
			Raw:  raw,
		}
		if err == nil {
			entry.Inst = inst
			entry.IsValid = true
		}

		result = append(result, entry)
	}

	return result
}

// FormatInstruction returns a formatted string for an instruction.
func FormatInstruction(inst Instruction) string {
	hex := fmt.Sprintf("%08x", inst.Raw)
	if inst.IsValid {
		return fmt.Sprintf("0x%x:  %s  %s", inst.Addr, hex, inst.Inst)
	}
	return fmt.Sprintf("0x%x:  %s  <unknown>", inst.Addr, hex)
}
