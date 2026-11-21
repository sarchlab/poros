package ooo

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/sarchlab/poros/pkg/emu"
)

func TestConflictStreakBelongsToOneSourceRead(t *testing.T) {
	for _, tc := range []struct {
		name     string
		mixed    bool
		wantHold bool
	}{{"single", false, true}, {"alternating", true, false}} {
		t.Run(tc.name, func(t *testing.T) {
			const addr = 0x4000
			core := &Comp{}
			s := &core.State
			s.FPReload.TwoRecentLines = true
			s.StoreBuffer = []storeBufferEntry{{Addr: addr, Size: 8, ScalarStorePosition: 1, ReadyCycle: 100}}
			p := &middleware{comp: core}
			for i := range 32 {
				p.partialReloadWait(0x2000, emu.MemAccess{Addr: addr, Size: 8}, 10)
				pc := uint64(0x1000)
				if tc.mixed && i%2 == 1 {
					pc += 4
				}
				p.partialReloadWait(pc, emu.MemAccess{Addr: addr + 128, Size: 8}, 10)
			}
			// The following pending reload consumes the last side-match observation.
			// A pooled mixed-PC streak is active only until that observation retires.
			p.partialReloadWait(0x2000, emu.MemAccess{Addr: addr, Size: 8}, 10)
			if s.FPReload.TransientThrough != s.InstDone+uint64(s.RobCount)+1 {
				t.Fatal("missing retirement-bound pooled conflict")
			}
			if got := s.FPReload.HoldThrough > 0; got != tc.wantHold {
				t.Fatalf("conservative window active = %t, want %t", got, tc.wantHold)
			}
		})
	}
}

func TestSourceSetConflictScope(t *testing.T) {
	for _, tc := range []struct {
		name                                        string
		late, changeChain, retireFirst, differentPC bool
		want                                        bool
	}{
		{name: "late recurring reads", late: true, want: true},
		{name: "ordinary hits"},
		{name: "different recurrence", late: true, changeChain: true},
		{name: "first read retired", late: true, retireFirst: true},
		{name: "different source instruction", late: true, differentPC: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const pc = 0x4200
			s := State{}
			s.FPReload.LastFeedback = 200
			s.FPReload.FeedbackAddress = 0x9000
			read := func(source, a uint64, late bool) {
				s.observeSourceSet(source, a, 256, 64, late)
				s.RobCount++
			}
			read(pc, 0x10000, tc.late)
			read(pc, 0x10040, false)
			if tc.changeChain {
				s.FPReload.FeedbackAddress += 8
			}
			if tc.retireFirst {
				s.InstDone++
				s.RobCount--
			}
			read(pc, 0x14000, false)
			source := uint64(pc)
			if tc.differentPC {
				source += 4
			}
			read(source, 0x14040, false)
			if got := s.FPReload.CompetingReadThrough != 0; got != tc.want {
				t.Fatalf("source conflict active = %t, want %t", got, tc.want)
			}
			if tc.want && s.FPReload.CompetingFeedbackAddress != s.FPReload.FeedbackAddress {
				t.Fatal("trained conflict attached to a different recurrence")
			}
		})
	}
}

func TestStableSublineInterferenceUsesYoungestStoreData(t *testing.T) {
	for _, tc := range []struct {
		name    string
		offsets [3]uint64
		want    uint64
	}{
		{"stable same sector", [3]uint64{8, 8, 8}, 100},
		{"stable opposite sector", [3]uint64{64, 64, 64}, 10},
		{"varying offsets", [3]uint64{8, 16, 24}, 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const addr = 0x4000
			core := &Comp{}
			s := &core.State
			s.FPReload.LateReload = observedLateReload{Addr: addr, Ready: 50, Valid: true}
			s.StoreBuffer = []storeBufferEntry{{Addr: addr, Size: 8, ScalarStorePosition: 7, ReadyCycle: 100}}
			p := &middleware{comp: core}
			var issue uint64
			for _, off := range tc.offsets {
				issue, _ = p.partialReloadWait(0x1000, emu.MemAccess{Addr: addr + 128 + off, Size: 8}, 10)
			}
			if issue != tc.want {
				t.Fatalf("issue %d, want %d", issue, tc.want)
			}
		})
	}
}

func TestSourcePredictionRestoresInFlightObservation(t *testing.T) {
	const pc = 0x4200
	state := State{}
	state.FPReload.LastFeedback = 100
	state.FPReload.FeedbackAddress = 0x9000
	state.observeSourceSet(pc, 0x10000, 256, 64, true)
	state.RobCount++
	state.observeSourceSet(pc, 0x10040, 256, 64, false)
	state.RobCount++
	state.observeSourceSet(pc, 0x14000, 256, 64, false)
	state.RobCount++
	payload, err := json.Marshal(&state)
	if err != nil {
		t.Fatal(err)
	}
	var restored State
	if err := json.Unmarshal(payload, &restored); err != nil {
		t.Fatal(err)
	}
	for _, s := range []*State{&state, &restored} {
		s.observeSourceSet(pc, 0x14040, 256, 64, false)
		if s.FPReload.CompetingReadThrough != 4 || s.FPReload.CompetingFeedbackAddress != 0x9000 {
			t.Fatalf("lost in-flight source evidence: position %d address %#x", s.FPReload.CompetingReadThrough, s.FPReload.CompetingFeedbackAddress)
		}
	}
}

