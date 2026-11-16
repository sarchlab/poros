package ooo

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"

	instinfo "github.com/sarchlab/poros/pkg/cpu/internal/inst"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/mem"
	"golang.org/x/arch/arm64/arm64asm"
)

const (
	sdivW0W1W2  = 0x1ac20c20 // SDIV W0, W1, W2
	sdivX0X1X2  = 0x9ac20c20 // SDIV X0, X1, X2
	udivW0W1W2  = 0x1ac20820 // UDIV W0, W1, W2
	udivX0X1X2  = 0x9ac20820 // UDIV X0, X1, X2
	sdivWZRW1W2 = 0x1ac20c3f // SDIV WZR, W1, W2
	udivW1W1W2  = 0x1ac20821 // UDIV W1, W1, W2
	mulX0X1X2   = 0x9b027c20 // MUL X0, X1, X2
	umulhX0X1X2 = 0x9bc27c20 // UMULH X0, X1, X2
	scvtfD2W1   = 0x1e620022 // SCVTF D2, W1
	faddD0D1D2  = 0x1e622820 // FADD D0, D1, D2
	faddD3D1D2  = 0x1e622823 // FADD D3, D1, D2
	faddD2D1D1  = 0x1e612822 // FADD D2, D1, D1
	faddD3D1D1  = 0x1e612823 // FADD D3, D1, D1
	faddD4D1D1  = 0x1e612824 // FADD D4, D1, D1
	faddD5D6D6  = 0x1e6628c5 // FADD D5, D6, D6
	faddD7D6D6  = 0x1e6628c7 // FADD D7, D6, D6
	fmulD0D1D2  = 0x1e620820 // FMUL D0, D1, D2
	fmulD3D1D2  = 0x1e620823 // FMUL D3, D1, D2
	fmaddD3     = 0x1f420c23 // FMADD D3, D1, D2, D3
	fmaddD3D3   = 0x1f420c63 // FMADD D3, D3, D2, D3
	ldrD0X1     = 0xfd400020 // LDR D0, [X1]
	ldrD2X1     = 0xfd400022 // LDR D2, [X1]
	strD0X1     = 0xfd000020 // STR D0, [X1]
	strD1X2     = 0xfd000041 // STR D1, [X2]
	ldpD2D3X1   = 0x6d400c22 // LDP D2, D3, [X1]
	ldpX6X7X4   = 0xa9401c86 // LDP X6, X7, [X4]
	ldrX8X9     = 0xf9400128 // LDR X8, [X9]
	stpX1X3X2   = 0xa9000c41 // STP X1, X3, [X2]
	ldurX3X4p4  = 0xf8404083 // LDUR X3, [X4, #4]
)

// The zero value changes no latency, and an override reaches exactly the
// instructions of its class.
func TestLatenciesOverrideOnlyTheirClass(t *testing.T) {
	grace := Latencies{Multiply: 2, FPArithmetic: 2, FPMultiply: 3, FPDivide: 15}
	for _, tc := range []struct {
		word uint32
		want uint64 // with the overrides; the default is instinfo.ExecuteLatency
	}{
		{mulX0X1X2, 2}, {umulhX0X1X2, 3}, {faddD0D1D2, 2}, {fmulD0D1D2, 3}, {fdivD0D1D2, 15},
		{scvtfD2W1, 3}, {fmaddD3, 4}, {addX9, 1}, {sdivW0W1W2, 10},
	} {
		inst := decodeWord(t, tc.word)
		plain := specPipeline(t, []uint32{tc.word}, nil, nil)
		if got, want := plain.executeLatency(inst), uint64(instinfo.ExecuteLatency(inst.Op)); got != want {
			t.Errorf("%v: zero Latencies give %d, default is %d", inst, got, want)
		}
		changed := specPipeline(t, []uint32{tc.word}, nil, func(s *Spec) { s.Latencies = grace })
		if got := changed.executeLatency(inst); got != tc.want {
			t.Errorf("%v: %d cycles with overrides, want %d", inst, got, tc.want)
		}
	}
}

