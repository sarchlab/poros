package emu

import "testing"

func TestVirtualSystemCounterRequiresState(t *testing.T) {
	for _, word := range []uint32{0xd53be048, 0xd53be0c8, 0xd53cfac8, 0xd53be008} {
		r := Registers{PC: 0x1000}
		if _, e := StepChecked(&r, authMemory{word}); e == nil {
			t.Fatal("unconfigured counter succeeded")
		}
		r.Counter = CounterState{Enabled: true, Value: 123, Frequency: 24000000}
		step, e := StepChecked(&r, authMemory{word})
		want := uint64(123)
		if word == 0xd53be008 {
			want = 24000000
		}
		if e != nil || r.GPR[8] != want || r.PC != 0x1004 || step.Instruction.Enc != word {
			t.Fatalf("%#x %+v %v", word, r, e)
		}
	}
	// Unrelated system state is not silently zero-filled.
	r := Registers{PC: 0x1000, Counter: CounterState{Enabled: true}}
	before := r
	if _, e := StepChecked(&r, authMemory{0xd53bd048}); e == nil || r != before {
		t.Fatal("unconfigured TPIDR_EL0 succeeded")
	}
}
