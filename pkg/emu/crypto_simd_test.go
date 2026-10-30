package emu

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestCryptoSIMDRecordedNativeCases(t *testing.T) {
	replaySIMDNative(t, "crypto-simd-probe.c", 9408)
}
func TestAESMCRecordedNativeCases(t *testing.T) { replaySIMDNative(t, "aesmc-probe.c", 640) }
func replaySIMDNative(t *testing.T, source string, count int) {
	t.Helper()
	var native struct {
		Patterns []string
		Cases    []struct {
			Kind       string
			Encoding   uint32
			Pattern    int
			InputNZCV  uint64 `json:"input_nzcv"`
			OutputNZCV uint64 `json:"output_nzcv"`
			Untouched  bool   `json:"untouched_match"`
			Output     string `json:"output_hex"`
		}
	}
	nativeAtomicObservation(t, source, &native)
	if len(native.Cases) != count {
		t.Fatal(len(native.Cases))
	}
	patterns := make([][]byte, len(native.Patterns))
	for i, s := range native.Patterns {
		var e error
		patterns[i], e = hex.DecodeString(s)
		if e != nil || len(patterns[i]) != 512 {
			t.Fatal(e)
		}
	}
	counts := map[string]int{}
	for _, tc := range native.Cases {
		counts[tc.Kind]++
		r := Registers{PC: 0x1000}
		setNZCV(&r, tc.InputNZCV)
		p := patterns[tc.Pattern]
		for i := range r.FP {
			r.WriteQ(i, binary.LittleEndian.Uint64(p[16*i:]), binary.LittleEndian.Uint64(p[16*i+8:]))
		}
		r.GPR[10] = 0x123456789abcdef0
		want := r
		want.PC += 4
		b, e := hex.DecodeString(tc.Output)
		if e != nil || len(b) != 16 {
			t.Fatal(e)
		}
		want.WriteQ(int(tc.Encoding&31), binary.LittleEndian.Uint64(b), binary.LittleEndian.Uint64(b[8:]))
		if _, e := StepChecked(&r, authMemory{tc.Encoding}); e != nil || r != want || !tc.Untouched || tc.InputNZCV != tc.OutputNZCV {
			t.Fatal("native crypto SIMD mismatch", tc, e)
		}
	}
	if count == 9408 && (counts["AESE"] != 1152 || counts["EXT"] != 3072 || counts["TBL"] != 2304) {
		t.Fatal(counts)
	}
}
func TestINSGPRRecordedNativeCases(t *testing.T) {
	var native struct {
		Initial string `json:"initial_hex"`
		Cases   []struct {
			Encoding   uint32
			Value      uint64
			InputNZCV  uint64 `json:"input_nzcv"`
			OutputNZCV uint64 `json:"output_nzcv"`
			Untouched  bool   `json:"untouched_match"`
			Output     string `json:"output_hex"`
		}
	}
	nativeAtomicObservation(t, "ins-gpr-probe.c", &native)
	if len(native.Cases) != 2880 {
		t.Fatal(len(native.Cases))
	}
	initial, e := hex.DecodeString(native.Initial)
	if e != nil || len(initial) != 512 {
		t.Fatal(e)
	}
	for _, tc := range native.Cases {
		r := Registers{PC: 0x1000}
		setNZCV(&r, tc.InputNZCV)
		r.GPR[3] = tc.Value
		for i := range r.FP {
			r.WriteQ(i, binary.LittleEndian.Uint64(initial[16*i:]), binary.LittleEndian.Uint64(initial[16*i+8:]))
		}
		want := r
		want.PC += 4
		b, e := hex.DecodeString(tc.Output)
		if e != nil || len(b) != 16 {
			t.Fatal(e)
		}
		want.WriteQ(int(tc.Encoding&31), binary.LittleEndian.Uint64(b), binary.LittleEndian.Uint64(b[8:]))
		if _, e := StepChecked(&r, authMemory{tc.Encoding}); e != nil || r != want || !tc.Untouched || tc.InputNZCV != tc.OutputNZCV {
			t.Fatal(tc, e)
		}
	}
}
func TestCryptoSIMDRejectsUnimplementedNeighbors(t *testing.T) {
	for _, word := range []uint32{0x4e285820, 0x4e287820, 0x4e031041, 0x2e0440c1, 0x5e000420, 0x4e101c67, 0x0ee08420, 0x5ee08420} {
		// AESD/AESIMC, TBX, invalid 8B EXT offset, invalid scalar DUP/INS size.
		r := Registers{PC: 0x1000, N: true}
		r.WriteQ(0, 1, 2)
		before := r
		if _, e := StepChecked(&r, authMemory{word}); e == nil || r != before {
			t.Fatal("unimplemented SIMD accepted", word, e)
		}
	}
}

func TestVectorADDRecordedNativeCases(t *testing.T) { replaySIMDNative(t, "vector-add-probe.c", 1260) }