// An early-ending divider takes its fixed part plus one cycle for every four
// significant bits of the quotient's magnitude, read from the operands: the
// destination may be the dividend or the zero register.
func TestDividerEndsEarlyOnShortQuotients(t *testing.T) {
	const minInt32, minInt64 = uint64(1) << 31, uint64(1) << 63
	for _, tc := range []struct {
		name              string
		word              uint32
		dividend, divisor uint64
		want              uint64
	}{
		{"quotient zero", udivW0W1W2, 7, 8, 5},
		{"four bits", udivW0W1W2, 15, 1, 5},
		{"five bits", udivW0W1W2, 16, 1, 6},
		{"32 bits", udivW0W1W2, math.MaxUint32, 1, 12},
		{"64 bits", udivX0X1X2, math.MaxUint64, 1, 20},
		{"upper half ignored by the 32-bit form", udivW0W1W2, 0xffffffff00000007, 8, 5},
		{"negative quotient uses its magnitude", sdivW0W1W2, uint64(uint32(0xfffffff9)), 2, 5}, // -7 / 2
		{"most negative 32-bit dividend", sdivW0W1W2, minInt32, uint64(uint32(0xffffffff)), 12},
		{"most negative 64-bit dividend", sdivX0X1X2, minInt64, 1, 20},
		{"division by zero", sdivW0W1W2, 1 << 30, 0, 5},
		{"quotient discarded", sdivWZRW1W2, math.MaxInt32, 1, 12},
		{"destination is the dividend", udivW1W1W2, math.MaxUint32, 1, 12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := specPipeline(t, []uint32{tc.word}, nil, func(s *Spec) {
				s.Latencies.DivideBase, s.Latencies.DivideBitsPerCycle = 4, 4
			})
			p.comp.Resources.Emu.Regs.WriteX(1, tc.dividend)
			p.comp.Resources.Emu.Regs.WriteX(2, tc.divisor)
			p.fetchAndProcess()
			if got := p.comp.State.Rob[0] - 1; p.comp.State.Err != "" || got != tc.want {
				t.Fatalf("%d cycles, want %d (err %q)", got, tc.want, p.comp.State.Err)
			}
			// The divide keeps its unit for as long as it runs.
			if want := []UnitReservation{{1, 1 + tc.want}}; !reflect.DeepEqual(p.comp.State.IntUnits[0], want) {
				t.Fatalf("reservations %+v, want %+v", p.comp.State.IntUnits[0], want)
			}
		})
	}
	// A fixed latency without early ending.
	p := specPipeline(t, []uint32{udivW0W1W2}, nil, func(s *Spec) { s.Latencies.DivideBase = 7 })
	p.comp.Resources.Emu.Regs.WriteX(1, math.MaxUint32)
	p.comp.Resources.Emu.Regs.WriteX(2, 1)
	p.fetchAndProcess()
	if p.comp.State.Rob[0] != 8 {
		t.Fatalf("fixed divide completes at %d, want 8", p.comp.State.Rob[0])
	}
}

// By default a divide occupies an integer pipe, so other work on that pipe
// waits for it. A divider of its own blocks only the next divide.
func TestIntegerDividerBlocksOnlyDivides(t *testing.T) {
	for _, tc := range []struct {
		dividers       int
		add, secondDiv uint64 // completion cycles
	}{
		{0, 12, 22}, // divide 1..11, ADD 11..12, divide 12..22
		{1, 2, 21},  // ADD 1..2 on the pipe; dividers 1..11 and 11..21
	} {
		p := specPipeline(t, []uint32{udivW0W1W2, addX9, udivW0W1W2}, nil, func(s *Spec) { s.NumIntDiv = tc.dividers })
		p.comp.Resources.Emu.Regs.WriteX(2, 1)
		p.fetchAndProcess()
		state := p.comp.State
		if state.Err != "" || state.Rob[0] != 11 || state.Rob[1] != tc.add || state.Rob[2] != tc.secondDiv {
			t.Fatalf("dividers=%d: divide %d, add %d, divide %d (err %q)", tc.dividers, state.Rob[0], state.Rob[1], state.Rob[2], state.Err)
		}
		if got := len(state.IntDividers); got != tc.dividers {
			t.Fatalf("dividers=%d: %d divider calendars", tc.dividers, got)
		}
	}
}

// With an occupancy of its own, an FDIV holds one of the FP dividers and no FP
// pipe; without it the divide takes one issue slot of its pipe as before.
func TestFPDividerOccupancy(t *testing.T) {
	for _, tc := range []struct {
		occupancy, dividers int
		add, secondDiv      uint64 // issue cycles
	}{
		{0, 1, 2, 3},   // one shared FP pipe: FDIV 1, FADD 2, FDIV 3
		{15, 1, 1, 16}, // the divider is busy 1..16; the FADD is not delayed
		{15, 2, 1, 1},  // a second divider takes the second divide at once
	} {
		p := specPipeline(t, []uint32{fdivD0D1D2, faddD3D1D2, fdivD0D1D2}, nil, func(s *Spec) {
			s.Latencies.FPDivideOccupancy, s.NumFPDiv, s.NumFP = tc.occupancy, tc.dividers, max(1, tc.dividers)
		})
		p.fetchAndProcess()
		state := p.comp.State
		addLatency, divLatency := uint64(3), uint64(10)
		if state.Err != "" || state.Rob[1] != tc.add+addLatency || state.Rob[2] != tc.secondDiv+divLatency {
			t.Fatalf("occupancy=%d dividers=%d: add completes %d, second divide %d (err %q)", tc.occupancy, tc.dividers, state.Rob[1], state.Rob[2], state.Err)
		}
	}
}

