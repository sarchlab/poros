package main

import (
	"flag"
	"fmt"
	"math"
	"os"

	"github.com/sarchlab/akita/v4/sim"
	"github.com/sarchlab/m2sim3/pkg/benchmark"
	"github.com/sarchlab/m2sim3/pkg/cpu"
	"github.com/sarchlab/m2sim3/pkg/mem"
)

const freq = 3.49e9 // M2 Avalanche core frequency

func usage() {
	fmt.Fprintf(os.Stderr, `m2sim: Apple M2 cycle-accurate simulator

Usage:
  m2sim list                            List all available benchmarks
  m2sim run <name>                      Run a single benchmark
  m2sim run <name> --cores N            Run a benchmark on N cores
  m2sim run --all                       Run all benchmarks (full comparison table)
  m2sim run --suite <suite>             Run all benchmarks in a suite
  m2sim run --suite <suite> --cores N   Run suite benchmarks on N cores
  m2sim eval                            Quick 6-benchmark evaluation
`)
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}

	switch os.Args[1] {
	case "list":
		cmdList()
	case "run":
		cmdRun(os.Args[2:])
	case "eval":
		cmdEval()
	default:
		usage()
	}
}

// cmdList prints all available benchmarks.
func cmdList() {
	benchmarks := benchmark.All()
	fmt.Printf("%-16s | %s\n", "Name", "Suite")
	fmt.Printf("%-16s-+-%-16s\n", "----------------", "----------------")
	for _, b := range benchmarks {
		fmt.Printf("%-16s | %s\n", b.Name, b.Suite)
	}
	fmt.Printf("\nTotal: %d benchmarks\n", len(benchmarks))
}

// cmdRun handles the 'run' subcommand.
func cmdRun(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	allFlag := fs.Bool("all", false, "Run all benchmarks (full comparison table)")
	suiteFlag := fs.String("suite", "", "Run all benchmarks in the given suite")
	coresFlag := fs.Int("cores", 1, "Number of cores (default: 1, single-core)")
	fs.Parse(args)

	nCores := *coresFlag
	if nCores < 1 {
		fmt.Fprintf(os.Stderr, "error: --cores must be >= 1\n")
		os.Exit(1)
	}

	if *allFlag {
		if nCores > 1 {
			runMultiCoreAll(benchmark.All(), nCores)
		} else {
			runComparisonTable(benchmark.All())
		}
		return
	}

	if *suiteFlag != "" {
		all := benchmark.All()
		var filtered []benchmark.Benchmark
		for _, b := range all {
			if b.Suite == *suiteFlag {
				filtered = append(filtered, b)
			}
		}
		if len(filtered) == 0 {
			fmt.Fprintf(os.Stderr, "error: no benchmarks found for suite %q\n", *suiteFlag)
			os.Exit(1)
		}
		if nCores > 1 {
			runMultiCoreAll(filtered, nCores)
		} else {
			runComparisonTable(filtered)
		}
		return
	}

	// Single benchmark by name.
	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: run requires a benchmark name, --all, or --suite\n")
		os.Exit(1)
	}

	name := fs.Arg(0)
	all := benchmark.All()
	for _, b := range all {
		if b.Name == name {
			if nCores > 1 {
				runMultiCore(b, nCores)
			} else {
				runSingle(b)
			}
			return
		}
	}

	fmt.Fprintf(os.Stderr, "error: unknown benchmark %q\n", name)
	os.Exit(1)
}

