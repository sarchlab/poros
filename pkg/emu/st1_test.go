package emu

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"github.com/sarchlab/poros/pkg/usermem"
	"strings"
	"testing"
)

func TestST1NativeCompleteStores(t *testing.T) {
	var native struct {
		Initial string `json:"initial_hex"`
		Cases   []struct {
			Encoding     uint32
			Offset, Span int
			InputNZCV    uint64 `json:"input_nzcv"`
			OutputNZCV   uint64 `json:"output_nzcv"`
			Delta        uint64 `json:"base_delta"`
			Unchanged    bool   `json:"registers_unchanged"`
			Guards       bool   `json:"guards_unchanged"`
			Stored       string `json:"stored_hex"`
		}
	}
	nativeAtomicObservation(t, "st1-probe.c", &native)
	if len(native.Cases) != 2304 {
		t.Fatal(len(native.Cases))
	}
	initial, e := hex.DecodeString(native.Initial)
	if e != nil || len(initial) != 512 {
		t.Fatal(e)
	}
	for _, tc := range native.Cases {
		r := Registers{PC: 0x1000}
		setNZCV(&r, tc.InputNZCV)
		r.GPR[4] = 0x203d + uint64(tc.Offset)
		r.GPR[3] = 17
		for i := range r.FP {
			r.WriteQ(i, binary.LittleEndian.Uint64(initial[16*i:]), binary.LittleEndian.Uint64(initial[16*i+8:]))
		}
		want := r
		want.PC += 4
		want.GPR[4] += tc.Delta
		data := make([]byte, 128)
		for i := range data {
			data[i] = byte(i*37 + 11)
		}
		expected := bytes.Clone(data)
		stored, e := hex.DecodeString(tc.Stored)
		if e != nil || len(stored) != tc.Span {
			t.Fatal(e)
		}
		copy(expected[tc.Offset:], stored)
		m := ld1Memory(t, tc.Encoding, 0x203d, nil)
		space := m.Space.(*usermem.AddressSpace)
		if e := space.Map(0x203d, 128, usermem.Read|usermem.Write, data); e != nil {
			t.Fatal(e)
		}
		if _, e := StepChecked(&r, m); e != nil || r != want || !tc.Unchanged || !tc.Guards || tc.InputNZCV != tc.OutputNZCV {
			t.Fatal(tc, e)
		}
		actual := make([]byte, 128)
		if e := space.Read(0x203d, actual); e != nil || !bytes.Equal(actual, expected) {
			t.Fatal(tc, e)
		}
		cursor := uint64(0x203d + tc.Offset)
		for _, a := range m.Accesses {
			if !a.IsWrite || a.Addr != cursor {
				t.Fatal(a, cursor)
			}
			cursor += uint64(a.Size)
		}
		if cursor != uint64(0x203d+tc.Offset+tc.Span) {
			t.Fatal("incomplete ST1 tracked span")
		}
	}
}
func TestST1NativePartialFaultIsExplicitlyUnsupported(t *testing.T) {
	var native struct {
		Partial int `json:"partial_memory_cases"`
		Cases   []struct {
			Encoding  uint32
			Prefix    int `json:"writable_prefix"`
			Signal    int
			Delta     uint64 `json:"base_delta"`
			Flags     uint64 `json:"output_nzcv"`
			Unchanged bool   `json:"memory_unchanged"`
		}
	}
	nativeAtomicObservation(t, "st1-fault-probe.c", &native)
	if len(native.Cases) != 912 || native.Partial != 360 {
		t.Fatal(len(native.Cases))
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
		r.GPR[4] = 0x2000
		r.GPR[3] = 17
		r.Exclusive.Policy = ExclusiveM2Local64
		r.Exclusive.Valid = true
		r.Exclusive.Base = 0x2000
		for i := range r.FP {
			r.WriteQ(i, ^uint64(0), ^uint64(0))
		}
		before := r
		data := make([]byte, tc.Prefix)
		m := ld1Memory(t, tc.Encoding, 0x2000, nil)
		space := m.Space.(*usermem.AddressSpace)
		if len(data) > 0 {
			if e := space.Map(0x2000, uint64(len(data)), usermem.Read|usermem.Write, data); e != nil {
				t.Fatal(e)
			}
		}
		_, e := StepChecked(&r, m)
		var step *StepError
		if !errors.As(e, &step) || step.Kind != "unsupported_instruction" || !strings.Contains(e.Error(), "partial SIMD store fault state") || r != before || len(m.Accesses) != 0 {
			t.Fatal(tc, e)
		}
		if len(data) > 0 {
			got := make([]byte, len(data))
			if e := space.Read(0x2000, got); e != nil || !bytes.Equal(got, data) {
				t.Fatal("partial store mutated memory", tc, e)
			}
		}
	}
	if partial != native.Partial {
		t.Fatal(partial)
	}
}

func TestST1InvalidatesOnlyOverlappingExclusiveReservation(t *testing.T) {
	for _, base := range []uint64{0x2000, 0x2040, 0x2080} {
		r := Registers{PC: 0x1000}
		r.GPR[4] = 0x203d
		r.Exclusive = ExclusiveState{Policy: ExclusiveM2Local64, Valid: true, Base: base}
		m := ld1Memory(t, 0x4c002c80, 0, nil)
		space := m.Space.(*usermem.AddressSpace)
		if e := space.Map(0x2000, 128, usermem.Read|usermem.Write, nil); e != nil {
			t.Fatal(e)
		}
		if _, e := StepChecked(&r, m); e != nil {
			t.Fatal(e)
		}
		if r.Exclusive.Valid != (base == 0x2080) {
			t.Fatal(base, r.Exclusive)
		}
	}
}
