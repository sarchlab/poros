package cpu

import "testing"

// Exercise decoded compare operands and flag consumers through the scoreboard,
// rather than only checking instruction classification tables.
func TestConditionalAliasesWaitForCompareFlags(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		body                      []uint32
		minimumCyclesPerIteration uint64
	}{
		{"CINC", []uint32{0x71000c1f, 0x1a801400}, 2},             // CMP W0,#3; CINC W0,W0,EQ
		{"CSET", []uint32{0x71000c1f, 0x1a9f17e0, 0x0b000000}, 3}, // CMP; CSET W0,EQ; ADD W0,W0,W0
	} {
		t.Run(tc.name, func(t *testing.T) {
			const iterations = 64
			var code []uint32
			for i := 0; i < iterations; i++ {
				code = append(code, tc.body...)
			}
			code = append(code, 0xd65f03c0) // RET
			core := runOoOPipeline(t, setupEmu(code, 0x10000, 0xffff0000))
			if core.State.Cycles < iterations*tc.minimumCyclesPerIteration {
				t.Fatalf("flag-dependent chain completed in %d cycles, want at least %d", core.State.Cycles, iterations*tc.minimumCyclesPerIteration)
			}
			if core.State.InstDone != uint64(len(code)) {
				t.Fatalf("retired %d instructions, want %d", core.State.InstDone, len(code))
			}
		})
	}
}