// runSingle runs a single benchmark through the OoO pipeline and prints results.
func runSingle(b benchmark.Benchmark) {
	em := b.Setup()
	cache := mem.NewCache(mem.DefaultCacheConfig())
	benchmark.WarmCache(cache, b.Regions)

	engine := sim.NewSerialEngine()
	p := cpu.NewOoOPipeline("M2sim", engine, 3490*sim.MHz, em)
	p.Cache = cache
	if b.FuncHookCycles != nil {
		p.FuncHookCycles = b.FuncHookCycles
	}
	p.TickLater()
	if err := engine.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: engine.Run() failed: %v\n", err)
		os.Exit(1)
	}
	if p.Err != nil {
		fmt.Fprintf(os.Stderr, "error: pipeline error: %v\n", p.Err)
		os.Exit(1)
	}
	if !em.Halted {
		fmt.Fprintf(os.Stderr, "error: emulator did not halt\n")
		os.Exit(1)
	}

	simTime := float64(p.Cycles) / freq
	ratio := simTime / b.NativeTime
	minVal := math.Min(simTime, b.NativeTime)
	errVal := math.Abs(simTime-b.NativeTime) / minVal

	stats := cache.Stats()
	l1Hit := float64(stats.L1Hits) / float64(stats.L1Hits+stats.L1Misses) * 100
	l2Hit := float64(stats.L2Hits) / float64(stats.L2Hits+stats.L2Misses) * 100

	var bpAcc float64
	if p.BranchPred != nil && p.BranchPred.Predictions > 0 {
		bpAcc = float64(p.BranchPred.Hits) / float64(p.BranchPred.Predictions) * 100
	}

	fmt.Printf("Benchmark: %s\n", b.Name)
	fmt.Printf("  Instructions:  %d\n", p.InstDone)
	fmt.Printf("  Cycles:        %d\n", p.Cycles)
	fmt.Printf("  CPI:           %.2f\n", p.CPI())
	fmt.Printf("  Sim Time:      %e s\n", simTime)
	fmt.Printf("  Native Time:   %e s\n", b.NativeTime)
	fmt.Printf("  Ratio:         %.1f\n", ratio)
	fmt.Printf("  Error:         %.1f%%\n", errVal*100)
	fmt.Printf("  BP Accuracy:   %.1f%%\n", bpAcc)
	fmt.Printf("  L1 Hit Rate:   %.1f%%\n", l1Hit)
	fmt.Printf("  L2 Hit Rate:   %.1f%%\n", l2Hit)
}

// runMultiCore runs a single benchmark on nCores cores and prints results.
func runMultiCore(b benchmark.Benchmark, nCores int) {
	configs := make([]cpu.CoreConfig, nCores)
	for i := 0; i < nCores; i++ {
		configs[i] = cpu.CoreConfig{
			Emulator:       b.Setup(),
			FuncHookCycles: b.FuncHookCycles,
		}
	}

	engine := sim.NewSerialEngine()
	mc := cpu.NewMultiCorePipeline("M2sim", engine, 3490*sim.MHz, configs)

	// Warm caches for each core.
	for _, core := range mc.Cores {
		benchmark.WarmCache(core.Cache, b.Regions)
	}

	if err := mc.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: multi-core Run() failed: %v\n", err)
		os.Exit(1)
	}

	for i, core := range mc.Cores {
		if !core.Emu.Halted {
			fmt.Fprintf(os.Stderr, "error: core %d did not halt\n", i)
			os.Exit(1)
		}
	}

	simTime := float64(mc.TotalCycles) / freq

	fmt.Printf("Benchmark: %s (%d cores)\n", b.Name, nCores)
	fmt.Printf("  Total Instructions: %d\n", mc.TotalInstructions)
	fmt.Printf("  Max Cycles:         %d\n", mc.TotalCycles)
	fmt.Printf("  Sim Time:           %e s\n", simTime)
	for i, core := range mc.Cores {
		fmt.Printf("  Core %d:  %d insts, %d cycles, CPI=%.2f\n",
			i, core.InstDone, core.Cycles, core.CPI())
	}
}

