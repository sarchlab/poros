package emu

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math/bits"
	"os"
	"testing"
)

func TestByteSIMDScalarOraclesAndAliasing(t *testing.T) {
	for _, form := range []uint32{0x0e201c00, 0x0ea01c00, 0x2e201c00, 0x2e203400, 0x2e208c00, 0x2e31a800, 0x0e205800, 0x2e303800, 0x6f00e400} {
		for _, n := range []int{8, 16} {
			if form == 0x6f00e400 && n == 8 {
				continue
			}
			for _, dest := range []uint32{0, 1, 2, 31} {
				for trial := 0; trial < 256; trial++ {
					r := Registers{PC: 0x1000, N: true, C: true}
					var a, b, want [16]byte
					for i := 0; i < 16; i++ {
						a[i] = byte(trial*37 + i*17)
						b[i] = byte(trial*71 + i*13)
					}
					r.WriteQ(1, binary.LittleEndian.Uint64(a[:8]), binary.LittleEndian.Uint64(a[8:]))
					r.WriteQ(2, binary.LittleEndian.Uint64(b[:8]), binary.LittleEndian.Uint64(b[8:]))
					word := form | dest | 1<<5
					if n == 16 {
						word |= 1 << 30
					}
					if form == 0x0e201c00 || form == 0x0ea01c00 || form == 0x2e201c00 || form == 0x2e203400 || form == 0x2e208c00 {
						word |= 2 << 16
					}
					if form == 0x6f00e400 {
						word = form | dest
					}
					var sum uint16
					minimum := byte(255)
					for i := 0; i < n; i++ {
						switch form {
						case 0x0e201c00:
							want[i] = a[i] & b[i]
						case 0x0ea01c00:
							want[i] = a[i] | b[i]
						case 0x2e201c00:
							want[i] = a[i] ^ b[i]
						case 0x2e203400:
							if a[i] > b[i] {
								want[i] = 255
							}
						case 0x2e208c00:
							if a[i] == b[i] {
								want[i] = 255
							}
						case 0x0e205800:
							want[i] = byte(bits.OnesCount8(a[i]))
						case 0x2e31a800:
							minimum = min(minimum, a[i])
						case 0x2e303800:
							sum += uint16(a[i])
						}
					}
					if form == 0x2e31a800 {
						want[0] = minimum
					}
					if form == 0x2e303800 {
						binary.LittleEndian.PutUint16(want[:2], sum)
					}
					before := r
					if _, err := StepChecked(&r, authMemory{word}); err != nil {
						t.Fatalf("%x: %v", word, err)
					}
					if r.FP[dest].Lo != binary.LittleEndian.Uint64(want[:8]) || r.FP[dest].Hi != binary.LittleEndian.Uint64(want[8:]) {
						t.Fatalf("%x n=%d trial=%d got=%+v want=%x", word, n, trial, r.FP[dest], want)
					}
					before.FP[dest] = r.FP[dest]
					before.PC += 4
					if before != r {
						t.Fatal("unrelated state changed")
					}
				}
			}
		}
	}
}
func TestByteSIMDRejectsOtherLaneSizes(t *testing.T) {
	for _, word := range []uint32{0x6e623420, 0x6eb1a820, 0x6e703820, 0x0e605820, 0x6f01e400} {
		r := Registers{PC: 0x1000}
		before := r
		if _, err := StepChecked(&r, authMemory{word}); err == nil || r != before {
			t.Fatalf("accepted %x", word)
		}
	}
}

