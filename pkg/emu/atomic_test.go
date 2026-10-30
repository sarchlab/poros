package emu

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/sarchlab/poros/pkg/usermem"
	"golang.org/x/arch/arm64/arm64asm"
	"os"
	"strconv"
	"strings"
	"testing"
)

func atomicSpace(t *testing.T, word uint32, flags usermem.Permission) *TrackedMemory {
	t.Helper()
	m := &usermem.AddressSpace{}
	var code [4]byte
	binary.LittleEndian.PutUint32(code[:], word)
	if err := m.Map(0x1000, 4, usermem.Execute, code[:]); err != nil {
		t.Fatal(err)
	}
	var data [64]byte
	binary.LittleEndian.PutUint64(data[:], 7)
	if err := m.Map(0x2000, 64, flags, data[:]); err != nil {
		t.Fatal(err)
	}
	return &TrackedMemory{Space: m}
}
func TestAtomicOperandsResultsAndPermissions(t *testing.T) {
	for _, width := range []int{4, 8} {
		for _, order := range []uint32{0, 1 << 22, 1 << 15, (1 << 22) | (1 << 15)} {
			for _, match := range []bool{false, true} {
				word := uint32(0x88a17c62) | order
				if width == 8 {
					word |= 1 << 30
				}
				r := Registers{PC: 0x1000, C: true}
				r.GPR[1] = 0xffffffff00000007
				if width == 8 {
					r.GPR[1] = 7
				}
				if !match {
					r.GPR[1]++
				}
				r.GPR[2] = 9
				r.GPR[3] = 0x2000
				m := atomicSpace(t, word, usermem.Read|usermem.Write)
				step, err := StepChecked(&r, m)
				if err != nil || step.Instruction.Op != OpCAS || r.GPR[1] != 7 || r.PC != 0x1004 || !r.C {
					t.Fatal(r, err)
				}
				var data [8]byte
				if err := m.Space.Read(0x2000, data[:]); err != nil {
					t.Fatal(err)
				}
				want := uint64(7)
				events := 1
				if match {
					want = 9
					events = 2
				}
				if binary.LittleEndian.Uint64(data[:]) != want || len(m.Accesses) != events || m.Accesses[0].IsWrite {
					t.Fatal("atomic memory/events", data, m.Accesses)
				}
				for _, flags := range []usermem.Permission{usermem.Read, usermem.Write} {
					r.PC = 0x1000
					r.GPR[1] = 8
					before := r
					m = atomicSpace(t, word, flags)
					if _, err := StepChecked(&r, m); err == nil || r != before || len(m.Accesses) != 0 {
						t.Fatal("fault mutated state or failed compare bypassed permission")
					}
				}
			}
		}
	}
}
func TestAtomicAliasZeroAndAlignment(t *testing.T) {
	for _, word := range []uint32{0xc8e3fc62, 0xc8e1fc61, 0xc8fffc7f, 0xc8e1fc7f, 0xf8e18061} {
		m := atomicSpace(t, word, usermem.Read|usermem.Write)
		r := Registers{PC: 0x1000}
		r.GPR[1] = 7
		r.GPR[2] = 9
		r.GPR[3] = 0x2000
		// Rs==Rn reads the original address before overwriting Rs with old data.
		if _, err := StepChecked(&r, m); err != nil {
			t.Fatal(fmt.Sprintf("%x", word), err)
		}
		if word == 0xc8e3fc62 && r.GPR[3] != 7 {
			t.Fatal("Rs/Rn alias")
		}
	}
	for _, policy := range []AtomicAlignmentPolicy{AtomicNaturalAlignment, AtomicWithin16Bytes} {
		for off := uint64(0); off < 32; off++ {
			r := Registers{PC: 0x1000, AtomicAlignment: policy}
			r.GPR[3] = 0x2000 + off
			r.GPR[1] = 123
			r.GPR[2] = 9
			before := r
			m := atomicSpace(t, 0xc8e1fc62, usermem.Read|usermem.Write)
			_, err := StepChecked(&r, m)
			valid := off%8 == 0
			if policy == AtomicWithin16Bytes {
				valid = off%16 <= 8
			}
			if (err == nil) != valid || (!valid && (r != before || len(m.Accesses) != 0)) {
				t.Fatalf("policy%d off%d: %v", policy, off, err)
			}
		}
	}
}
func TestAtomicExtensionDecodeContract(t *testing.T) {
	for op := arm64asm.Op(0xff00); ; op++ {
		if op.String() != fmt.Sprintf("Op(%d)", op) {
			t.Fatal("x/arch extension opcode collision", op)
		}
		if op == 0xffff {
			break
		}
	}
	for _, word := range []uint32{0x08a17c62, 0x48a17c62, 0x08217c62, 0x88a10062, 0xb8211002} {
		if _, ok := decodeAtomic(word); ok {
			t.Fatalf("unsupported atomic decoded %x", word)
		}
	}
	inst, ok := decodeAtomic(0x88e87c03)
	if !ok || InstructionString(inst) != "CASA W8, W3, [X0]" {
		t.Fatal(InstructionString(inst))
	}
}
func nativeAtomicObservation(t *testing.T, source string, out any) {
	t.Helper()
	b, err := os.ReadFile("../../tests/macho/isa-native-m2.json")
	if err != nil {
		t.Fatal(err)
	}
	var e struct {
		Observations []struct {
			Source       string
			Hash         string            `json:"source_sha256"`
			Dependencies map[string]string `json:"source_dependencies"`
			Result       json.RawMessage
		}
	}
	if err = json.Unmarshal(b, &e); err != nil {
		t.Fatal(err)
	}
	for _, o := range e.Observations {
		if o.Source != source {
			continue
		}
		b, err := os.ReadFile("../../tests/macho/" + source)
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(b)
		if hex.EncodeToString(hash[:]) != o.Hash {
			t.Fatal("native source identity changed")
		}
		for path, want := range o.Dependencies {
			b, err := os.ReadFile("../../tests/macho/" + path)
			if err != nil {
				t.Fatal(err)
			}
			got := sha256.Sum256(b)
			if hex.EncodeToString(got[:]) != want {
				t.Fatal("native dependency identity changed", path)
			}
		}
		if err = json.Unmarshal(o.Result, out); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Fatal("missing native observation", source)
}
func TestAtomicRecordedNativeResults(t *testing.T) {
	var result struct {
		Vectors []struct {
			Width, Mode                              int
			Before, Compare, Replacement, Old, After uint64
		}
	}
	nativeAtomicObservation(t, "atomic-probe.c", &result)
	if len(result.Vectors) != 32 {
		t.Fatal(len(result.Vectors))
	}
	for _, v := range result.Vectors {
		word := uint32(0x88e1fc62)
		if v.Mode == 2 {
			word = 0x88e1fc61
		}
		if v.Mode == 3 {
			word = 0xb8e18061
		}
		if v.Width == 8 {
			word |= 1 << 30
		}
		m := atomicSpace(t, word, usermem.Read|usermem.Write)
		var data [8]byte
		binary.LittleEndian.PutUint64(data[:], v.Before)
		m.Space.Write(0x2000, data[:])
		r := Registers{PC: 0x1000}
		r.GPR[1] = v.Compare
		r.GPR[2] = v.Replacement
		r.GPR[3] = 0x2000
		if v.Mode == 3 {
			r.GPR[1] = v.Replacement
		}
		if _, err := StepChecked(&r, m); err != nil {
			t.Fatal(err)
		}
		m.Space.Read(0x2000, data[:])
		if r.GPR[1] != v.Old || binary.LittleEndian.Uint64(data[:]) != v.After {
			t.Fatalf("native mismatch %+v", v)
		}
	}
}

func TestAtomicRecordedNativeAlignment(t *testing.T) {
	var result struct {
		Cases []struct{ Width, Offset, Exit, Signal int }
	}
	nativeAtomicObservation(t, "cas-alignment-probe.c", &result)
	if len(result.Cases) != 64 {
		t.Fatal("missing native offsets")
	}
	for _, v := range result.Cases {
		word := uint32(0x88e1fc62)
		if v.Width == 8 {
			word |= 1 << 30
		}
		m := atomicSpace(t, word, usermem.Read|usermem.Write)
		m.Space.Write(0x2000, make([]byte, 64))
		r := Registers{PC: 0x1000, AtomicAlignment: AtomicWithin16Bytes}
		r.GPR[3] = 0x2000 + uint64(v.Offset)
		r.GPR[2] = 9
		before := r
		_, err := StepChecked(&r, m)
		nativeOK := v.Exit == 0 && v.Signal == 0
		if (err == nil) != nativeOK || (!nativeOK && r != before) {
			t.Fatalf("native alignment %+v: %v", v, err)
		}
	}
}

func TestAtomicSPAlignmentNativeContract(t *testing.T) {
	var result struct {
		Cases []struct {
			Offset       int `json:"sp_offset"`
			Exit, Signal int
		}
	}
	nativeAtomicObservation(t, "cas-sp-probe.c", &result)
	if len(result.Cases) != 16 {
		t.Fatal("missing SP offsets")
	}
	for _, v := range result.Cases {
		m := atomicSpace(t, 0xc8e1ffe2, usermem.Read|usermem.Write)
		r := Registers{PC: 0x1000, AtomicAlignment: AtomicWithin16Bytes, SPAlignmentCheck: true, SP: 0x2000 + uint64(v.Offset)}
		r.GPR[1] = 7
		r.GPR[2] = 9
		before := r
		_, err := StepChecked(&r, m)
		ok := v.Exit == 0 && v.Signal == 0
		if (err == nil) != ok || (!ok && (r != before || len(m.Accesses) != 0)) {
			t.Fatalf("SP native mismatch %+v %v", v, err)
		}
	}
}

func TestLDADDNativeValues(t *testing.T) {
	var native struct {
		Cases []struct {
			Bits, Alias                      int
			Order, Initial, Add, Old, Memory string
		}
	}
	nativeAtomicObservation(t, "ldadd-probe.c", &native)
	if len(native.Cases) != 160 {
		t.Fatal("missing native LDADD vectors")
	}
	for _, tc := range native.Cases {
		parse := func(s string) uint64 {
			v, e := strconv.ParseUint(s, 16, 64)
			if e != nil {
				t.Fatal(e)
			}
			return v
		}
		rs, rt := 1, 2
		switch tc.Alias {
		case 1:
			rt = 1
		case 2:
			rt = 31
		case 3:
			rs = 31
		}
		word := uint32(0xb8200060) | uint32(rs)<<16 | uint32(rt)
		if tc.Bits == 64 {
			word |= 1 << 30
		}
		if strings.Contains(tc.Order, "a") {
			word |= 1 << 23
		}
		if strings.Contains(tc.Order, "l") && tc.Order != "plain" {
			word |= 1 << 22
		}
		// The native order labels are plain/A/L/AL in lower case.
		if tc.Order == "plain" {
			word &^= 3 << 22
		}
		m := atomicSpace(t, word, usermem.Read|usermem.Write)
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], parse(tc.Initial))
		m.Space.Write(0x2000, b[:])
		r := Registers{PC: 0x1000, C: true}
		r.GPR[1] = parse(tc.Add)
		r.GPR[3] = 0x2000
		before := r
		if _, e := StepChecked(&r, m); e != nil {
			t.Fatal(tc, e)
		}
		if rt != 31 {
			before.GPR[rt] = parse(tc.Old)
		}
		before.PC += 4
		if before != r {
			t.Fatal(tc, "register result", r.GPR[:4])
		}
		m.Space.Read(0x2000, b[:])
		if binary.LittleEndian.Uint64(b[:]) != parse(tc.Memory) || len(m.Accesses) != 2 || m.Accesses[0].IsWrite || !m.Accesses[1].IsWrite {
			t.Fatal(tc, b, m.Accesses)
		}
	}
}