// A fused multiply-add whose addend comes from an FP multiply or another
// fused multiply-add may issue before the addend is ready, by its latency less
// the accumulate latency. Any other producer, and a multiplicand, is needed at
// issue.
func TestFusedMultiplyAddForwardsItsAddendLate(t *testing.T) {
	chain := func(accumulate int, words ...uint32) []uint64 {
		p := specPipeline(t, words, nil, func(s *Spec) { s.Latencies.FPAccumulate, s.NumFP = accumulate, 4 })
		p.fetchAndProcess()
		if p.comp.State.Err != "" {
			t.Fatal(p.comp.State.Err)
		}
		return append([]uint64{}, p.comp.State.Rob[:len(words)]...)
	}
	for _, tc := range []struct {
		name       string
		accumulate int
		words      []uint32
		want       []uint64
	}{
		{"default: every operand at issue", 0, []uint32{fmaddD3, fmaddD3, fmaddD3}, []uint64{5, 9, 13}},
		{"accumulate chain", 2, []uint32{fmaddD3, fmaddD3, fmaddD3}, []uint64{5, 7, 9}},
		{"addend from an FP multiply", 2, []uint32{fmulD3D1D2, fmaddD3}, []uint64{5, 7}},
		{"addend from an FP add is not forwarded late", 2, []uint32{faddD3D1D2, fmaddD3}, []uint64{4, 8}},
		{"addend that is also a multiplicand", 2, []uint32{fmaddD3, fmaddD3D3}, []uint64{5, 9}},
		{"accumulate latency no shorter than the result latency", 4, []uint32{fmaddD3, fmaddD3}, []uint64{5, 9}},
	} {
		if got := chain(tc.accumulate, tc.words...); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: completions %v, want %v", tc.name, got, tc.want)
		}
	}
	// A load into the register ends the late forwarding of its old producer.
	p := specPipeline(t, []uint32{fmaddD3, 0xfd400023, fmaddD3}, nil, func(s *Spec) { s.Latencies.FPAccumulate, s.NumFP = 2, 4 }) // LDR D3, [X1]
	p.comp.Resources.Emu.Regs.WriteX(1, 0x2000)
	p.fetchAndProcess()
	if state := p.comp.State; state.Err != "" || state.FpLateForward[3] != true || state.Rob[2] != state.Rob[1]+4 {
		t.Fatalf("after a load: load ready %d, multiply-add %d (err %q)", state.Rob[1], state.Rob[2], state.Err)
	}
}

// storeThenLoad dispatches a store whose data is ready at cycle 100 and then
// a load, and returns the load's completion cycle.
func storeThenLoad(t *testing.T, store, load uint32, storeAddress, loadAddress uint64, adjust func(*Spec)) uint64 {
	t.Helper()
	p := specPipeline(t, []uint32{store, load}, nil, func(s *Spec) {
		s.NumLDST = 2
		adjust(s)
	})
	regs := p.comp.Resources.Emu.Regs
	for _, register := range []int{1, 2} { // base of either store form
		regs.WriteX(register, storeAddress)
	}
	if store == strX1X2 {
		regs.WriteX(2, storeAddress)
		p.comp.State.GprReady[1] = 100
	} else {
		p.comp.State.FpReady[0], p.comp.State.FpReady[1] = 100, 100
	}
	if load == ldrX3X4 || load == ldurX3X4p4 {
		regs.WriteX(4, loadAddress)
	}
	p.fetchAndProcess()
	if p.comp.State.Err != "" || p.comp.State.RobCount != 2 {
		t.Fatalf("err=%q dispatched=%d", p.comp.State.Err, p.comp.State.RobCount)
	}
	return p.comp.State.Rob[1]
}

// With a store-forward latency, every load that one in-flight store supplies
// has its value that many cycles after the store's data issues, whatever the
// forms and the register class; FPLoadExtra belongs to the cache path only.
func TestStoreForwardLatencyIsOnePath(t *testing.T) {
	single := func(s *Spec) { s.StoreForwardLatency, s.Latencies.FPLoadExtra = 6, 2 }
	if got := storeThenLoad(t, strX1X2, ldrX3X4, 0x2000, 0x2000, single); got != 106 {
		t.Errorf("integer load of the stored bytes ready at %d, want 106", got)
	}
	if got := storeThenLoad(t, strD0X1, ldrD2X1, 0x2000, 0, single); got != 106 {
		t.Errorf("FP load of the stored bytes ready at %d, want 106", got)
	}
	// Half of a pair comes from the store and half from memory.
	if got := storeThenLoad(t, strD0X1, ldpD2D3X1, 0x2000, 0, single); got != 106 {
		t.Errorf("pair load with one stored half ready at %d, want 106", got)
	}
	// A store that holds only part of an access is not forwarded: the load
	// waits for the store to complete (three cycles) and then reads memory.
	if got, want := storeThenLoad(t, strX1X2, ldurX3X4p4, 0x2000, 0x2000, single), uint64(100+3+5); got != want {
		t.Errorf("partly overlapping load ready at %d, want %d", got, want)
	}
	// Without the knob the two paths of forwardLoad remain: one cycle for the
	// exact integer form, completion plus five for the FP form.
	off := func(*Spec) {}
	if got := storeThenLoad(t, strX1X2, ldrX3X4, 0x2000, 0x2000, off); got != 101 {
		t.Errorf("default exact path ready at %d, want 101", got)
	}
	if got := storeThenLoad(t, strD0X1, ldrD2X1, 0x2000, 0, off); got != 108 {
		t.Errorf("default FP path ready at %d, want 108", got)
	}
}

// A forwarded value is never ready before the load itself could issue.
func TestStoreForwardWaitsForTheLoadAddress(t *testing.T) {
	p := specPipeline(t, []uint32{strX1X2, ldrX3X4}, nil, func(s *Spec) { s.NumLDST, s.StoreForwardLatency = 2, 6 })
	regs := p.comp.Resources.Emu.Regs
	regs.WriteX(2, 0x2000)
	regs.WriteX(4, 0x2000)
	p.comp.State.GprReady[4] = 50 // the load's base register is late; the store's data is not
	p.fetchAndProcess()
	if got := p.comp.State.Rob[1]; p.comp.State.Err != "" || got != 56 {
		t.Fatalf("load ready at %d, want 56 (err %q)", got, p.comp.State.Err)
	}
}