func TestByteSIMDRecordedNativeVectors(t *testing.T) {
	data, err := os.ReadFile("../../tests/macho/isa-native-m2.json")
	if err != nil {
		t.Fatal(err)
	}
	var evidence struct {
		Observations []struct {
			Source     string
			SourceHash string `json:"source_sha256"`
			Result     json.RawMessage
		}
	}
	if err = json.Unmarshal(data, &evidence); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, observation := range evidence.Observations {
		if observation.Source != "simd-probe.c" {
			continue
		}
		source, err := os.ReadFile("../../tests/macho/" + observation.Source)
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(source)
		if hex.EncodeToString(hash[:]) != observation.SourceHash {
			t.Fatal("native vector source changed")
		}
		var result struct {
			Vectors []struct {
				A, B, Or, Xor, Hi, Cnt, Eq, And string
				Min, Sum                        uint64
			}
		}
		if err = json.Unmarshal(observation.Result, &result); err != nil {
			t.Fatal(err)
		}
		for _, v := range result.Vectors {
			decode := func(s string) []byte {
				b, err := hex.DecodeString(s)
				if err != nil || len(b) != 16 {
					t.Fatalf("invalid vector %q", s)
				}
				return b
			}
			a, b := decode(v.A), decode(v.B)
			for _, tc := range []struct {
				word   uint32
				vector string
				scalar uint64
			}{{0x4ea21c20, v.Or, 0}, {0x6e221c20, v.Xor, 0}, {0x6e223420, v.Hi, 0}, {0x6e228c20, v.Eq, 0}, {0x4e221c20, v.And, 0}, {0x0e205820, v.Cnt, 0}, {0x6e31a820, "", v.Min}, {0x2e303820, "", v.Sum}} {
				r := Registers{PC: 0x1000}
				r.WriteQ(1, binary.LittleEndian.Uint64(a[:8]), binary.LittleEndian.Uint64(a[8:]))
				r.WriteQ(2, binary.LittleEndian.Uint64(b[:8]), binary.LittleEndian.Uint64(b[8:]))
				if _, err := StepChecked(&r, authMemory{tc.word}); err != nil {
					t.Fatal(err)
				}
				lo, hi := tc.scalar, uint64(0)
				if tc.vector != "" {
					want := decode(tc.vector)
					lo = binary.LittleEndian.Uint64(want[:8])
					hi = binary.LittleEndian.Uint64(want[8:])
				}
				if r.FP[0].Lo != lo || r.FP[0].Hi != hi {
					t.Fatalf("native mismatch word%x vector%d", tc.word, count)
				}
			}
			count++
		}
	}
	if count != 256 {
		t.Fatal("missing native vectors", count)
	}
}

func TestMOVIWordLanesNativeVectors(t *testing.T) {
	var native struct {
		Vectors []struct {
			Lanes, Shift, Immediate int
			Bytes                   string
		}
	}
	nativeAtomicObservation(t, "movi-probe.c", &native)
	if len(native.Vectors) != 32 {
		t.Fatal("native MOVI vectors missing")
	}
	for _, tc := range native.Vectors {
		word := uint32(0x0f000400) | uint32(tc.Immediate>>5)<<16 | uint32(tc.Immediate&31)<<5 | uint32(tc.Shift/8)<<13 | 17
		if tc.Lanes == 4 {
			word |= 1 << 30
		}
		r := Registers{PC: 0x1000, N: true, C: true}
		r.WriteQ(17, ^uint64(0), ^uint64(0))
		before := r
		if _, err := StepChecked(&r, authMemory{word}); err != nil {
			t.Fatal(tc, err)
		}
		want, err := hex.DecodeString(tc.Bytes)
		if err != nil || len(want) != 16 {
			t.Fatal("bad native bytes")
		}
		if r.FP[17].Lo != binary.LittleEndian.Uint64(want) || r.FP[17].Hi != binary.LittleEndian.Uint64(want[8:]) {
			t.Fatal(tc, r.FP[17])
		}
		before.FP[17] = r.FP[17]
		before.PC += 4
		if before != r {
			t.Fatal("MOVI changed unrelated state")
		}
	}
}

func TestXTNNativeVectors(t *testing.T) {
	var native struct {
		Bad   int
		Cases []struct {
			Bits          int
			Alias, OK     bool
			Input, Output string
		}
	}
	nativeAtomicObservation(t, "xtn-probe.c", &native)
	if native.Bad != 0 || len(native.Cases) != 384 {
		t.Fatal("missing native XTN cases")
	}
	for _, tc := range native.Cases {
		word := uint32(0x0e212800)
		if tc.Bits == 16 {
			word |= 1 << 22
		}
		if tc.Bits == 32 {
			word |= 2 << 22
		}
		src := 1
		if tc.Alias {
			src = 0
		}
		word |= uint32(src) << 5
		input, _ := hex.DecodeString(tc.Input)
		want, _ := hex.DecodeString(tc.Output)
		if len(input) != 16 || len(want) != 16 || !tc.OK {
			t.Fatal("bad native vector")
		}
		r := Registers{PC: 0x1000, C: true}
		r.WriteQ(0, ^uint64(0), ^uint64(0))
		r.WriteQ(src, binary.LittleEndian.Uint64(input), binary.LittleEndian.Uint64(input[8:]))
		before := r
		if _, err := StepChecked(&r, authMemory{word}); err != nil {
			t.Fatal(tc, err)
		}
		if r.FP[0].Lo != binary.LittleEndian.Uint64(want) || r.FP[0].Hi != binary.LittleEndian.Uint64(want[8:]) {
			t.Fatal(tc, r.FP[0])
		}
		before.FP[0] = r.FP[0]
		before.PC += 4
		if before != r {
			t.Fatal("XTN changed unrelated state")
		}
	}
}
