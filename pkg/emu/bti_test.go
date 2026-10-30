package emu

import (
	"encoding/binary"
	"github.com/sarchlab/poros/pkg/usermem"
	"testing"
)

func TestBTIRequiresExplicitNonEnforcedState(t *testing.T) {
	for _, word := range []uint32{0xd503241f, 0xd503245f, 0xd503249f, 0xd50324df} {
		for _, mode := range []BranchTargetMode{BranchTargetUnspecified, BranchTargetNonEnforced, BranchTargetMode(2)} {
			r := Registers{PC: 0x1000, BranchTarget: mode, N: true, C: true}
			r.GPR[16] = 17
			r.GPR[30] = 0x2000
			before := r
			step, err := StepChecked(&r, authMemory{word})
			if mode == BranchTargetNonEnforced {
				before.PC += 4
				if err != nil || r != before || step.Instruction.Enc != word {
					t.Fatal("BTI altered state/encoding", r, err)
				}
			} else if err == nil || r != before {
				t.Fatal("unknown/enforced BTI silently ignored")
			}
		}
	}
	r := Registers{PC: 0x1000, BranchTarget: BranchTargetNonEnforced}
	before := r
	if _, err := StepChecked(&r, authMemory{0xd503281f}); err == nil || r != before {
		t.Fatal("unrelated unknown HINT silently ignored")
	}
}

func TestBTINativeNonEnforcedBranches(t *testing.T) {
	var observed struct {
		Feature int `json:"feat_bti"`
		Cases   []struct {
			Target, Branch string
			Returned       bool `json:"returned_99"`
			Signal         int
		}
	}
	nativeAtomicObservation(t, "bti-probe.c", &observed)
	if observed.Feature != 1 || len(observed.Cases) != 10 {
		t.Fatal("native feature/state distinction missing")
	}
	for i, tc := range observed.Cases {
		if !tc.Returned || tc.Signal != 0 {
			t.Fatal("native branch counterexample failed", tc)
		}
		if i >= 8 {
			continue
		} // Real dyld stack-probe checks are reproduced by native capture.
		m := &usermem.AddressSpace{}
		main := make([]byte, 4)
		word := uint32(0xd63f0180)
		if tc.Branch == "br_x12" {
			word = 0xd61f0180
		}
		binary.LittleEndian.PutUint32(main, word)
		m.Map(0x1000, 4, usermem.Execute, main)
		words := []uint32{0xd2800c60, 0xd65f03c0}
		hint := map[string]uint32{"bti_c": 0xd503245f, "bti_j": 0xd503249f, "bti_jc": 0xd50324df}[tc.Target]
		if hint != 0 {
			words = append([]uint32{hint}, words...)
		}
		target := make([]byte, len(words)*4)
		for i, w := range words {
			binary.LittleEndian.PutUint32(target[i*4:], w)
		}
		m.Map(0x2000, uint64(len(target)), usermem.Execute, target)
		r := Registers{PC: 0x1000, BranchTarget: BranchTargetNonEnforced, C: true}
		r.GPR[12] = 0x2000
		r.GPR[30] = 0x1004
		for step := 0; step < 8 && r.PC != 0x1004; step++ {
			if _, err := StepChecked(&r, m); err != nil {
				t.Fatal(tc, err)
			}
		}
		if r.PC != 0x1004 || r.GPR[0] != 99 || !r.C {
			t.Fatal(tc, r)
		}
	}
}