// runMultiCoreAll runs all given benchmarks in multi-core mode and prints a summary table.
func runMultiCoreAll(benchmarks []benchmark.Benchmark, nCores int) {
	fmt.Printf("\n%-16s | %6s | %12s | %12s | %14s\n",
		"Benchmark", "Cores", "Total Insts", "Max Cycles", "Sim Time (s)")
	fmt.Printf("%-16s-+-%6s-+-%12s-+-%12s-+-%14s\n",
		"----------------", "------", "------------", "------------", "--------------")

	for _, b := range benchmarks {
		configs := make([]cpu.CoreConfig, nCores)
		for i := 0; i < nCores; i++ {
			configs[i] = cpu.CoreConfig{
				Emulator:       b.Setup(),
				FuncHookCycles: b.FuncHookCycles,
			}
		}

		engine := sim.NewSerialEngine()
		mc := cpu.NewMultiCorePipeline("M2sim", engine, 3490*sim.MHz, configs)
		for _, core := range mc.Cores {
			benchmark.WarmCache(core.Cache, b.Regions)
		}

		if err := mc.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %s multi-core Run() failed: %v\n", b.Name, err)
			continue
		}

		simTime := float64(mc.TotalCycles) / freq
		fmt.Printf("%-16s | %6d | %12d | %12d | %14e\n",
			b.Name, nCores, mc.TotalInstructions, mc.TotalCycles, simTime)
	}
}

