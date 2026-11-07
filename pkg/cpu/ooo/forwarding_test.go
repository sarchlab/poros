package ooo

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/sarchlab/poros/pkg/emu"
)

func TestForwardingSelectsOnlyEligibleYoungestStores(t *testing.T) {
	oldSlow := storeBufferEntry{Addr: 0x1000, Size: 4, ReadyCycle: 20, CompleteCycle: 23, GPRData: true}
	youngFast := storeBufferEntry{Addr: 0x1000, Size: 4, ReadyCycle: 8, CompleteCycle: 11, GPRData: true}
	load := []emu.MemAccess{{Addr: 0x1000, Size: 4}}
	for _, tc := range []struct {
		name     string
		stores   []storeBufferEntry
		accesses []emu.MemAccess
		gpr      bool
		want     loadForwarding
	}{
		{"youngest exact shadows older slow store", []storeBufferEntry{oldSlow, youngFast}, load, true, loadForwarding{5, 8, 1, true}},
		{"youngest partial shadows older exact", []storeBufferEntry{oldSlow, {Addr: 0x1002, Size: 2, ReadyCycle: 8, CompleteCycle: 11, GPRData: true}}, load, true, loadForwarding{23, 0, 5, true}},
		{"youngest FP shadows older exact", []storeBufferEntry{oldSlow, {Addr: 0x1000, Size: 4, ReadyCycle: 8, CompleteCycle: 11}}, load, true, loadForwarding{23, 0, 5, true}},
		{"already completed", []storeBufferEntry{{Addr: 0x1000, Size: 4, ReadyCycle: 2, CompleteCycle: 5, GPRData: true}}, load, true, loadForwarding{5, 0, 5, true}},
		{"partial without covering store", []storeBufferEntry{{Addr: 0x1002, Size: 2, ReadyCycle: 8, CompleteCycle: 11, GPRData: true}}, load, true, loadForwarding{11, 0, 5, false}},
		{"FP load", []storeBufferEntry{youngFast}, load, false, loadForwarding{11, 0, 5, true}},
		{"pair waits for both store data", []storeBufferEntry{youngFast, {Addr: 0x1004, Size: 4, ReadyCycle: 12, CompleteCycle: 15, GPRData: true}}, []emu.MemAccess{{Addr: 0x1000, Size: 4}, {Addr: 0x1004, Size: 4}}, true, loadForwarding{5, 12, 1, true}},
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
	original := State{StoreBuffer: []storeBufferEntry{{Addr: 0x1000, Size: 4, ReadyCycle: 8, CompleteCycle: 11, GPRData: true}}}
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