// A pair load with one half in an in-flight store and the other in the cache
// takes both paths. The stored half is forwarded from when both the load and
// the store's data have issued; the load is ready when both halves are.
func TestStoreForwardLatencyOnPartlySuppliedLoads(t *testing.T) {
	const forward, l1 = 6, 4
	// mixedPair runs LDP X6, X7, [X4] at load, after STR X1, [X2] at store
	// when stored, and returns the load's completion cycle. The lines of the
	// warm addresses are in L1 before the run.
	mixedPair := func(stored bool, store, load, storeData, loadBase uint64, warm ...uint64) uint64 {
		t.Helper()
		cfg := fillConfig()
		cfg.L1Latency = l1
		cache := mem.NewCache(cfg)
		for _, addr := range warm {
			cache.Lookup(addr, 8, false)
		}
		words := []uint32{nop, ldpX6X7X4}
		if stored {
			words[0] = strX1X2
		}
		p := specPipeline(t, words, cache, func(s *Spec) { s.NumLDST, s.StoreForwardLatency = 2, forward })
		regs := p.comp.Resources.Emu.Regs
		regs.WriteX(2, store)
		regs.WriteX(4, load)
		p.comp.State.GprReady[1], p.comp.State.GprReady[4] = storeData, loadBase
		p.fetchAndProcess()
		if p.comp.State.Err != "" || p.comp.State.RobCount != 2 {
			t.Fatalf("err=%q dispatched=%d", p.comp.State.Err, p.comp.State.RobCount)
		}
		return p.comp.State.Rob[1]
	}
	// Both halves are in one warm line; the store holds the first.
	for _, tc := range []struct {
		name                string
		storeData, loadBase uint64
		want                uint64
	}{
		{"load issues after the store's data", 48, 50, 50 + forward},
		{"store's data issues after the load", 100, 50, 100 + forward},
		{"both issue at once", 50, 50, 50 + forward},
		{"neither waits", 0, 0, 1 + forward},
	} {
		if got := mixedPair(true, 0x2000, 0x2000, tc.storeData, tc.loadBase, 0x2000); got != tc.want {
			t.Errorf("%s: pair ready at %d, want %d", tc.name, got, tc.want)
		}
	}
	// Without the store the same warm load takes the cache's latency alone.
	if got := mixedPair(false, 0x2000, 0x2000, 0, 50, 0x2000); got != 50+l1 {
		t.Errorf("warm pair without a store ready at %d, want %d", got, 50+l1)
	}
	// The store holds the second half, in a warm line; the first half is in a
	// line that only L2 holds. The cache is then the later of the two paths,
	// with or without the store.
	fromL2 := 50 + uint64(fillConfig().L2Latency)
	for _, stored := range []bool{false, true} {
		if got := mixedPair(stored, 0x2040, 0x2038, 48, 50, 0x2040); got != fromL2 || got <= 50+forward {
			t.Errorf("stored=%v: pair with a half from L2 ready at %d, want %d", stored, got, fromL2)
		}
	}
}

