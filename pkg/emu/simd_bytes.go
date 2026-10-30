package emu

import (
	"encoding/binary"
	"math/bits"
)

// byteSIMDForm selects only implemented encodings. Q chooses 8B/16B; size
// bits remain in the mask so halfword/word variants cannot silently execute.
func byteSIMDForm(word uint32) uint32 {
	switch form := word & 0xbfe0fc00; form {
	case 0x0ea01c00, 0x2e201c00, 0x2e203400:
		return form // ORR, EOR, CMHI
	}
	switch form := word & 0xbffffc00; form {
	case 0x2e31a800, 0x0e205800, 0x2e303800:
		return form // UMINV, CNT, UADDLV
	}
	if word&^31 == 0x6f00e400 {
		return 0x6f00e400
	} // MOVI Vd.2D,#0
	return 0
}

func (e *executor) execByteSIMD(word uint32) {
	form := byteSIMDForm(word)
	n := 8
	if word&(1<<30) != 0 {
		n = 16
	}
	var a, b, out [16]byte
	src := e.regs.FP[(word>>5)&31]
	rhs := e.regs.FP[(word>>16)&31]
	binary.LittleEndian.PutUint64(a[:8], src.Lo)
	binary.LittleEndian.PutUint64(a[8:], src.Hi)
	binary.LittleEndian.PutUint64(b[:8], rhs.Lo)
	binary.LittleEndian.PutUint64(b[8:], rhs.Hi)
	var sum uint16
	minimum := byte(255)
	for i := 0; i < n; i++ {
		switch form {
		case 0x0ea01c00:
			out[i] = a[i] | b[i]
		case 0x2e201c00:
			out[i] = a[i] ^ b[i]
		case 0x2e203400:
			if a[i] > b[i] {
				out[i] = 255
			}
		case 0x0e205800:
			out[i] = byte(bits.OnesCount8(a[i]))
		case 0x2e31a800:
			minimum = min(minimum, a[i])
		case 0x2e303800:
			sum += uint16(a[i])
		}
	}
	if form == 0x2e31a800 {
		out[0] = minimum
	}
	if form == 0x2e303800 {
		binary.LittleEndian.PutUint16(out[:2], sum)
	}
	e.regs.WriteQ(int(word&31), binary.LittleEndian.Uint64(out[:8]), binary.LittleEndian.Uint64(out[8:]))
}
