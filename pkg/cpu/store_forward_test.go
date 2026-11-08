package cpu

import (
	"testing"

	"github.com/sarchlab/poros/pkg/cpu/ooo"
)

// Store-to-load forwarding microbenchmarks. Each probe is an unrolled
// straight-line body (no branches, legacy emulator, no cache) so the cycle
// count isolates the memory-carried dependency chain.
const (
	forwardProbeIterations = 64
	forwardDataAddress     = 0x600000
)

// Encodings use X4 = buffer base, W6 = 3, W0/W3/W5 = integer values, D0/D1 = FP.
const (
	encLdrW3X4    = 0xb9400083 // LDR W3, [X4]
	encLdrW3X4p4  = 0xb9400483 // LDR W3, [X4,#4]
	encLdrX3X4    = 0xf9400083 // LDR X3, [X4]
	encStrW0X4    = 0xb9000080 // STR W0, [X4]
	encStrW5X4    = 0xb9000085 // STR W5, [X4]
	encStrX0X4    = 0xf9000080 // STR X0, [X4]
	encCmpW5W3    = 0x6b0300bf // CMP W5, W3
	encCselW0W5W3 = 0x1a83a0a0 // CSEL W0, W5, W3, GE
	encAddW5Imm3  = 0x11000ca5 // ADD W5, W5, #3
	encAddW5W3Im3 = 0x11000c65 // ADD W5, W3, #3
	encMulW5W3W6  = 0x1b067c65 // MUL W5, W3, W6
	encMulW0W3W6  = 0x1b067c60 // MUL W0, W3, W6
	encLdrD0X4    = 0xfd400080 // LDR D0, [X4]
	encStrD0X4    = 0xfd000080 // STR D0, [X4]
	encFaddD0D0D1 = 0x1e612800 // FADD D0, D0, D1
	encStrW0X4Idx = 0xb8207880 // STR W0, [X4, X0, LSL #2]
)

func runForwardProbe(t *testing.T, body []uint32) *ooo.Comp {
	t.Helper()
	var code []uint32
	for i := 0; i < forwardProbeIterations; i++ {
		code = append(code, body...)
	}
	code = append(code, 0xd65f03c0) // RET
	em := setupEmu(code, 0x10000, 0xffff0000)
	em.Regs.SP = 0x500000 + 8192
	em.Regs.WriteX(4, forwardDataAddress)
	em.Regs.WriteX(6, 3)
	for off := uint64(0); off < 4096; off += 4 {
		em.Mem.WriteU32(forwardDataAddress+off, 0)
	}
	core := runOoOPipeline(t, em)
	if core.State.InstDone != uint64(len(code)) {
		t.Fatalf("probe retired %d of %d instructions", core.State.InstDone, len(code))
	}
	return core
}

func perIteration(core *ooo.Comp) float64 {
	return float64(core.State.Cycles) / forwardProbeIterations
}

// Nussinov inner-loop shape. W5 is an independent running value (1-cycle
// register chain); the only long loop-carried chain goes through memory:
// STR -> LDR -> CMP -> CSEL -> STR.
var maxIntoMemoryBody = []uint32{encLdrW3X4, encCmpW5W3, encCselW0W5W3, encStrW0X4, encAddW5Imm3}

// The candidate exact-match GPR path waits for store data, so the modeled
// loop-carried chain is forward + CMP + CSEL rather
// than store completion + L1 latency + CMP + CSEL.
func TestExactMatchIntegerStoreLoadChainUsesDataReadyForwarding(t *testing.T) {
	chain := runForwardProbe(t, maxIntoMemoryBody)
	got := perIteration(chain)
	if got < 3 || got > 4 {
		t.Fatalf("exact-match integer chain costs %.2f cycles/iteration, want 3..4", got)
	}
	if v := chain.Resources.Emu.Regs.ReadX(0) & 0xffffffff; v != 3*(forwardProbeIterations-1) {
		t.Fatalf("chain value %d, want %d", v, 3*(forwardProbeIterations-1))
	}
}

// The forwarded load becomes ready when the store's data is ready, not when the
// store is dispatched: a multiply-produced value (3 cycles) lengthens the
// memory-carried chain by the multiply latency.
func TestForwardedLoadWaitsForStoreData(t *testing.T) {
	fast := runForwardProbe(t, []uint32{encLdrW3X4, encAddW5W3Im3, encStrW5X4})
	slow := runForwardProbe(t, []uint32{encLdrW3X4, encMulW5W3W6, encStrW5X4})
	if f, s := perIteration(fast), perIteration(slow); f < 2 || f > 3 || s < f+1.5 || s > 5 {
		t.Fatalf("data-dependent chain: add=%.2f mul=%.2f cycles/iteration, want ~2 and ~4", f, s)
	}
}

// A load from a different, non-overlapping word never waits for the store.
func TestNonOverlappingStoreAndLoadAreIndependent(t *testing.T) {
	apart := runForwardProbe(t, []uint32{encLdrW3X4p4, encCmpW5W3, encCselW0W5W3, encStrW0X4, encAddW5Imm3})
	same := runForwardProbe(t, maxIntoMemoryBody)
	if perIteration(apart)*1.5 > perIteration(same) {
		t.Fatalf("independent load/store did not run faster than the chain: apart=%.2f same=%.2f", perIteration(apart), perIteration(same))
	}
}