// Use the dispatch path so the origin must pass through the floating-point
// registers and the live store buffer before the final reload can use it.
func TestScalarFPFeedbackThroughDispatch(t *testing.T) {
	run := func(enabled bool, gap int) uint64 {
		words := []uint32{ldrD0X1, 0x1e612800, strD0X1} // sum = *ptr + D1; *ptr = sum
		for range gap {
			words = append(words, nop)
		}
		words = append(words, ldrD2X1)
		p := specPipeline(t, words, nil, func(s *Spec) { s.ScalarFPReloadPrediction = enabled })
		state := &p.comp.State
		p.comp.Resources.Emu.Regs.WriteX(1, 0x2000)
		p.comp.Resources.Emu.Regs.WriteD(1, 3)
		state.FpReady[1] = 100
		p.fetchAndProcess()
		if state.Err != "" || state.RobCount != len(words) || p.comp.Resources.Emu.Regs.ReadD(2) != 3 {
			t.Fatalf("admitted %d/%d, D2=%v, err=%s", state.RobCount, len(words), p.comp.Resources.Emu.Regs.ReadD(2), state.Err)
		}
		if !enabled && !reflect.DeepEqual(state.FPReload, fpReloadState{}) {
			t.Fatal("disabled profile trained the FP reload predictor")
		}
		return state.FpReady[2]
	}
	if on, off := run(true, 8), run(false, 8); on >= off {
		t.Fatalf("eligible feedback reload ready with prediction at %d, without at %d", on, off)
	}
	if on, off := run(true, 7), run(false, 7); on != off {
		t.Fatalf("short forwarding distance changed timing: %d vs %d", on, off)
	}
}

func TestOverlappingFPRefillsTrainAndExpire(t *testing.T) {
	for _, tc := range []struct {
		name        string
		secondIssue uint64
		wantHold    bool
	}{
		{"overlapping intervals", 9, true},
		{"touching intervals", 10, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := State{RobCount: 12}
			state.FPReload.LastFeedback = 12
			// Two PCs read streams 16 KiB apart, eight bytes at a time.
			// Two reads establish the stride, then two matching refills train.
			for i := uint64(0); i < 4; i++ {
				at, offset := 20*i, i*8
				state.noteFPRefill(0x1000, 0x20000+offset, at, at+10, true)
				state.noteFPRefill(0x1004, 0x24000+offset, at+tc.secondIssue, at+20, true)
				if i < 3 && state.conservativeReloadActive() {
					t.Fatal("single matching refill opened the conservative window")
				}
			}
			if got := state.conservativeReloadActive(); got != tc.wantHold {
				t.Fatalf("active after training = %t, want %t", got, tc.wantHold)
			}
			if tc.wantHold {
				state.RobCount = 0
				state.InstDone = 12 + conservativeReloadWindow - 1
				if !state.conservativeReloadActive() {
					t.Fatal("window expired before its final position")
				}
				state.InstDone++
				if state.conservativeReloadActive() {
					t.Fatal("window survived its expiry position")
				}
			}
		})
	}
}

func TestNewerReadyProducerUsesYoungestOverlap(t *testing.T) {
	const loadPC, predictedPC, addr, issue = 0x1000, 0x2000, 0x4000, 10
	load := emu.MemAccess{Addr: addr, Size: 8}
	predicted := storeBufferEntry{PC: predictedPC, Addr: addr + 128, Size: 8, CompleteCycle: 30}
	ready := storeBufferEntry{PC: 0x2004, Addr: addr, Size: 8, ReadyCycle: issue}
	partial := storeBufferEntry{PC: 0x2008, Addr: addr + 4, Size: 4, ReadyCycle: issue}
	pending := storeBufferEntry{PC: 0x200c, Addr: addr, Size: 8, ReadyCycle: issue + 1}
	for _, tc := range []struct {
		name   string
		stores []storeBufferEntry
		want   bool
	}{
		{"newer exact ready store", []storeBufferEntry{predicted, ready}, true},
		{"exact store precedes prediction", []storeBufferEntry{ready, predicted}, false},
		{"younger partial overlap", []storeBufferEntry{predicted, ready, partial}, false},
		{"younger data still pending", []storeBufferEntry{predicted, ready, pending}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &middleware{comp: &Comp{}}
			state := &p.comp.State
			state.StoreBuffer = tc.stores
			state.StoreDependence.LoadStore = map[uint64]uint64{loadPC: predictedPC}
			p.recordStoreInstance(predictedPC, []emu.MemAccess{{Addr: predicted.Addr, Size: predicted.Size}}, predicted.CompleteCycle)
			if got := p.newerReadyProducer(loadPC, load, issue); got != tc.want {
				t.Fatalf("bypass predicted wait = %t, want %t", got, tc.want)
			}
		})
	}
}
