package emu

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math/bits"
	"os"
	"strconv"
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
	for _, word := range []uint32{0x6e623420, 0x6eb1a820, 0x6e703820, 0x0e605820, 0x6f01ec00} {
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

func TestVectorLaneInsertNativeVectors(t *testing.T) {
	var native struct {
		Inputs []struct{ Destination, Source string }
		Cases  []struct {
			Bits, Sample    int
			DestinationLane int `json:"destination_lane"`
			SourceLane      int `json:"source_lane"`
			Alias           bool
			Output          string
		}
	}
	nativeAtomicObservation(t, "ins-lane-probe.c", &native)
	if len(native.Cases) != 1360 || len(native.Inputs) != 2 {
		t.Fatal("missing native lane vectors")
	}
	for _, tc := range native.Cases {
		decode := func(s string) []byte {
			v, e := hex.DecodeString(s)
			if e != nil || len(v) != 16 {
				t.Fatal(s, e)
			}
			return v
		}
		input := native.Inputs[tc.Sample]
		d, s := decode(input.Destination), decode(input.Source)
		rn := uint32(1)
		if tc.Alias {
			rn = 0
			s = d
		}
		size := bits.TrailingZeros(uint(tc.Bits / 8))
		imm5 := uint32(1<<size) | uint32(tc.DestinationLane)<<(size+1)
		imm4 := uint32(tc.SourceLane) << size
		word := uint32(0x6e000400) | imm5<<16 | imm4<<11 | rn<<5
		r := Registers{PC: 0x1000, C: true}
		r.WriteQ(1, binary.LittleEndian.Uint64(s[:8]), binary.LittleEndian.Uint64(s[8:]))
		r.WriteQ(0, binary.LittleEndian.Uint64(d[:8]), binary.LittleEndian.Uint64(d[8:]))
		before := r
		want := decode(tc.Output)
		if _, err := StepChecked(&r, authMemory{word}); err != nil {
			t.Fatal(tc, err)
		}
		before.PC += 4
		before.WriteQ(0, binary.LittleEndian.Uint64(want[:8]), binary.LittleEndian.Uint64(want[8:]))
		if r != before {
			t.Fatal(tc, "lane preservation or alias failure", r.FP[0])
		}
	}
	for _, word := range []uint32{0x6e000420, 0x6e180c20} {
		if simdForm(word) != 0 {
			t.Fatal("reserved INS encoding accepted")
		}
	}
}

func TestUnsignedLaneMoveNativeVectors(t *testing.T) {
	var cases []struct {
		Encoding  string
		Lo        string `json:"vector_lo"`
		Hi        string `json:"vector_hi"`
		Initial   string `json:"initial_x11"`
		Result    string `json:"result_x11"`
		Other     string `json:"preserved_x10"`
		NZCV      string
		Unchanged bool `json:"source_unchanged"`
	}
	nativeAtomicObservation(t, "umov-probe.c", &cases)
	if len(cases) != 300 {
		t.Fatal(len(cases))
	}
	parse := func(s string) uint64 {
		v, e := strconv.ParseUint(s, 16, 64)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	for _, tc := range cases {
		if !tc.Unchanged {
			t.Fatal(tc)
		}
		flags := parse(tc.NZCV)
		r := Registers{PC: 0x1000, N: flags&(1<<31) != 0, Z: flags&(1<<30) != 0, C: flags&(1<<29) != 0, V: flags&(1<<28) != 0}
		r.GPR[10] = parse(tc.Other)
		r.GPR[11] = parse(tc.Initial)
		r.WriteQ(0, parse(tc.Lo), parse(tc.Hi))
		before := r
		if _, e := StepChecked(&r, authMemory{uint32(parse(tc.Encoding))}); e != nil {
			t.Fatal(tc, e)
		}
		before.GPR[11] = parse(tc.Result)
		before.PC += 4
		if r != before {
			t.Fatal("native lane transfer or preserved state mismatch", tc, r)
		}
	}
	for _, word := range []uint32{0x0e003c00, 0x0e083c00, 0x4e043c00, 0x4e103c00} {
		if simdForm(word) != 0 {
			t.Fatal("reserved lane transfer accepted", word)
		}
	}
}

func TestVectorZeroComparisonNativeVectors(t *testing.T) {
	var cases []struct {
		Encoding string
		Alias    bool
		Lo       string `json:"input_lo"`
		Hi       string `json:"input_hi"`
		OutputLo string `json:"output_lo"`
		OutputHi string `json:"output_hi"`
		NZCV     string
		Source   bool `json:"source_v1_unchanged"`
		Other    bool `json:"other_v2_unchanged"`
		GPR      bool `json:"gpr_x10_unchanged"`
	}
	nativeAtomicObservation(t, "cmeq-zero-probe.c", &cases)
	if len(cases) != 84 {
		t.Fatal(len(cases))
	}
	parse := func(s string) uint64 {
		v, e := strconv.ParseUint(s, 16, 64)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	for _, tc := range cases {
		if !tc.Source || !tc.Other || !tc.GPR {
			t.Fatal(tc)
		}
		f := parse(tc.NZCV)
		r := Registers{PC: 0x1000, N: f&(1<<31) != 0, Z: f&(1<<30) != 0, C: f&(1<<29) != 0, V: f&(1<<28) != 0}
		r.GPR[10] = 0x12345678
		r.WriteQ(0, ^uint64(0), ^uint64(0))
		r.WriteQ(1, parse(tc.Lo), parse(tc.Hi))
		r.WriteQ(2, 0x1234, 0xabcd)
		if tc.Alias {
			r.WriteQ(0, parse(tc.Lo), parse(tc.Hi))
		}
		before := r
		if _, e := StepChecked(&r, authMemory{uint32(parse(tc.Encoding))}); e != nil {
			t.Fatal(tc, e)
		}
		before.PC += 4
		before.WriteQ(0, parse(tc.OutputLo), parse(tc.OutputHi))
		if before != r {
			t.Fatal("native zero comparison or preserved state", tc, r.FP[:3])
		}
	}
	if simdForm(0x0ee09800) != 0 || simdForm(0x5ee09800) != 0 {
		t.Fatal("unsupported 1D/scalar D comparison admitted")
	}
}

func TestVectorORNAndBICNativeVectors(t *testing.T) {
	for _, source := range []string{"orn-probe.c", "bic-vector-probe.c"} {
		t.Run(source, func(t *testing.T) { testVectorLogicNative(t, source) })
	}
}
func testVectorLogicNative(t *testing.T, source string) {
	var cases []struct {
		Encoding  string
		InitialLo string `json:"initial_lo"`
		InitialHi string `json:"initial_hi"`
		RnLo      string `json:"rn_lo"`
		RnHi      string `json:"rn_hi"`
		RmLo      string `json:"rm_lo"`
		RmHi      string `json:"rm_hi"`
		OutputLo  string `json:"output_lo"`
		OutputHi  string `json:"output_hi"`
		NZCV      string
		Preserved bool `json:"v1_v2_unchanged"`
	}
	nativeAtomicObservation(t, source, &cases)
	if len(cases) != 200 {
		t.Fatal(len(cases))
	}
	parse := func(s string) uint64 {
		v, e := strconv.ParseUint(s, 16, 64)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	for _, tc := range cases {
		if !tc.Preserved {
			t.Fatal(tc)
		}
		f := parse(tc.NZCV)
		r := Registers{PC: 0x1000, N: f&(1<<31) != 0, Z: f&(1<<30) != 0, C: f&(1<<29) != 0, V: f&(1<<28) != 0}
		r.WriteQ(0, parse(tc.InitialLo), parse(tc.InitialHi))
		r.WriteQ(1, parse(tc.RnLo), parse(tc.RnHi))
		r.WriteQ(2, parse(tc.RmLo), parse(tc.RmHi))
		before := r
		if _, e := StepChecked(&r, authMemory{uint32(parse(tc.Encoding))}); e != nil {
			t.Fatal(tc, e)
		}
		before.PC += 4
		before.WriteQ(0, parse(tc.OutputLo), parse(tc.OutputHi))
		if before != r {
			t.Fatal("native vector logic or preserved state mismatch", tc, r.FP[:3])
		}
	}
}

func TestVectorDuplicateGPRNativeVectors(t *testing.T) {
	var cases []struct {
		Encoding, Input, Output string
		Destination             int
		SourceAfter             string `json:"source_after"`
		NZCV                    uint32
		OutputHex               string `json:"output_hex"`
	}
	nativeAtomicObservation(t, "dup-probe.c", &cases)
	if len(cases) != 252 {
		t.Fatal(len(cases))
	}
	for _, tc := range cases {
		word, e := strconv.ParseUint(tc.Encoding, 16, 32)
		if e != nil {
			t.Fatal(e)
		}
		input, e := strconv.ParseUint(tc.Input, 16, 64)
		if e != nil {
			t.Fatal(e)
		}
		r := Registers{PC: 0x1000, N: true, C: true}
		r.GPR[1] = input
		for i := range r.FP {
			r.WriteQ(i, ^uint64(0), ^uint64(0))
		}
		before := r
		if _, e := StepChecked(&r, authMemory{uint32(word)}); e != nil {
			t.Fatal(tc, e)
		}
		out, e := hex.DecodeString(tc.OutputHex)
		if e != nil || len(out) != 16 {
			t.Fatal(tc)
		}
		before.WriteQ(tc.Destination, binary.LittleEndian.Uint64(out[:8]), binary.LittleEndian.Uint64(out[8:]))
		before.PC += 4
		if r != before || tc.SourceAfter != tc.Input || tc.NZCV != 0xa0000000 {
			t.Fatal("DUP native mismatch", tc, r.FP[tc.Destination])
		}
	}
	for _, word := range []uint32{0x0e000c00, 0x0e030c00, 0x0e080c00, 0x4e100c00} {
		if simdForm(word) != 0 {
			t.Fatal("reserved DUP accepted", word)
		}
	}
}

func TestVectorBICImmediateNativeVectors(t *testing.T) {
	var cases []struct {
		Encoding string
		NZCV     uint32
		Input    string `json:"input_hex"`
		Output   string `json:"output_hex"`
	}
	nativeAtomicObservation(t, "bic-immediate-probe.c", &cases)
	if len(cases) != 240 {
		t.Fatal(len(cases))
	}
	for _, tc := range cases {
		word, e := strconv.ParseUint(tc.Encoding, 16, 32)
		if e != nil {
			t.Fatal(e)
		}
		in, e := hex.DecodeString(tc.Input)
		if e != nil || len(in) != 16 {
			t.Fatal(tc)
		}
		out, e := hex.DecodeString(tc.Output)
		if e != nil || len(out) != 16 {
			t.Fatal(tc)
		}
		r := Registers{PC: 0x1000, Z: true, C: true}
		r.WriteQ(0, binary.LittleEndian.Uint64(in[:8]), binary.LittleEndian.Uint64(in[8:]))
		before := r
		if _, e := StepChecked(&r, authMemory{uint32(word)}); e != nil {
			t.Fatal(tc, e)
		}
		before.WriteQ(0, binary.LittleEndian.Uint64(out[:8]), binary.LittleEndian.Uint64(out[8:]))
		before.PC += 4
		if r != before || tc.NZCV != 0x60000000 {
			t.Fatal(tc, r.FP[0])
		}
	}
}

func TestVectorShiftAndUnzipNativeVectors(t *testing.T) {
	for source, count := range map[string]int{"ushl-probe.c": 616, "uzp1-probe.c": 112} {
		var cases []struct {
			Encoding string
			NZCV     uint32
			A        string `json:"a_hex"`
			B        string `json:"b_hex"`
			Output   string `json:"output_hex"`
		}
		nativeAtomicObservation(t, source, &cases)
		if len(cases) != count {
			t.Fatal(source, len(cases))
		}
		for _, tc := range cases {
			word, e := strconv.ParseUint(tc.Encoding, 16, 32)
			if e != nil {
				t.Fatal(e)
			}
			a, e := hex.DecodeString(tc.A)
			if e != nil || len(a) != 16 {
				t.Fatal(tc)
			}
			b, e := hex.DecodeString(tc.B)
			if e != nil || len(b) != 16 {
				t.Fatal(tc)
			}
			out, e := hex.DecodeString(tc.Output)
			if e != nil || len(out) != 16 {
				t.Fatal(tc)
			}
			r := Registers{PC: 0x1000, N: true, V: true}
			r.WriteQ(0, binary.LittleEndian.Uint64(a[:8]), binary.LittleEndian.Uint64(a[8:]))
			r.WriteQ(1, binary.LittleEndian.Uint64(b[:8]), binary.LittleEndian.Uint64(b[8:]))
			before := r
			if _, e := StepChecked(&r, authMemory{uint32(word)}); e != nil {
				t.Fatal(source, tc, e)
			}
			before.WriteQ(int(word&31), binary.LittleEndian.Uint64(out[:8]), binary.LittleEndian.Uint64(out[8:]))
			before.PC += 4
			if r != before || tc.NZCV != 0x90000000 {
				t.Fatal(source, tc, r.FP[:3])
			}
		}
	}
}

func TestVectorMOVINativeImmediates(t *testing.T) {
	for source, count := range map[string]int{"movi-2d-probe.c": 256, "movi-byte-probe.c": 512} {
		t.Run(source, func(t *testing.T) { testVectorMOVINative(t, source, count) })
	}
}
func testVectorMOVINative(t *testing.T, source string, count int) {
	var cases []struct {
		Encoding string
		NZCV     uint32
		Output   string `json:"output_hex"`
	}
	nativeAtomicObservation(t, source, &cases)
	if len(cases) != count {
		t.Fatal(len(cases))
	}
	for _, tc := range cases {
		word, e := strconv.ParseUint(tc.Encoding, 16, 32)
		if e != nil {
			t.Fatal(e)
		}
		out, e := hex.DecodeString(tc.Output)
		if e != nil || len(out) != 16 {
			t.Fatal(tc)
		}
		r := Registers{PC: 0x1000, N: true, Z: source == "movi-2d-probe.c", C: true, V: true}
		r.WriteQ(0, 0x12345678, 0x87654321)
		before := r
		if _, e := StepChecked(&r, authMemory{uint32(word)}); e != nil {
			t.Fatal(tc, e)
		}
		before.WriteQ(0, binary.LittleEndian.Uint64(out[:8]), binary.LittleEndian.Uint64(out[8:]))
		before.PC += 4
		expectedFlags := uint32(0xb0000000)
		if source == "movi-2d-probe.c" {
			expectedFlags = 0xf0000000
		}
		if r != before || tc.NZCV != expectedFlags {
			t.Fatal(tc, r.FP[0])
		}
	}
}

func checkNativeVectorFullState(t *testing.T, source string, count int) {
	t.Helper()
	var cases []struct {
		Instruction, Encoding string
		Before, After         [3]string
		NZCVBefore            uint32 `json:"nzcv_before"`
		NZCVAfter             uint32 `json:"nzcv_after"`
	}
	nativeAtomicObservation(t, source, &cases)
	if len(cases) != count {
		t.Fatal(len(cases))
	}
	set := func(r *Registers, values [3]string) {
		for i, s := range values {
			b, e := hex.DecodeString(s)
			if e != nil || len(b) != 16 {
				t.Fatal(s, e)
			}
			r.WriteQ(i, binary.LittleEndian.Uint64(b[:8]), binary.LittleEndian.Uint64(b[8:]))
		}
	}
	for _, tc := range cases {
		word, e := strconv.ParseUint(tc.Encoding, 16, 32)
		if e != nil {
			t.Fatal(e)
		}
		r := Registers{PC: 0x1000, N: tc.NZCVBefore&(1<<31) != 0, Z: tc.NZCVBefore&(1<<30) != 0, C: tc.NZCVBefore&(1<<29) != 0, V: tc.NZCVBefore&(1<<28) != 0}
		set(&r, tc.Before)
		before := r
		if _, e := StepChecked(&r, authMemory{uint32(word)}); e != nil {
			t.Fatal(tc.Instruction, e)
		}
		set(&before, tc.After)
		before.PC += 4
		if r != before || tc.NZCVBefore != tc.NZCVAfter {
			t.Fatal("native vector mismatch", tc, r.FP[:3])
		}
	}
}

func TestVectorREV64NativeFullState(t *testing.T) {
	checkNativeVectorFullState(t, "rev64-probe.c", 2304)
	for _, word := range []uint32{0x0ee00800, 0x4ee00800} {
		r := Registers{PC: 0x1000}
		before := r
		if _, e := StepChecked(&r, authMemory{word}); e == nil || r != before {
			t.Fatal("unallocated D arrangement accepted")
		}
	}
}

func TestVectorUXTLNativeFullState(t *testing.T) {
	checkNativeVectorFullState(t, "uxtl-probe.c", 2304)
	for _, word := range []uint32{0x2f09a400, 0x6f11a400, 0x2f40a400} {
		r := Registers{PC: 0x1000}
		before := r
		if _, e := StepChecked(&r, authMemory{word}); e == nil || r != before {
			t.Fatal("shifted/unallocated widening accepted", word)
		}
	}
}

func TestVectorBSLAndSMAXVNativeFullState(t *testing.T) {
	checkNativeVectorFullState(t, "bsl-probe.c", 1280)
	checkNativeVectorFullState(t, "smaxv-probe.c", 1920)
	for _, word := range []uint32{0x0eb0a800, 0x0ef0a800, 0x4ef0a800} {
		r := Registers{PC: 0x1000}
		before := r
		if _, e := StepChecked(&r, authMemory{word}); e == nil || r != before {
			t.Fatal("unallocated SMAXV arrangement accepted", word)
		}
	}
}