// A load that an in-flight store supplies in part sends only its other accesses
// to the cache. The stored bytes take no translation, no miss entry and no
// cache read, and a fill of their own line does not hold the load: the cache
// sees what it sees when the other access is loaded alone.
func TestPartlySuppliedLoadSendsOnlyItsOtherAccessesToTheCache(t *testing.T) {
	const forward, l1 = 6, 4
	// LDP X6, X7, [X4] at low reads the words at low and high, which are in
	// different lines of both cache levels.
	const low, high = 0x2078, 0x2080
	type outcome struct {
		complete uint64 // of the last instruction
		stats    mem.CacheStats
		miss     uint64 // until when the only miss entry is held
	}
	// run executes the instructions with the store's data ready at cycle 48
	// and the base of the last instruction, the load, at 50. The store is at
	// store, the load at load, an LDR X8, [X9] between them at other.
	run := func(cfg mem.CacheConfig, latency int, store, load, other uint64, prepare func(*mem.Cache), words ...uint32) outcome {
		t.Helper()
		cfg.L1Latency = l1
		cache := mem.NewCache(cfg)
		prepare(cache)
		p := specPipeline(t, words, cache, func(s *Spec) { s.NumLDST, s.StoreForwardLatency = 3, latency })
		regs := p.comp.Resources.Emu.Regs
		regs.WriteX(2, store)
		regs.WriteX(4, load)
		regs.WriteX(9, other)
		p.comp.State.GprReady[1], p.comp.State.GprReady[3], p.comp.State.GprReady[4] = 48, 48, 50
		p.fetchAndProcess()
		if p.comp.State.Err != "" || p.comp.State.RobCount != len(words) {
			t.Fatalf("err=%q dispatched=%d", p.comp.State.Err, p.comp.State.RobCount)
		}
		return outcome{p.comp.State.Rob[len(words)-1], cache.Stats(), p.comp.State.MshrBusyUntil[0]}
	}
	warm := func(addrs ...uint64) func(*mem.Cache) {
		return func(cache *mem.Cache) {
			for _, addr := range addrs {
				cache.Lookup(addr, 8, false)
			}
		}
	}
	plain := fillConfig()
	fromMemory := 50 + uint64(plain.DRAMLatency)

	// The store misses and its line is still on its way; the other word is in
	// L1. The pair is ready by the forwarding path, not when that line arrives.
	pair := run(plain, forward, low, low, 0, warm(high), strX1X2, ldpX6X7X4)
	alone := run(plain, forward, low, high, 0, warm(high), strX1X2, ldrX3X4)
	if pair.complete != 50+forward || alone.complete != 50+l1 || pair.stats != alone.stats || pair.miss != alone.miss {
		t.Errorf("stored word's line pending: pair ready at %d, want %d; cache saw\n%+v, miss entry until %d\nfor the other word alone (ready at %d)\n%+v, miss entry until %d",
			pair.complete, 50+forward, pair.stats, pair.miss, alone.complete, alone.stats, alone.miss)
	}
	// The reverse: the stored word's line is in L1 and the other word comes
	// from memory. The cache decides, and reads that word only.
	pair = run(plain, forward, high, low, 0, warm(high), strX1X2, ldpX6X7X4)
	alone = run(plain, forward, high, low, 0, warm(high), strX1X2, ldrX3X4)
	if pair.complete != fromMemory || pair != alone {
		t.Errorf("other word from memory: pair %+v\nthe other word alone %+v\nwant both ready at %d", pair, alone, fromMemory)
	}
	// The stored word is on a page whose translation the store has just
	// started; the other word's page is translated and both lines are in L1.
	const pageEnd = 0x8000
	translated := func(cache *mem.Cache) {
		warm(pageEnd-8, pageEnd)(cache)
		cache.Translate(pageEnd, 8, 0)
	}
	paged := mem.DefaultCacheConfig()
	pair = run(paged, forward, pageEnd-8, pageEnd-8, 0, translated, strX1X2, ldpX6X7X4)
	alone = run(paged, forward, pageEnd-8, pageEnd, 0, translated, strX1X2, ldrX3X4)
	if pair.complete != 50+forward || alone.complete != 50+l1 || pair.stats != alone.stats {
		t.Errorf("stored word's page being walked: pair ready at %d, want %d; cache saw\n%+v\nfor the other word alone (ready at %d)\n%+v",
			pair.complete, 50+forward, pair.stats, alone.complete, alone.stats)
	}
	// A load between them evicts the stored word's line from a direct-mapped
	// L1 and holds the only miss entry. The pair needs neither.
	direct := plain
	direct.L1Ways = 1
	conflict := uint64(low + direct.L1Size)
	pair = run(direct, forward, low, low, conflict, warm(high), strX1X2, ldrX8X9, ldpX6X7X4)
	alone = run(direct, forward, low, high, conflict, warm(high), strX1X2, ldrX8X9, ldrX3X4)
	if pair.complete != 50+forward || alone.complete != 50+l1 || pair.stats != alone.stats || pair.miss != alone.miss {
		t.Errorf("stored word's line evicted: pair ready at %d, want %d; cache saw\n%+v, miss entry until %d\nfor the other word alone (ready at %d)\n%+v, miss entry until %d",
			pair.complete, 50+forward, pair.stats, pair.miss, alone.complete, alone.stats, alone.miss)
	}

	// Stores supply both words: the cache sees the stores only.
	pair = run(plain, forward, low, low, 0, warm(high), stpX1X3X2, ldpX6X7X4)
	alone = run(plain, forward, low, low, 0, warm(high), stpX1X3X2, nop)
	if pair.complete != 50+forward || pair.stats != alone.stats || pair.miss != alone.miss {
		t.Errorf("both words stored: pair ready at %d, want %d; cache saw\n%+v\nfor the stores alone\n%+v", pair.complete, 50+forward, pair.stats, alone.stats)
	}
	// No store in flight: the cache supplies both words, as with the path off.
	pair = run(plain, forward, 0, low, 0, warm(high), nop, ldpX6X7X4)
	alone = run(plain, 0, 0, low, 0, warm(high), nop, ldpX6X7X4)
	if pair.complete != fromMemory || pair != alone {
		t.Errorf("no store: pair %+v\nwith the path off %+v\nwant both ready at %d", pair, alone, fromMemory)
	}
	// With the path off the load still reads the stored word from the cache
	// and waits for the line its store allocated at cycle 48.
	off := run(plain, 0, low, low, 0, warm(high), strX1X2, ldpX6X7X4)
	if want := 48 + uint64(plain.DRAMLatency); off.complete != want || off.stats.L1HitsInFlight != 1 {
		t.Errorf("path off: pair ready at %d, want %d, with %d reads of a pending line, want 1", off.complete, want, off.stats.L1HitsInFlight)
	}
}

