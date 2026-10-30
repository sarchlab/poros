package emu

import "golang.org/x/arch/arm64/arm64asm"

// OpLDAPR preserves the RCpc acquire-load operation absent from x/arch v0.24.
// The single-owner memory stream is ordered; timing models currently serialize
// this operation conservatively rather than claiming calibrated RCpc throughput.
const OpLDAPR arm64asm.Op = 0xff05

func decodeRCpc(word uint32) (arm64asm.Inst, bool) {
	// FEAT_LRCPC LDAPR Wt/Xt,[Xn|SP]. Byte/halfword and unscaled
	// immediate forms are separate encodings and remain unsupported.
	if word&0xbffffc00 != 0xb8bfc000 {
		return arm64asm.Inst{}, false
	}
	base := arm64asm.W0
	if word>>30 == 3 {
		base = arm64asm.X0
	}
	return arm64asm.Inst{Op: OpLDAPR, Enc: word, Args: arm64asm.Args{base + arm64asm.Reg(word&31), arm64asm.MemImmediate{Base: arm64asm.RegSP(arm64asm.X0 + arm64asm.Reg((word>>5)&31)), Mode: arm64asm.AddrOffset}}}, true
}
