package ooo

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestScalarFAddWakeupDistanceThroughDispatch(t *testing.T) {
	const add = 0x1e612800 // FADD D0, D0, D1
	for _, tc := range []struct {
		name    string
		gap     int
		enabled bool
		want    uint64
	}{
		{"disabled", 4, false, 25},
		{"one slot per edge", 0, true, 26},
		{"five slots per edge", 4, true, 30},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var words []uint32
			for i := 0; i < 12; i++ {
				words = append(words, add)
				for j := 0; j < tc.gap; j++ {
					words = append(words, nop)
				}
			}
			p := specPipeline(t, words, nil, func(s *Spec) {
				s.FetchWidth = 10
				s.Latencies.FPArithmetic = 2
				if tc.enabled {
					s.Latencies.FAddWakeupSpan = 6
				}
			})
			admitted(p, (len(words)+9)/10)
			if got := p.comp.State.FpReady[0]; got != tc.want {
				t.Errorf("last FADD ready at %d, want %d", got, tc.want)
			}
			state := p.comp.State.FAddWakeup
			if !tc.enabled && state != (faddWakeupState{}) {
				t.Error("disabled profile changed wakeup state")
			}
			data, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			var restored faddWakeupState
			if err := json.Unmarshal(data, &restored); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(state, restored) {
				t.Error("wakeup timing state did not survive checkpoint round trip")
			}
		})
	}
}

func TestScalarFAddWakeupSeesTakenBranchToNextInstruction(t *testing.T) {
	const branch = 0x54000021 // B.NE .+4
	for _, taken := range []bool{false, true} {
		p := specPipeline(t, []uint32{branch}, nil, func(s *Spec) {
			s.Latencies.FAddWakeupSpan = 6
			s.Latencies.FAddWakeupTaken = 2
		})
		p.comp.Resources.Emu.Regs.Z = !taken
		admitted(p, 1)
		want := uint64(1)
		if taken {
			want += 2
		}
		if got := p.comp.State.FAddWakeup.Position; got != want {
			t.Errorf("taken=%v distance %d, want %d", taken, got, want)
		}
	}
}