// suppliedLoad hands back the list it was given when the cache supplies every
// access, without allocating, and a list of its own with the remaining
// accesses of a partly supplied load. The list it was given stays as it was.
func TestSuppliedLoadListsTheAccessesLeftToTheCache(t *testing.T) {
	first, second := emu.MemAccess{Addr: 0x2078, Size: 8}, emu.MemAccess{Addr: 0x2080, Size: 8}
	stored := func(acc emu.MemAccess, ready uint64) storeBufferEntry {
		return storeBufferEntry{Addr: acc.Addr, Size: acc.Size, ReadyCycle: ready}
	}
	for _, tc := range []struct {
		name    string
		forward int
		stores  []storeBufferEntry
		ready   uint64
		cached  []emu.MemAccess
		ok      bool
	}{
		{"path off", 0, []storeBufferEntry{stored(first, 48)}, 0, []emu.MemAccess{first, second}, false},
		{"no store", 6, nil, 0, []emu.MemAccess{first, second}, false},
		{"first stored", 6, []storeBufferEntry{stored(first, 48)}, 48, []emu.MemAccess{second}, true},
		{"second stored", 6, []storeBufferEntry{stored(second, 48)}, 48, []emu.MemAccess{first}, true},
		{"both stored", 6, []storeBufferEntry{stored(first, 48), stored(second, 52)}, 52, nil, true},
		{"part of an access stored", 6, []storeBufferEntry{{Addr: first.Addr + 4, Size: 8, ReadyCycle: 48}}, 0, []emu.MemAccess{first, second}, false},
	} {
		p := specPipeline(t, []uint32{nop}, nil, func(s *Spec) { s.StoreForwardLatency = tc.forward })
		p.comp.State.StoreBuffer = tc.stores
		accesses := []emu.MemAccess{first, second}
		ready, cached, ok := p.suppliedLoad(accesses)
		if ready != tc.ready || ok != tc.ok || !reflect.DeepEqual(cached, tc.cached) {
			t.Errorf("%s: ready %d, cached %v, ok %v; want %d, %v, %v", tc.name, ready, cached, ok, tc.ready, tc.cached, tc.ok)
		}
		if !reflect.DeepEqual(accesses, []emu.MemAccess{first, second}) {
			t.Errorf("%s: the load's access list became %v", tc.name, accesses)
		}
		// Only a partly supplied load is given a list of its own.
		same := len(cached) == len(accesses) && &cached[0] == &accesses[0]
		allocations := testing.AllocsPerRun(10, func() { p.suppliedLoad(accesses) })
		if partly := tc.ok && len(tc.cached) > 0; same != (len(tc.cached) == len(accesses)) || (allocations > 0) != partly {
			t.Errorf("%s: the list given is returned: %v; %v allocations", tc.name, same, allocations)
		}
	}
}

// FPLoadExtra delays a load into an FP register that memory supplies, and no
// other load.
func TestFPLoadExtraOnMemorySuppliedLoads(t *testing.T) {
	for _, tc := range []struct {
		word  uint32
		extra uint64
	}{{ldrD0X1, 2}, {ldpD2D3X1, 2}, {ldrX3X4, 0}} {
		p := specPipeline(t, []uint32{tc.word}, nil, func(s *Spec) { s.Latencies.FPLoadExtra = 2 })
		p.comp.Resources.Emu.Regs.WriteX(1, 0x2000)
		p.comp.Resources.Emu.Regs.WriteX(4, 0x2000)
		p.fetchAndProcess()
		if got, want := p.comp.State.Rob[0], 1+uint64(p.comp.Spec.LoadLatency)+tc.extra; p.comp.State.Err != "" || got != want {
			t.Errorf("%v ready at %d, want %d (err %q)", decodeWord(t, tc.word), got, want, p.comp.State.Err)
		}
	}
}

// A full FP issue queue stops dispatch until an entry issues. FP operations and
// the data of FP stores take entries; integer work takes none but cannot pass
// a stalled FP instruction.
func TestFPIssueQueueBoundsWaitingWork(t *testing.T) {
	words := []uint32{faddD2D1D1, strD1X2, faddD3D1D1, addX9, faddD4D1D1}
	dispatched := func(queue int) (*middleware, int) {
		p := specPipeline(t, words, nil, func(s *Spec) { s.FPIssueQueue, s.NumFP = queue, 4 })
		p.comp.Resources.Emu.Regs.WriteX(2, 0x2000)
		p.comp.State.FpReady[1] = 100
		p.fetchAndProcess()
		if p.comp.State.Err != "" {
			t.Fatal(p.comp.State.Err)
		}
		return p, p.comp.State.RobCount
	}
	if _, got := dispatched(0); got != len(words) {
		t.Fatalf("unlimited queue dispatched %d of %d", got, len(words))
	}
	p, got := dispatched(2)
	if got != 2 || len(p.comp.State.FpQueue) != 2 {
		t.Fatalf("two entries admitted %d instructions with %d queued; the add and the FP store should fill them", got, len(p.comp.State.FpQueue))
	}
	// Nothing more enters while both wait, however many cycles pass.
	p.comp.State.Cycles = 99
	p.fetchAndProcess()
	if p.comp.State.RobCount != 2 {
		t.Fatalf("dispatched %d while the queue was full", p.comp.State.RobCount)
	}
	// Both issue at cycle 100; from then on the rest is admitted.
	p.comp.State.Cycles = 100
	p.fetchAndProcess()
	if p.comp.State.Err != "" || p.comp.State.RobCount != len(words) {
		t.Fatalf("after the queue drained: dispatched %d of %d (err %q)", p.comp.State.RobCount, len(words), p.comp.State.Err)
	}
}

