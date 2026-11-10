package ooo

import (
	"encoding/binary"
	"reflect"
	"testing"

	"golang.org/x/arch/arm64/arm64asm"
)

const (
	cmpW2W6    = 0x6b06005f // CMP W2, W6
	cselW2W2W6 = 0x1a86a042 // CSEL W2, W2, W6, GE
	cmpW3W7    = 0x6b07007f // CMP W3, W7
	cselW3W3W7 = 0x1a87a063 // CSEL W3, W3, W7, GE
	cmpW4W8    = 0x6b08009f // CMP W4, W8
	cselW4W4W8 = 0x1a88a084 // CSEL W4, W4, W8, GE
	csetW9     = 0x1a9fb7e9 // CSET W9, GE
	nop        = 0xd503201f
	addX9      = 0x91000529 // ADD X9, X9, #1
)

func decodeWord(t *testing.T, word uint32) arm64asm.Inst {
	t.Helper()
	var raw [4]byte
	binary.LittleEndian.PutUint32(raw[:], word)
	inst, err := arm64asm.Decode(raw[:])
	if err != nil {
		t.Fatal(err)
	}
	return inst
}

// The measured forms fuse: a 32-bit compare of two registers, or of a register
// and a small unshifted immediate, with a 32-bit select, reading at most two
// different registers between them.
func TestCompareSelectFusionEligibility(t *testing.T) {
	const cmpW2Imm15, cselW2W6W2 = 0x71003c5f, 0x1a8230c2 // CMP W2,#15; CSEL W2,W6,W2,LO
	for _, tc := range []struct {
		name            string
		compare, choose uint32
		want            bool
	}{
		{"select of the compared registers", cmpW2W6, cselW2W2W6, true},
		{"destination is the second compared register", 0x6b0300df, 0x1a83a0c3, true}, // CMP W6,W3; CSEL W3,W6,W3,GE
		{"destination is a third register", cmpW2W6, 0x1a86a043, true},                // CSEL W3,W2,W6,GE
		{"swapped select operands", cmpW2W6, 0x1a82b0c2, true},                        // CSEL W2,W6,W2,LT
		{"immediate compare, select uses the compared register", cmpW2Imm15, cselW2W6W2, true},
		{"immediate zero", 0x7100005f, cselW2W6W2, true},                       // CMP W2,#0
		{"unrelated select operands", cmpW2W6, 0x1a853082, false},              // CSEL W2,W4,W5,LO
		{"one unrelated select operand", cmpW2W6, 0x1a85a042, false},           // CSEL W2,W2,W5,GE
		{"immediate compare, unrelated select", cmpW2Imm15, 0x1a8530c2, false}, // CSEL W2,W6,W5,LO
		{"immediate above the limit", 0x7100405f, cselW2W6W2, false},           // CMP W2,#16
		{"shifted immediate", 0x7140045f, cselW2W6W2, false},                   // CMP W2,#1,LSL #12
		{"shifted register compare", 0x6b06045f, cselW2W2W6, false},            // CMP W2,W6,LSL #1
		{"extended register compare", 0x6b26005f, cselW2W2W6, false},           // CMP W2,W6,UXTB
		{"stack pointer compare", 0x71003fff, cselW2W6W2, false},               // CMP WSP,#15
		{"zero register operand", 0x6b1f005f, 0x1a9fa042, false},               // CMP W2,WZR; CSEL W2,W2,WZR,GE
		{"compare negative", 0x2b06005f, cselW2W2W6, false},                    // CMN W2,W6
		{"conditional increment", cmpW2W6, 0x1a86a442, false},                  // CSINC W2,W2,W6,GE
		{"64-bit pair", 0xeb06005f, 0x9a86a042, false},                         // CMP X2,X6; CSEL X2,X2,X6,GE
		{"64-bit select after 32-bit compare", cmpW2W6, 0x9a86a042, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := compareSelectFuses(decodeWord(t, tc.compare), decodeWord(t, tc.choose)); got != tc.want {
				t.Fatalf("fuses=%v, want %v", got, tc.want)
			}
		})
	}
}

