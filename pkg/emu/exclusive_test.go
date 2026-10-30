package emu

import (
	"encoding/binary"
	"github.com/sarchlab/poros/pkg/usermem"
	"testing"
)

func exclusiveFixture(t *testing.T, words []uint32, q uint64) (Registers, *TrackedMemory) {
	t.Helper()
	m := &usermem.AddressSpace{}
	code := make([]byte, len(words)*4)
	for i, w := range words {
		binary.LittleEndian.PutUint32(code[i*4:], w)
	}
	if err := m.Map(0x1000, uint64(len(code)), usermem.Execute, code); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 32768)
	binary.LittleEndian.PutUint32(data, 7)
	if err := m.Map(0x2000, 32768, usermem.Read|usermem.Write, data); err != nil {
		t.Fatal(err)
	}
	r := Registers{PC: 0x1000, Exclusive: ExclusiveState{Policy: ExclusiveM2Local64}}
	r.GPR[3] = 0x2000
	r.GPR[4] = q
	r.GPR[5] = 9
	r.GPR[6] = 11
	return r, &TrackedMemory{Space: m}
}
func TestExclusiveNativeContract(t *testing.T) {
	var native struct {
		Cases []struct {
			Name                        string
			Successes, Failures, Loaded int
			Final                       uint32 `json:"final_original"`
			Signal                      int
		}
	}
	nativeAtomicObservation(t, "exclusive-probe.c", &native)
	if len(native.Cases) != 12 {
		t.Fatal("native exclusive cases missing")
	}
	for _, tc := range native.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			words := []uint32{0x885f7c61}
			q := uint64(0x6000)
			store := uint32(0x88027c65)
			switch tc.Name {
			case "normal", "loaded-readonly-target":
			case "clrex":
				words = append(words, 0xd5033f5f)
			case "same-address-store":
				words = append(words, 0xb9000066)
			case "adjacent-store":
				q = 0x2004
				words = append(words, 0xb9000086)
			case "store-plus64":
				q = 0x2040
				words = append(words, 0xb9000086)
			case "store-plus128":
				q = 0x2080
				words = append(words, 0xb9000086)
			case "second-load-other-address":
				q = 0x2004
				words = append(words, 0x885f7c81)
			case "width-mismatch":
				store = 0xc8027c65
			case "no-monitor", "no-monitor-readonly-target":
				words = []uint32{0xd5033f5f}
			case "monitor-other-readonly-target":
				store = 0x88027c85
			default:
				t.Fatal(tc.Name)
			}
			words = append(words, store)
			r, m := exclusiveFixture(t, words, q)
			space := m.Space.(*usermem.AddressSpace)
			if tc.Name == "no-monitor-readonly-target" || tc.Name == "loaded-readonly-target" {
				space.Protect(0x2000, 16384, usermem.Read, false)
			}
			if tc.Name == "monitor-other-readonly-target" {
				space.Protect(q, 16384, usermem.Read, false)
			}
			var fault error
			for range words {
				before := r
				m.Accesses = nil
				_, fault = StepChecked(&r, m)
				if fault != nil {
					if r != before || len(m.Accesses) != 0 {
						t.Fatal("fault changed state")
					}
					break
				}
			}
			if tc.Signal != 0 {
				if fault == nil {
					t.Fatal("native permission fault missing")
				}
				return
			}
			if fault != nil {
				t.Fatal(fault)
			}
			status := uint64(1)
			if tc.Successes != 0 {
				status = 0
			}
			var data [4]byte
			space.Read(0x2000, data[:])
			if r.GPR[2] != status || r.GPR[1] != uint64(tc.Loaded) || binary.LittleEndian.Uint32(data[:]) != tc.Final || r.Exclusive.Valid {
				t.Fatal(r.GPR[:7], data, r.Exclusive)
			}
		})
	}
}
func TestExclusiveNativeGranuleSweep(t *testing.T) {
	var native struct {
		Cases []struct {
			Kind                        string
			Offset, Successes, Failures int
		}
	}
	nativeAtomicObservation(t, "exclusive-granule-probe.c", &native)
	if len(native.Cases) != 66 {
		t.Fatal("native sweep missing")
	}
	for _, tc := range native.Cases {
		middle := uint32(0xb9000086)
		if tc.Kind == "second-exclusive-load" {
			middle = 0x885f7c81
		}
		r, m := exclusiveFixture(t, []uint32{0x885f7c61, middle, 0x88027c65}, 0x2000+uint64(tc.Offset))
		for i := 0; i < 3; i++ {
			if _, err := StepChecked(&r, m); err != nil {
				t.Fatal(tc, err)
			}
		}
		if (r.GPR[2] == 0) != (tc.Successes != 0) {
			t.Fatal("native granule mismatch", tc, r.GPR[2])
		}
	}
}
func TestExclusiveProfileZeroRegisterAndAlias(t *testing.T) {
	r, m := exclusiveFixture(t, []uint32{0x885f7c7f, 0x881f7c65}, 0x6000)
	r.Exclusive = ExclusiveState{}
	before := r
	if _, err := StepChecked(&r, m); err == nil || r != before {
		t.Fatal("unspecified monitor profile accepted")
	}
	r.Exclusive.Policy = ExclusiveM2Local64
	for i := 0; i < 2; i++ {
		if _, err := StepChecked(&r, m); err != nil {
			t.Fatal(err)
		}
	}
	var data [4]byte
	m.Space.Read(0x2000, data[:])
	if binary.LittleEndian.Uint32(data[:]) != 9 {
		t.Fatal("zero register changed reservation/store")
	}
	for _, word := range []uint32{0x88057c65, 0x88037c65} {
		r, m = exclusiveFixture(t, []uint32{word}, 0x6000)
		before = r
		if _, err := StepChecked(&r, m); err == nil || r != before {
			t.Fatal("constrained register alias accepted")
		}
	}
}

