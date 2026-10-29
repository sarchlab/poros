package cpu

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/sarchlab/akita/v5/sim"
	"github.com/sarchlab/akita/v5/sim/modeling"
	"github.com/sarchlab/akita/v5/sim/timing"
	"github.com/sarchlab/poros/pkg/cpu/ooo"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/mem"
)

// cndfGolden computes the cumulative normal distribution function.
func cndfGolden(x float64) float64 {
	a1 := 0.31938153
	a2 := -0.356563782
	a3 := 1.781477937
	a4 := -1.821255978
	a5 := 1.330274429

	L := math.Abs(x)
	K := 1.0 / (1.0 + 0.2316419*L)
	w := 1.0 - 1.0/math.Sqrt(2.0*3.141592653589793)*
		math.Exp(-L*L/2.0)*
		(a1*K+a2*K*K+a3*K*K*K+a4*K*K*K*K+a5*K*K*K*K*K)
	if x < 0.0 {
		w = 1.0 - w
	}
	return w
}

// blkSchlsGoldenCPU computes the Black-Scholes option price.
func blkSchlsGoldenCPU(S, X, r, v, T float64, otype int) float64 {
	sqrtT := math.Sqrt(T)
	d1 := (math.Log(S/X) + (r+v*v*0.5)*T) / (v * sqrtT)
	d2 := d1 - v*sqrtT
	NofXd1 := cndfGolden(d1)
	NofXd2 := cndfGolden(d2)
	if otype == 0 {
		return S*NofXd1 - X*math.Exp(-r*T)*NofXd2
	}
	return X*math.Exp(-r*T)*(1.0-NofXd2) - S*(1.0-NofXd1)
}