// An immediate pair reads the compared register and one other; it issues when
// both are ready, not when the compare alone could.
func TestFusedPairWaitsForEveryRegisterItReads(t *testing.T) {
	p := specPipeline(t, []uint32{0x71003c5f, 0x1a8230c2}, nil, nil) // CMP W2,#15; CSEL W2,W6,W2,LO
	p.comp.State.GprReady[6] = 10
	p.fetchAndProcess()
	state := p.comp.State
	if state.Err != "" || state.Rob[0] != 11 || state.Rob[1] != 11 || state.FlagsReady != 11 || state.GprReady[2] != 11 {
		t.Fatalf("err=%q compare=%d select=%d flags=%d register=%d", state.Err, state.Rob[0], state.Rob[1], state.FlagsReady, state.GprReady[2])
	}
	if want := []UnitReservation{{10, 11}}; !reflect.DeepEqual(state.IntUnits[0], want) {
		t.Fatalf("reservations %+v", state.IntUnits[0])
	}
}

// The pair issues once when both compared registers are ready and delivers
// the flags and the selected register one cycle later. Without fusion pipes
// the select waits a further cycle for the flags and reserves its own slot.
func TestFusedPairIssuesOnceWithOneCycleLatency(t *testing.T) {
	for _, tc := range []struct {
		name         string
		pipes        int
		selectReady  uint64
		reservations []UnitReservation
	}{
		{"fused", 1, 11, []UnitReservation{{10, 11}}},
		{"separate", 0, 12, []UnitReservation{{10, 11}, {11, 12}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := specPipeline(t, []uint32{cmpW2W6, cselW2W2W6, csetW9}, nil, func(s *Spec) { s.NumSelectFusion = tc.pipes })
			p.comp.Resources.Emu.Regs.WriteX(2, 7)
			p.comp.Resources.Emu.Regs.WriteX(6, 9)
			p.comp.State.GprReady[2] = 10
			p.fetchAndProcess()
			if p.comp.State.Err != "" {
				t.Fatal(p.comp.State.Err)
			}
			state := p.comp.State
			if state.Rob[0] != 11 || state.Rob[1] != tc.selectReady || state.FlagsReady != 11 || state.GprReady[2] != tc.selectReady {
				t.Fatalf("compare=%d select=%d flags=%d register=%d", state.Rob[0], state.Rob[1], state.FlagsReady, state.GprReady[2])
			}
			// A later flag reader is an ordinary operation fed by the pair's flags.
			later := append(append([]UnitReservation{}, tc.reservations...), UnitReservation{tc.selectReady, tc.selectReady + 1})
			if !reflect.DeepEqual(state.IntUnits[0], later) || state.Rob[2] != tc.selectReady+1 {
				t.Fatalf("reservations %+v, flag reader ready %d", state.IntUnits[0], state.Rob[2])
			}
			if state.RobCount != 3 || p.comp.Resources.Emu.Regs.ReadX(2) != 9 || p.comp.Resources.Emu.Regs.ReadX(9) != 0 {
				t.Fatalf("both instructions must execute and occupy the ROB: count=%d w2=%d w9=%d", state.RobCount,
					p.comp.Resources.Emu.Regs.ReadX(2), p.comp.Resources.Emu.Regs.ReadX(9))
			}
		})
	}
}

// Adjacency within one 64-byte line is required: an instruction between the
// two, or a line boundary between them, leaves two separate operations.
func TestSeparatedOrStraddlingPairDoesNotFuse(t *testing.T) {
	straddle := make([]uint32, 15, 17)
	for i := range straddle {
		straddle[i] = nop
	}
	for name, words := range map[string][]uint32{
		"separated": {cmpW2W6, nop, cselW2W2W6},
		"straddle":  append(straddle, cmpW2W6, cselW2W2W6), // compare at byte 60 of the line
	} {
		t.Run(name, func(t *testing.T) {
			p := specPipeline(t, words, nil, nil)
			p.comp.State.GprReady[6] = 10
			p.fetchAndProcess()
			state := p.comp.State
			last := len(words) - 1
			if state.Err != "" || state.GprReady[2] != 12 || state.Rob[last] != 12 || len(state.IntUnits[0]) != 2 {
				t.Fatalf("err=%q select ready %d, reservations %+v", state.Err, state.GprReady[2], state.IntUnits[0])
			}
		})
	}
	aligned := specPipeline(t, append(append([]uint32{}, straddle[:14]...), cmpW2W6, cselW2W2W6, nop), nil, nil)
	aligned.comp.State.GprReady[6] = 10
	aligned.fetchAndProcess()
	if aligned.comp.State.GprReady[2] != 11 {
		t.Fatalf("pair ending at the line boundary must still fuse: ready %d", aligned.comp.State.GprReady[2])
	}
}

