package emu

import "testing"

func TestAppleThreadProtectionReadRequiresExplicitState(t *testing.T) {
	for _, destination := range []uint32{0, 8, 30, 31} {
		for _, value := range []uint64{0, 0x2010002030100000, ^uint64(0)} {
			word := uint32(0xd53ef1a0) | destination
			r := Registers{PC: 0x1000, N: true, C: true, Counter: CounterState{Enabled: true}, ThreadPointer: ThreadPointerState{Enabled: true}}
			r.GPR[8] = 0xabcdef
			before := r
			if _, e := StepChecked(&r, authMemory{word}); e == nil || r != before {
				t.Fatal("unconfigured architectural value guessed")
			}
			r.AppleThreadProtection = AppleThreadProtectionState{Enabled: true, Value: value}
			before = r
			step, e := StepChecked(&r, authMemory{word})
			if e != nil || step.Instruction.Enc != word || r.PC != before.PC+4 {
				t.Fatal(step, e)
			}
			want := before
			want.PC += 4
			want.WriteX(int(destination), value)
			if r != want {
				t.Fatal("MRS changed unrelated state", r, want)
			}
		}
	}
	r := Registers{PC: 0x1000, AppleThreadProtection: AppleThreadProtectionState{Enabled: true, Value: 0x2010002030100000}}
	// Unknown neighboring registers and MSR of the known register remain faults;
	// successful read support is not permission to ignore protection writes.
	for _, word := range []uint32{0xd53ef180, 0xd51ef1a8} {
		before := r
		if _, e := StepChecked(&r, authMemory{word}); e == nil || r != before {
			t.Fatalf("unmodeled system operation %#x accepted", word)
		}
	}
}
