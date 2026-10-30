package emu

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"github.com/sarchlab/poros/pkg/usermem"
	"testing"
)

func TestNonTemporalPairsNative(t *testing.T) {
	var native struct {
		Bad   int
		Cases []struct {
			Width, Offset, Misalignment int
			Load, OK                    bool
			Bytes                       string
		}
	}
	nativeAtomicObservation(t, "non-temporal-probe.c", &native)
	if native.Bad != 0 || len(native.Cases) != 288 {
		t.Fatal("native pair cases missing")
	}
	for _, tc := range native.Cases {
		base := uint32(0x28000000)
		if tc.Width == 8 {
			base = 0xa8000000
		}
		if tc.Width == 16 {
			base = 0xac000000
		}
		immediate := uint32((tc.Offset/tc.Width)&127) << 15
		load, store := base|(1<<22)|2|(3<<10)|(1<<5), base|2|(3<<10)
		if tc.Load {
			load |= immediate
			store |= 1 << 24
		} else {
			load |= 1 << 24
			store |= immediate
		}
		m := &usermem.AddressSpace{}
		var code [8]byte
		binary.LittleEndian.PutUint32(code[:], load)
		binary.LittleEndian.PutUint32(code[4:], store)
		m.Map(0x1000, 8, usermem.Execute, code[:])
		src := make([]byte, 512)
		for i := range src {
			src[i] = byte(i*17 + 23)
		}
		m.Map(0x2000, 512, usermem.Read, src)
		m.Map(0x3000, 512, usermem.Read|usermem.Write, nil)
		r := Registers{PC: 0x1000}
		r.GPR[0] = 0x3000 + 128 + uint64(tc.Misalignment)
		r.GPR[1] = 0x2000 + 128 + uint64(tc.Misalignment)
		memory := &TrackedMemory{Space: m}
		for i := 0; i < 2; i++ {
			if _, err := StepChecked(&r, memory); err != nil {
				t.Fatal(tc, err)
			}
		}
		want := make([]byte, 512)
		offset := 128 + tc.Misalignment
		if !tc.Load {
			offset += tc.Offset
		}
		data, err := hex.DecodeString(tc.Bytes)
		if err != nil || len(data) != tc.Width*2 || !tc.OK {
			t.Fatal("bad native bytes")
		}
		copy(want[offset:], data)
		got := make([]byte, 512)
		m.Read(0x3000, got)
		if !bytes.Equal(got, want) {
			t.Fatal("native pair mismatch", tc)
		}
	}
}
func TestNonTemporalPairStoreFaultIsAtomic(t *testing.T) {
	// STNP Q0,Q1,[X3], crossing into read-only memory must not store Q0 alone.
	m := &usermem.AddressSpace{}
	var code [4]byte
	binary.LittleEndian.PutUint32(code[:], 0xac000460)
	m.Map(0x1000, 4, usermem.Execute, code[:])
	m.Map(0x2000, 16, usermem.Read|usermem.Write, nil)
	m.Map(0x2010, 16, usermem.Read, nil)
	r := Registers{PC: 0x1000}
	r.GPR[3] = 0x2000
	r.WriteQ(0, 1, 2)
	r.WriteQ(1, 3, 4)
	before := r
	memory := &TrackedMemory{Space: m}
	if _, err := StepChecked(&r, memory); err == nil || r != before || len(memory.Accesses) != 0 {
		t.Fatal("pair fault mutated state", err)
	}
	var data [32]byte
	m.Read(0x2000, data[:])
	if data != [32]byte{} {
		t.Fatal("partial pair store")
	}
}
