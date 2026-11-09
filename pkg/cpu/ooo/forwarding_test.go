package ooo

import (
	"encoding/binary"
	"encoding/json"
	"golang.org/x/arch/arm64/arm64asm"
	"reflect"
	"testing"

	"github.com/sarchlab/poros/pkg/emu"
)

func TestForwardingSelectsOnlyEligibleYoungestStores(t *testing.T) {
	oldSlow := storeBufferEntry{Addr: 0x1000, Size: 4, ReadyCycle: 20, CompleteCycle: 23, FastForwardEligible: true}
	youngFast := storeBufferEntry{Addr: 0x1000, Size: 4, ReadyCycle: 8, CompleteCycle: 11, FastForwardEligible: true}
	load := []emu.MemAccess{{Addr: 0x1000, Size: 4}}
	for _, tc := range []struct {
		name     string
		stores   []storeBufferEntry
		accesses []emu.MemAccess
		gpr      bool
		want     loadForwarding
	}{
		{"youngest exact shadows older slow store", []storeBufferEntry{oldSlow, youngFast}, load, true, loadForwarding{5, 8, 1, true}},
		{"youngest partial shadows older exact", []storeBufferEntry{oldSlow, {Addr: 0x1002, Size: 2, ReadyCycle: 8, CompleteCycle: 11, FastForwardEligible: true}}, load, true, loadForwarding{23, 0, 5, true}},
		{"youngest FP shadows older exact", []storeBufferEntry{oldSlow, {Addr: 0x1000, Size: 4, ReadyCycle: 8, CompleteCycle: 11}}, load, true, loadForwarding{23, 0, 5, true}},
		{"already completed", []storeBufferEntry{{Addr: 0x1000, Size: 4, ReadyCycle: 2, CompleteCycle: 5, FastForwardEligible: true}}, load, true, loadForwarding{5, 0, 5, true}},
		{"partial without covering store", []storeBufferEntry{{Addr: 0x1002, Size: 2, ReadyCycle: 8, CompleteCycle: 11, FastForwardEligible: true}}, load, true, loadForwarding{11, 0, 5, false}},
		{"FP load", []storeBufferEntry{youngFast}, load, false, loadForwarding{11, 0, 5, true}},
		{"multiple accesses retain conventional path", []storeBufferEntry{youngFast, {Addr: 0x1004, Size: 4, ReadyCycle: 12, CompleteCycle: 15, FastForwardEligible: true}}, []emu.MemAccess{{Addr: 0x1000, Size: 4}, {Addr: 0x1004, Size: 4}}, true, loadForwarding{15, 0, 5, true}},
		{"pair with one unmatched access", []storeBufferEntry{youngFast}, []emu.MemAccess{{Addr: 0x1000, Size: 4}, {Addr: 0x1004, Size: 4}}, true, loadForwarding{11, 0, 5, false}},
		{"pair with one FP store", []storeBufferEntry{youngFast, {Addr: 0x1004, Size: 4, ReadyCycle: 12, CompleteCycle: 15}}, []emu.MemAccess{{Addr: 0x1000, Size: 4}, {Addr: 0x1004, Size: 4}}, true, loadForwarding{15, 0, 5, true}},
		{"no accesses", []storeBufferEntry{youngFast}, nil, true, loadForwarding{5, 0, 5, false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			core := &Comp{State: State{StoreBuffer: tc.stores}}
			pipe := &middleware{comp: core}
			if got := pipe.forwardLoad(tc.accesses, 5, tc.gpr); got != tc.want {
				t.Fatalf("forwarding %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestStoreForwardingStateRoundTripAndLegacyFallback(t *testing.T) {
	original := State{StoreBuffer: []storeBufferEntry{{Addr: 0x1000, Size: 4, ReadyCycle: 8, CompleteCycle: 11, FastForwardEligible: true}}}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var restored State
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored, original) {
		t.Fatal("store readiness/classification lost across state round-trip")
	}
	accesses := []emu.MemAccess{{Addr: 0x1000, Size: 4}}
	pipe := &middleware{comp: &Comp{State: restored}}
	if got := pipe.forwardLoad(accesses, 5, true); got != (loadForwarding{5, 8, 1, true}) {
		t.Fatalf("restored timing %+v", got)
	}
	var legacy State
	if err := json.Unmarshal([]byte(`{"StoreBuffer":[{"Addr":4096,"Size":4,"CompleteCycle":11}]}`), &legacy); err != nil {
		t.Fatal(err)
	}
	pipe.comp.State = legacy
	if got := pipe.forwardLoad(accesses, 5, true); got != (loadForwarding{11, 0, 5, true}) {
		t.Fatalf("legacy entry incorrectly used new forwarding path: %+v", got)
	}
}

func TestDirectGPRTransferEligibility(t *testing.T) {
	for _, tc := range []struct {
		name string
		word uint32
		want bool
	}{
		{"ldr x immediate", 0xf9400083, true}, {"str x immediate", 0xf9000080, true},
		{"ldr w displacement", 0xb9400483, true}, {"str w immediate", 0xb9000080, true},
		{"indexed load", 0xf8646883, false}, {"indexed store", 0xf8246880, false},
		{"preindexed load", 0xf8408c83, false}, {"postindexed store", 0xf8008480, false},
		{"pair load", 0xa9401483, false}, {"pair store", 0xa9001480, false},
		{"byte load", 0x39400083, false}, {"halfword store", 0x79000080, false},
		{"unscaled load", 0xf8400083, false}, {"unscaled store", 0xf8000080, false},
		{"fp load", 0xfd400080, false}, {"fp store", 0xfd000080, false},
		{"sign extending load", 0xb9800083, false}, {"zero register store", 0xf900009f, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var code [4]byte
			binary.LittleEndian.PutUint32(code[:], tc.word)
			decoded, err := arm64asm.Decode(code[:])
			if err != nil {
				t.Fatal(err)
			}
			if got := directGPRTransfer(decoded); got != tc.want {
				t.Fatalf("%s: eligible=%v, want %v", decoded, got, tc.want)
			}
		})
	}
}

func TestCompactedStoresRetainYoungestBypassAndOlderOrdering(t *testing.T) {
	p := &middleware{comp: &Comp{State: State{StoreBuffer: []storeBufferEntry{
		{Addr: 0x1000, Size: 8, ReadyCycle: 100, CompleteCycle: 103, DrainCycle: 400, FastForwardEligible: true},
	}}}}
	p.appendStoreRecord(storeBufferEntry{Addr: 0x1000, Size: 8, ReadyCycle: 8, CompleteCycle: 11, DrainCycle: 50, FastForwardEligible: true})
	if len(p.comp.State.StoreBuffer) != 1 || p.comp.State.StoreBuffer[0].PriorCompleteCycle != 103 || p.comp.State.StoreBuffer[0].DrainCycle != 400 {
		t.Fatal("compaction lost ordering/drain")
	}
	exact := p.forwardLoad([]emu.MemAccess{{Addr: 0x1000, Size: 8}}, 5, true)
	if exact.dataReady != 8 || exact.issueCycle != 5 || exact.latency != 1 {
		t.Fatal("youngest exact bypass changed", exact)
	}
	partial := p.forwardLoad([]emu.MemAccess{{Addr: 0x1002, Size: 2}}, 5, true)
	if partial.issueCycle != 103 || partial.latency != 5 {
		t.Fatal("partial ordering lost", partial)
	}
	p.appendStoreRecord(storeBufferEntry{Addr: 0x1002, Size: 2, CompleteCycle: 15, DrainCycle: 50})
	if len(p.comp.State.StoreBuffer) != 2 {
		t.Fatal("partial shadow erased unowned bytes")
	}
	partial = p.forwardLoad([]emu.MemAccess{{Addr: 0x1000, Size: 8}}, 5, true)
	if partial.issueCycle != 103 || partial.latency != 5 {
		t.Fatal("young partial store incorrectly took exact bypass")
	}
	raw, _ := json.Marshal(p.comp.State)
	var restored State
	if err := json.Unmarshal(raw, &restored); err != nil || !reflect.DeepEqual(restored, p.comp.State) {
		t.Fatal("forwarding state did not round trip", err)
	}
}
