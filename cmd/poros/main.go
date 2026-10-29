package main

import (
	"errors"
	"flag"
	"fmt"
	"math"
	"os"

	"github.com/sarchlab/akita/v5/sim"
	"github.com/sarchlab/akita/v5/sim/modeling"
	"github.com/sarchlab/akita/v5/sim/timing"
	"github.com/sarchlab/poros/pkg/benchmark"
	"github.com/sarchlab/poros/pkg/cpu"
	"github.com/sarchlab/poros/pkg/cpu/inorder"
	"github.com/sarchlab/poros/pkg/cpu/ooo"
	"github.com/sarchlab/poros/pkg/mem"
)

const freq = 3.49e9 // M2 Avalanche core frequency

func usage() {
	fmt.Fprintf(os.Stderr, `poros: Apple M2 cycle-accurate simulator

Usage:
  poros list                            List all available benchmarks
  poros run <name>                      Run a single benchmark
  poros run --cores N <name>            Run a benchmark on N cores
  poros run --all                       Run all benchmarks (full comparison table)
  poros run --suite <suite>             Run all benchmarks in a suite
  poros run --suite <suite> --cores N   Run suite benchmarks on N cores
  poros eval                            Quick 6-benchmark evaluation
  poros elf [options] file.elf [args]    Run static Linux ELF; emit JSON result
`)
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}

	switch os.Args[1] {
	case "elf":
		os.Exit(runELF(os.Args[2:], os.Stdout, os.Stderr))
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

	simulation := sim.MakeBuilder().Build()
	em.Mem.TrackAccesses = true
	pSpec := ooo.DefaultSpec()
	pSpec.Freq = 3490 * timing.MHz
	pSpec.FuncHookCycles = cpu.HookCycleSpec(b.FuncHookCycles)
	p := modeling.MakeBuilder[*ooo.Comp](pSpec).WithSimulation(simulation).WithResources(ooo.Resources{Emu: em, Cache: cache}).Build("Poros")
	if err := runSimulation(simulation, p); err != nil {
		fmt.Fprintf(os.Stderr, "error: simulation.Run() failed: %v\n", err)
		os.Exit(1)
	}
	if p.State.Err != "" {
		fmt.Fprintf(os.Stderr, "error: pipeline error: %v\n", p.State.Err)
		os.Exit(1)
	}
	if !em.Halted {
		fmt.Fprintf(os.Stderr, "error: emulator did not halt\n")
		os.Exit(1)
	}

	simTime := float64(p.State.Cycles) / freq
	ratio := simTime / b.NativeTime
	minVal := math.Min(simTime, b.NativeTime)
	errVal := math.Abs(simTime-b.NativeTime) / minVal

	stats := cache.Stats()
	l1Hit := float64(stats.L1Hits) / float64(stats.L1Hits+stats.L1Misses) * 100
	l2Hit := float64(stats.L2Hits) / float64(stats.L2Hits+stats.L2Misses) * 100

	var bpAcc float64
	if p.Spec.BranchPrediction && p.State.Predictor.Predictions > 0 {
		bpAcc = float64(p.State.Predictor.Hits) / float64(p.State.Predictor.Predictions) * 100
	}

	fmt.Printf("Benchmark: %s\n", b.Name)
	fmt.Printf("  Instructions:  %d\n", p.State.InstDone)
	fmt.Printf("  Cycles:        %d\n", p.State.Cycles)
	fmt.Printf("  CPI:           %.2f\n", p.State.CPI())
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

	simulation := sim.MakeBuilder().Build()
	mc := cpu.NewMultiCorePipeline("Poros", simulation, 3490*timing.MHz, configs)

	// Warm caches for each core.
	for _, core := range mc.Cores {
		benchmark.WarmCache(core.Resources.Cache, b.Regions)
	}

	if err := runMultiCoreSimulation(mc); err != nil {
		fmt.Fprintf(os.Stderr, "error: multi-core Run() failed: %v\n", err)
		os.Exit(1)
	}

	for i, core := range mc.Cores {
		if !core.Resources.Emu.Halted {
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
			i, core.State.InstDone, core.State.Cycles, core.State.CPI())
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

		simulation := sim.MakeBuilder().Build()
		mc := cpu.NewMultiCorePipeline("Poros", simulation, 3490*timing.MHz, configs)
		for _, core := range mc.Cores {
			benchmark.WarmCache(core.Resources.Cache, b.Regions)
		}

		if err := runMultiCoreSimulation(mc); err != nil {
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
		simulationIO := sim.MakeBuilder().Build()
		emIO.Mem.TrackAccesses = true
		pIOSpec := inorder.DefaultSpec()
		pIOSpec.Freq = 3490 * timing.MHz
		pIO := modeling.MakeBuilder[*inorder.Comp](pIOSpec).WithSimulation(simulationIO).WithResources(inorder.Resources{Emu: emIO, Cache: cacheIO}).Build("Poros")
		if err := runSimulation(simulationIO, pIO); err != nil {
			fmt.Fprintf(os.Stderr, "error: %s in-order simulation.Run() failed: %v\n", b.Name, err)
			os.Exit(1)
		}
		if pIO.State.Err != "" {
			fmt.Fprintf(os.Stderr, "error: %s in-order pipeline error: %v\n", b.Name, pIO.State.Err)
			os.Exit(1)
		}

		// Run OoO pipeline (with branch predictor).
		emOoO := b.Setup()
		cacheOoO := mem.NewCache(mem.DefaultCacheConfig())
		benchmark.WarmCache(cacheOoO, b.Regions)
		simulationOoO := sim.MakeBuilder().Build()
		emOoO.Mem.TrackAccesses = true
		pOoOSpec := ooo.DefaultSpec()
		pOoOSpec.Freq = 3490 * timing.MHz
		pOoOSpec.FuncHookCycles = cpu.HookCycleSpec(b.FuncHookCycles)
		pOoO := modeling.MakeBuilder[*ooo.Comp](pOoOSpec).WithSimulation(simulationOoO).WithResources(ooo.Resources{Emu: emOoO, Cache: cacheOoO}).Build("Poros")
		if err := runSimulation(simulationOoO, pOoO); err != nil {
			fmt.Fprintf(os.Stderr, "error: %s OoO simulation.Run() failed: %v\n", b.Name, err)
			os.Exit(1)
		}
		if pOoO.State.Err != "" {
			fmt.Fprintf(os.Stderr, "error: %s OoO pipeline error: %v\n", b.Name, pOoO.State.Err)
			os.Exit(1)
		}

		// Capture branch prediction stats.
		var bpAcc float64
		if pOoO.Spec.BranchPrediction && pOoO.State.Predictor.Predictions > 0 {
			bpAcc = float64(pOoO.State.Predictor.Hits) / float64(pOoO.State.Predictor.Predictions) * 100
		}

		// Run OoO pipeline without branch predictor.
		emNoBP := b.Setup()
		cacheNoBP := mem.NewCache(mem.DefaultCacheConfig())
		benchmark.WarmCache(cacheNoBP, b.Regions)
		simulationNoBP := sim.MakeBuilder().Build()
		emNoBP.Mem.TrackAccesses = true
		pNoBPSpec := ooo.DefaultSpec()
		pNoBPSpec.Freq = 3490 * timing.MHz
		pNoBPSpec.BranchPrediction = false
		pNoBPSpec.FuncHookCycles = cpu.HookCycleSpec(b.FuncHookCycles)
		pNoBP := modeling.MakeBuilder[*ooo.Comp](pNoBPSpec).WithSimulation(simulationNoBP).WithResources(ooo.Resources{Emu: emNoBP, Cache: cacheNoBP}).Build("Poros")
		if err := runSimulation(simulationNoBP, pNoBP); err != nil {
			fmt.Fprintf(os.Stderr, "error: %s no-BP simulation.Run() failed: %v\n", b.Name, err)
			os.Exit(1)
		}
		if pNoBP.State.Err != "" {
			fmt.Fprintf(os.Stderr, "error: %s no-BP pipeline error: %v\n", b.Name, pNoBP.State.Err)
			os.Exit(1)
		}

		stats := cacheOoO.Stats()
		l1Hit := float64(stats.L1Hits) / float64(stats.L1Hits+stats.L1Misses)
		l2Hit := float64(stats.L2Hits) / float64(stats.L2Hits+stats.L2Misses)

		results = append(results, kernelResult{
			name:          b.Name,
			insts:         pOoO.State.InstDone,
			iooCycles:     pIO.State.Cycles,
			iooCPI:        pIO.State.CPI(),
			oooCycles:     pOoO.State.Cycles,
			oooCPI:        pOoO.State.CPI(),
			oooNoBPCycles: pNoBP.State.Cycles,
			oooNoBPCPI:    pNoBP.State.CPI(),
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

		simulation := sim.MakeBuilder().Build()
		em.Mem.TrackAccesses = true
		pSpec := ooo.DefaultSpec()
		pSpec.Freq = 3490 * timing.MHz
		pSpec.FuncHookCycles = cpu.HookCycleSpec(k.FuncHookCycles)
		p := modeling.MakeBuilder[*ooo.Comp](pSpec).WithSimulation(simulation).WithResources(ooo.Resources{Emu: em, Cache: cache}).Build("Poros")
		if err := runSimulation(simulation, p); err != nil {
			fmt.Fprintf(os.Stderr, "error: %s simulation.Run() failed: %v\n", k.Name, err)
			os.Exit(1)
		}
		if p.State.Err != "" {
			fmt.Fprintf(os.Stderr, "error: %s pipeline error: %v\n", k.Name, p.State.Err)
			os.Exit(1)
		}
		if !em.Halted {
			fmt.Fprintf(os.Stderr, "error: %s emulator did not halt\n", k.Name)
			os.Exit(1)
		}

		simTime := float64(p.State.Cycles) / freq
		ratio := simTime / k.NativeTime
		minVal := math.Min(simTime, k.NativeTime)
		errVal := math.Abs(simTime-k.NativeTime) / minVal

		results = append(results, quickResult{
			name:    k.Name,
			insts:   p.State.InstDone,
			cycles:  p.State.Cycles,
			cpi:     p.State.CPI(),
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

// Each helper bounds the simulation lifetime to one run, including failures.
func runSimulation(s *sim.Simulation, c interface{ TickLater() }) (err error) {
	defer func() { err = errors.Join(err, s.Terminate()) }()
	if err = s.Initialize(); err != nil {
		return err
	}
	c.TickLater()
	return s.Run()
}
func runMultiCoreSimulation(mc *cpu.MultiCorePipeline) (err error) {
	defer func() { err = errors.Join(err, mc.Simulation.Terminate()) }()
	if err = mc.Simulation.Initialize(); err != nil {
		return err
	}
	return mc.Run()
}
