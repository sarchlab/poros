package emu

import (
	"encoding/binary"
	"github.com/sarchlab/poros/pkg/usermem"
	"strconv"
	"testing"
)

func TestRCpcEncodingAndFaultPrecision(t *testing.T) {
	for _, w := range []uint32{0x38bfc000, 0x78bfc000, 0xf8bfc000 ^ (1 << 16), 0xf8400000} {
		if _, ok := decodeRCpc(w); ok {
			t.Fatalf("other instruction admitted %x", w)
		}
	}
	i, ok := decodeRCpc(0xf8bfc108)
	if !ok || InstructionString(i) != "LDAPR X8, [X8]" || i.Enc != 0xf8bfc108 {
		t.Fatal(InstructionString(i))
	}
	for _, word := range []uint32{0xb8bfc061, 0xf8bfc061, 0xf8bfc07f} {
		m := atomicSpace(t, word, usermem.Write)
		r := Registers{PC: 0x1000}
		r.GPR[3] = 0x2000
		r.GPR[1] = 99
		before := r
		if _, e := StepChecked(&r, m); e == nil || r != before || len(m.Accesses) != 0 {
			t.Fatal("RCpc memory fault changed state", e)
		}
	}
	for _, word := range []uint32{0xb8bfc063, 0xf8bfc063, 0xf8bfc07f} {
		m := atomicSpace(t, word, usermem.Read|usermem.Write)
		value := uint64(0x89abcdef87654321)
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], value)
		m.Space.Write(0x2000, b[:])
		r := Registers{PC: 0x1000}
		r.GPR[3] = 0x2000
		if _, e := StepChecked(&r, m); e != nil {
			t.Fatal(e)
		}
		if word&31 != 31 {
			want := value
			if word>>30 == 2 {
				want = uint64(uint32(want))
			}
			if r.GPR[3] != want {
				t.Fatal("alias result", r.GPR[3])
			}
		}
		if len(m.Accesses) != 1 || m.Accesses[0].IsWrite {
			t.Fatal(m.Accesses)
		}
	}
}

func TestRCpcNativeValuesAndFlags(t *testing.T) {
	var native struct {
		Cases []struct {
			Width, Dest   int
			Encoding      uint32
			Memory        string
			Before, After [2]string
			NZCVBefore    uint32 `json:"nzcv_before"`
			NZCVAfter     uint32 `json:"nzcv_after"`
		}
	}
	nativeAtomicObservation(t, "ldapr-values-probe.c", &native)
	if len(native.Cases) != 576 {
		t.Fatal(len(native.Cases))
	}
	parse := func(s string) uint64 {
		v, e := strconv.ParseUint(s, 16, 64)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	for _, tc := range native.Cases {
		m := atomicSpace(t, tc.Encoding, usermem.Read|usermem.Write)
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], parse(tc.Memory))
		m.Space.Write(0x2000, b[:])
		r := Registers{PC: 0x1000, N: tc.NZCVBefore>>31&1 != 0, Z: tc.NZCVBefore>>30&1 != 0, C: tc.NZCVBefore>>29&1 != 0, V: tc.NZCVBefore>>28&1 != 0}
		r.GPR[8] = 0x2000
		r.GPR[9] = parse(tc.Before[1])
		want := r
		want.PC += 4
		if tc.Dest != 31 {
			want.GPR[tc.Dest] = parse(tc.After[tc.Dest-8])
		}
		if _, e := StepChecked(&r, m); e != nil || r != want || tc.NZCVBefore != tc.NZCVAfter {
			t.Fatal(tc, e, r.GPR[8:10])
		}
	}
}
func TestRCpcNativeMonitorPreservation(t *testing.T) {
	var native struct {
		Cases []struct {
			Width   int
			Offset  uint64 `json:"load_offset_bytes"`
			Trials  int
			Success int `json:"stxr_success"`
			Failure int `json:"stxr_failure"`
		}
	}
	nativeAtomicObservation(t, "ldapr-monitor-probe.c", &native)
	if len(native.Cases) != 6 {
		t.Fatal(len(native.Cases))
	}
	for _, tc := range native.Cases {
		if tc.Trials != 1000 || tc.Success != 1000 || tc.Failure != 0 {
			t.Fatal(tc)
		}
		words := []uint32{0x885f7c61, 0xb8bfc082, 0x88057c61}
		if tc.Width == 8 {
			for i := range words {
				words[i] |= 1 << 30
			}
		}
		code := make([]byte, 12)
		for i, w := range words {
			binary.LittleEndian.PutUint32(code[i*4:], w)
		}
		space := &usermem.AddressSpace{}
		space.Map(0x1000, 12, usermem.Execute, code)
		data := make([]byte, 32768)
		binary.LittleEndian.PutUint64(data, 7)
		space.Map(0x10000, 32768, usermem.Read|usermem.Write, data)
		m := &TrackedMemory{Space: space}
		r := Registers{PC: 0x1000}
		r.Exclusive.Policy = ExclusiveM2Local64
		r.GPR[3] = 0x10000
		r.GPR[4] = 0x10000 + tc.Offset
		for range words {
			if _, e := StepChecked(&r, m); e != nil {
				t.Fatal(e)
			}
		}
		if r.GPR[5] != 0 {
			t.Fatal("RCpc cleared local monitor", tc, r.GPR[5])
		}
	}
}
func TestRCpcNativeFaultsIncludingZeroDestination(t *testing.T) {
	var native struct {
		Cases []struct {
			Width, Mode, Exit, Signal int
			ZR                        bool
		}
	}
	nativeAtomicObservation(t, "ldapr-fault-probe.c", &native)
	if len(native.Cases) != 24 {
		t.Fatal(len(native.Cases))
	}
	for _, tc := range native.Cases {
		word := uint32(0xb8bfc061)
		if tc.Width == 8 {
			word |= 1 << 30
		}
		if tc.ZR {
			word |= 31
		}
		m := atomicSpace(t, word, usermem.Read|usermem.Write)
		address := uint64(0x2000)
		switch tc.Mode {
		case 1:
			m.Space.(*usermem.AddressSpace).Protect(address, 64, usermem.Read, false)
		case 2:
			m.Space.(*usermem.AddressSpace).Protect(address, 64, 0, false)
		case 3:
			m.Space.(*usermem.AddressSpace).Unmap(address, 64)
		case 4:
			address = 0
		case 5:
			address += 64 - uint64(tc.Width)
		}
		r := Registers{PC: 0x1000}
		r.GPR[3] = address
		before := r
		_, e := StepChecked(&r, m)
		if tc.Signal != 0 {
			if e == nil || r != before || len(m.Accesses) != 0 {
				t.Fatal(tc, e)
			}
		} else if tc.Exit != 0 || e != nil || len(m.Accesses) != 1 {
			t.Fatal(tc, e)
		}
	}
}