// A pair is admitted whole. With one fetch slot or one ROB entry left the
// compare waits for the next cycle rather than issuing unfused.
func TestFusedPairIsAdmittedWhole(t *testing.T) {
	width := specPipeline(t, []uint32{nop, cmpW2W6, cselW2W2W6}, nil, func(s *Spec) { s.FetchWidth = 2 })
	width.fetchAndProcess()
	if width.comp.State.RobCount != 1 {
		t.Fatalf("compare admitted without its select: %d entries", width.comp.State.RobCount)
	}
	width.comp.State.Cycles++
	width.fetchAndProcess()
	if state := width.comp.State; state.RobCount != 3 || state.Rob[1] != 3 || state.Rob[2] != 3 {
		t.Fatalf("pair not admitted together: count=%d compare=%d select=%d", state.RobCount, state.Rob[1], state.Rob[2])
	}

	rob := specPipeline(t, []uint32{nop, cmpW2W6, cselW2W2W6}, nil, func(s *Spec) { s.ROBCapacity = 2 })
	rob.fetchAndProcess()
	if rob.comp.State.RobCount != 1 {
		t.Fatalf("compare admitted into the last ROB entry: %d entries", rob.comp.State.RobCount)
	}
	rob.comp.State.Cycles = 2
	rob.retire()
	rob.fetchAndProcess()
	if state := rob.comp.State; state.RobCount != 2 || state.GprReady[2] != 3 {
		t.Fatalf("pair did not follow once two entries were free: count=%d ready=%d", state.RobCount, state.GprReady[2])
	}
}

// A configuration that can never hold a pair in one cycle runs unfused
// instead of waiting forever, and both instructions still retire.
func TestPairThatCanNeverFitRunsUnfused(t *testing.T) {
	for name, adjust := range map[string]func(*Spec){
		"one fetch slot": func(s *Spec) { s.FetchWidth = 1 },
		"one ROB entry":  func(s *Spec) { s.ROBCapacity = 1 },
	} {
		t.Run(name, func(t *testing.T) {
			p := specPipeline(t, []uint32{cmpW2W6, cselW2W2W6}, nil, adjust)
			p.comp.Resources.Emu.Regs.WriteX(6, 9)
			p.comp.Resources.Emu.SetHaltPC(0x1008)
			for i := 0; i < 100 && p.Handle(nil); i++ {
			}
			if state := p.comp.State; !state.Done || state.Err != "" || state.InstDone != 2 || p.comp.Resources.Emu.Regs.ReadX(2) != 9 {
				t.Fatalf("done=%v err=%q retired=%d", state.Done, state.Err, state.InstDone)
			}
		})
	}
}

// Fused pairs issue only on the fusion pipes, one per pipe per cycle, while
// ordinary integer work fills the other pipes first and shares the fusion
// pipe only when they are busy.
func TestFusionPipesBoundPairThroughputAndAreSharedLast(t *testing.T) {
	pairs := []uint32{cmpW2W6, cselW2W2W6, cmpW3W7, cselW3W3W7, cmpW4W8, cselW4W4W8}
	for pipes, want := range map[int][]uint64{1: {2, 2, 3, 3, 4, 4}, 2: {2, 2, 2, 2, 3, 3}} {
		p := specPipeline(t, pairs, nil, func(s *Spec) { s.NumINT, s.NumSelectFusion = 6, pipes })
		p.fetchAndProcess()
		if got := p.comp.State.Rob[:6]; !reflect.DeepEqual(got, want) {
			t.Fatalf("%d fusion pipes: completions %v, want %v", pipes, got, want)
		}
		for unit := 0; unit < 6-pipes; unit++ {
			if len(p.comp.State.IntUnits[unit]) != 0 {
				t.Fatalf("fused pair issued on ordinary pipe %d", unit)
			}
		}
	}
	// Five independent adds leave the last pipe free for the pair; a sixth
	// takes it first and delays the pair by one cycle.
	adds := []uint32{addX9, 0x9100054a, 0x9100056b, 0x9100058c, 0x910005ad, 0x910005ce}
	for count, ready := range map[int]uint64{5: 2, 6: 3} {
		words := append(append([]uint32{}, adds[:count]...), cmpW2W6, cselW2W2W6)
		p := specPipeline(t, words, nil, func(s *Spec) { s.NumINT = 6 })
		p.fetchAndProcess()
		if got := p.comp.State.GprReady[2]; got != ready {
			t.Fatalf("%d adds before the pair: ready %d, want %d", count, got, ready)
		}
	}
}
