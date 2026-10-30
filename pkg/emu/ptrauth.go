package emu

import (
	"encoding/binary"
	"fmt"

	"golang.org/x/arch/arm64/arm64asm"
)

// PointerAuthMode is architectural process state, not an instruction workaround.
// Darwin disables address authentication for generic ARM64 main executables,
// even though the system dyld is arm64e. Enabled authentication is not yet modeled.
type PointerAuthMode uint8

const (
	PointerAuthUnspecified PointerAuthMode = iota
	PointerAuthDisabled
	PointerAuthEnabled
)

// DecodeChecked preserves the original encoding while presenting the effective
// operation in a process with address authentication disabled. Both functional
// execution and timing Peek must use this decoder. PAC/AUT have no register
// effects in that mode; authenticated branches still branch/link/return normally.
// Unspecified/enabled profiles fail closed, and PACGA/XPAC/LDRAA are not lowered.
func DecodeChecked(code []byte, mode PointerAuthMode) (arm64asm.Inst, error) {
	if len(code) < 4 {
		return arm64asm.Inst{}, fmt.Errorf("truncated instruction")
	}
	word := binary.LittleEndian.Uint32(code)
	op, args, recognized := disabledAuthOperation(word)
	if !recognized {
		return arm64asm.Decode(code)
	}
	if mode != PointerAuthDisabled {
		return arm64asm.Inst{}, fmt.Errorf("pointer authentication mode %d is not implemented for encoding %#08x", mode, word)
	}
	return arm64asm.Inst{Op: op, Args: args, Enc: word}, nil
}
func disabledAuthOperation(word uint32) (arm64asm.Op, arm64asm.Args, bool) {
	noArgs := arm64asm.Args{}
	switch word {
	case 0xd503211f, 0xd503215f, 0xd503219f, 0xd50321df, // PAC/AUT I{A,B}1716
		0xd503231f, 0xd503233f, 0xd503235f, 0xd503237f, // PAC I{A,B}{Z,SP}
		0xd503239f, 0xd50323bf, 0xd50323df, 0xd50323ff: // AUT I{A,B}{Z,SP}
		return arm64asm.NOP, noArgs, true
	case 0xd65f0bff, 0xd65f0fff: // RETAA/RETAB
		return arm64asm.RET, arm64asm.Args{arm64asm.X30}, true
	}
	// PAC/AUT {I,D}{A,B} Xd,Xn|SP. The zero-modifier forms have
	// their separate fixed Rn=31 encoding; reserved encodings must not match.
	if word&0xffffe000 == 0xdac10000 || word&0xffffe3e0 == 0xdac123e0 {
		return arm64asm.NOP, noArgs, true
	}
	base := word & 0xfffffc00
	switch base {
	case 0xd71f0800, 0xd71f0c00: // BRAA/BRAB Xn,Xm|SP
		return arm64asm.BR, arm64asm.Args{arm64asm.X0 + arm64asm.Reg((word>>5)&31)}, true
	case 0xd73f0800, 0xd73f0c00: // BLRAA/BLRAB Xn,Xm|SP
		return arm64asm.BLR, arm64asm.Args{arm64asm.X0 + arm64asm.Reg((word>>5)&31)}, true
	}
	base = word & 0xfffffc1f
	switch base {
	case 0xd61f081f, 0xd61f0c1f: // BRAAZ/BRABZ
		return arm64asm.BR, arm64asm.Args{arm64asm.X0 + arm64asm.Reg((word>>5)&31)}, true
	case 0xd63f081f, 0xd63f0c1f: // BLRAAZ/BLRABZ
		return arm64asm.BLR, arm64asm.Args{arm64asm.X0 + arm64asm.Reg((word>>5)&31)}, true
	}
	return 0, noArgs, false
}
