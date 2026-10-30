package emu

import "testing"

func TestADCRecordedNativeCases(t *testing.T) {
	var native struct {
		Cases []struct {
			Encoding   uint32
			A, B       uint64
			InitialC   uint64 `json:"initial_c"`
			InputNZCV  uint64 `json:"input_nzcv"`
			OutA       uint64 `json:"out_a"`
			OutB       uint64 `json:"out_b"`
			OutC       uint64 `json:"out_c"`
			OutputNZCV uint64 `json:"output_nzcv"`
		}
	}
	nativeAtomicObservation(t, "adc-probe.c", &native)
	if len(native.Cases) != 2560 {
		t.Fatal("missing ADC native cases")
	}
	for _, tc := range native.Cases {
		r := Registers{PC: 0x1000}
		r.GPR[0] = tc.A
		r.GPR[1] = tc.B
		r.GPR[2] = tc.InitialC
		r.N = tc.InputNZCV&(1<<31) != 0
		r.Z = tc.InputNZCV&(1<<30) != 0
		r.C = tc.InputNZCV&(1<<29) != 0
		r.V = tc.InputNZCV&(1<<28) != 0
		want := r
		want.PC += 4
		want.GPR[0] = tc.OutA
		want.GPR[1] = tc.OutB
		want.GPR[2] = tc.OutC
		if _, e := StepChecked(&r, authMemory{tc.Encoding}); e != nil || r != want || tc.InputNZCV != tc.OutputNZCV {
			t.Fatal("native ADC state", tc, e, r, want)
		}
	}
}
