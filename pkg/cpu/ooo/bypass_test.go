package ooo

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/sarchlab/poros/pkg/emu"
	"golang.org/x/arch/arm64/arm64asm"
)

const (
	ldrX3X2     = 0xf9400043 // LDR X3, [X2]
	ldrX3X2Off8 = 0xf9400443 // LDR X3, [X2, #8]
	ldrW3X2Off8 = 0xb9400843 // LDR W3, [X2, #8]
)

func TestNamedAddressIsBaseAndScaledDisplacement(t *testing.T) {
	x2 := arm64asm.RegSP(arm64asm.X2)
	for _, tc := range []struct {
		word uint32
		want addressOperand
	}{
		{ldrX3X2, addressOperand{x2, 0, true}}, {strX1X2, addressOperand{x2, 0, true}},
		{ldrX3X2Off8, addressOperand{x2, 8, true}}, {strX3X2Off8, addressOperand{x2, 8, true}},
		{ldrW3X2Off8, addressOperand{x2, 8, true}},
		{0xf8646883, addressOperand{}}, // LDR X3, [X4, X4]: no immediate operand
	} {
		if got := namedAddress(decodeWord(t, tc.word)); got != tc.want {
			t.Errorf("%#x: operand %+v, want %+v", tc.word, got, tc.want)
		}
	}
}

// The path follows the name of the address, not the address: a store through
// another base register supplies the load in the plain forward latency.
func TestBypassNeedsTheSameAddressOperand(t *testing.T) {
	bypass := func(s *Spec) { s.BypassRecovery, s.PlainForwardLatency = 4, 6 }
	if got := storeThenLoad(t, strX1X2, ldrX3X2, 0x2000, 0x2000, bypass); got != 101 {
		t.Errorf("load through the store's base register ready at %d, want 101", got)
	}
	if got := storeThenLoad(t, strX1X2, ldrX3X4, 0x2000, 0x2000, bypass); got != 106 {
		t.Errorf("load through another base register ready at %d, want 106", got)
	}
	if got := storeThenLoad(t, strX1X2, ldrX3X4, 0x2000, 0x2000, func(*Spec) {}); got != 101 {
		t.Errorf("without the prediction the load is ready at %d, want 101", got)
	}
}

// The named store wrote another address. A load that reads the value that
// store wrote goes unnoticed; a different value costs the load's PC the path
// for the recovery, also when the named store then does supply it.
func TestBypassStaleValueDistrustsTheLoad(t *testing.T) {
	const pc = 0x4000
	x2 := addressOperand{arm64asm.RegSP(arm64asm.X2), 0, true}
	stale := storeBufferEntry{Addr: 0x1000, Size: 8, ReadyCycle: 20, CompleteCycle: 23, FastForwardEligible: true, Operand: x2, Value: 7}
	fresh := storeBufferEntry{Addr: 0x1008, Size: 8, ReadyCycle: 30, CompleteCycle: 33, FastForwardEligible: true, Operand: x2, Value: 9}
	load := decodeWord(t, ldrX3X2)
	accesses := []emu.MemAccess{{Addr: 0x1008, Size: 8}}
	em := emu.NewEmulator()
	p := &middleware{comp: &Comp{Spec: Spec{BypassRecovery: 2, PlainForwardLatency: 6}, Resources: Resources{Emu: em}}}
	step := func(stores []storeBufferEntry, loaded uint64) loadForwarding {
		p.comp.State.StoreBuffer = stores
		em.Regs.WriteX(3, loaded)
		return p.bypassLoad(load, pc, accesses, 5, p.forwardLoad(accesses, 5, true))
	}
	if got := step([]storeBufferEntry{stale}, 7); got.forwarded || len(p.comp.State.Bypass.Distrust) != 0 {
		t.Fatalf("equal value: %+v, distrust %v", got, p.comp.State.Bypass.Distrust)
	}
	if got := step([]storeBufferEntry{stale, fresh}, 9); got != (loadForwarding{5, 30, 1, true}) {
		t.Fatalf("trusted load of its named store: %+v", got)
	}
	if got := step([]storeBufferEntry{stale}, 9); got.forwarded || p.comp.State.Bypass.Distrust[pc] != 2 {
		t.Fatalf("different value: %+v, distrust %v", got, p.comp.State.Bypass.Distrust)
	}
	raw, _ := json.Marshal(p.comp.State)
	var restored State
	if err := json.Unmarshal(raw, &restored); err != nil || !reflect.DeepEqual(restored.Bypass, p.comp.State.Bypass) ||
		!reflect.DeepEqual(restored.StoreBuffer, p.comp.State.StoreBuffer) {
		t.Fatal("bypass state did not round trip", err)
	}
	for range 2 {
		if got := step([]storeBufferEntry{stale, fresh}, 9); got != (loadForwarding{5, 30, 6, true}) {
			t.Fatalf("distrusted load: %+v", got)
		}
	}
	if got := step([]storeBufferEntry{stale, fresh}, 9); got != (loadForwarding{5, 30, 1, true}) || len(p.comp.State.Bypass.Distrust) != 0 {
		t.Fatalf("recovered load: %+v, distrust %v", got, p.comp.State.Bypass.Distrust)
	}
	// A named store that has executed can no longer be bypassed.
	if step([]storeBufferEntry{{Addr: 0x1000, Size: 8, ReadyCycle: 1, CompleteCycle: 4, FastForwardEligible: true, Operand: x2, Value: 7}}, 9); len(p.comp.State.Bypass.Distrust) != 0 {
		t.Fatal("completed store was bypassed")
	}
}
