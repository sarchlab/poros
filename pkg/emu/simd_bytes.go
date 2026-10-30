package emu

import (
	"encoding/binary"
	"math/bits"
)

// simdForm selects only implemented encodings. Byte operations keep size
// bits in their mask; MOVI explicitly selects its implemented lane form.
func simdForm(word uint32) uint32 {
	if word&0xbf3ffc00 == 0x0e209800 && ((word>>22)&3 != 3 || word&(1<<30) != 0) {
		return 0x0e209800
	} // CMEQ vector,#0

	if word&0xbfe0fc00 == 0x0e003c00 {
		size := bits.TrailingZeros32((word >> 16) & 31)
		if (word&(1<<30) == 0 && size <= 2) || (word&(1<<30) != 0 && size == 3) {
			return 0x0e003c00
		}
	} // UMOV, including the MOV W/X, Vn.S/D[index] aliases.

	if word&0xffe08400 == 0x6e000400 {
		size := bits.TrailingZeros32((word >> 16) & 31)
		if size <= 3 && ((word>>11)&15)&((1<<size)-1) == 0 {
			return 0x6e000400
		}
	} // INS (element), disassembled as MOV Vd.<T>[i],Vn.<T>[j]

	switch form := word & 0xbfe0fc00; form {
	case 0x0e201c00, 0x0ea01c00, 0x0ee01c00, 0x2e201c00, 0x2e203400, 0x2e208c00:
		return form // ORR, EOR, CMHI
	}
	switch form := word & 0xbffffc00; form {
	case 0x2e31a800, 0x0e205800, 0x2e303800:
		return form // UMINV, CNT, UADDLV
	}
	if word&0xff3ffc00 == 0x0e212800 && (word>>22)&3 != 3 {
		return 0x0e212800
	} // XTN, lower half
	if word&0xbff89c00 == 0x0f000400 {
		return 0x0f000400
	} // MOVI Vd.2S/4S,#imm8,LSL #0/8/16/24
	if word&^31 == 0x6f00e400 {
		return 0x6f00e400
	} // MOVI Vd.2D,#0
	return 0
}

func (e *executor) execSIMD(word uint32) {
	form := simdForm(word)
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
	if form == 0x0e003c00 {
		imm5 := (word >> 16) & 31
		size := bits.TrailingZeros32(imm5)
		width := 1 << size
		offset := int(imm5>>uint(size+1)) * width
		var value uint64
		for i := 0; i < width; i++ {
			value |= uint64(a[offset+i]) << uint(8*i)
		}
		if rd := word & 31; rd != 31 {
			e.regs.GPR[rd] = value
		}
		return
	}
	var sum uint16
	minimum := byte(255)
	for i := 0; i < n; i++ {
		switch form {
		case 0x0e201c00:
			out[i] = a[i] & b[i]
		case 0x0ea01c00:
			out[i] = a[i] | b[i]
		case 0x0ee01c00:
			out[i] = a[i] | ^b[i]
		case 0x2e201c00:
			out[i] = a[i] ^ b[i]
		case 0x2e203400:
			if a[i] > b[i] {
				out[i] = 255
			}
		case 0x2e208c00:
			if a[i] == b[i] {
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
	if form == 0x0e209800 {
		width := 1 << ((word >> 22) & 3)
		for off := 0; off < n; off += width {
			zero := true
			for i := 0; i < width; i++ {
				zero = zero && a[off+i] == 0
			}
			if zero {
				for i := 0; i < width; i++ {
					out[off+i] = 255
				}
			}
		}
	}
	if form == 0x6e000400 {
		dst := e.regs.FP[word&31]
		binary.LittleEndian.PutUint64(out[:8], dst.Lo)
		binary.LittleEndian.PutUint64(out[8:], dst.Hi)
		imm5, imm4 := (word>>16)&31, (word>>11)&15
		size := bits.TrailingZeros32(imm5)
		width := 1 << size
		di := int(imm5>>uint(size+1)) * width
		si := int(imm4>>uint(size)) * width
		copy(out[di:di+width], a[si:si+width])
	}
	if form == 0x0e212800 {
		width := 1 << ((word >> 22) & 3)
		for lane := 0; lane < 8/width; lane++ {
			copy(out[lane*width:(lane+1)*width], a[lane*width*2:lane*width*2+width])
		}
	}
	if form == 0x0f000400 {
		value := (((word >> 16) & 7) << 5) | ((word >> 5) & 31)
		value <<= ((word >> 13) & 3) * 8
		for i := 0; i < n; i += 4 {
			binary.LittleEndian.PutUint32(out[i:], value)
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
