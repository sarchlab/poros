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
	for _, form := range []uint32{0x0ea01c00, 0x2e201c00, 0x2e203400, 0x2e31a800, 0x0e205800, 0x2e303800, 0x6f00e400} {
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
					if form == 0x0ea01c00 || form == 0x2e201c00 || form == 0x2e203400 {
						word |= 2 << 16
					}
					if form == 0x6f00e400 {
						word = form | dest
					}
					var sum uint16
					minimum := byte(255)
					for i := 0; i < n; i++ {
						switch form {
						case 0x0ea01c00:
							want[i] = a[i] | b[i]
						case 0x2e201c00:
							want[i] = a[i] ^ b[i]
						case 0x2e203400:
							if a[i] > b[i] {
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
				A, B, Or, Xor, Hi, Cnt string
				Min, Sum               uint64
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
			}{{0x4ea21c20, v.Or, 0}, {0x6e221c20, v.Xor, 0}, {0x6e223420, v.Hi, 0}, {0x0e205820, v.Cnt, 0}, {0x6e31a820, "", v.Min}, {0x2e303820, "", v.Sum}} {
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
