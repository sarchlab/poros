package emu

import (
	"encoding/binary"
	"math/bits"
)

// simdForm selects only implemented encodings. Byte operations keep size
// bits in their mask; MOVI explicitly selects its implemented lane form.
func simdForm(word uint32) uint32 {
	if form := word & 0xbf20fc00; (form == 0x2e204400 || form == 0x0e001800) && ((word>>22)&3 != 3 || word&(1<<30) != 0) {
		return form
	} // USHL and UZP1 vectors.
	if word&0xbff80c00 == 0x2f000400 {
		cmode := (word >> 12) & 15
		if cmode < 12 && cmode&1 != 0 {
			return 0x2f000400
		}
	} // BIC vector, shifted immediate.
	if word&0xbfe0fc00 == 0x0e000c00 {
		imm5 := (word >> 16) & 31
		if (imm5 == 1 || imm5 == 2 || imm5 == 4) || (imm5 == 8 && word&(1<<30) != 0) {
			return 0x0e000c00
		}
	} // DUP vector from general-purpose register.

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
	case 0x0e201c00, 0x0e601c00, 0x0ea01c00, 0x0ee01c00, 0x2e201c00, 0x2e203400, 0x2e208c00:
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
	if word&0xbff8fc00 == 0x0f00e400 {
		return 0x0f00e400
	} // MOVI Vd.8B/16B,#imm8
	if word&0xfff8fc00 == 0x6f00e400 {
		return 0x6f00e400
	} // MOVI Vd.2D, byte-mask immediate
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
	if form == 0x0e000c00 {
		width := int((word >> 16) & 31)
		var value uint64
		if rn := (word >> 5) & 31; rn != 31 {
			value = e.regs.GPR[rn]
		}
		for off := 0; off < n; off += width {
			for i := 0; i < width; i++ {
				out[off+i] = byte(value >> uint(8*i))
			}
		}
		e.regs.WriteQ(int(word&31), binary.LittleEndian.Uint64(out[:8]), binary.LittleEndian.Uint64(out[8:]))
		return
	}
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
	if form == 0x0f00e400 {
		value := byte((((word >> 16) & 7) << 5) | ((word >> 5) & 31))
		for i := 0; i < n; i++ {
			out[i] = value
		}
	}
	if form == 0x6f00e400 {
		imm := (((word >> 16) & 7) << 5) | ((word >> 5) & 31)
		for i := 0; i < 16; i++ {
			if imm&(1<<uint(i%8)) != 0 {
				out[i] = 255
			}
		}
	}
	var sum uint16
	minimum := byte(255)
	for i := 0; i < n; i++ {
		switch form {
		case 0x0e201c00:
			out[i] = a[i] & b[i]
		case 0x0e601c00:
			out[i] = a[i] &^ b[i]
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
	if form == 0x2e204400 {
		width := 1 << ((word >> 22) & 3)
		for off := 0; off < n; off += width {
			var value uint64
			for i := 0; i < width; i++ {
				value |= uint64(a[off+i]) << uint(i*8)
			}
			shift := int(int8(b[off]))
			if shift >= width*8 || shift <= -width*8 {
				value = 0
			} else if shift >= 0 {
				value <<= uint(shift)
			} else {
				value >>= uint(-shift)
			}
			for i := 0; i < width; i++ {
				out[off+i] = byte(value >> uint(i*8))
			}
		}
	}
	if form == 0x0e001800 {
		width := 1 << ((word >> 22) & 3)
		half := n / 2
		for off := 0; off < half; off += width {
			copy(out[off:off+width], a[off*2:off*2+width])
			copy(out[half+off:half+off+width], b[off*2:off*2+width])
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
	if form == 0x2f000400 {
		cmode := (word >> 12) & 15
		width, shift := 4, int(cmode>>1)*8
		if cmode >= 8 {
			width = 2
			shift = int((cmode>>1)&1) * 8
		}
		imm := byte((((word >> 16) & 7) << 5) | ((word >> 5) & 31))
		dst := e.regs.FP[word&31]
		binary.LittleEndian.PutUint64(out[:8], dst.Lo)
		binary.LittleEndian.PutUint64(out[8:], dst.Hi)
		for off := 0; off < n; off += width {
			out[off+shift/8] &= ^imm
		}
		clear(out[n:])
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
