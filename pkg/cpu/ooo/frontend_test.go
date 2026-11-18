package ooo

import "testing"

const (
	subsX0    = 0xf1000400 // SUBS X0, X0, #1
	bneMinus4 = 0x54ffffe1 // B.NE .-4
	bMinus8   = 0x17fffffe // B .-8
	addX9X9   = 0x91000529 // ADD X9, X9, #1
	cmpX0Zero = 0xf100001f // CMP X0, #0
	subX0     = 0xd1000400 // SUB X0, X0, #1
	bneMinus8 = 0x54ffffc1 // B.NE .-8
)

// admitted runs fetchAndProcess for the given cycles and returns how many
// instructions each cycle admitted. Retirement keeps the ROB from filling.
func admitted(p *middleware, cycles int) []int {
	var out []int
	for i := 0; i < cycles; i++ {
		before := p.comp.State.InstDone + uint64(p.comp.State.RobCount)
		p.fetchAndProcess()
		out = append(out, int(p.comp.State.InstDone+uint64(p.comp.State.RobCount)-before))
		p.comp.State.Cycles++
		p.retire()
	}
	return out
}

// With fetch groups, a correctly predicted taken branch ends the group and a
// group is fetched per cycle: a three-instruction loop admits one trip per
// cycle although rename has room for more.
func TestFetchGroupsEndAtTakenBranches(t *testing.T) {
	p := specPipeline(t, []uint32{nop, nop, bMinus8}, nil, func(s *Spec) {
		s.FetchWidth, s.NumINT = 10, 4
		s.FetchGroupLimit = 16
	})
	p.comp.Resources.Emu.Regs.WriteX(0, 1<<20)
	admitted(p, 1) // the first trip trains the branch
	p.comp.State.Cycles = max(p.comp.State.Cycles, p.comp.State.FrontendReady)
	for i, n := range admitted(p, 6) {
		if n != 3 {
			t.Fatalf("cycle %d admitted %d instructions, want one trip of 3", i, n)
		}
	}
}

// A group holds at most FetchGroupLimit instructions, so straight-line code
// is fetched FetchGroupLimit at a time even when rename takes more.
func TestFetchGroupLimitBoundsStraightLineFetch(t *testing.T) {
	words := make([]uint32, 64)
	for i := range words {
		words[i] = nop
	}
	p := specPipeline(t, words, nil, func(s *Spec) {
		s.FetchWidth = 10
		s.FetchGroupLimit = 4
	})
	for i, n := range admitted(p, 8) {
		if n != 4 {
			t.Fatalf("cycle %d admitted %d instructions, want 4", i, n)
		}
	}
}

// A flag-setting compare and the conditional branch after it take one rename
// slot and one retire slot, and the branch reserves no integer pipe.
func TestCompareBranchFusionSharesOneSlot(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fused  bool
		perCyc int
	}{{"fused", true, 2}, {"separate", false, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			p := specPipeline(t, []uint32{subsX0, bneMinus4}, nil, func(s *Spec) {
				s.FetchWidth, s.RetireWidth = 1, 1
				s.CompareBranchFusion = tc.fused
			})
			p.comp.Resources.Emu.Regs.WriteX(0, 1<<20)
			if got := admitted(p, 1)[0]; got != tc.perCyc {
				t.Fatalf("admitted %d in one cycle with one rename slot, want %d", got, tc.perCyc)
			}
			if tc.fused {
				if !p.comp.State.RobFused[1] || p.comp.State.RobFused[0] {
					t.Fatalf("fused flags %v, want only the branch marked", p.comp.State.RobFused[:2])
				}
				if n := len(p.comp.State.IntUnits[0]); n != 1 {
					t.Fatalf("the pair reserved %d integer slots, want 1 (the compare's)", n)
				}
				// admitted retired once, with one retire slot, after both completed.
				if p.comp.State.InstDone != 2 {
					t.Fatalf("one retire slot retired %d instructions, want the fused pair", p.comp.State.InstDone)
				}
			}
		})
	}
}

// Only a flag-setting compare directly followed by a conditional branch fuses.
func TestCompareBranchFusionForms(t *testing.T) {
	for _, tc := range []struct {
		name  string
		words []uint32
		want  int
	}{
		{"cmp then b.ne", []uint32{subX0, cmpX0Zero, bneMinus8}, 3},
		{"add then b.ne", []uint32{subX0, addX9X9, bneMinus8}, 2},
		{"subs, nop, b.ne", []uint32{subsX0, nop, bneMinus8}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := specPipeline(t, tc.words, nil, func(s *Spec) {
				s.FetchWidth = 2
				s.NumINT = 4
				s.CompareBranchFusion = true
			})
			p.comp.Resources.Emu.Regs.WriteX(0, 1<<20)
			if got := admitted(p, 1)[0]; got != tc.want {
				t.Fatalf("admitted %d with two rename slots, want %d", got, tc.want)
			}
		})
	}
}

// After rename stalls, the fetch buffer supplies at most FetchLead+1 groups
// in one cycle, then one group per cycle.
func TestFetchBufferBoundsBurstAfterStall(t *testing.T) {
	p := specPipeline(t, []uint32{nop, bMinus8 + 1}, nil, func(s *Spec) { // NOP; B .-4
		s.FetchWidth = 16
		s.FetchGroupLimit = 16
		s.FetchLead = 2
	})
	admitted(p, 1) // train the branch
	p.comp.State.Cycles = max(p.comp.State.Cycles, p.comp.State.FrontendReady)
	admitted(p, 2)
	// Hold rename for ten cycles, as a full window would.
	p.comp.State.Cycles += 10
	got := admitted(p, 3)
	if want := []int{2 * 3, 2, 2}; got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("admitted %v after the stall, want %v", got, want)
	}
}

// A redirect empties the fetch buffer: no group fetched before it survives.
func TestRedirectEmptiesFetchBuffer(t *testing.T) {
	p := specPipeline(t, []uint32{nop}, nil, func(s *Spec) {
		s.FetchGroupLimit, s.FetchLead = 16, 3
	})
	p.comp.State.FetchGroupRenamed[1] = 40
	p.redirect(50)
	if p.comp.State.FrontendReady != 50 || !p.comp.State.FetchGroupEnded || p.comp.State.FetchGroupCycle != 49 {
		t.Fatalf("redirect state %+v", p.comp.State)
	}
	for _, c := range p.comp.State.FetchGroupRenamed {
		if c != 0 {
			t.Fatalf("fetch buffer not emptied: %v", p.comp.State.FetchGroupRenamed)
		}
	}
}

// Spec validation rejects negative group limits and leads.
func TestNegativeFetchGroupParametersRejected(t *testing.T) {
	for _, adjust := range []func(*Spec){
		func(s *Spec) { s.FetchGroupLimit = -1 },
		func(s *Spec) { s.FetchGroupLimit, s.FetchLead = 16, -1 },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatal("negative fetch group parameter accepted")
				}
			}()
			specPipeline(t, []uint32{nop}, nil, adjust)
		}()
	}
}
