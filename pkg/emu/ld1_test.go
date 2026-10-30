package emu

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/sarchlab/poros/pkg/usermem"
)

func ld1Memory(t *testing.T, word uint32, start uint64, data []byte) *TrackedMemory {
	t.Helper()
	m := &usermem.AddressSpace{}
	var code [4]byte
	binary.LittleEndian.PutUint32(code[:], word)
	if e := m.Map(0x1000, 4, usermem.Execute, code[:]); e != nil {
		t.Fatal(e)
	}
	if len(data) > 0 {
		if e := m.Map(start, uint64(len(data)), usermem.Read, data); e != nil {
			t.Fatal(e)
		}
	}
	return &TrackedMemory{Space: m}
}
func setNZCV(r *Registers, flags uint64) {
	r.N = flags&(1<<31) != 0
	r.Z = flags&(1<<30) != 0
	r.C = flags&(1<<29) != 0
	r.V = flags&(1<<28) != 0
}
func TestLD1NativeCompleteLoads(t *testing.T) {
	var native struct {
		Source  string `json:"source_hex"`
		Initial string `json:"initial_hex"`
		Cases   []struct {
			Encoding             uint32
			Count, First, Offset int
			InputNZCV            uint64 `json:"input_nzcv"`
			OutputNZCV           uint64 `json:"output_nzcv"`
			Delta                uint64 `json:"base_delta"`
			Loaded               string `json:"loaded_hex"`
			Untouched            bool   `json:"untouched_match"`
		}
	}
	nativeAtomicObservation(t, "ld1-probe.c", &native)
	if len(native.Cases) != 2304 {
		t.Fatal(len(native.Cases))
	}
	source, e := hex.DecodeString(native.Source)
	if e != nil {
		t.Fatal(e)
	}
	initial, e := hex.DecodeString(native.Initial)
	if e != nil || len(initial) != 512 {
		t.Fatal(e)
	}
	for _, tc := range native.Cases {
		r := Registers{PC: 0x1000}
		setNZCV(&r, tc.InputNZCV)
		for i := range r.FP {
			r.WriteQ(i, binary.LittleEndian.Uint64(initial[16*i:]), binary.LittleEndian.Uint64(initial[16*i+8:]))
		}
		r.GPR[2] = 0x203d + uint64(tc.Offset)
		r.GPR[3] = 17
		want := r
		want.PC += 4
		want.GPR[2] += tc.Delta
		loaded, e := hex.DecodeString(tc.Loaded)
		if e != nil || len(loaded) != tc.Count*16 {
			t.Fatal(e)
		}
		for i := 0; i < tc.Count; i++ {
			want.WriteQ((tc.First+i)&31, binary.LittleEndian.Uint64(loaded[16*i:]), binary.LittleEndian.Uint64(loaded[16*i+8:]))
		}
		m := ld1Memory(t, tc.Encoding, 0x203d, source)
		if _, e := StepChecked(&r, m); e != nil || r != want || !tc.Untouched || tc.InputNZCV != tc.OutputNZCV {
			t.Fatal("LD1 native mismatch", tc, e, r, want)
		}
		// Every byte of an unaligned load is tracked, including both cache lines.
		_, width, _ := multipleSIMDShape(tc.Encoding)
		cursor := uint64(0x203d + tc.Offset)
		for _, a := range m.Accesses {
			if a.IsWrite || a.Addr != cursor {
				t.Fatal(a, cursor)
			}
			cursor += uint64(a.Size)
		}
		if cursor != uint64(0x203d+tc.Offset+width*tc.Count) {
			t.Fatal("incomplete memory span")
		}
	}
}
func TestLD1NativePartialFaultIsExplicitlyUnsupported(t *testing.T) {
	var native struct {
		Partial int `json:"partial_register_cases"`
		Cases   []struct {
			Encoding  uint32
			Prefix    int `json:"readable_prefix"`
			Signal    int
			Unchanged bool   `json:"registers_unchanged"`
			Delta     uint64 `json:"base_delta"`
			Flags     uint64 `json:"output_nzcv"`
		}
	}
	nativeAtomicObservation(t, "ld1-fault-probe.c", &native)
	if len(native.Cases) != 912 || native.Partial != 360 {
		t.Fatal("missing native partial-fault evidence")
	}
	partial := 0
	for _, tc := range native.Cases {
		if !tc.Unchanged {
			partial++
		}
		if tc.Signal != 10 || tc.Delta != 0 || tc.Flags != 0xa0000000 {
			t.Fatal(tc)
		}
		r := Registers{PC: 0x1000, N: true, C: true}
		r.GPR[2] = 0x2000
		r.GPR[3] = 17
		for i := range r.FP {
			r.WriteQ(i, uint64(i+1), ^uint64(i))
		}
		before := r
		m := ld1Memory(t, tc.Encoding, 0x2000, make([]byte, tc.Prefix))
		_, e := StepChecked(&r, m)
		var step *StepError
		if !errors.As(e, &step) || step.Kind != "unsupported_instruction" || !strings.Contains(e.Error(), "partial SIMD load fault state") || r != before || len(m.Accesses) != 0 {
			t.Fatal("claimed native atomic fault", tc, e)
		}
	}
	if partial != native.Partial {
		t.Fatal(partial)
	}
}
func TestLD1RejectsOtherStructureFormsAndChecksSP(t *testing.T) {
	for _, word := range []uint32{0x4c408043, 0x4d40c043, 0x4c008043} { // LD2, LD1R, ST2
		r := Registers{PC: 0x1000}
		r.GPR[2] = 0x2000
		before := r
		m := ld1Memory(t, word, 0x2000, make([]byte, 64))
		if _, e := StepChecked(&r, m); e == nil || r != before || len(m.Accesses) != 0 {
			t.Fatal(word, e)
		}
	}
	word := uint32(0x4c406843&^uint32(31<<5) | 31<<5)
	r := Registers{PC: 0x1000, SP: 0x2001, SPAlignmentCheck: true}
	before := r
	if _, e := StepChecked(&r, ld1Memory(t, word, 0x2000, make([]byte, 64))); e == nil || r != before {
		t.Fatal("unaligned SP accepted", e)
	}
	r.SP = 0x2000
	if _, e := StepChecked(&r, ld1Memory(t, word, 0x2000, make([]byte, 64))); e != nil {
		t.Fatal(e)
	}
}