func TestLDADDFaultsAndBaseAliases(t *testing.T) {
	for _, word := range []uint32{0xf8210063, 0xf8230062} {
		m := atomicSpace(t, word, usermem.Read|usermem.Write)
		r := Registers{PC: 0x1000}
		r.GPR[1] = 9
		r.GPR[3] = 0x2000
		if _, err := StepChecked(&r, m); err != nil {
			t.Fatal(err)
		}
		if word == 0xf8210063 && r.GPR[3] != 7 {
			t.Fatal("destination/base alias")
		}
		var b [8]byte
		m.Space.Read(0x2000, b[:])
		want := uint64(16)
		if word == 0xf8230062 {
			want = 0x2007
		}
		if binary.LittleEndian.Uint64(b[:]) != want {
			t.Fatal("source/base alias", b)
		}
	}
	for _, flags := range []usermem.Permission{usermem.Read, usermem.Write} {
		m := atomicSpace(t, 0xf8210062, flags)
		r := Registers{PC: 0x1000, Exclusive: ExclusiveState{Policy: ExclusiveM2Local64, Valid: true, Base: 0x4000}}
		r.GPR[1] = 9
		r.GPR[3] = 0x2000
		before := r
		if _, err := StepChecked(&r, m); err == nil || r != before || len(m.Accesses) != 0 {
			t.Fatal("fault mutated state")
		}
	}
}