// An entry is held for the whole cycle its instruction is dispatched in, also
// by work that issues in that cycle, whatever FP pipes are free. The entry of
// work dispatched earlier is free again from the cycle that work issues.
func TestFPIssueQueueEntryLifetime(t *testing.T) {
	// dispatchedAt returns how many instructions are in the window after
	// dispatch has run in each of the cycles.
	dispatchedAt := func(p *middleware, cycles ...uint64) []int {
		t.Helper()
		var counts []int
		for _, cycle := range cycles {
			p.comp.State.Cycles = cycle
			p.fetchAndProcess()
			if p.comp.State.Err != "" {
				t.Fatal(p.comp.State.Err)
			}
			counts = append(counts, p.comp.State.RobCount)
		}
		return counts
	}
	// Three independent three-cycle operations that can each issue at once,
	// and four FP pipes: only the queue spreads them over cycles.
	ready := []uint32{faddD5D6D6, faddD7D6D6, faddD2D1D1}
	for _, tc := range []struct {
		queue      int
		dispatched []int    // after cycles 1, 2 and 3
		complete   []uint64 // of each operation
	}{
		{1, []int{1, 2, 3}, []uint64{4, 5, 6}},
		{2, []int{2, 3, 3}, []uint64{4, 4, 5}},
		{3, []int{3, 3, 3}, []uint64{4, 4, 4}},
		{0, []int{3, 3, 3}, []uint64{4, 4, 4}},
	} {
		p := specPipeline(t, ready, nil, func(s *Spec) { s.FPIssueQueue, s.NumFP = tc.queue, 4 })
		if got := dispatchedAt(p, 1, 2, 3); !reflect.DeepEqual(got, tc.dispatched) {
			t.Errorf("queue of %d: %v instructions dispatched by cycles 1, 2, 3, want %v", tc.queue, got, tc.dispatched)
		}
		if got := p.comp.State.Rob[:len(ready)]; !reflect.DeepEqual(got, tc.complete) {
			t.Errorf("queue of %d: completions %v, want %v", tc.queue, got, tc.complete)
		}
	}
	// One entry, taken by an operation that waits until cycle 100. It is free
	// for dispatch in cycle 100; the operation admitted then holds it for
	// that cycle although it issues at once, however often dispatch runs.
	p := specPipeline(t, []uint32{faddD2D1D1, faddD5D6D6, faddD7D6D6}, nil, func(s *Spec) { s.FPIssueQueue, s.NumFP = 1, 4 })
	p.comp.State.FpReady[1] = 100
	if got, want := dispatchedAt(p, 1, 2, 99, 100, 100, 101), []int{1, 1, 1, 2, 2, 3}; !reflect.DeepEqual(got, want) {
		t.Errorf("waiting entry: %v instructions dispatched by cycles 1, 2, 99, 100, 100, 101, want %v", got, want)
	}
}

// A full multiply issue queue stops dispatch until a multiply issues. Both
// multiply forms take entries; other integer work takes none but cannot pass a
// stalled multiply.
func TestMultiplyIssueQueueBoundsWaitingWork(t *testing.T) {
	words := []uint32{mulX0X1X2, umulhX0X1X2, mulX0X1X2, addX9}
	dispatched := func(queue int) (*middleware, int) {
		p := specPipeline(t, words, nil, func(s *Spec) { s.MultiplyIssueQueue = queue })
		p.comp.State.GprReady[1] = 100
		p.fetchAndProcess()
		if p.comp.State.Err != "" {
			t.Fatal(p.comp.State.Err)
		}
		return p, p.comp.State.RobCount
	}
	if _, got := dispatched(0); got != len(words) {
		t.Fatalf("unlimited queue dispatched %d of %d", got, len(words))
	}
	p, got := dispatched(2)
	if got != 2 || len(p.comp.State.MultiplyQueue) != 2 {
		t.Fatalf("two entries admitted %d instructions with %d queued; the two multiplies should fill them", got, len(p.comp.State.MultiplyQueue))
	}
	// Nothing more enters while both wait, however many cycles pass.
	p.comp.State.Cycles = 99
	p.fetchAndProcess()
	if p.comp.State.RobCount != 2 {
		t.Fatalf("dispatched %d while the queue was full", p.comp.State.RobCount)
	}
	// Both issue at cycle 100; from then on the rest is admitted.
	p.comp.State.Cycles = 100
	p.fetchAndProcess()
	if p.comp.State.Err != "" || p.comp.State.RobCount != len(words) {
		t.Fatalf("after the queue drained: dispatched %d of %d (err %q)", p.comp.State.RobCount, len(words), p.comp.State.Err)
	}
}

// A chain of dependent multiplies is dispatched only as far ahead of its
// oldest unissued link as the queue has entries, and no later than unbounded
// dispatch would complete it; work of other kinds does not use the queue.
func TestMultiplyIssueQueueLimitsChainNotOtherWork(t *testing.T) {
	const mulX1X1X2 = 0x9b027c21 // MUL X1, X1, X2
	if got := decodeWord(t, mulX1X1X2); got.Op != arm64asm.MUL || got.Args[0] != arm64asm.X1 || got.Args[1] != arm64asm.X1 {
		t.Fatalf("%#x decodes to %v", mulX1X1X2, got)
	}
	chain := []uint32{mulX1X1X2, mulX1X1X2, mulX1X1X2, mulX1X1X2, mulX1X1X2, mulX1X1X2}
	run := func(queue int) (inWindow []int, complete []uint64) {
		p := specPipeline(t, chain, nil, func(s *Spec) { s.MultiplyIssueQueue = queue })
		for cycle := uint64(1); cycle <= 4; cycle++ {
			p.comp.State.Cycles = cycle
			p.fetchAndProcess()
			if p.comp.State.Err != "" {
				t.Fatal(p.comp.State.Err)
			}
			inWindow = append(inWindow, p.comp.State.RobCount)
		}
		return inWindow, append([]uint64(nil), p.comp.State.Rob[:p.comp.State.RobCount]...)
	}
	// The links issue at cycles 1, 4, 7, ...: three cycles apart.
	free, freeComplete := run(0)
	if want := []int{6, 6, 6, 6}; !reflect.DeepEqual(free, want) {
		t.Fatalf("unlimited queue: %v in the window by cycles 1 to 4, want %v", free, want)
	}
	// Two entries: the first link issues in cycle 1 and frees its entry for
	// cycle 2, the second waits until cycle 4.
	bounded, boundedComplete := run(2)
	if want := []int{2, 3, 3, 4}; !reflect.DeepEqual(bounded, want) {
		t.Errorf("queue of two: %v in the window by cycles 1 to 4, want %v", bounded, want)
	}
	// The chain itself is not delayed: each link dispatched completes when it
	// would have without the queue.
	if !reflect.DeepEqual(boundedComplete, freeComplete[:len(boundedComplete)]) {
		t.Errorf("queue of two completes the links at %v, unlimited at %v", boundedComplete, freeComplete)
	}
	// FP and other integer work is admitted whatever the queue holds.
	p := specPipeline(t, []uint32{mulX0X1X2, faddD5D6D6, addX9, faddD7D6D6}, nil, func(s *Spec) { s.MultiplyIssueQueue = 1 })
	p.comp.State.GprReady[1] = 100
	p.fetchAndProcess()
	if p.comp.State.Err != "" || p.comp.State.RobCount != 4 || len(p.comp.State.MultiplyQueue) != 1 {
		t.Errorf("one waiting multiply: %d of 4 dispatched, %d queued (err %q)", p.comp.State.RobCount, len(p.comp.State.MultiplyQueue), p.comp.State.Err)
	}
}