func TestExclusiveNativeAlignment(t *testing.T) {
	var native struct {
		Cases []struct {
			Width, Offset, Exit, Signal int
			Load, SP                    bool
		}
	}
	nativeAtomicObservation(t, "exclusive-alignment-probe.c", &native)
	if len(native.Cases) != 192 {
		t.Fatal("missing alignment cases")
	}
	for _, tc := range native.Cases {
		word := uint32(0x88027c65)
		if tc.Load {
			word = 0x885f7c61
		}
		if tc.Width == 8 {
			word |= 1 << 30
		}
		if tc.SP {
			word = (word &^ uint32(31<<5)) | 31<<5
		}
		r, m := exclusiveFixture(t, []uint32{word}, 0x6000)
		r.AtomicAlignment = AtomicWithin16Bytes
		r.SPAlignmentCheck = true
		addr := uint64(0x2000 + tc.Offset)
		r.GPR[3] = addr
		r.SP = addr
		var data [8]byte
		binary.LittleEndian.PutUint64(data[:], 9)
		m.Space.Write(addr, data[:tc.Width])
		before := r
		_, err := StepChecked(&r, m)
		if tc.Signal != 0 {
			if err == nil || r != before || len(m.Accesses) != 0 {
				t.Fatal("native fault mismatch", tc, err)
			}
			continue
		}
		if tc.Exit != 0 || err != nil {
			t.Fatal(tc, err)
		}
		if tc.Load && r.GPR[1] != 9 || !tc.Load && r.GPR[2] != 1 {
			t.Fatal("native status/value mismatch", tc, r.GPR[:4])
		}
		var after [8]byte
		m.Space.Read(addr, after[:tc.Width])
		if after != data {
			t.Fatal("failed reservation wrote memory")
		}
	}
}

func TestExclusiveNativeLSEInvalidation(t *testing.T) {
	var native struct {
		Bad   int
		Cases []struct {
			Kind, Offset, Success, Failure int
			Match                          bool
		}
	}
	nativeAtomicObservation(t, "exclusive-cas-probe.c", &native)
	if native.Bad != 0 || len(native.Cases) != 198 {
		t.Fatal("native LSE/exclusive cases missing")
	}
	for _, tc := range native.Cases {
		middle := uint32(0xd503201f)
		if tc.Kind == 1 {
			middle = 0x88a77c86
		}
		if tc.Kind == 2 {
			middle = 0xb8268087
		}
		r, m := exclusiveFixture(t, []uint32{0x885f7c61, middle, 0x88027c65}, 0x2000+uint64(tc.Offset))
		r.GPR[5] = 13
		r.GPR[6] = 9
		r.GPR[7] = 0
		if tc.Match {
			r.GPR[7] = 7
		}
		m.Space.Write(r.GPR[4], []byte{7, 0, 0, 0})
		for i := 0; i < 3; i++ {
			if _, err := StepChecked(&r, m); err != nil {
				t.Fatal(tc, err)
			}
		}
		if (r.GPR[2] == 0) != (tc.Success != 0) || r.Exclusive.Valid {
			t.Fatal("native LSE monitor mismatch", tc, r.GPR[2])
		}
		want := uint32(7)
		if tc.Kind == 0 {
			want = 13
		} else if tc.Offset == 0 && (tc.Kind == 2 || tc.Match) {
			want = 9
		}
		var got [4]byte
		m.Space.Read(0x2000, got[:])
		if binary.LittleEndian.Uint32(got[:]) != want {
			t.Fatal("LSE value mismatch", tc, got)
		}
	}
}
