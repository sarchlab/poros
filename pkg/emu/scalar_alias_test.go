package emu

import (
	"encoding/binary"
	"strconv"
	"testing"
)

func TestScalarAliasesMatchNativeFullState(t *testing.T) {
	for _, probe := range []struct {
		source string
		count  int
	}{{"cneg-probe.c", 7168}, {"variable-shift-probe.c", 3584}, {"reverse-probe.c", 48}} {
		var cases []struct {
			Instruction, Encoding string
			Before, After         [3]string
			NZCVBefore            uint32 `json:"nzcv_before"`
			NZCVAfter             uint32 `json:"nzcv_after"`
		}
		nativeAtomicObservation(t, probe.source, &cases)
		if len(cases) != probe.count {
			t.Fatal(probe.source, len(cases))
		}
		parse := func(s string) uint64 {
			v, err := strconv.ParseUint(s, 16, 64)
			if err != nil {
				t.Fatal(err)
			}
			return v
		}
		for _, tc := range cases {
			r := Registers{PC: 0x1000, N: tc.NZCVBefore&(1<<31) != 0, Z: tc.NZCVBefore&(1<<30) != 0, C: tc.NZCVBefore&(1<<29) != 0, V: tc.NZCVBefore&(1<<28) != 0}
			for i, s := range tc.Before {
				r.GPR[i] = parse(s)
			}
			before := r
			if _, err := StepChecked(&r, authMemory{uint32(parse(tc.Encoding))}); err != nil {
				t.Fatal(tc.Instruction, err)
			}
			for i, s := range tc.After {
				before.GPR[i] = parse(s)
			}
			before.PC += 4
			before.N = tc.NZCVAfter&(1<<31) != 0
			before.Z = tc.NZCVAfter&(1<<30) != 0
			before.C = tc.NZCVAfter&(1<<29) != 0
			before.V = tc.NZCVAfter&(1<<28) != 0
			if r != before {
				t.Fatalf("native mismatch %s encoding%s before%v after%v got%#x", tc.Instruction, tc.Encoding, tc.Before, tc.After, r.GPR[:3])
			}
		}
	}
}

func TestSegmentNameComparisonUsesRegisterShift(t *testing.T) {
	// Actual dyld memcmp short-word path: compare the first eight bytes, locate
	// the first differing byte, then extract it with a register-count shift.
	// The prior alias bug shifted by zero and falsely returned equality for '__'.
	a, b := []byte("__PAGEZERO\x00"), []byte("__LINKEDIT\x00")
	r := Registers{PC: 0x1000}
	r.GPR[4] = binary.LittleEndian.Uint64(a)
	r.GPR[5] = binary.LittleEndian.Uint64(b)
	code := []uint32{0xeb050083, 0xdac00c63, 0xdac01063, 0x927df063, 0x9ac32484, 0x9ac324a5, 0x53001c84, 0x53001ca5, 0xcb050080}
	for _, word := range code {
		if _, err := StepChecked(&r, authMemory{word}); err != nil {
			t.Fatal(err)
		}
	}
	if r.GPR[0] != uint64('P'-'L') || r.GPR[3] != 16 {
		t.Fatal("distinct segment names compared equal", r.GPR[:6])
	}
}

func TestScalarAliasZeroRegisterAndUnsupportedVector(t *testing.T) {
	for _, word := range []uint32{0xda9f57e0, 0x9adf2420, 0x5ac0043f} {
		r := Registers{PC: 0x1000, C: true}
		r.GPR[1] = 0x123456789abcdef0
		before := r
		if _, err := StepChecked(&r, authMemory{word}); err != nil {
			t.Fatal(err)
		}
		before.PC += 4
		switch word {
		case 0xda9f57e0:
			before.GPR[0] = 0
		case 0x9adf2420:
			before.GPR[0] = before.GPR[1]
		}
		if r != before {
			t.Fatal("zero-register semantics", word, r.GPR[:3])
		}
	}
	r := Registers{PC: 0x1000}
	before := r
	if _, err := StepChecked(&r, authMemory{0x4e201820}); err == nil || r != before {
		t.Fatal("unsupported vector REV16 silently executed")
	}
}