// Width mismatches use the conventional path: wait for store completion and
// then the L1 latency. A wider load than the store is not forwarded at all.
func TestPartialOrMismatchedOverlapIsConventional(t *testing.T) {
	wideStore := runForwardProbe(t, []uint32{encLdrW3X4, encCmpW5W3, encCselW0W5W3, encStrX0X4, encAddW5Imm3})
	wideLoad := runForwardProbe(t, []uint32{encLdrX3X4, encCmpW5W3, encCselW0W5W3, encStrW0X4, encAddW5Imm3})
	for name, core := range map[string]*ooo.Comp{"covered narrower load": wideStore, "partially covered wider load": wideLoad} {
		if got := perIteration(core); got < 9.5 || got > 11 {
			t.Fatalf("%s costs %.2f cycles/iteration, want the conventional 10", name, got)
		}
	}
}

// Two in-flight stores to the same address: the youngest supplies the value.
// When the youngest store's data is ready early, an older store whose data
// arrives late must not delay the load.
func TestYoungestMatchingStoreSuppliesForwardedValue(t *testing.T) {
	youngFast := runForwardProbe(t, []uint32{encLdrW3X4, encMulW0W3W6, encStrW0X4, encStrW5X4})
	youngSlow := runForwardProbe(t, []uint32{encLdrW3X4, encMulW0W3W6, encStrW5X4, encStrW0X4})
	if f, s := perIteration(youngFast), perIteration(youngSlow); f > 2 || s < 3.5 || s > 5 {
		t.Fatalf("youngest-store semantics: youngest-fast=%.2f youngest-slow=%.2f cycles/iteration", f, s)
	}
}

// FP accumulations through memory keep the conventional forwarding path.
func TestFPStoreLoadChainKeepsConventionalForwarding(t *testing.T) {
	fp := runForwardProbe(t, []uint32{encLdrD0X4, encFaddD0D0D1, encStrD0X4})
	if got := perIteration(fp); got < 10.5 || got > 12 {
		t.Fatalf("FP memory chain costs %.2f cycles/iteration, want the conventional 11", got)
	}
}

// Many in-flight stores to distinct addresses fill the store buffer; the
// exact-match load still matches the youngest same-address store while the
// bounded allocation accounting is retained and drains at program completion.
func TestStoreBufferPressureKeepsExactMatch(t *testing.T) {
	pressure := runForwardProbe(t, append(append([]uint32{}, maxIntoMemoryBody...), encStrW0X4Idx))
	plain := runForwardProbe(t, maxIntoMemoryBody)
	if perIteration(pressure) > perIteration(plain)+1 {
		t.Fatalf("extra stores broke the exact-match match: %.2f vs %.2f", perIteration(pressure), perIteration(plain))
	}
	if len(pressure.State.StoreDrainUntil) != ooo.DefaultSpec().StoreBufferSize {
		t.Fatalf("allocation capacity %d, want %d", len(pressure.State.StoreDrainUntil), ooo.DefaultSpec().StoreBufferSize)
	}
	if len(pressure.State.StoreBuffer) != 0 {
		t.Fatalf("completed program retains %d forwarding records", len(pressure.State.StoreBuffer))
	}
	for _, until := range pressure.State.StoreDrainUntil {
		if until > pressure.State.Cycles {
			t.Fatalf("completed at %d before store allocation %d", pressure.State.Cycles, until)
		}
	}
}

// Addressing and transfer form are part of fast-path eligibility on both sides.
// X7 is zero, so indexed accesses reach the identical address as direct ones.
func TestUnmeasuredOrIndexedTransferFormsKeepConventionalTiming(t *testing.T) {
	for _, tc := range []struct {
		name        string
		load, store uint32
	}{
		{"indexed load", 0xb8676883, encStrW5X4},  // LDR W3,[X4,X7]
		{"indexed store", encLdrW3X4, 0xb8276885}, // STR W5,[X4,X7]
		{"both indexed", 0xb8676883, 0xb8276885},
		{"pair load", 0x29401c83, encStrW5X4},  // LDP W3,W7,[X4]
		{"pair store", encLdrW3X4, 0x29001c85}, // STP W5,W7,[X4]
		{"both pairs", 0x29401c83, 0x29001c85},
	} {
		t.Run(tc.name, func(t *testing.T) {
			core := runForwardProbe(t, []uint32{tc.load, encAddW5W3Im3, tc.store})
			if got := perIteration(core); got < 8.5 || got > 10 {
				t.Fatalf("%s chain costs %.2f, want conventional ~9 cycles/update", tc.name, got)
			}
			if got := core.Resources.Emu.Regs.ReadX(5); got != 3*forwardProbeIterations {
				t.Fatalf("chain value %d, want %d", got, 3*forwardProbeIterations)
			}
		})
	}
}
