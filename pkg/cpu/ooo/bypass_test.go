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
	bypass := func(s *Spec) { s.BypassRecovery, s.BypassValueRun, s.PlainForwardLatency = 4, 2, 6 }
	if got := storeThenLoad(t, strX1X2, ldrX3X2, 0x2000, 0x2000, bypass); got != 101 {
		t.Errorf("load through the store's base register ready at %d, want 101", got)
	}
	if got := storeThenLoad(t, strX1X2, ldrX3X4, 0x2000, 0x2000, bypass); got != 106 {
		t.Errorf("load through another base register ready at %d, want 106", got)
	}
	if got := storeThenLoad(t, strX1X2, ldrX3X4, 0x2000, 0x2000, func(*Spec) {}); got != 101 {
		t.Errorf("without the qualification the load is ready at %d, want 101", got)
	}
}

// A load keeps the path while its value changes at every execution or never
// changes. Another value after a full run of equal ones takes the path away
// from the load's PC for the recovery, whatever it reads meanwhile.
func TestBypassValueRunBreakDistrustsTheLoad(t *testing.T) {
	const pc = 0x4000
	x2 := addressOperand{arm64asm.RegSP(arm64asm.X2), 0, true}
	store := storeBufferEntry{Addr: 0x1008, Size: 8, ReadyCycle: 30, CompleteCycle: 33, FastForwardEligible: true, Operand: x2}
	load := decodeWord(t, ldrX3X2)
	accesses := []emu.MemAccess{{Addr: 0x1008, Size: 8}}
	em := emu.NewEmulator()
	p := &middleware{comp: &Comp{Spec: Spec{BypassRecovery: 2, BypassValueRun: 3, PlainForwardLatency: 6},
		State: State{StoreBuffer: []storeBufferEntry{store}}, Resources: Resources{Emu: em}}}
	fast, slow := loadForwarding{5, 30, 1, true}, loadForwarding{5, 30, 6, true}
	step := func(loaded uint64) loadForwarding {
		em.Regs.WriteX(3, loaded)
		return p.bypassLoad(load, pc, accesses, 5, p.forwardLoad(accesses, 5, true))
	}
	for i, value := range []uint64{1, 2, 3, 4, 4, 5, 5, 5} {
		if got := step(value); got != fast {
			t.Fatalf("execution %d reading %d before any full run: %+v", i, value, got)
		}
	}
	// The run of three fives is full; the six breaks it. The breaking
	// execution itself still has the path.
	if got := step(6); got != fast || p.comp.State.Bypass.Loads[pc].Distrust != 2 {
		t.Fatalf("breaking execution: %+v, history %+v", got, p.comp.State.Bypass.Loads[pc])
	}
	raw, _ := json.Marshal(p.comp.State)
	var restored State
	if err := json.Unmarshal(raw, &restored); err != nil || !reflect.DeepEqual(restored.Bypass, p.comp.State.Bypass) ||
		!reflect.DeepEqual(restored.StoreBuffer, p.comp.State.StoreBuffer) {
		t.Fatal("bypass state did not round trip", err)
	}
	for _, value := range []uint64{6, 7} {
		if got := step(value); got != slow {
			t.Fatalf("distrusted execution reading %d: %+v", value, got)
		}
	}
	if got := step(7); got != fast || p.comp.State.Bypass.Loads[pc].Distrust != 0 {
		t.Fatalf("recovered execution: %+v, history %+v", got, p.comp.State.Bypass.Loads[pc])
	}
	// Another PC has its own history, and a load the cache supplies only
	// records its value.
	p.comp.State.StoreBuffer = nil
	for _, value := range []uint64{9, 9, 9, 8} {
		if got := p.bypassLoad(load, pc+4, accesses, 5, func() loadForwarding { em.Regs.WriteX(3, value); return p.forwardLoad(accesses, 5, true) }()); got.forwarded {
			t.Fatalf("cache-supplied load was forwarded: %+v", got)
		}
	}
	if got := p.comp.State.Bypass.Loads[pc+4]; got != (valueHistory{Value: 8, Run: 1, Distrust: 2}) {
		t.Fatalf("history of the second PC: %+v", got)
	}
}
