package emu

import (
	"unsafe"

	"golang.org/x/arch/arm64/arm64asm"
)

// Accessor helpers for arm64asm types with unexported fields.
// These use unsafe.Pointer to match the struct layout exactly.

// immShiftLayout matches arm64asm.ImmShift: { imm uint16; shift uint8 }
type immShiftLayout struct {
	Imm   uint16
	Shift uint8
}

func getImmShift(is arm64asm.ImmShift) (uint16, uint8) {
	s := *(*immShiftLayout)(unsafe.Pointer(&is))
	return s.Imm, s.Shift
}

// regExtshiftAmountLayout matches arm64asm.RegExtshiftAmount:
// { reg Reg(uint16); extShift ExtShift(uint8); amount uint8; show_zero bool }
type regExtshiftAmountLayout struct {
	Reg      arm64asm.Reg
	ExtShift uint8 // arm64asm.ExtShift is uint8
	Amount   uint8
	ShowZero bool
}

func getRegExtshiftAmount(rea arm64asm.RegExtshiftAmount) (arm64asm.Reg, uint8, uint8) {
	s := *(*regExtshiftAmountLayout)(unsafe.Pointer(&rea))
	return s.Reg, s.ExtShift, s.Amount
}

// memImmediateLayout matches arm64asm.MemImmediate:
// { Base RegSP(uint16); Mode AddrMode(uint8); imm int32 }
// Note: there may be padding between Mode and imm.
type memImmediateLayout struct {
	Base uint16 // RegSP is underlying Reg which is uint16
	Mode uint8  // AddrMode is uint8
	_    uint8  // padding
	Imm  int32
}

func getMemImmediate(mi arm64asm.MemImmediate) (arm64asm.RegSP, arm64asm.AddrMode, int32) {
	s := *(*memImmediateLayout)(unsafe.Pointer(&mi))
	return arm64asm.RegSP(s.Base), arm64asm.AddrMode(s.Mode), s.Imm
}

// regWithArrangementLayout matches arm64asm.RegisterWithArrangement:
// { r Reg(uint16); a Arrangement(uint8); cnt uint8 }
type regWithArrangementLayout struct {
	Reg         arm64asm.Reg
	Arrangement uint8
	Cnt         uint8
}

func getRegWithArrangement(ra arm64asm.RegisterWithArrangement) (arm64asm.Reg, uint8) {
	s := *(*regWithArrangementLayout)(unsafe.Pointer(&ra))
	return s.Reg, s.Arrangement
}

// immFpLayout matches arm64asm.Imm_fp: { s uint8; exp int8; pre uint8 }
type immFpLayout struct {
	S   uint8
	Exp int8
	Pre uint8
}

func getImmFp(fp arm64asm.Imm_fp) float64 {
	s := *(*immFpLayout)(unsafe.Pointer(&fp))
	var sign int16
	if s.S == 0 {
		sign = 1
	} else {
		sign = -1
	}
	pre := sign * int16(16+s.Pre)
	var numerator, denominator int16
	if s.Exp > 0 {
		numerator = pre << uint8(s.Exp)
		denominator = 16
	} else {
		numerator = pre
		denominator = 16 << uint8(-1*s.Exp)
	}
	return float64(numerator) / float64(denominator)
}
