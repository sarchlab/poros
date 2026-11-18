package ooo

import (
	"encoding/json"
	"reflect"
	"testing"
)

const (
	subsX0     = 0xf1000400 // SUBS X0, X0, #1
	bneMinus4  = 0x54ffffe1 // B.NE .-4
	bMinus8    = 0x17fffffe // B .-8
	addX9X9    = 0x91000529 // ADD X9, X9, #1
	cmpX0Zero  = 0xf100001f // CMP X0, #0
	subX0      = 0xd1000400 // SUB X0, X0, #1
	bneMinus8  = 0x54ffffc1 // B.NE .-8
	cbnzX0     = 0xb5ffffe0 // CBNZ X0, .-4
	cmpW2Imm   = 0x71003c5f // CMP W2, #15
	cselW2W6W2 = 0x1a8230c2 // CSEL W2, W6, W2, LO
	bPlus60    = 0x1400000f // B .+0x3c
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
// slot and one retire slot. The pair still takes an integer pipe for each
// half, both in the cycle it issues.
func TestCompareBranchFusionSharesOneSlot(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fused  bool
		perCyc int
	}{{"fused", true, 2}, {"separate", false, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			p := specPipeline(t, []uint32{subsX0, bneMinus4}, nil, func(s *Spec) {
				s.FetchWidth, s.RetireWidth = 1, 1
				s.NumINT = 2
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
				want := []UnitReservation{{1, 2}}
				if units := p.comp.State.IntUnits; !reflect.DeepEqual(units[0], want) || !reflect.DeepEqual(units[1], want) {
					t.Fatalf("pair reservations %+v, want one slot on each pipe in cycle 1", units)
				}
				// admitted retired once, with one retire slot, after both completed.
				if p.comp.State.InstDone != 2 {
					t.Fatalf("one retire slot retired %d instructions, want the fused pair", p.comp.State.InstDone)
				}
			}
		})
	}
}

// A fused branch resolves when its pair issues, which is when its producer
// issues: a cold misprediction behind a late producer restarts fetch a branch
// penalty after the producer's issue, and both halves complete together.
// Contention for the pipes delays the whole pair.
func TestFusedBranchResolvesWithItsProducer(t *testing.T) {
	for _, form := range []struct {
		name  string
		words []uint32
	}{{"subs, b.ne", []uint32{subsX0, bneMinus4}}, {"sub, cbnz", []uint32{subX0, cbnzX0}}} {
		for _, tc := range []struct {
			name       string
			ready      uint64              // cycle at which X0 is ready
			busy       [][]UnitReservation // pipe bookings before the pair
			issue      uint64
			unitsAfter [][]UnitReservation
		}{
			{"delayed producer", 100, [][]UnitReservation{nil, nil}, 100,
				[][]UnitReservation{{{100, 101}}, {{100, 101}}}},
			{"contended pipes", 1, [][]UnitReservation{{{1, 2}}, {{2, 3}}}, 3,
				[][]UnitReservation{{{1, 2}, {3, 4}}, {{2, 3}, {3, 4}}}},
		} {
			t.Run(form.name+"/"+tc.name, func(t *testing.T) {
				p := specPipeline(t, form.words, nil, func(s *Spec) {
					s.NumINT = 2
					s.CompareBranchFusion = true
					s.FlagBranchDependency = true
				})
				p.comp.Resources.Emu.Regs.WriteX(0, 2) // the backedge is taken; a cold predictor misses it
				p.comp.State.GprReady[0] = tc.ready
				copy(p.comp.State.IntUnits, tc.busy)
				p.fetchAndProcess()
				state := p.comp.State
				if state.Err != "" || state.RobCount != 2 || !state.RobFused[1] {
					t.Fatalf("err=%q count=%d fused=%v", state.Err, state.RobCount, state.RobFused[:2])
				}
				if state.Rob[0] != tc.issue+1 || state.Rob[1] != tc.issue+1 {
					t.Fatalf("producer completes %d, branch %d, want both %d", state.Rob[0], state.Rob[1], tc.issue+1)
				}
				if want := tc.issue + uint64(p.comp.Spec.BranchPenalty); state.FrontendReady != want {
					t.Fatalf("fetch restarts at %d, want %d", state.FrontendReady, want)
				}
				if !reflect.DeepEqual(state.IntUnits, tc.unitsAfter) {
					t.Fatalf("reservations %+v, want %+v", state.IntUnits, tc.unitsAfter)
				}
			})
		}
	}
}

