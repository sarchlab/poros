package ooo

import (
	"encoding/json"
	"testing"

	"github.com/sarchlab/poros/pkg/emu"
)

func TestActiveReloadLines(t *testing.T) {
	s := State{Rob: make([]uint64, 4)}
	if s.observeActiveLine(10) {
		t.Fatal("first read counted as reuse")
	}
	s.RobCount, s.RobTail = 1, 1
	if s.observeActiveLine(20) {
		t.Fatal("a different line counted as reuse")
	}
	s.RobCount, s.RobTail = 2, 2
	if !s.observeActiveLine(10) {
		t.Fatal("an intervening request hid an active read")
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var restored State
	if err = json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	for _, state := range []*State{&s, &restored} {
		state.InstDone, state.RobCount, state.RobTail = 2, 2, 0
		if state.observeActiveLine(30) {
			t.Fatal("unseen line counted as reuse")
		}
		state.InstDone, state.RobCount, state.RobTail = 2, 3, 1
		if !state.observeActiveLine(10) {
			t.Fatal("recycling an older ROB slot discarded a younger request for the same line")
		}
		state.InstDone, state.RobCount, state.RobTail = 6, 0, 2
		if state.observeActiveLine(10) {
			t.Fatal("a retired request counted as live")
		}
	}
}

func TestPageAliasExpiresWhenTrainingLoadRetires(t *testing.T) {
	const addr = 0x4000
	core := &Comp{}
	state := &core.State
	state.InstDone, state.RobCount = 10, 50
	state.FPReload.TwoRecentLines = true
	state.FPReload.LastFeedback = 15
	state.FPReload.FeedbackAddress = addr
	state.FPReload.LateReload = observedLateReload{Addr: addr, Ready: 90, Valid: true}
	pipe := &middleware{comp: core}
	pipe.partialReloadWait(0x1000, emu.MemAccess{Addr: addr + 0x4000, Size: 8}, 10)
	if state.FPReload.PageAliasLoad != 61 {
		t.Fatalf("training load position = %d, want 61", state.FPReload.PageAliasLoad)
	}
	fallback := loadForwarding{issueCycle: 100, latency: 8, forwarded: true}
	load := emu.MemAccess{Addr: addr, Size: 8}
	// A subsequent store to the same address is still in flight on both sides of retirement.
	state.StoreBuffer = []storeBufferEntry{{Addr: addr, Size: 8, Feedback: true, ScalarStorePosition: 70, ReadyCycle: 110, CompleteCycle: 113}}
	state.InstDone = 60
	if got := pipe.trainAndForwardFeedback(load, 100, fallback); got != fallback {
		t.Fatalf("live alias used fast forwarding: %+v", got)
	}
	state.InstDone = 61
	want := loadForwarding{issueCycle: 100, dataReady: 110, latency: 1, forwarded: true}
	if got := pipe.trainAndForwardFeedback(load, 100, fallback); got != want {
		t.Fatalf("retired alias still delayed forwarding: %+v", got)
	}
}