func TestInvalidLatencyConfigurationRejected(t *testing.T) {
	for name, adjust := range map[string]func(*Spec){
		"negative latency":             func(s *Spec) { s.Latencies.FPMultiply = -1 },
		"early ending without a base":  func(s *Spec) { s.Latencies.DivideBitsPerCycle = 4 },
		"negative dividers":            func(s *Spec) { s.NumIntDiv = -1 },
		"negative store-forward":       func(s *Spec) { s.StoreForwardLatency = -1 },
		"negative FP issue queue":      func(s *Spec) { s.FPIssueQueue = -1 },
		"negative multiply queue":      func(s *Spec) { s.MultiplyIssueQueue = -1 },
		"negative FP divide occupancy": func(s *Spec) { s.Latencies.FPDivideOccupancy = -1 },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("accepted")
				}
			}()
			specPipeline(t, []uint32{nop}, nil, adjust)
		})
	}
}

// The new fields are absent from a default Spec's JSON, so existing declared
// profiles are unchanged, and the new state survives a round trip.
func TestNewTimingFieldsDefaultToAbsent(t *testing.T) {
	raw, err := json.Marshal(DefaultSpec())
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err = json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"latencies", "num_int_div", "store_forward_latency", "fp_issue_queue", "multiply_issue_queue"} {
		if _, present := fields[name]; present {
			t.Errorf("default Spec serializes %q", name)
		}
	}
	p := specPipeline(t, []uint32{fdivD0D1D2, udivW0W1W2, faddD2D1D1}, nil, func(s *Spec) {
		s.NumIntDiv, s.Latencies.FPDivideOccupancy, s.FPIssueQueue, s.Latencies.FPAccumulate = 1, 15, 8, 2
	})
	p.comp.Resources.Emu.Regs.WriteX(2, 1)
	p.comp.State.FpReady[1] = 50
	p.fetchAndProcess()
	raw, err = json.Marshal(p.comp.State)
	if err != nil {
		t.Fatal(err)
	}
	var restored State
	if err = json.Unmarshal(raw, &restored); err != nil || !reflect.DeepEqual(restored, p.comp.State) {
		t.Fatalf("state with dividers and an FP queue did not round trip: %v", err)
	}
	if len(restored.IntDividers[0]) != 1 || len(restored.FpDividers[0]) != 1 || len(restored.FpQueue) != 2 {
		t.Fatalf("dividers %v %v, queue %v", restored.IntDividers, restored.FpDividers, restored.FpQueue)
	}
}

// Every instruction the tests name decodes to what its comment says.
func TestLatencyTestEncodings(t *testing.T) {
	for word, want := range map[uint32]arm64asm.Op{
		sdivW0W1W2: arm64asm.SDIV, sdivX0X1X2: arm64asm.SDIV, udivW0W1W2: arm64asm.UDIV, udivX0X1X2: arm64asm.UDIV,
		sdivWZRW1W2: arm64asm.SDIV, udivW1W1W2: arm64asm.UDIV, mulX0X1X2: arm64asm.MUL, umulhX0X1X2: arm64asm.UMULH,
		scvtfD2W1: arm64asm.SCVTF, faddD0D1D2: arm64asm.FADD, fmulD0D1D2: arm64asm.FMUL, fdivD0D1D2: arm64asm.FDIV,
		fmaddD3: arm64asm.FMADD, fmaddD3D3: arm64asm.FMADD, ldrD0X1: arm64asm.LDR, strD0X1: arm64asm.STR,
		ldpD2D3X1: arm64asm.LDP, ldurX3X4p4: arm64asm.LDUR, strD1X2: arm64asm.STR,
		ldpX6X7X4: arm64asm.LDP, faddD5D6D6: arm64asm.FADD, faddD7D6D6: arm64asm.FADD,
		ldrX8X9: arm64asm.LDR, stpX1X3X2: arm64asm.STP,
	} {
		if got := decodeWord(t, word).Op; got != want {
			t.Errorf("%#x decodes to %v, want %v", word, got, want)
		}
	}
}
