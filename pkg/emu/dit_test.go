package emu

import "testing"

func TestDITAndSBRecordedNativeState(t *testing.T) {
	var evidence struct {
		Feature, Count int
		Initial        uint64 `json:"initial_main"`
		Restored       uint64
		Cases          []struct {
			Operation     string
			Input, Output uint64
			InputNZCV     uint64 `json:"input_nzcv"`
			OutputNZCV    uint64 `json:"output_nzcv"`
		}
	}
	nativeAtomicObservation(t, "dit-probe.c", &evidence)
	if evidence.Feature != 1 || evidence.Count != 224 || len(evidence.Cases) != 224 || evidence.Initial != evidence.Restored {
		t.Fatal("native evidence incomplete")
	}
	for _, tc := range evidence.Cases {
		r := Registers{PC: 0x1000, DIT: DITState{Available: true, Value: true}, SpeculationBarrier: true}
		r.GPR[0] = tc.Input
		r.GPR[1] = 0xfeed
		r.N = tc.InputNZCV&(1<<31) != 0
		r.Z = tc.InputNZCV&(1<<30) != 0
		r.C = tc.InputNZCV&(1<<29) != 0
		r.V = tc.InputNZCV&(1<<28) != 0
		before := r
		var word uint32
		switch tc.Operation {
		case "register":
			word = 0xd51b42a0
		case "immediate":
			word = 0xd503405f | uint32(tc.Input)<<8
		case "write-xzr":
			word = 0xd51b42bf
		case "read-xzr":
			word = 0xd53b42bf
		case "sb":
			word = 0xd50330ff
		default:
			t.Fatal(tc.Operation)
		}
		step, e := StepChecked(&r, authMemory{word})
		if e != nil || step.Instruction.Enc != word {
			t.Fatal(tc, e)
		}
		want := before
		want.PC += 4
		want.DIT.Value = tc.Output != 0
		if r != want || tc.OutputNZCV != tc.InputNZCV {
			t.Fatal("native state mismatch", tc, r, want)
		}
		// Independently exercise MRS destination/zero register semantics.
		for _, dest := range []uint32{0, 1, 9, 30, 31} {
			read := r
			read.PC = 0x1000
			want := read
			want.PC += 4
			want.WriteX(int(dest), tc.Output)
			if _, e := StepChecked(&read, authMemory{0xd53b42a0 | dest}); e != nil || read != want {
				t.Fatal("DIT read", tc, dest, e, read, want)
			}
		}
	}
}
func TestDITAndSBRejectUnavailableAndUnknownStateOperations(t *testing.T) {
	for _, word := range []uint32{0xd53b42a9, 0xd51b42a0, 0xd503405f, 0xd503415f, 0xd50330ff} {
		r := Registers{PC: 0x1000, Counter: CounterState{Enabled: true}}
		before := r
		if _, e := StepChecked(&r, authMemory{word}); e == nil || r != before {
			t.Fatalf("unavailable feature %#x", word)
		}
	}
	for _, word := range []uint32{0xd503425f, 0xd51b4280, 0xd53b4280, 0xd50331ff} {
		r := Registers{PC: 0x1000, DIT: DITState{Available: true}, SpeculationBarrier: true}
		before := r
		if _, e := StepChecked(&r, authMemory{word}); e == nil || r != before {
			t.Fatalf("unknown system operation %#x", word)
		}
	}
	// Availability is independent: DIT does not authorize SB and vice versa.
	for _, r := range []Registers{{PC: 0x1000, DIT: DITState{Available: true}}, {PC: 0x1000, SpeculationBarrier: true}} {
		word := uint32(0xd50330ff)
		if r.SpeculationBarrier {
			word = 0xd503415f
		}
		before := r
		if _, e := StepChecked(&r, authMemory{word}); e == nil || r != before {
			t.Fatal("feature coupling")
		}
	}
}