// runComparisonTable runs all benchmarks with 3 pipeline variants and prints the full table.
func runComparisonTable(benchmarks []benchmark.Benchmark) {
	type kernelResult struct {
		name          string
		insts         uint64
		iooCycles     uint64
		iooCPI        float64
		oooCycles     uint64
		oooCPI        float64
		oooNoBPCycles uint64
		oooNoBPCPI    float64
		bpAccuracy    float64
		native        float64
		l1HitRate     float64
		l2HitRate     float64
	}

	results := make([]kernelResult, 0, len(benchmarks))

	for _, b := range benchmarks {
		// Run in-order pipeline.
		emIO := b.Setup()
		cacheIO := mem.NewCache(mem.DefaultCacheConfig())
		benchmark.WarmCache(cacheIO, b.Regions)
		engineIO := sim.NewSerialEngine()
		pIO := cpu.NewPipeline("M2sim", engineIO, 3490*sim.MHz, emIO)
		pIO.Cache = cacheIO
		pIO.TickLater()
		if err := engineIO.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %s in-order engine.Run() failed: %v\n", b.Name, err)
			os.Exit(1)
		}
		if pIO.Err != nil {
			fmt.Fprintf(os.Stderr, "error: %s in-order pipeline error: %v\n", b.Name, pIO.Err)
			os.Exit(1)
		}

		// Run OoO pipeline (with branch predictor).
		emOoO := b.Setup()
		cacheOoO := mem.NewCache(mem.DefaultCacheConfig())
		benchmark.WarmCache(cacheOoO, b.Regions)
		engineOoO := sim.NewSerialEngine()
		pOoO := cpu.NewOoOPipeline("M2sim", engineOoO, 3490*sim.MHz, emOoO)
		pOoO.Cache = cacheOoO
		if b.FuncHookCycles != nil {
			pOoO.FuncHookCycles = b.FuncHookCycles
		}
		pOoO.TickLater()
		if err := engineOoO.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %s OoO engine.Run() failed: %v\n", b.Name, err)
			os.Exit(1)
		}
		if pOoO.Err != nil {
			fmt.Fprintf(os.Stderr, "error: %s OoO pipeline error: %v\n", b.Name, pOoO.Err)
			os.Exit(1)
		}

		// Capture branch prediction stats.
		var bpAcc float64
		if pOoO.BranchPred != nil && pOoO.BranchPred.Predictions > 0 {
			bpAcc = float64(pOoO.BranchPred.Hits) / float64(pOoO.BranchPred.Predictions) * 100
		}

		// Run OoO pipeline without branch predictor.
		emNoBP := b.Setup()
		cacheNoBP := mem.NewCache(mem.DefaultCacheConfig())
		benchmark.WarmCache(cacheNoBP, b.Regions)
		engineNoBP := sim.NewSerialEngine()
		pNoBP := cpu.NewOoOPipeline("M2sim", engineNoBP, 3490*sim.MHz, emNoBP)
		pNoBP.Cache = cacheNoBP
		if b.FuncHookCycles != nil {
			pNoBP.FuncHookCycles = b.FuncHookCycles
		}
		pNoBP.BranchPred = nil
		pNoBP.TickLater()
		if err := engineNoBP.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %s no-BP engine.Run() failed: %v\n", b.Name, err)
			os.Exit(1)
		}
		if pNoBP.Err != nil {
			fmt.Fprintf(os.Stderr, "error: %s no-BP pipeline error: %v\n", b.Name, pNoBP.Err)
			os.Exit(1)
		}

		stats := cacheOoO.Stats()
		l1Hit := float64(stats.L1Hits) / float64(stats.L1Hits+stats.L1Misses)
		l2Hit := float64(stats.L2Hits) / float64(stats.L2Hits+stats.L2Misses)

		results = append(results, kernelResult{
			name:          b.Name,
			insts:         pOoO.InstDone,
			iooCycles:     pIO.Cycles,
			iooCPI:        pIO.CPI(),
			oooCycles:     pOoO.Cycles,
			oooCPI:        pOoO.CPI(),
			oooNoBPCycles: pNoBP.Cycles,
			oooNoBPCPI:    pNoBP.CPI(),
			bpAccuracy:    bpAcc,
			native:        b.NativeTime,
			l1HitRate:     l1Hit,
			l2HitRate:     l2Hit,
		})
	}

	// Print comparison table.
	fmt.Printf("\n%-8s | %12s | %11s | %9s | %10s | %7s | %9s | %7s | %14s | %12s | %8s | %8s | %8s | %8s\n",
		"Kernel", "Instructions", "InOrder CPI", "OoO CPI", "NoBP CPI", "BP Acc%", "BP Improv", "Speedup", "Sim Time (s)", "Native (s)", "Ratio", "L1 Hit%", "L2 Hit%", "Error")
	fmt.Printf("%-8s-+-%12s-+-%11s-+-%9s-+-%10s-+-%7s-+-%9s-+-%7s-+-%14s-+-%12s-+-%8s-+-%8s-+-%8s-+-%8s\n",
		"--------", "------------", "-----------", "---------", "----------", "-------", "---------", "-------", "--------------", "------------", "--------", "--------", "--------", "--------")

	var totalError float64
	var maxError float64
	var maxErrorName string
	countInRange := 0
	countLE20 := 0
	countLE50 := 0

	for _, r := range results {
		simTime := float64(r.oooCycles) / freq
		ratio := simTime / r.native
		speedup := r.iooCPI / r.oooCPI
		bpImprov := (r.oooNoBPCPI - r.oooCPI) / r.oooNoBPCPI * 100
		minVal := math.Min(simTime, r.native)
		errVal := math.Abs(simTime-r.native) / minVal

		totalError += errVal
		if errVal > maxError {
			maxError = errVal
			maxErrorName = r.name
		}
		if ratio >= 0.67 && ratio <= 1.50 {
			countInRange++
		}
		if errVal <= 0.20 {
			countLE20++
		}
		if errVal <= 0.50 {
			countLE50++
		}

		fmt.Printf("%-8s | %12d | %11.2f | %9.2f | %10.2f | %6.1f%% | %8.1f%% | %6.2fx | %14e | %12e | %8.1f | %7.1f%% | %7.1f%% | %7.1f%%\n",
			r.name, r.insts, r.iooCPI, r.oooCPI, r.oooNoBPCPI, r.bpAccuracy, bpImprov, speedup, simTime, r.native, ratio, r.l1HitRate*100, r.l2HitRate*100, errVal*100)
	}

	n := len(results)
	avgError := totalError / float64(n)

	fmt.Printf("\nSummary (%d benchmarks):\n", n)
	fmt.Printf("  Avg Error:          %6.1f%%\n", avgError*100)
	fmt.Printf("  Max Error:          %6.1f%% (%s)\n", maxError*100, maxErrorName)
	fmt.Printf("  In range [0.67-1.50]: %d / %d\n", countInRange, n)
	fmt.Printf("  Within 20%% error:    %d / %d\n", countLE20, n)
	fmt.Printf("  Within 50%% error:    %d / %d\n", countLE50, n)

	passAvg := avgError <= 0.20
	passMax := maxError <= 0.50
	if passAvg && passMax {
		fmt.Printf("  Result: PASS (avgError=%.1f%% <= 20%%, maxError=%.1f%% <= 50%%)\n", avgError*100, maxError*100)
	} else {
		reasons := ""
		if !passAvg {
			reasons += fmt.Sprintf("avgError=%.1f%% > 20%%", avgError*100)
		}
		if !passMax {
			if reasons != "" {
				reasons += ", "
			}
			reasons += fmt.Sprintf("maxError=%.1f%% > 50%%", maxError*100)
		}
		fmt.Printf("  Result: FAIL (%s)\n", reasons)
	}
}

