# Project Specification

## What do you want to build?

M2Sim is a cycle-accurate, execution-driven Apple M2 CPU simulator built on the Akita framework.

## How do you consider the project is success?

We need to support simulating the whole polybench benchmark suite AND the PARSEC benchmark suite with 20% average time estimation error. The maximum error of each benchmark execution cannot be greater than 50%. Error is calculated by error = abs(sim-hw)/min(sim,hw).

You run on an Apple M2 machine, so you can run locally to measure.

## Multi-Core Requirements (from human issue #119)

- Support multi-core simulation for Apple M2 chips (M2 has up to 8 P-cores + 4 E-cores)
- Introduce the PARSEC benchmark suite for multi-threaded workload evaluation
- All benchmarks (PolyBench and PARSEC) must meet ≤20% average error, ≤50% max error targets

## Constraints

- Built on Akita v4 framework in Go
- Only ARM64 instruction set (no x86)
- No virtual memory needed for statically linked binaries
- Use MINI/SMALL/MEDIUM dataset sizes to keep simulation time manageable
- Native timing references measured on the same Apple M2 machine

## Notes

- PolyBench single-core simulation: COMPLETE (avg 13.1%, max 36.0%)
- Multi-core simulation for PARSEC: NEW REQUIREMENT
- PARSEC benchmarks use pthreads; emulator needs thread scheduling model
- M2 P-core (Avalanche): 3.49 GHz, 8-wide OoO, 330-entry ROB
- M2 has coherent shared L2 cache (16MB, shared across P-cores)
