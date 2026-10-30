package emu

import (
	"encoding/binary"
	"github.com/sarchlab/poros/pkg/usermem"
	"strconv"
	"testing"
)

func TestPointerStripNativeVectors(t *testing.T) {
	var observation struct {
		Vectors []struct{ Input, XPACI, XPACD string }
	}
	nativeAtomicObservation(t, "xpac-probe.c", &observation)
	if len(observation.Vectors) != 16 {
		t.Fatal("missing native vectors")
	}
	for _, v := range observation.Vectors {
		input, _ := strconv.ParseUint(v.Input, 16, 64)
		for _, word := range []uint32{0xdac143f1, 0xdac147f1} {
			wantText := v.XPACI
			if word == 0xdac147f1 {
				wantText = v.XPACD
			}
			want, _ := strconv.ParseUint(wantText, 16, 64)
			for _, auth := range []PointerAuthMode{PointerAuthUnspecified, PointerAuthDisabled, PointerAuthEnabled} {
				m := atomicSpace(t, word, usermem.Read|usermem.Write)
				r := Registers{PC: 0x1000, PointerAuth: auth, PointerStrip: PointerStripDarwin47Bit, C: true}
				r.GPR[17] = input
				step, err := StepChecked(&r, m)
				if err != nil || r.GPR[17] != want || r.PC != 0x1004 || !r.C || step.Instruction.Enc != word {
					t.Fatalf("%x %s: %#x want %#x %v", word, v.Input, r.GPR[17], want, err)
				}
			}
		}
	}
}
func TestPointerStripPolicyFaultAndZero(t *testing.T) {
	for _, policy := range []PointerStripPolicy{PointerStripUnspecified, 99} {
		m := atomicSpace(t, 0xdac147f1, usermem.Read|usermem.Write)
		r := Registers{PC: 0x1000, PointerStrip: policy}
		r.GPR[17] = ^uint64(0)
		before := r
		if _, err := StepChecked(&r, m); err == nil || r != before {
			t.Fatal("missing policy did not fail atomically", err)
		}
	}
	m := atomicSpace(t, 0xdac147ff, usermem.Read|usermem.Write)
	r := Registers{PC: 0x1000, SP: 123, PointerStrip: PointerStripDarwin47Bit}
	if _, err := StepChecked(&r, m); err != nil || r.SP != 123 {
		t.Fatal(err, r)
	}
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], 0xdac147f1)
	inst, err := DecodeChecked(b[:], PointerAuthUnspecified)
	if err != nil || InstructionString(inst) != "XPACD X17" {
		t.Fatal(InstructionString(inst), err)
	}
}
