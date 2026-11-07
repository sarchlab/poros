package emu

import (
	"encoding/binary"

	"golang.org/x/arch/arm64/arm64asm"
)

// DecodeCache reuses decoded AArch64 instructions within one execution.
// Like the registers it accompanies, it has a single owner. Its zero value
// is ready to use.
type DecodeCache struct {
	entries [256]arm64asm.Inst
}

// Decode takes the bytes returned by the current fetch. The full encoding
// distinguishes collisions and permits code at a guest address to change.
func (c *DecodeCache) Decode(b [4]byte) (arm64asm.Inst, error) {
	word := binary.LittleEndian.Uint32(b[:])
	// Multiplicative hashing mixes the opcode and operand bits into the index.
	slot := &c.entries[(word*0x9e3779b9)>>24]
	if slot.Op != 0 && slot.Enc == word {
		return *slot, nil
	}
	inst, err := arm64asm.Decode(b[:])
	if err == nil {
		*slot = inst
	}
	return inst, err
}