// A compare in the last slot of a fetch group cannot fuse with the select the
// next group delivers: the compare does not wait for the select's operands or
// take the fusion pipe, and the select runs as an ordinary instruction.
func TestSelectPairSplitByFetchGroupRunsUnfused(t *testing.T) {
	words := make([]uint32, 18)
	for i := range words {
		words[i] = nop
	}
	words[1] = bPlus60     // 0x1004: B to 0x1040, inside a group that starts at 0x1004
	words[16] = cmpW2Imm   // 0x1040: that group's last slot
	words[17] = cselW2W6W2 // 0x1044: the next group, in the same 64-byte line
	for _, tc := range []struct {
		name    string
		start   uint64 // first fetch group
		compare uint64 // completion cycles
		choose  uint64
		fusion  []UnitReservation // bookings on the fusion pipe
	}{
		{"split", 0x1004, 2, 101, nil},
		// From 0x1000 the branch target lies beyond the first group, so
		// the next group starts at the compare and holds both.
		{"whole", 0x1000, 101, 101, []UnitReservation{{100, 101}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := specPipeline(t, words, nil, func(s *Spec) {
				s.FetchWidth, s.FetchGroupLimit, s.FetchLead = 10, 16, 2
				s.NumINT = 3 // the third is the select-fusion pipe
			})
			predictor := p.predictor()
			for i := 0; i < 4; i++ {
				predictor.Update(0x1004, true, 0x1040, false, false, false)
			}
			if taken, target := predictor.Predict(0x1004, false, false); !taken || target != 0x1040 {
				t.Fatalf("branch not trained: %v %#x", taken, target)
			}
			p.comp.Resources.Emu.Regs.PC = tc.start
			p.comp.State.GprReady[6] = 100
			p.comp.Resources.Emu.SetHaltPC(0x1048)
			for i := 0; i < 4 && !p.comp.Resources.Emu.Stopped(); i++ {
				p.fetchAndProcess()
				p.comp.State.Cycles++
			}
			state := p.comp.State
			last := state.RobCount - 1
			if state.Err != "" || !p.comp.Resources.Emu.Stopped() || state.FrontendReady > 2 {
				t.Fatalf("err=%q stopped=%v frontend=%d", state.Err, p.comp.Resources.Emu.Stopped(), state.FrontendReady)
			}
			if state.Rob[last-1] != tc.compare || state.Rob[last] != tc.choose || state.GprReady[2] != tc.choose {
				t.Fatalf("compare completes %d, select %d (W2 %d), want %d and %d",
					state.Rob[last-1], state.Rob[last], state.GprReady[2], tc.compare, tc.choose)
			}
			if !reflect.DeepEqual(state.IntUnits[2], tc.fusion) {
				t.Fatalf("fusion pipe bookings %+v, want %+v", state.IntUnits[2], tc.fusion)
			}
		})
	}
}

// A compare/branch pair split by a fetch group, or without two ROB entries
// left, runs as two ordinary instructions: the branch takes its own rename
// slot and pipe and is not marked fused.
func TestUndeliverableBranchPairRunsUnfused(t *testing.T) {
	split := make([]uint32, 18)
	for i := range split {
		split[i] = nop
	}
	split[16], split[17] = subsX0, bneMinus4 // 0x1040 ends the group that starts at 0x1004
	for _, tc := range []struct {
		name   string
		words  []uint32
		start  uint64
		adjust func(*Spec)
	}{
		{"fetch group", split, 0x1004, func(s *Spec) { s.FetchGroupLimit = 16 }},
		{"one ROB entry", []uint32{nop, subsX0, bneMinus4}, 0x1000, func(s *Spec) { s.ROBCapacity = 2 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := specPipeline(t, tc.words, nil, func(s *Spec) {
				s.FetchWidth, s.NumINT = 10, 2
				s.CompareBranchFusion = true
				tc.adjust(s)
			})
			p.comp.Resources.Emu.Regs.PC = tc.start
			p.comp.Resources.Emu.Regs.WriteX(0, 1) // the branch falls through
			p.comp.Resources.Emu.SetHaltPC(0x1000 + 4*uint64(len(tc.words)))
			for i := 0; i < 8 && !p.comp.Resources.Emu.Stopped(); i++ {
				p.fetchAndProcess()
				p.comp.State.Cycles++
				p.retire()
			}
			state := p.comp.State
			if state.Err != "" || !p.comp.Resources.Emu.Stopped() {
				t.Fatalf("err=%q stopped=%v", state.Err, p.comp.Resources.Emu.Stopped())
			}
			for i, fused := range state.RobFused {
				if fused {
					t.Fatalf("ROB entry %d marked fused", i)
				}
			}
			if booked := len(state.IntUnits[0]) + len(state.IntUnits[1]); booked != 2 {
				t.Fatalf("%d integer bookings, want one each for the compare and the branch", booked)
			}
		})
	}
}

