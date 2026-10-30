package emu

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"testing"
)

// Exercise the four instructions together over padded message blocks. Go's
// standard SHA256 supplies an independent complete-digest oracle, including the
// padding boundary and chaining between blocks.
func TestSHA256InstructionSequenceDigests(t *testing.T) {
	constants := [...]uint32{
		0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
		0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
		0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
		0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
		0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
		0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
		0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
		0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
	}
	for _, message := range [][]byte{nil, []byte("abc"), bytes.Repeat([]byte{0xa5}, 55), bytes.Repeat([]byte{0x37}, 56), bytes.Repeat([]byte("abcdefgh"), 20)} {
		padded := append(append([]byte(nil), message...), 0x80)
		for len(padded)%64 != 56 {
			padded = append(padded, 0)
		}
		padded = binary.BigEndian.AppendUint64(padded, uint64(len(message))*8)
		state := [8]uint32{0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a, 0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19}
		r := Registers{PC: 0x1000, N: true, C: true}
		put := func(index int, values []uint32) {
			r.WriteQ(index, uint64(values[0])|uint64(values[1])<<32, uint64(values[2])|uint64(values[3])<<32)
		}
		step := func(word uint32) {
			t.Helper()
			if _, err := StepChecked(&r, authMemory{word}); err != nil {
				t.Fatalf("%08x: %v", word, err)
			}
		}
		for start := 0; start < len(padded); start += 64 {
			put(16, state[:4])
			put(17, state[4:])
			for i := 0; i < 4; i++ {
				var words [4]uint32
				for j := range words {
					words[j] = binary.BigEndian.Uint32(padded[start+16*i+4*j:])
				}
				put(i, words[:])
			}
			for group := uint32(0); group < 16; group++ {
				current := group % 4
				put(4, constants[4*group:4*group+4])
				step(0x4ea48405 | current<<5) // ADD V5.4S,Vcurrent.4S,V4.4S
				step(0x4eb01e12)              // preserve ABCD in V18
				step(0x5e054230)              // SHA256H Q16,Q17,V5.4S
				step(0x5e055251)              // SHA256H2 Q17,Q18,V5.4S
				if group < 12 {
					step(0x5e282800 | ((current+1)%4)<<5 | current)
					step(0x5e006000 | ((current+3)%4)<<16 | ((current+2)%4)<<5 | current)
				}
			}
			for i := 0; i < 2; i++ {
				lo, hi := r.ReadQ(16 + i)
				state[4*i] += uint32(lo)
				state[4*i+1] += uint32(lo >> 32)
				state[4*i+2] += uint32(hi)
				state[4*i+3] += uint32(hi >> 32)
			}
		}
		var got [32]byte
		for i, word := range state {
			binary.BigEndian.PutUint32(got[4*i:], word)
		}
		if want := sha256.Sum256(message); got != want || !r.N || !r.C || r.Z || r.V {
			t.Fatalf("message length %d: got %x, want %x; flags %v", len(message), got, want, r)
		}
	}
}