func TestOoO_BlackScholes(t *testing.T) {
	// End-to-end PARSEC BlackScholes through the OoO pipeline with cache.
	//
	// Native timing (M2, -O1 -fno-vectorize, 16 options, median of 7 runs):
	//   322 ns/iteration → ~1124 cycles @ 3.49 GHz
	//   CPI ≈ 1124/1733 ≈ 0.65
	const nativeTimeBlackScholes = 5.152e-06 // seconds, M2 -O1, 16 options, median of 7 runs (16 × 322ns)

	numOptions := 16

	stockPrices := []float64{
		42.0, 100.0, 50.0, 80.0, 120.0, 30.0, 95.0, 70.0,
		55.0, 110.0, 45.0, 85.0, 60.0, 75.0, 90.0, 65.0,
	}
	strikePrices := []float64{
		40.0, 105.0, 48.0, 82.0, 115.0, 32.0, 100.0, 68.0,
		52.0, 108.0, 50.0, 80.0, 58.0, 72.0, 88.0, 70.0,
	}
	riskFreeRate := []float64{
		0.05, 0.04, 0.06, 0.03, 0.05, 0.04, 0.05, 0.06,
		0.03, 0.05, 0.04, 0.06, 0.05, 0.03, 0.04, 0.05,
	}
	volatility := []float64{
		0.20, 0.25, 0.30, 0.15, 0.22, 0.35, 0.18, 0.28,
		0.25, 0.20, 0.30, 0.22, 0.25, 0.18, 0.20, 0.32,
	}
	timeToExp := []float64{
		0.50, 1.00, 0.25, 0.75, 1.50, 0.30, 0.80, 0.60,
		1.00, 0.50, 0.75, 1.25, 0.40, 0.90, 1.00, 0.50,
	}
	optionType := []int32{
		0, 1, 0, 1, 0, 1, 0, 1,
		0, 1, 0, 1, 0, 1, 0, 1,
	}

	expectedPrices := make([]float64, numOptions)
	for i := 0; i < numOptions; i++ {
		expectedPrices[i] = blkSchlsGoldenCPU(
			stockPrices[i], strikePrices[i],
			riskFreeRate[i], volatility[i], timeToExp[i],
			int(optionType[i]),
		)
	}

	// Machine code: 173 instructions from compiled BlkSchlsEqEuroNoDiv.
	// BL instructions patched: log→0xF000, exp→0xF100
	code := []uint32{
		0x710004ff, 0x5400156b, 0xd10443ff, 0x6d073bef,
		0x6d0833ed, 0x6d092beb, 0x6d0a23e9, 0xa90b6ffc,
		0xa90c67fa, 0xa90d5ff8, 0xa90e57f6, 0xa90f4ff4,
		0xa9107bfd, 0x910403fd, 0xaa0603f3, 0xaa0503f4,
		0xaa0403f5, 0xaa0303f6, 0xaa0203f7, 0xaa0103f8,
		0xaa0003f9, 0xd28201c8, 0xf2a30e28, 0xf2d4ce28,
		0xf2e7f9a8, 0xf9001be8, 0xd2864b68, 0xf2bcb508,
		0xf2da3e08, 0xf2f7fac8, 0xf90017e8, 0xd29f1d88,
		0xf2a75248, 0xf2ce17e8, 0xf2e7fa88, 0xf90013e8,
		0xd28bcd08, 0xf2a04be8, 0xf2d01de8, 0xf2e7ff88,
		0xf9000fe8, 0xd28f1a08, 0xf2a9de48, 0xf2c47ba8,
		0xf2f7ffa8, 0xf9000be8, 0xd2852868, 0xf2bade88,
		0xf2c919a8, 0xf2e7fea8, 0xf90033e8, 0xd286ca28,
		0xf2a67a88, 0xf2d108a8, 0xf2f7fb28, 0xf90007e8,
		0x2a0703fa, 0xfd400280, 0x1e61c00e, 0x1e604008,
		0xfd0023e0, 0xfc408700, 0xfc4086e1, 0x6d0503e1,
		0x1e611800, 0x940037bf, 0xfc4086c3, 0xfd0027e3,
		0xfc4086a1, 0x1e610822, 0x1e6c1004, 0x1f440c42,
		0x1e6c1003, 0x1f480040, 0x1e6109c2, 0x1e62180f,
		0x1f4ebc20, 0xfd0037e0, 0x1e60c1e0, 0xfd401beb,
		0x1e6e1008, 0x1f4b2001, 0x1e611909, 0x1e608800,
		0x1e630800, 0x940037eb, 0x6d4237ea, 0x1e6d0921,
		0x1e610921, 0x1f4a0521, 0x6d413bec, 0x1e6e0922,
		0x1e620922, 0x1f490441, 0x1e6c0922, 0x1e620922,
		0x1e620922, 0x1f490441, 0xfd4033e2, 0x1e620922,
		0x1e620922, 0x1e620922, 0x1e620922, 0x1f490441,
		0xfd4007e9, 0x1e690800, 0x1f412000, 0x1e603901,
		0x1e6021e8, 0x1e604c20, 0xfd001fe0, 0xfd4037e0,
		0x1e60c000, 0x1f4b2001, 0x1e61190f, 0x1e608800,
		0x1e6c1001, 0x1e610800, 0x940037ca, 0x1e6d09e1,
		0x1e6109e1, 0x1f4a05e1, 0x1e6e09e2, 0x1e6209e2,
		0x1f4f0441, 0x1e6c09e2, 0x1e6209e2, 0x1e6209e2,
		0xfd4033e3, 0x1e6309e3, 0x1f4f0441, 0x1e6309e2,
		0x1e6209e2, 0x1e6209e2, 0x1f4f0441, 0x1e690800,
		0x1f412001, 0x1e613902, 0xfd4037e0, 0x1e602008,
		0xb840467b, 0x6d4403e3, 0x1e638800, 0x1e614c4c,
		0x940037b0, 0x6d4513e5, 0x1e6008a1, 0x1e6c3902,
		0xfd401fe6, 0x1e663903, 0x1e638883, 0x1f420c21,
		0x1e6088a0, 0x1e6c0800, 0x1f460080, 0x7100037f,
		0x1e601c20, 0xfc008720, 0x91002294, 0xf100075a,
		0x54fff321, 0xa9507bfd, 0xa94f4ff4, 0xa94e57f6,
		0xa94d5ff8, 0xa94c67fa, 0xa94b6ffc, 0x6d4a23e9,
		0x6d492beb, 0x6d4833ed, 0x6d473bef, 0x910443ff,
		0xd65f03c0,
	}

	codeBase := uint64(0x1000)
	logAddr := uint64(0xF000)
	expAddr := uint64(0xF100)

	pricesBase := uint64(0x100000)
	sBase := uint64(0x200000)
	xBase := uint64(0x300000)
	rBase := uint64(0x400000)
	vBase := uint64(0x500000)
	tBase := uint64(0x600000)
	otypeBase := uint64(0x700000)
	stackBase := uint64(0x800000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000

	codeBuf := make([]byte, len(code)*4)
	for i, inst := range code {
		binary.LittleEndian.PutUint32(codeBuf[i*4:], inst)
	}
	em.LoadProgram(codeBase, codeBuf)

	writeF64 := func(base uint64, data []float64) {
		buf := make([]byte, len(data)*8)
		for i, v := range data {
			binary.LittleEndian.PutUint64(buf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(base, buf)
	}
	writeI32 := func(base uint64, data []int32) {
		buf := make([]byte, len(data)*4)
		for i, v := range data {
			binary.LittleEndian.PutUint32(buf[i*4:], uint32(v))
		}
		em.LoadProgram(base, buf)
	}

	writeF64(pricesBase, make([]float64, numOptions))
	writeF64(sBase, stockPrices)
	writeF64(xBase, strikePrices)
	writeF64(rBase, riskFreeRate)
	writeF64(vBase, volatility)
	writeF64(tBase, timeToExp)
	writeI32(otypeBase, optionType)

	stackBuf := make([]byte, 8192)
	em.LoadProgram(stackBase, stackBuf)

	em.Regs.PC = codeBase
	em.Regs.WriteX(0, pricesBase)
	em.Regs.WriteX(1, sBase)
	em.Regs.WriteX(2, xBase)
	em.Regs.WriteX(3, rBase)
	em.Regs.WriteX(4, vBase)
	em.Regs.WriteX(5, tBase)
	em.Regs.WriteX(6, otypeBase)
	em.Regs.WriteX(7, uint64(numOptions))
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 4096
	em.SetHaltPC(haltPC)

	// Register BL-intercept hooks
	em.RegisterFuncHook(logAddr, func(em *emu.Emulator) {
		x := em.Regs.ReadD(0)
		em.Regs.WriteD(0, math.Log(x))
	})
	em.RegisterFuncHook(expAddr, func(em *emu.Emulator) {
		x := em.Regs.ReadD(0)
		em.Regs.WriteD(0, math.Exp(x))
	})

	// Run through OoO pipeline with cache and function hook cycle costs.
	cache := mem.NewCache(mem.DefaultCacheConfig())

	simulation := sim.MakeBuilder().Build()
	em.Mem.TrackAccesses = true
	pSpec := ooo.DefaultSpec()
	pSpec.Freq = 3490 * timing.MHz
	pSpec.FuncHookCycles = HookCycleSpec(map[uint64]int{
		expAddr: 25, // exp() costs 25 cycles
		logAddr: 20, // log() costs 20 cycles
	})
	p := modeling.MakeBuilder[*ooo.Comp](pSpec).WithSimulation(simulation).WithResources(ooo.Resources{Emu: em, Cache: cache}).Build("TestOoO")
	runTestSimulation(t, simulation, p)
	if p.State.Err != "" {
		t.Fatalf("pipeline error: %v", p.State.Err)
	}

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Verify correctness
	maxRelErr := 0.0
	for i := 0; i < numOptions; i++ {
		bits := em.Mem.ReadU64(pricesBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expectedPrices[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			t.Errorf("prices[%d] = %g, want %g (relErr=%e)", i, got, want, relErr)
		}
	}

	t.Logf("BlackScholes: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		p.State.InstDone, p.State.Cycles, p.State.CPI(), maxRelErr)

	simTime := float64(p.State.Cycles) / 3.49e9
	ratio := simTime / nativeTimeBlackScholes
	t.Logf("BlackScholes native comparison: simTime=%e, native=%e, ratio=%.2f",
		simTime, nativeTimeBlackScholes, ratio)
}