// At most ConditionalBranches conditional branches are renamed per cycle.
// Prediction is off so that no cold misprediction stalls fetch.
func TestConditionalBranchBudget(t *testing.T) {
	const cbnzX17 = 0xb5000051 // CBNZ X17, .+8 (X17 = 0: falls through)
	p := specPipeline(t, []uint32{cbnzX17, cbnzX17, cbnzX17, cbnzX17, cbnzX17}, nil, func(s *Spec) {
		s.FetchWidth, s.NumINT = 10, 4
		s.ConditionalBranches = 2
		s.BranchPrediction = false
	})
	p.comp.Resources.Emu.SetHaltPC(0x1014)
	if got := admitted(p, 3); !reflect.DeepEqual(got, []int{2, 2, 1}) {
		t.Fatalf("admitted %v per cycle, want [2 2 1]", got)
	}
}

// The measured pairs fuse; the shifted-register compare and other branch
// kinds do not. Other widths, immediates and ADD/CBZ forms are the model's
// extension of the measured pairs.
func TestCompareBranchFusionEligibility(t *testing.T) {
	for _, tc := range []struct {
		name          string
		compare, next uint32
		want          bool
	}{
		{"measured: subs x imm, b.ne", subsX0, bneMinus4, true},
		{"measured: cmp x imm, b.ne", cmpX0Zero, bneMinus4, true},
		{"measured: cmp x reg, b.ne", 0xeb11001f, bneMinus4, true}, // CMP X0, X17
		{"measured: cmp w reg, b.ne", 0x6b11001f, bneMinus4, true}, // CMP W0, W17
		{"measured: tst x reg, b.ne", 0xea00001f, bneMinus4, true}, // TST X0, X0
		{"measured: cmp x imm, b.gt", cmpX0Zero, 0x54ffffec, true}, // B.GT .-4
		{"measured: sub x imm, cbnz x", subX0, cbnzX0, true},
		{"measured: shifted register", 0xeb00063f, bneMinus4, false},  // CMP X17, X0, LSL #1
		{"extension: cmp w imm", 0x7100001f, bneMinus4, true},         // CMP W0, #0
		{"extension: tst w imm", 0x7200001f, bneMinus4, true},         // TST W0, #1
		{"extension: shifted immediate", 0xf140041f, bneMinus4, true}, // CMP X0, #1, LSL #12
		{"extension: add, cbz", 0x91000400, 0xb4ffffe0, true},         // ADD X0, X0, #1; CBZ X0
		{"extension: sub w, cbnz x", 0x51000400, cbnzX0, true},        // SUB W0, W0, #1
		{"cbnz of another register", subX0, 0xb5ffffe1, false},        // CBNZ X1
		{"sub of a register", 0xcb010000, cbnzX0, false},              // SUB X0, X0, X1
		{"tbnz", subX0, 0x3707ffe0, false},                            // TBNZ X0, #0
		{"unconditional branch", cmpX0Zero, bMinus8, false},
		{"not a producer", addX9X9, bneMinus4, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := fusesWithBranch(decodeWord(t, tc.compare), decodeWord(t, tc.next)); got != tc.want {
				t.Fatalf("fuses=%v, want %v", got, tc.want)
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
		func(s *Spec) { s.FetchRepeatMin = -1 },
		func(s *Spec) { s.FetchRepeatExitDelay = -1 },
		func(s *Spec) { s.FetchGroupLimit, s.FetchRepeatMin = 16, 1 },
		func(s *Spec) { s.FetchGroupLimit, s.FetchRepeatExitDelay = 16, 4 },
		func(s *Spec) { s.FetchRepeatMin, s.FetchRepeatExitDelay = 6, 4 },
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

// An ordinary branch resolves on a pipe after its flags arrive. Correct
// predictions can still deliver the following instructions in the same cycle.
func TestUnfusedFlagBranchResolution(t *testing.T) {
	for _, tc := range []struct {
		name string
		x0   uint64
		miss bool
	}{{"cold taken", 2, true}, {"correct fallthrough", 1, false}} {
		t.Run(tc.name, func(t *testing.T) {
			p := specPipeline(t, []uint32{subsX0, nop, bneMinus8, nop}, nil, func(s *Spec) {
				s.FlagBranchDependency = true
				s.NumINT = 1
				s.FetchWidth = 4
			})
			p.comp.Resources.Emu.Regs.WriteX(0, tc.x0)
			s := &p.comp.State
			s.GprReady[0] = 100
			s.IntUnits[0] = []UnitReservation{{101, 104}}
			bp := p.predictor()
			bp.Bimodal[bp.bimodalIndex(0x1008)] = 0
			p.fetchAndProcess()
			if s.Rob[0] != 101 || s.Rob[2] != 105 {
				t.Fatalf("producer completed at %d, branch at %d", s.Rob[0], s.Rob[2])
			}
			if tc.miss {
				if s.RobCount != 3 || s.FrontendReady != 104+uint64(p.comp.Spec.BranchPenalty) {
					t.Fatalf("branch miss dispatched %d, fetch restarts at %d", s.RobCount, s.FrontendReady)
				}
			} else if s.RobCount != 4 || s.Rob[3] >= 100 {
				t.Fatalf("correctly predicted fallthrough dispatched %d, completed at %d", s.RobCount, s.Rob[3])
			}
		})
	}
}

func TestRepeatedFetchGroupExit(t *testing.T) {
	for _, tc := range []struct {
		name      string
		count     int
		end, next uint64
		delay     uint64
		warm      int
	}{
		{"cold exit", 5, 0x1030, 0x1040, 0, 0},
		{"saturated self transfer", 12, 0x101c, 0x1000, 0, 6},
		{"different successor", 6, 0x101c, 0x1040, 4, 0},
		{"different terminator same target", 6, 0x1030, 0x1000, 4, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := specPipeline(t, []uint32{nop}, nil, func(s *Spec) {
				s.FetchGroupLimit, s.FetchRepeatMin, s.FetchRepeatExitDelay = 16, 6, 4
			})
			s := &p.comp.State
			s.FetchGroupStart, s.FetchGroupCycle, s.Cycles = 0x1000, 10, 30
			for i := 0; i < tc.count; i++ {
				p.finishRepeatedGroup(0x101c, 0x1000)
			}
			p.finishRepeatedGroup(tc.end, tc.next)
			if s.FetchGroupCycle != 10+tc.delay || s.FrontendReady != 0 || s.FetchRepeatCount != tc.warm {
				t.Fatalf("got fetch cycle %d, rename gate %d, repetition count %d", s.FetchGroupCycle, s.FrontendReady, s.FetchRepeatCount)
			}
			p.startFetchGroup(tc.next)
			if s.FetchGroupCycle != 11+tc.delay {
				t.Fatalf("buffered next fetch available at %d", s.FetchGroupCycle)
			}
		})
	}
}

func TestRepeatedFetchGroupCheckpointAndRedirect(t *testing.T) {
	p := specPipeline(t, []uint32{nop}, nil, func(s *Spec) {
		s.FetchGroupLimit, s.FetchRepeatMin, s.FetchRepeatExitDelay = 16, 2, 4
	})
	s := &p.comp.State
	s.FetchGroupStart = 0x1000
	p.finishRepeatedGroup(0x100c, 0x1000)
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var restored State
	if err := json.Unmarshal(b, &restored); err != nil {
		t.Fatal(err)
	}
	*s = restored
	p.finishRepeatedGroup(0x100c, 0x1000)
	if s.FetchRepeatCount != 2 || s.FetchRepeatStart != 0x1000 || s.FetchRepeatEnd != 0x100c {
		t.Fatal("checkpoint failed to preserve the partially warmed group")
	}
	p.redirect(100)
	p.finishRepeatedGroup(0x1010, 0x1008)
	if s.FetchGroupCycle != 99 || s.FetchRepeatCount != 0 {
		t.Fatalf("misprediction recovery retained repeat delay: %+v", s)
	}
	p.finishRepeatedGroup(0x100c, 0x1000)
	s.Initialize(p.comp)
	if s.FetchRepeatStart != 0 || s.FetchRepeatEnd != 0 || s.FetchRepeatCount != 0 {
		t.Fatal("initialization retained a repeated group")
	}
}
