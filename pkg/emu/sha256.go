package emu

import "math/bits"

func vectorWords(r *Registers, index uint32) [4]uint32 {
	low, high := r.ReadQ(int(index))
	return [4]uint32{uint32(low), uint32(low >> 32), uint32(high), uint32(high >> 32)}
}
func shaSmall0(x uint32) uint32 {
	return bits.RotateLeft32(x, -7) ^ bits.RotateLeft32(x, -18) ^ (x >> 3)
}
func shaSmall1(x uint32) uint32 {
	return bits.RotateLeft32(x, -17) ^ bits.RotateLeft32(x, -19) ^ (x >> 10)
}
func shaBig0(x uint32) uint32 {
	return bits.RotateLeft32(x, -2) ^ bits.RotateLeft32(x, -13) ^ bits.RotateLeft32(x, -22)
}
func shaBig1(x uint32) uint32 {
	return bits.RotateLeft32(x, -6) ^ bits.RotateLeft32(x, -11) ^ bits.RotateLeft32(x, -25)
}

// SHA256 instructions operate on four 32-bit lanes. Snapshot all operands before
// committing Rd so aliases never change subsequent round/schedule inputs.
func (e *executor) execSHA256(word, form uint32) {
	rd := (word & 31)
	d, n, m := vectorWords(e.regs, rd), vectorWords(e.regs, (word>>5)&31), vectorWords(e.regs, (word>>16)&31)
	var out [4]uint32
	switch form {
	case 0x5e282800: // SHA256SU0
		next := [4]uint32{d[1], d[2], d[3], n[0]}
		for i := range out {
			out[i] = d[i] + shaSmall0(next[i])
		}
	case 0x5e006000: // SHA256SU1
		next := [4]uint32{n[1], n[2], n[3], m[0]}
		for i := range out {
			var schedule uint32
			if i < 2 {
				schedule = m[i+2]
			} else {
				schedule = out[i-2]
			}
			out[i] = d[i] + next[i] + shaSmall1(schedule)
		}
	case 0x5e004000: // SHA256H: update ABCD using original EFGH.
		a, b, c, dd := d[0], d[1], d[2], d[3]
		ee, f, g, h := n[0], n[1], n[2], n[3]
		for _, wk := range m {
			t1 := h + shaBig1(ee) + ((ee & f) ^ (^ee & g)) + wk
			t2 := shaBig0(a) + ((a & b) ^ (a & c) ^ (b & c))
			a, b, c, dd, ee, f, g, h = t1+t2, a, b, c, dd+t1, ee, f, g
		}
		out = [4]uint32{a, b, c, dd}
	case 0x5e005000: // SHA256H2: the four consumed D values are old D,C,B,A.
		ee, f, g, h := d[0], d[1], d[2], d[3]
		for i, wk := range m {
			next := h + shaBig1(ee) + ((ee & f) ^ (^ee & g)) + wk + n[3-i]
			ee, f, g, h = next, ee, f, g
		}
		out = [4]uint32{ee, f, g, h}
	}
	e.regs.WriteQ(int(rd), uint64(out[0])|uint64(out[1])<<32, uint64(out[2])|uint64(out[3])<<32)
}