// cmdEval runs the quick 6-benchmark evaluation.
func cmdEval() {
	quickNames := map[string]bool{
		"atax": true, "bicg": true, "gemm": true,
		"floydwarshall": true, "trisolv": true, "seidel2d": true,
	}

	allKernels := benchmark.All()
	var kernels []benchmark.Benchmark
	for _, k := range allKernels {
		if quickNames[k.Name] {
			kernels = append(kernels, k)
		}
	}

	type quickResult struct {
		name    string
		insts   uint64
		cycles  uint64
		cpi     float64
		simTime float64
		native  float64
		ratio   float64
		errVal  float64
	}

	results := make([]quickResult, 0, len(kernels))

	for _, k := range kernels {
		em := k.Setup()
		cache := mem.NewCache(mem.DefaultCacheConfig())
		benchmark.WarmCache(cache, k.Regions)

		engine := sim.NewSerialEngine()
		p := cpu.NewOoOPipeline("M2sim", engine, 3490*sim.MHz, em)
		p.Cache = cache
		if k.FuncHookCycles != nil {
			p.FuncHookCycles = k.FuncHookCycles
		}
		p.TickLater()
		if err := engine.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %s engine.Run() failed: %v\n", k.Name, err)
			os.Exit(1)
		}
		if p.Err != nil {
			fmt.Fprintf(os.Stderr, "error: %s pipeline error: %v\n", k.Name, p.Err)
			os.Exit(1)
		}
		if !em.Halted {
			fmt.Fprintf(os.Stderr, "error: %s emulator did not halt\n", k.Name)
			os.Exit(1)
		}

		simTime := float64(p.Cycles) / freq
		ratio := simTime / k.NativeTime
		minVal := math.Min(simTime, k.NativeTime)
		errVal := math.Abs(simTime-k.NativeTime) / minVal

		results = append(results, quickResult{
			name:    k.Name,
			insts:   p.InstDone,
			cycles:  p.Cycles,
			cpi:     p.CPI(),
			simTime: simTime,
			native:  k.NativeTime,
			ratio:   ratio,
			errVal:  errVal,
		})
	}

	// Print QuickEval table.
	fmt.Printf("\n%-14s | %10s | %10s | %7s | %14s | %12s | %8s | %8s\n",
		"Kernel", "Insts", "Cycles", "CPI", "Sim Time (s)", "Native (s)", "Ratio", "Error")
	fmt.Printf("%-14s-+-%10s-+-%10s-+-%7s-+-%14s-+-%12s-+-%8s-+-%8s\n",
		"--------------", "----------", "----------", "-------", "--------------", "------------", "--------", "--------")

	var totalError float64
	var maxError float64
	var maxErrorName string
	for _, r := range results {
		fmt.Printf("%-14s | %10d | %10d | %7.2f | %14e | %12e | %8.1f | %7.1f%%\n",
			r.name, r.insts, r.cycles, r.cpi, r.simTime, r.native, r.ratio, r.errVal*100)
		totalError += r.errVal
		if r.errVal > maxError {
			maxError = r.errVal
			maxErrorName = r.name
		}
	}

	n := len(results)
	avgError := totalError / float64(n)
	fmt.Printf("\nQuickEval Summary (%d benchmarks):\n", n)
	fmt.Printf("  Avg Error: %6.1f%%\n", avgError*100)
	fmt.Printf("  Max Error: %6.1f%% (%s)\n", maxError*100, maxErrorName)
}
